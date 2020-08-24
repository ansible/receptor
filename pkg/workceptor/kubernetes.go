package workceptor

import (
	"context"
	"fmt"
	"github.com/google/shlex"
	"github.com/project-receptor/receptor/pkg/cmdline"
	"github.com/project-receptor/receptor/pkg/logger"
	"io"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	watch2 "k8s.io/client-go/tools/watch"
	"strings"
	"sync"
)

// kubeUnit implements the WorkUnit interface
type kubeUnit struct {
	namePrefix string
	kubeConfig string
	image      string
	command    []string
	args       []string
	ctx        context.Context
	cancelFunc context.CancelFunc
	config     *rest.Config
	clientset  *kubernetes.Clientset
	namespace  string
	unitdir    string
	pod        *corev1.Pod
}

// ErrPodCompleted is returned when pod has already completed before we could attach
var ErrPodCompleted = fmt.Errorf("pod ran to completion")

// podRunningAndReady is a completion criterion for pod ready to be attached to
func podRunningAndReady(event watch.Event) (bool, error) {
	switch event.Type {
	case watch.Deleted:
		return false, errors.NewNotFound(schema.GroupResource{Resource: "pods"}, "")
	}
	switch t := event.Object.(type) {
	case *corev1.Pod:
		switch t.Status.Phase {
		case corev1.PodFailed, corev1.PodSucceeded:
			return false, ErrPodCompleted
		case corev1.PodRunning:
			conditions := t.Status.Conditions
			if conditions == nil {
				return false, nil
			}
			for i := range conditions {
				if conditions[i].Type == corev1.PodReady &&
					conditions[i].Status == corev1.ConditionTrue {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

func (kw *kubeUnit) runWork() {

	// Create the pod
	var err error
	kw.pod, err = kw.clientset.CoreV1().Pods(kw.namespace).Create(kw.ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: kw.namePrefix,
			Namespace:    kw.namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:      "worker",
				Image:     kw.image,
				Command:   kw.command,
				Args:      kw.args,
				Stdin:     true,
				StdinOnce: true,
				TTY:       false,
			}},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		errStr := fmt.Sprintf("Error creating pod: %s", err)
		logger.Error(errStr)
		err = saveStatus(kw.unitdir, WorkStateFailed, errStr, 0)
		if err != nil {
			logger.Error("Unable to save status: %s", err)
		}
		return
	}
	select {
	case <-kw.ctx.Done():
		err = saveStatus(kw.unitdir, WorkStateFailed, "Cancelled", 0)
		if err != nil {
			logger.Error("Unable to save status: %s", err)
		}
		return
	default:
	}

	// Wait for the pod to be running
	err = saveStatus(kw.unitdir, WorkStatePending, "Waiting for pod to start", 0)
	if err != nil {
		logger.Error("Unable to save status: %s", err)
	}
	fieldSelector := fields.OneTermEqualSelector("metadata.name", kw.pod.Name).String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return kw.clientset.CoreV1().Pods(kw.namespace).List(kw.ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = fieldSelector
			return kw.clientset.CoreV1().Pods(kw.namespace).Watch(kw.ctx, options)
		},
	}
	ev, err := watch2.UntilWithSync(kw.ctx, lw, &corev1.Pod{}, nil, podRunningAndReady)
	skipStdin := false
	if err == ErrPodCompleted {
		skipStdin = true
	} else if err != nil {
		errStr := fmt.Sprintf("Error waiting for pod to be running: %s", err)
		logger.Error(errStr)
		err = saveStatus(kw.unitdir, WorkStateFailed, errStr, 0)
		if err != nil {
			logger.Error("Unable to save status: %s", err)
		}
		return
	}
	if ev == nil {
		errStr := "Pod disappeared during watch"
		logger.Error(errStr)
		err = saveStatus(kw.unitdir, WorkStateFailed, errStr, 0)
		if err != nil {
			logger.Error("Unable to save status: %s", err)
		}
		return
	}
	kw.pod = ev.Object.(*corev1.Pod)

	// Attach stdin stream to the pod
	var exec remotecommand.Executor
	if !skipStdin {
		req := kw.clientset.CoreV1().RESTClient().Post().
			Resource("pods").
			Name(kw.pod.Name).
			Namespace(kw.pod.Namespace).
			SubResource("attach")
		req.VersionedParams(&corev1.PodExecOptions{
			Container: "worker",
			Stdin:     true,
			Stdout:    false,
			Stderr:    false,
			TTY:       false,
		}, scheme.ParameterCodec)
		exec, err = remotecommand.NewSPDYExecutor(kw.config, "POST", req.URL())
		if err != nil {
			err = saveStatus(kw.unitdir, WorkStateFailed, fmt.Sprintf("Error attaching to pod: %s", err), 0)
			if err != nil {
				logger.Error("Unable to save status: %s", err)
			}
			return
		}
	}

	// Open the pod log for stdout
	logreq := kw.clientset.CoreV1().Pods(kw.namespace).GetLogs(kw.pod.Name, &corev1.PodLogOptions{
		Follow: true,
	})
	logStream, err := logreq.Stream(kw.ctx)
	if err != nil {
		err = saveStatus(kw.unitdir, WorkStateFailed, fmt.Sprintf("Error opening pod stream: %s", err), 0)
		if err != nil {
			logger.Error("Unable to save status: %s", err)
		}
		return
	}
	defer logStream.Close()

	// Check if we were cancelled before starting the streams
	select {
	case <-kw.ctx.Done():
		err = saveStatus(kw.unitdir, WorkStateFailed, "Cancelled", 0)
		if err != nil {
			logger.Error("Unable to save status: %s", err)
		}
		return
	default:
	}

	// Open stdin reader
	var stdin *stdinReader
	if !skipStdin {
		stdin, err = newStdinReader(kw.unitdir)
		if err != nil {
			errStr := fmt.Sprintf("Error opening stdin file: %s", err)
			logger.Error(errStr)
			err = saveStatus(kw.unitdir, WorkStateFailed, errStr, 0)
			if err != nil {
				logger.Error("Unable to save status: %s", err)
			}
			return
		}
	}

	// Open stdout writer
	stdout, err := newStdoutWriter(kw.unitdir)
	if err != nil {
		errStr := fmt.Sprintf("Error opening stdout file: %s", err)
		logger.Error(errStr)
		err = saveStatus(kw.unitdir, WorkStateFailed, errStr, 0)
		if err != nil {
			logger.Error("Unable to save status: %s", err)
		}
		return
	}
	err = saveStatus(kw.unitdir, WorkStatePending, "Sending stdin to pod", 0)
	if err != nil {
		logger.Error("Unable to save status: %s", err)
	}

	// Goroutine to update status when stdin is fully sent to the pod, which is when we
	// update from WorkStatePending to WorkStateRunning.
	finishedChan := make(chan struct{})
	if !skipStdin {
		go func() {
			select {
			case <-finishedChan:
				return
			case <-stdin.Done():
				serr := stdin.Error()
				var err error
				if serr == io.EOF {
					err = saveStatus(kw.unitdir, WorkStateRunning, "Pod Running", stdout.Size())
				} else {
					err = saveStatus(kw.unitdir, WorkStateFailed, fmt.Sprintf("Error reading stdin: %s", serr), stdout.Size())
				}
				if err != nil {
					logger.Error("Unable to save status: %s", err)
				}
			}
		}()
	}

	// Actually run the streams.  This blocks until the pod finishes.
	var errStdin error
	var errStdout error
	streamWait := sync.WaitGroup{}
	streamWait.Add(2)
	if skipStdin {
		streamWait.Done()
	} else {
		go func() {
			errStdin = exec.Stream(remotecommand.StreamOptions{
				Stdin: stdin,
				Tty:   false,
			})
			streamWait.Done()
		}()
	}
	go func() {
		_, errStdout = io.Copy(stdout, logStream)
		streamWait.Done()
	}()
	streamWait.Wait()
	close(finishedChan)
	if errStdin != nil || errStdout != nil {
		var errDetail string
		if errStdin == nil {
			errDetail = fmt.Sprintf("%s", errStdout)
		} else if errStdout == nil {
			errDetail = fmt.Sprintf("%s", errStdin)
		} else {
			errDetail = fmt.Sprintf("stdin: %s, stdout: %s", errStdin, errStdout)
		}
		err = saveStatus(kw.unitdir, WorkStateFailed, fmt.Sprintf("Stream error running pod: %s", errDetail), stdout.Size())
		if err != nil {
			logger.Error("Unable to save status: %s", err)
		}
		return
	}
	err = saveStatus(kw.unitdir, WorkStateSucceeded, "Finished", stdout.Size())
	if err != nil {
		logger.Error("Unable to save status: %s", err)
	}
}

// Start launches a job with given parameters.
func (kw *kubeUnit) Start(params string, unitdir string) error {
	kw.unitdir = unitdir
	err := saveStatus(kw.unitdir, WorkStatePending, "Connecting to Kubernetes", 0)
	if err != nil {
		logger.Error("Unable to save status: %s", err)
	}
	kw.ctx, kw.cancelFunc = context.WithCancel(context.Background())

	// Figure out command and args
	var command []string = nil
	args, err := shlex.Split(params)
	if err != nil {
		return err
	}
	if len(kw.command) > 0 {
		command, err = shlex.Split(kw.command[0])
		if err != nil {
			return err
		}
		command = append(command, args...)
		args = nil
	}
	kw.command = command
	kw.args = args

	// Set up Kubernetes API connection
	clr := clientcmd.NewDefaultClientConfigLoadingRules()
	if kw.kubeConfig != "" {
		clr.ExplicitPath = kw.kubeConfig
	}
	if kw.namespace == "" {
		c, err := clr.Load()
		if err != nil {
			return err
		}
		kw.namespace = c.Contexts[c.CurrentContext].Namespace
	}
	kw.config, err = clientcmd.BuildConfigFromFlags("", clr.GetDefaultFilename())
	if err != nil {
		return err
	}
	kw.clientset, err = kubernetes.NewForConfig(kw.config)
	if err != nil {
		return err
	}

	// Launch runner process
	go kw.runWork()

	return nil
}

// Cancel releases resources associated with a job, including cancelling it if running.
func (kw *kubeUnit) Cancel() error {
	if kw.pod != nil {
		go func(podName string) {
			err := kw.clientset.CoreV1().Pods(kw.namespace).Delete(context.Background(), podName, metav1.DeleteOptions{})
			if err != nil {
				logger.Error("Error deleting pod %s: %s", podName, err)
			}
		}(kw.pod.Name)
	}
	kw.cancelFunc()
	return nil
}

// **************************************************************************
// Command line
// **************************************************************************

// WorkKubeCfg is the cmdline configuration object for a Kubernetes worker plugin
type WorkKubeCfg struct {
	WorkType   string `required:"true" description:"Name for this worker type"`
	KubeConfig string `description:"Kubeconfig file (defaults to environment)"`
	Namespace  string `required:"true" description:"Kubernetes namespace to create pods in"`
	Image      string `required:"true" description:"Container image to use for the worker pod"`
	Command    string `description:"Command to run in the container (defaults to entrypoint)"`
}

// newWorker is a factory to produce worker instances
func (cfg WorkKubeCfg) newWorker() WorkType {
	var command []string
	if cfg.Command != "" {
		command = []string{cfg.Command}
	}
	return &kubeUnit{
		namePrefix: fmt.Sprintf("%s-", strings.ToLower(cfg.WorkType)),
		kubeConfig: cfg.KubeConfig,
		namespace:  cfg.Namespace,
		image:      cfg.Image,
		command:    command,
	}
}

// Run runs the action
func (cfg WorkKubeCfg) Run() error {
	err := MainInstance.RegisterWorker(cfg.WorkType, cfg.newWorker)
	return err
}

func init() {
	cmdline.AddConfigType("work-kubernetes", "Run a worker using Kubernetes", WorkKubeCfg{}, false, false, false, workersSection)
}
