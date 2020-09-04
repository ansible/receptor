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
	BaseWorkUnit
	ctx        context.Context
	cancel     context.CancelFunc
	kubeConfig string
	namespace  string
	namePrefix string
	image      string
	command    []string
	args       []string
	config     *rest.Config
	clientset  *kubernetes.Clientset
	pod        *corev1.Pod
}

// kubeExtraData is the content of the ExtraData JSON field for a Kubernetes worker
type kubeExtraData struct {
	podName string
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
		kw.UpdateBasicStatus(WorkStateFailed, errStr, 0)
		return
	}
	select {
	case <-kw.ctx.Done():
		kw.UpdateBasicStatus(WorkStateFailed, "Cancelled", 0)
		return
	default:
	}
	kw.UpdateFullStatus(func(status *StatusFileData) {
		status.State = WorkStatePending
		status.Detail = "Pod created"
		status.StdoutSize = 0
		status.ExtraData.(*kubeExtraData).podName = kw.pod.Name
	})

	// Wait for the pod to be running
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
		kw.UpdateBasicStatus(WorkStateFailed, errStr, 0)
		return
	}
	if ev == nil {
		errStr := "Pod disappeared during watch"
		logger.Error(errStr)
		kw.UpdateBasicStatus(WorkStateFailed, errStr, 0)
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
			kw.UpdateBasicStatus(WorkStateFailed, fmt.Sprintf("Error attaching to pod: %s", err), 0)
			return
		}
	}

	// Open the pod log for stdout
	logreq := kw.clientset.CoreV1().Pods(kw.namespace).GetLogs(kw.pod.Name, &corev1.PodLogOptions{
		Follow: true,
	})
	logStream, err := logreq.Stream(kw.ctx)
	if err != nil {
		kw.UpdateBasicStatus(WorkStateFailed, fmt.Sprintf("Error opening pod stream: %s", err), 0)
		return
	}
	defer logStream.Close()

	// Check if we were cancelled before starting the streams
	select {
	case <-kw.ctx.Done():
		kw.UpdateBasicStatus(WorkStateFailed, "Cancelled", 0)
		return
	default:
	}

	// Open stdin reader
	var stdin *stdinReader
	if !skipStdin {
		stdin, err = newStdinReader(kw.UnitDir())
		if err != nil {
			errStr := fmt.Sprintf("Error opening stdin file: %s", err)
			logger.Error(errStr)
			kw.UpdateBasicStatus(WorkStateFailed, errStr, 0)
			return
		}
	}

	// Open stdout writer
	stdout, err := newStdoutWriter(kw.UnitDir())
	if err != nil {
		errStr := fmt.Sprintf("Error opening stdout file: %s", err)
		logger.Error(errStr)
		kw.UpdateBasicStatus(WorkStateFailed, errStr, 0)
		return
	}
	kw.UpdateBasicStatus(WorkStatePending, "Sending stdin to pod", 0)

	// Goroutine to update status when stdin is fully sent to the pod, which is when we
	// update from WorkStatePending to WorkStateRunning.
	finishedChan := make(chan struct{})
	if !skipStdin {
		go func() {
			select {
			case <-finishedChan:
				return
			case <-stdin.Done():
				err := stdin.Error()
				if err == io.EOF {
					kw.UpdateBasicStatus(WorkStateRunning, "Pod Running", stdout.Size())
				} else {
					kw.UpdateBasicStatus(WorkStateFailed, fmt.Sprintf("Error reading stdin: %s", err), stdout.Size())
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
		kw.UpdateBasicStatus(WorkStateFailed, fmt.Sprintf("Stream error running pod: %s", errDetail), stdout.Size())
		return
	}
	kw.UpdateBasicStatus(WorkStateSucceeded, "Finished", stdout.Size())
}

func (kw *kubeUnit) connectToKube() error {
	var err error
	kw.config = nil
	if kw.kubeConfig == "" {
		// Use in-cluster config/auth, if possible
		config, err := rest.InClusterConfig()
		if err == nil {
			kw.config = config
		}
	}
	if kw.config == nil {
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
	}
	kw.clientset, err = kubernetes.NewForConfig(kw.config)
	if err != nil {
		return err
	}
	return nil
}

// Init initializes the work unit
func (kw *kubeUnit) Init(w *Workceptor, ident string, workType string, params string) {
	kw.BaseWorkUnit.Init(w, ident, workType, params)
	kw.status.ExtraData = &kubeExtraData{}
}

// Status returns a copy of the status currently loaded in memory
func (kw *kubeUnit) Status() *StatusFileData {
	kw.statusLock.RLock()
	defer kw.statusLock.RUnlock()
	status := kw.getStatus()
	ed, ok := kw.status.ExtraData.(*kubeExtraData)
	if ok {
		edCopy := *ed
		status.ExtraData = &edCopy
	}
	return status
}

// Start launches a job with given parameters.
func (kw *kubeUnit) Start() error {
	kw.UpdateBasicStatus(WorkStatePending, "Connecting to Kubernetes", 0)
	kw.ctx, kw.cancel = context.WithCancel(kw.w.ctx)

	// Figure out command and args
	var command []string = nil
	args, err := shlex.Split(kw.Status().Params)
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

	// Connect to the Kubernetes API
	err = kw.connectToKube()
	if err != nil {
		return err
	}

	// Launch runner process
	go kw.runWork()

	return nil
}

// Restart resumes monitoring a job after a Receptor restart
func (kw *kubeUnit) Restart() error {
	return fmt.Errorf("restart of Kubernetes pod not implemented")
}

// Cancel releases resources associated with a job, including cancelling it if running.
func (kw *kubeUnit) Cancel() error {
	if kw.pod != nil {
		go func(pod string) {
			err := kw.clientset.CoreV1().Pods(kw.namespace).Delete(context.Background(), pod, metav1.DeleteOptions{})
			if err != nil {
				logger.Error("Error deleting pod %s: %s", pod, err)
			}
		}(kw.pod.Name)
	}
	if kw.cancel != nil {
		kw.cancel()
	}
	return nil
}

// Release releases resources associated with a job.  Implies Cancel.
func (kw *kubeUnit) Release(force bool) error {
	err := kw.Cancel()
	if err != nil && !force {
		return err
	}
	return kw.BaseWorkUnit.Release(force)
}

// **************************************************************************
// Command line
// **************************************************************************

// WorkKubeCfg is the cmdline configuration object for a Kubernetes worker plugin
type WorkKubeCfg struct {
	WorkType   string `required:"true" description:"Name for this worker type"`
	KubeConfig string `description:"Kubeconfig file (default: in-cluster or environment)"`
	Namespace  string `required:"true" description:"Kubernetes namespace to create pods in"`
	Image      string `required:"true" description:"Container image to use for the worker pod"`
	Command    string `description:"Command to run in the container (default: entrypoint)"`
}

// newWorker is a factory to produce worker instances
func (cfg WorkKubeCfg) newWorker() WorkUnit {
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
	cmdline.AddConfigType("work-kubernetes", "Run a worker using Kubernetes", WorkKubeCfg{}, false, false, false, false, workersSection)
}
