// +build !no_workceptor

package workceptor

import (
	"context"
	"encoding/pem"
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
	"os"
	"strconv"
	"strings"
	"sync"
)

// kubeUnit implements the WorkUnit interface
type kubeUnit struct {
	BaseWorkUnit
	ctx                 context.Context
	cancel              context.CancelFunc
	authMethod          string
	allowRuntimeAuth    bool
	allowRuntimeTLS     bool
	allowRuntimeCommand bool
	allowRuntimeParams  bool
	kubeConfig          string
	namePrefix          string
	config              *rest.Config
	clientset           *kubernetes.Clientset
	pod                 *corev1.Pod
}

// kubeExtraData is the content of the ExtraData JSON field for a Kubernetes worker
type kubeExtraData struct {
	Image           string
	Command         string
	Params          string
	KubeHost        string
	KubeAPIPath     string
	KubeNamespace   string
	KubeUsername    string
	KubePassword    string
	KubeBearerToken string
	KubeVerifyTLS   bool
	KubeTLSCAData   string
	PodName         string
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
	ked := kw.Status().ExtraData.(*kubeExtraData)
	command, err := shlex.Split(ked.Command)
	var args []string
	if err == nil {
		args, err = shlex.Split(ked.Params)
	}
	if err != nil {
		errStr := fmt.Sprintf("Error tokenizing command: %s", err)
		logger.Error(errStr)
		kw.UpdateBasicStatus(WorkStateFailed, errStr, 0)
		return
	}
	kw.pod, err = kw.clientset.CoreV1().Pods(ked.KubeNamespace).Create(kw.ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: kw.namePrefix,
			Namespace:    ked.KubeNamespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:      "worker",
				Image:     ked.Image,
				Command:   command,
				Args:      args,
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
		status.ExtraData.(*kubeExtraData).PodName = kw.pod.Name
	})

	// Wait for the pod to be running
	fieldSelector := fields.OneTermEqualSelector("metadata.name", kw.pod.Name).String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return kw.clientset.CoreV1().Pods(ked.KubeNamespace).List(kw.ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = fieldSelector
			return kw.clientset.CoreV1().Pods(ked.KubeNamespace).Watch(kw.ctx, options)
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

	// Open the pod log for stdout
	logreq := kw.clientset.CoreV1().Pods(ked.KubeNamespace).GetLogs(kw.pod.Name, &corev1.PodLogOptions{
		Follow: true,
	})
	logStream, err := logreq.Stream(kw.ctx)
	if err != nil {
		kw.UpdateBasicStatus(WorkStateFailed, fmt.Sprintf("Error opening pod stream: %s", err), 0)
		return
	}
	defer logStream.Close()

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

func (kw *kubeUnit) connectUsingKubeconfig() error {
	var err error
	clr := clientcmd.NewDefaultClientConfigLoadingRules()
	if kw.kubeConfig != "" {
		clr.ExplicitPath = kw.kubeConfig
	}
	ked := kw.Status().ExtraData.(*kubeExtraData)
	if ked.KubeNamespace == "" {
		c, err := clr.Load()
		if err != nil {
			return err
		}
		kw.UpdateFullStatus(func(sfd *StatusFileData) {
			sfd.ExtraData.(*kubeExtraData).KubeNamespace = c.Contexts[c.CurrentContext].Namespace
		})
	}
	kw.config, err = clientcmd.BuildConfigFromFlags("", clr.GetDefaultFilename())
	if err != nil {
		return err
	}
	return nil
}

func (kw *kubeUnit) connectUsingIncluster() error {
	var err error
	kw.config, err = rest.InClusterConfig()
	if err != nil {
		return err
	}
	return nil
}

func (kw *kubeUnit) connectUsingParams() error {
	ked := kw.UnredactedStatus().ExtraData.(*kubeExtraData)
	kw.config = &rest.Config{
		Host:        ked.KubeHost,
		APIPath:     ked.KubeAPIPath,
		Username:    ked.KubeUsername,
		Password:    ked.KubePassword,
		BearerToken: ked.KubeBearerToken,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: !ked.KubeVerifyTLS,
		},
	}
	if ked.KubeTLSCAData != "" {
		kw.config.TLSClientConfig.CAData = []byte(ked.KubeTLSCAData)
	}
	return nil
}

func (kw *kubeUnit) connectToKube() error {
	var err error
	if kw.authMethod == "kubeconfig" {
		err = kw.connectUsingKubeconfig()
	} else if kw.authMethod == "incluster" {
		err = kw.connectUsingIncluster()
	} else if kw.authMethod == "params" {
		err = kw.connectUsingParams()
	} else {
		return fmt.Errorf("unknown auth method %s", kw.authMethod)
	}
	if err != nil {
		return err
	}
	kw.clientset, err = kubernetes.NewForConfig(kw.config)
	if err != nil {
		return err
	}
	return nil
}

// SetFromParams sets the in-memory state from parameters
func (kw *kubeUnit) SetFromParams(params map[string]string) error {
	ked := kw.status.ExtraData.(*kubeExtraData)
	type value struct {
		name       string
		permission bool
		setter     func(string) error
	}
	setString := func(target *string) func(string) error {
		ssf := func(value string) error {
			*target = value
			return nil
		}
		return ssf
	}
	setBool := func(target *bool) func(string) error {
		ssf := func(value string) error {
			bv, err := strconv.ParseBool(value)
			if err != nil {
				return err
			}
			*target = bv
			return nil
		}
		return ssf
	}
	values := []value{
		{name: "kube_command", permission: kw.allowRuntimeCommand, setter: setString(&ked.Command)},
		{name: "kube_image", permission: kw.allowRuntimeCommand, setter: setString(&ked.Image)},
		{name: "kube_params", permission: kw.allowRuntimeParams, setter: setString(&ked.Params)},
		{name: "kube_namespace", permission: kw.allowRuntimeAuth, setter: setString(&ked.KubeNamespace)},
		{name: "kube_host", permission: kw.allowRuntimeAuth, setter: setString(&ked.KubeHost)},
		{name: "kube_api_path", permission: kw.allowRuntimeAuth, setter: setString(&ked.KubeAPIPath)},
		{name: "kube_username", permission: kw.allowRuntimeAuth, setter: setString(&ked.KubeUsername)},
		{name: "secret_kube_password", permission: kw.allowRuntimeAuth, setter: setString(&ked.KubePassword)},
		{name: "secret_kube_bearer_token", permission: kw.allowRuntimeAuth, setter: setString(&ked.KubeBearerToken)},
		{name: "kube_verify_tls", permission: kw.allowRuntimeTLS, setter: setBool(&ked.KubeVerifyTLS)},
		{name: "kube_tls_ca", permission: kw.allowRuntimeTLS, setter: setString(&ked.KubeTLSCAData)},
	}
	for i := range values {
		v := values[i]
		value, ok := params[v.name]
		if ok && value != "" {
			if !v.permission {
				return fmt.Errorf("%s provided but not allowed", v.name)
			}
			err := v.setter(value)
			if err != nil {
				return fmt.Errorf("error setting value for %s: %s", v.name, err)
			}
		}
	}
	return nil
}

// Status returns a copy of the status currently loaded in memory
func (kw *kubeUnit) Status() *StatusFileData {
	status := kw.UnredactedStatus()
	ed, ok := status.ExtraData.(*kubeExtraData)
	if ok {
		ed.KubePassword = ""
		ed.KubeBearerToken = ""
	}
	return status
}

// Status returns a copy of the status currently loaded in memory
func (kw *kubeUnit) UnredactedStatus() *StatusFileData {
	kw.statusLock.RLock()
	defer kw.statusLock.RUnlock()
	status := kw.getStatus()
	ked, ok := kw.status.ExtraData.(*kubeExtraData)
	if ok {
		kedCopy := *ked
		status.ExtraData = &kedCopy
	}
	return status
}

// Start launches a job with given parameters.
func (kw *kubeUnit) Start() error {
	kw.UpdateBasicStatus(WorkStatePending, "Connecting to Kubernetes", 0)
	kw.ctx, kw.cancel = context.WithCancel(kw.w.ctx)

	// Connect to the Kubernetes API
	err := kw.connectToKube()
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
			err := kw.clientset.CoreV1().Pods(kw.status.ExtraData.(*kubeExtraData).KubeNamespace).Delete(context.Background(), pod, metav1.DeleteOptions{})
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
	WorkType            string `required:"true" description:"Name for this worker type"`
	Namespace           string `description:"Kubernetes namespace to create pods in"`
	Image               string `description:"Container image to use for the worker pod"`
	Command             string `description:"Command to run in the container (default: entrypoint)"`
	AuthMethod          string `description:"One of: kubeconfig, incluster, params" default:"incluster" required:"true"`
	KubeConfig          string `description:"Kubeconfig filename (for authmethod=kubeconfig)"`
	KubeHost            string `description:"k8s API hostname (for authmethod=params)"`
	KubeAPIPath         string `description:"k8s API path (for authmethod=params)"`
	KubeUsername        string `description:"k8s API username (for authmethod=params)"`
	KubePassword        string `description:"k8s API password (for authmethod=params)"`
	KubeBearerToken     string `description:"k8s API bearer token (for authmethod=params)"`
	KubeVerifyTLS       bool   `description:"verify server TLS certificate/hostname" default:"true"`
	KubeTLSCAData       string `description:"CA certificate PEM data to verify against"`
	AllowRuntimeAuth    bool   `description:"Allow passing API parameters at runtime" default:"false"`
	AllowRuntimeTLS     bool   `description:"Allow passing TLS parameters at runtime" default:"false"`
	AllowRuntimeCommand bool   `description:"Allow specifying image & command at runtime" default:"false"`
	AllowRuntimeParams  bool   `description:"Allow adding command parameters at runtime" default:"false"`
}

// newWorker is a factory to produce worker instances
func (cfg WorkKubeCfg) newWorker(w *Workceptor, unitID string, workType string) WorkUnit {
	ku := &kubeUnit{
		BaseWorkUnit: BaseWorkUnit{
			status: StatusFileData{
				ExtraData: &kubeExtraData{
					Image:           cfg.Image,
					Command:         cfg.Command,
					KubeHost:        cfg.KubeHost,
					KubeAPIPath:     cfg.KubeAPIPath,
					KubeNamespace:   cfg.Namespace,
					KubeUsername:    cfg.KubeUsername,
					KubePassword:    cfg.KubePassword,
					KubeBearerToken: cfg.KubeBearerToken,
					KubeVerifyTLS:   cfg.KubeVerifyTLS,
					KubeTLSCAData:   cfg.KubeTLSCAData,
				},
			},
		},
		namePrefix:          fmt.Sprintf("%s-", strings.ToLower(cfg.WorkType)),
		authMethod:          strings.ToLower(cfg.AuthMethod),
		kubeConfig:          cfg.KubeConfig,
		allowRuntimeAuth:    cfg.AllowRuntimeAuth,
		allowRuntimeTLS:     cfg.AllowRuntimeTLS,
		allowRuntimeCommand: cfg.AllowRuntimeCommand,
		allowRuntimeParams:  cfg.AllowRuntimeParams,
	}
	ku.BaseWorkUnit.Init(w, unitID, workType)
	return ku
}

// Prepare inspects the configuration for validity
func (cfg WorkKubeCfg) Prepare() error {
	lcAuth := strings.ToLower(cfg.AuthMethod)
	if lcAuth != "kubeconfig" && lcAuth != "incluster" && lcAuth != "params" {
		return fmt.Errorf("invalid AuthMethod: %s", cfg.AuthMethod)
	}
	if cfg.Namespace == "" && !(lcAuth == "kubeconfig" || cfg.AllowRuntimeAuth) {
		return fmt.Errorf("must provide namespace when AuthMethod is not kubeconfig")
	}
	if cfg.KubeConfig != "" {
		if lcAuth != "kubeconfig" {
			return fmt.Errorf("can only provide KubeConfig when AuthMethod=kubeconfig")
		}
		_, err := os.Stat(cfg.KubeConfig)
		if err != nil {
			return fmt.Errorf("error accessing kubeconfig file: %s", err)
		}
	}
	if lcAuth == "params" && !cfg.AllowRuntimeAuth {
		if cfg.KubeHost == "" {
			return fmt.Errorf("when AuthMethod=params, must provide KubeHost")
		}
		if (cfg.KubeUsername == "" || cfg.KubePassword == "") && cfg.KubeBearerToken == "" {
			return fmt.Errorf("when AuthMethod=params, must provide either KubeBearerToken or KubeUsername and KubePassword")
		}
	}
	if cfg.KubeTLSCAData != "" {
		block, _ := pem.Decode([]byte(cfg.KubeTLSCAData))
		if block == nil || block.Type != "BEGIN CERTIFICATE" {
			return fmt.Errorf("could not decode KubeTLSCAData as a PEM formatted certificate")
		}
	}
	if cfg.Image == "" && !cfg.AllowRuntimeCommand {
		return fmt.Errorf("must specify a container image to run")
	}
	return nil
}

// Run runs the action
func (cfg WorkKubeCfg) Run() error {
	err := MainInstance.RegisterWorker(cfg.WorkType, cfg.newWorker)
	return err
}

func init() {
	cmdline.AddConfigType("work-kubernetes", "Run a worker using Kubernetes", WorkKubeCfg{}, cmdline.Section(workersSection))
}
