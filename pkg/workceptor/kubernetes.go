// +build !no_workceptor

package workceptor

import (
	"context"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ghjm/cmdline"
	"github.com/google/shlex"
	"github.com/project-receptor/receptor/pkg/logger"
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
)

// kubeUnit implements the WorkUnit interface.
type kubeUnit struct {
	BaseWorkUnit
	authMethod          string
	streamMethod        string
	baseParams          string
	allowRuntimeAuth    bool
	allowRuntimeTLS     bool
	allowRuntimeCommand bool
	allowRuntimeParams  bool
	allowRuntimePod     bool
	deletePodOnRestart  bool
	namePrefix          string
	config              *rest.Config
	clientset           *kubernetes.Clientset
	pod                 *corev1.Pod
	podPendingTimeout   time.Duration
}

// kubeExtraData is the content of the ExtraData JSON field for a Kubernetes worker.
type kubeExtraData struct {
	Image         string
	Command       string
	Params        string
	KubeNamespace string
	KubeVerifyTLS bool
	KubeTLSCAData string
	KubeConfig    string
	KubePod       string
	PodName       string
}

// ErrPodCompleted is returned when pod has already completed before we could attach.
var ErrPodCompleted = fmt.Errorf("pod ran to completion")

// podRunningAndReady is a completion criterion for pod ready to be attached to.
func podRunningAndReady(event watch.Event) (bool, error) {
	if event.Type == watch.Deleted {
		return false, errors.NewNotFound(schema.GroupResource{Resource: "pods"}, "")
	}

	if t, ok := event.Object.(*corev1.Pod); ok {
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

func (kw *kubeUnit) createPod(env map[string]string) error {
	ked := kw.UnredactedStatus().ExtraData.(*kubeExtraData)
	command, err := shlex.Split(ked.Command)
	if err != nil {
		return err
	}
	params, err := shlex.Split(ked.Params)
	if err != nil {
		return err
	}

	pod := &corev1.Pod{}
	var spec *corev1.PodSpec
	var objectMeta *metav1.ObjectMeta
	if ked.KubePod != "" {
		decode := scheme.Codecs.UniversalDeserializer().Decode
		_, _, err := decode([]byte(ked.KubePod), nil, pod)
		if err != nil {
			return err
		}
		foundWorker := false
		spec = &pod.Spec
		for i := range spec.Containers {
			if spec.Containers[i].Name == "worker" {
				spec.Containers[i].Stdin = true
				spec.Containers[i].StdinOnce = true
				foundWorker = true

				break
			}
		}
		if !foundWorker {
			return fmt.Errorf("at least one container must be named worker")
		}
		spec.RestartPolicy = corev1.RestartPolicyNever
		userNamespace := pod.ObjectMeta.Namespace
		if userNamespace != "" {
			ked.KubeNamespace = userNamespace
		}
		userPodName := pod.ObjectMeta.Name
		if userPodName != "" {
			kw.namePrefix = userPodName + "-"
		}
		objectMeta = &pod.ObjectMeta
		objectMeta.Name = ""
		objectMeta.GenerateName = kw.namePrefix
		objectMeta.Namespace = ked.KubeNamespace
	} else {
		objectMeta = &metav1.ObjectMeta{
			GenerateName: kw.namePrefix,
			Namespace:    ked.KubeNamespace,
		}
		spec = &corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:      "worker",
				Image:     ked.Image,
				Command:   command,
				Args:      params,
				Stdin:     true,
				StdinOnce: true,
				TTY:       false,
			}},
			RestartPolicy: corev1.RestartPolicyNever,
		}
	}

	pod = &corev1.Pod{
		ObjectMeta: *objectMeta,
		Spec:       *spec,
	}

	if env != nil {
		evs := make([]corev1.EnvVar, 0)
		for k, v := range env {
			evs = append(evs, corev1.EnvVar{
				Name:  k,
				Value: v,
			})
		}
		pod.Spec.Containers[0].Env = evs
	}
	kw.pod, err = kw.clientset.CoreV1().Pods(ked.KubeNamespace).Create(kw.ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	select {
	case <-kw.ctx.Done():
		return fmt.Errorf("cancelled")
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

	ctxPodReady := kw.ctx
	if kw.podPendingTimeout != time.Duration(0) {
		ctxPodReady, _ = context.WithTimeout(kw.ctx, kw.podPendingTimeout)
	}
	ev, err := watch2.UntilWithSync(ctxPodReady, lw, &corev1.Pod{}, nil, podRunningAndReady)
	var ok bool
	kw.pod, ok = ev.Object.(*corev1.Pod)
	if !ok {
		return fmt.Errorf("watch did not return a pod")
	}
	if err == ErrPodCompleted {
		if len(kw.pod.Status.ContainerStatuses) != 1 {
			return fmt.Errorf("expected 1 container in pod but there were %d", len(kw.pod.Status.ContainerStatuses))
		}
		cstat := kw.pod.Status.ContainerStatuses[0]
		if cstat.State.Terminated != nil && cstat.State.Terminated.ExitCode != 0 {
			return fmt.Errorf("container failed with exit code %d: %s", cstat.State.Terminated.ExitCode, cstat.State.Terminated.Message)
		}

		return err
	} else if err != nil {
		kw.Cancel()
		if len(kw.pod.Status.ContainerStatuses) == 1 {
			if kw.pod.Status.ContainerStatuses[0].State.Waiting != nil {
				return fmt.Errorf("%s, %s", err.Error(), kw.pod.Status.ContainerStatuses[0].State.Waiting.Reason)
			}
		}

		return err
	}
	if ev == nil {
		return fmt.Errorf("pod disappeared during watch")
	}

	return nil
}

func (kw *kubeUnit) runWorkUsingLogger() {
	skipStdin := false
	status := kw.Status()
	ked := status.ExtraData.(*kubeExtraData)
	var err error
	var errMsg string
	if ked.PodName == "" {
		// Create the pod
		err := kw.createPod(nil)
		if err == ErrPodCompleted {
			skipStdin = true
		} else if err != nil {
			errMsg = fmt.Sprintf("Error creating pod: %s", err)
		}
	} else {
		skipStdin = true
		kw.pod, err = kw.clientset.CoreV1().Pods(ked.KubeNamespace).Get(kw.ctx, ked.PodName, metav1.GetOptions{})
		if err != nil {
			errMsg = fmt.Sprintf("Error getting pod: %s", err)
		}
	}
	if errMsg != "" {
		kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)
		logger.Error(errMsg)

		return
	}

	// Open the pod log for stdout
	logreq := kw.clientset.CoreV1().Pods(kw.pod.ObjectMeta.Namespace).GetLogs(kw.pod.Name, &corev1.PodLogOptions{
		Container: "worker",
		Follow:    true,
	})
	logStream, err := logreq.Stream(kw.ctx)
	if err != nil {
		errMsg := fmt.Sprintf("Error opening pod stream: %s", err)
		kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)
		logger.Error(errMsg)

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
			errMsg := fmt.Sprintf("Error opening stdin file: %s", err)
			logger.Error(errMsg)
			kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)

			return
		}
	}

	// Open stdout writer
	stdout, err := newStdoutWriter(kw.UnitDir())
	if err != nil {
		errMsg := fmt.Sprintf("Error opening stdout file: %s", err)
		logger.Error(errMsg)
		kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)

		return
	}

	// Goroutine to update status when stdin is fully sent to the pod, which is when we
	// update from WorkStatePending to WorkStateRunning.
	finishedChan := make(chan struct{})
	if !skipStdin {
		kw.UpdateFullStatus(func(status *StatusFileData) {
			status.State = WorkStatePending
			status.Detail = "Sending stdin to pod"
		})
		go func() {
			select {
			case <-kw.ctx.Done():
				return
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
			if errStdin != nil {
				logStream.Close()
			}
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
		switch {
		case errStdin == nil:
			errDetail = fmt.Sprintf("%s", errStdout)
		case errStdout == nil:
			errDetail = fmt.Sprintf("%s", errStdin)
		default:
			errDetail = fmt.Sprintf("stdin: %s, stdout: %s", errStdin, errStdout)
		}
		kw.UpdateBasicStatus(WorkStateFailed, fmt.Sprintf("Stream error running pod: %s", errDetail), stdout.Size())

		return
	}
	kw.UpdateBasicStatus(WorkStateSucceeded, "Finished", stdout.Size())
}

func getDefaultInterface() (string, error) {
	nifs, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for i := range nifs {
		nif := nifs[i]
		if nif.Flags&net.FlagUp != 0 && nif.Flags&net.FlagLoopback == 0 {
			ads, err := nif.Addrs()
			if err == nil && len(ads) > 0 {
				for j := range ads {
					ad := ads[j]
					ip, ok := ad.(*net.IPNet)
					if ok {
						if !ip.IP.IsLoopback() && !ip.IP.IsMulticast() {
							return ip.IP.String(), nil
						}
					}
				}
			}
		}
	}

	return "", fmt.Errorf("could not determine local address")
}

func (kw *kubeUnit) runWorkUsingTCP() {
	// Create local cancellable context
	ctx, cancel := kw.ctx, kw.cancel
	defer cancel()

	// Create the TCP listener
	lc := net.ListenConfig{}
	defaultInterfaceIP, err := getDefaultInterface()
	var li net.Listener
	if err == nil {
		li, err = lc.Listen(ctx, "tcp", fmt.Sprintf("%s:", defaultInterfaceIP))
	}
	if ctx.Err() != nil {
		return
	}
	var listenHost, listenPort string
	if err == nil {
		listenHost, listenPort, err = net.SplitHostPort(li.Addr().String())
	}
	if err != nil {
		errMsg := fmt.Sprintf("Error listening: %s", err)
		kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)
		logger.Error(errMsg)

		return
	}

	// Wait for a single incoming connection
	connChan := make(chan *net.TCPConn)
	go func() {
		conn, err := li.Accept()
		_ = li.Close()
		if ctx.Err() != nil {
			return
		}
		var tcpConn *net.TCPConn
		if err == nil {
			var ok bool
			tcpConn, ok = conn.(*net.TCPConn)
			if !ok {
				err = fmt.Errorf("connection was not a TCPConn")
			}
		}
		if err != nil {
			errMsg := fmt.Sprintf("Error accepting: %s", err)
			kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)
			logger.Error(errMsg)
			cancel()

			return
		}
		connChan <- tcpConn
	}()

	// Create the pod
	err = kw.createPod(map[string]string{"RECEPTOR_HOST": listenHost, "RECEPTOR_PORT": listenPort})
	if err != nil {
		errMsg := fmt.Sprintf("Error creating pod: %s", err)
		kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)
		logger.Error(errMsg)
		cancel()

		return
	}

	// Wait for the pod to connect back to us
	var conn *net.TCPConn
	select {
	case <-ctx.Done():
		return
	case conn = <-connChan:
	}

	// Open stdin reader
	var stdin *stdinReader
	stdin, err = newStdinReader(kw.UnitDir())
	if err != nil {
		errMsg := fmt.Sprintf("Error opening stdin file: %s", err)
		logger.Error(errMsg)
		kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)
		cancel()

		return
	}

	// Open stdout writer
	stdout, err := newStdoutWriter(kw.UnitDir())
	if err != nil {
		errMsg := fmt.Sprintf("Error opening stdout file: %s", err)
		logger.Error(errMsg)
		kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)
		cancel()

		return
	}

	kw.UpdateBasicStatus(WorkStatePending, "Sending stdin to pod", 0)

	// Write stdin to pod
	go func() {
		_, err := io.Copy(conn, stdin)
		if ctx.Err() != nil {
			return
		}
		_ = conn.CloseWrite()
		if err != nil {
			errMsg := fmt.Sprintf("Error sending stdin to pod: %s", err)
			logger.Error(errMsg)
			kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)
			cancel()

			return
		}
	}()

	// Goroutine to update status when stdin is fully sent to the pod, which is when we
	// update from WorkStatePending to WorkStateRunning.
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-stdin.Done():
			err := stdin.Error()
			if err == io.EOF {
				kw.UpdateBasicStatus(WorkStateRunning, "Pod Running", stdout.Size())
			} else {
				kw.UpdateBasicStatus(WorkStateFailed, fmt.Sprintf("Error reading stdin: %s", err), stdout.Size())
				cancel()
			}
		}
	}()

	// Read stdout from pod
	_, err = io.Copy(stdout, conn)
	if ctx.Err() != nil {
		return
	}
	if err != nil {
		errMsg := fmt.Sprintf("Error reading stdout from pod: %s", err)
		logger.Error(errMsg)
		kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)
		cancel()

		return
	}

	if ctx.Err() == nil {
		kw.UpdateBasicStatus(WorkStateSucceeded, "Finished", stdout.Size())
	}
}

func (kw *kubeUnit) connectUsingKubeconfig() error {
	var err error
	ked := kw.UnredactedStatus().ExtraData.(*kubeExtraData)
	if ked.KubeConfig == "" {
		clr := clientcmd.NewDefaultClientConfigLoadingRules()
		kw.config, err = clientcmd.BuildConfigFromFlags("", clr.GetDefaultFilename())
		if ked.KubeNamespace == "" {
			c, err := clr.Load()
			if err != nil {
				return err
			}
			curContext, ok := c.Contexts[c.CurrentContext]
			if ok && curContext != nil {
				kw.UpdateFullStatus(func(sfd *StatusFileData) {
					sfd.ExtraData.(*kubeExtraData).KubeNamespace = curContext.Namespace
				})
			} else {
				return fmt.Errorf("could not determine namespace")
			}
		}
	} else {
		cfg, err := clientcmd.NewClientConfigFromBytes([]byte(ked.KubeConfig))
		if err != nil {
			return err
		}
		if ked.KubeNamespace == "" {
			namespace, _, err := cfg.Namespace()
			if err != nil {
				return err
			}
			kw.UpdateFullStatus(func(sfd *StatusFileData) {
				sfd.ExtraData.(*kubeExtraData).KubeNamespace = namespace
			})
		}
		kw.config, err = cfg.ClientConfig()
		if err != nil {
			return err
		}
	}
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

func (kw *kubeUnit) connectToKube() error {
	var err error
	switch {
	case kw.authMethod == "kubeconfig" || kw.authMethod == "runtime":
		err = kw.connectUsingKubeconfig()
	case kw.authMethod == "incluster":
		err = kw.connectUsingIncluster()
	default:
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

func readFileToString(filename string) (string, error) {
	// If filename is "", the function returns ""
	if filename == "" {
		return "", nil
	}
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// SetFromParams sets the in-memory state from parameters.
//nolint:ifshort // Method to magical for linter
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

	var err error
	ked.KubePod, err = readFileToString(ked.KubePod)
	if err != nil {
		return fmt.Errorf("could not read pod: %s", err)
	}
	ked.KubeConfig, err = readFileToString(ked.KubeConfig)
	if err != nil {
		return fmt.Errorf("could not read kubeconfig: %s", err)
	}
	userParams := ""
	userCommand := ""
	userImage := ""
	userPod := ""
	podPendingTimeoutString := ""
	values := []value{
		{name: "kube_command", permission: kw.allowRuntimeCommand, setter: setString(&userCommand)},
		{name: "kube_image", permission: kw.allowRuntimeCommand, setter: setString(&userImage)},
		{name: "kube_params", permission: kw.allowRuntimeParams, setter: setString(&userParams)},
		{name: "kube_namespace", permission: kw.allowRuntimeAuth, setter: setString(&ked.KubeNamespace)},
		{name: "secret_kube_config", permission: kw.allowRuntimeAuth, setter: setString(&ked.KubeConfig)},
		{name: "secret_kube_pod", permission: kw.allowRuntimePod, setter: setString(&userPod)},
		{name: "kube_verify_tls", permission: kw.allowRuntimeTLS, setter: setBool(&ked.KubeVerifyTLS)},
		{name: "kube_tls_ca", permission: kw.allowRuntimeTLS, setter: setString(&ked.KubeTLSCAData)},
		{name: "pod_pending_timeout", permission: kw.allowRuntimeParams, setter: setString(&podPendingTimeoutString)},
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
	if kw.authMethod == "runtime" && ked.KubeConfig == "" {
		return fmt.Errorf("param secret_kube_config must be provided if AuthMethod=runtime")
	}
	if userPod != "" && (userParams != "" || userCommand != "" || userImage != "") {
		return fmt.Errorf("params kube_command, kube_image, kube_params not compatible with secret_kube_pod")
	}

	if podPendingTimeoutString != "" {
		podPendingTimeout, err := time.ParseDuration(podPendingTimeoutString)
		if err != nil {
			logger.Error("Failed to parse pod_pending_timeout -- valid examples include '1.5h', '30m', '30m10s'")

			return err
		}
		kw.podPendingTimeout = podPendingTimeout
	}

	if userCommand != "" {
		ked.Command = userCommand
	}
	if userImage != "" {
		ked.Image = userImage
	}
	if userPod != "" {
		ked.KubePod = userPod
		ked.Image = ""
		ked.Command = ""
		kw.baseParams = ""
	} else {
		ked.Params = combineParams(kw.baseParams, userParams)
	}

	return nil
}

// Status returns a copy of the status currently loaded in memory.
func (kw *kubeUnit) Status() *StatusFileData {
	status := kw.UnredactedStatus()
	ed, ok := status.ExtraData.(*kubeExtraData)
	if ok {
		ed.KubeConfig = ""
		ed.KubePod = ""
	}

	return status
}

// Status returns a copy of the status currently loaded in memory.
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

// startOrRestart is a shared implementation of Start() and Restart().
func (kw *kubeUnit) startOrRestart() error {
	// Connect to the Kubernetes API
	if err := kw.connectToKube(); err != nil {
		return err
	}
	// Launch runner process
	if kw.streamMethod == "tcp" {
		go kw.runWorkUsingTCP()
	} else {
		go kw.runWorkUsingLogger()
	}
	go kw.monitorLocalStatus()

	return nil
}

// Restart resumes monitoring a job after a Receptor restart.
func (kw *kubeUnit) Restart() error {
	status := kw.Status()
	ked := status.ExtraData.(*kubeExtraData)
	if IsComplete(status.State) {
		return nil
	}
	isTCP := kw.streamMethod == "tcp"
	if status.State == WorkStateRunning && !isTCP {
		return kw.startOrRestart()
	}
	// Work unit is in Pending state
	if kw.deletePodOnRestart {
		err := kw.connectToKube()
		if err != nil {
			logger.Warning("Pod %s could not be deleted: %s", ked.PodName, err.Error())
		} else {
			err := kw.clientset.CoreV1().Pods(ked.KubeNamespace).Delete(context.Background(), ked.PodName, metav1.DeleteOptions{})
			if err != nil {
				logger.Warning("Pod %s could not be deleted: %s", ked.PodName, err.Error())
			}
		}
	}
	if isTCP {
		return fmt.Errorf("restart not implemented for streammethod tcp")
	}

	return fmt.Errorf("work unit is not in running state, cannot be restarted")
}

// Start launches a job with given parameters.
func (kw *kubeUnit) Start() error {
	kw.UpdateBasicStatus(WorkStatePending, "Connecting to Kubernetes", 0)

	return kw.startOrRestart()
}

// Cancel releases resources associated with a job, including cancelling it if running.
func (kw *kubeUnit) Cancel() error {
	kw.cancel()
	if kw.pod != nil {
		err := kw.clientset.CoreV1().Pods(kw.pod.Namespace).Delete(context.Background(), kw.pod.Name, metav1.DeleteOptions{})
		if err != nil {
			logger.Error("Error deleting pod %s: %s", kw.pod.Name, err)
		}
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

// workKubeCfg is the cmdline configuration object for a Kubernetes worker plugin.
type workKubeCfg struct {
	WorkType            string `required:"true" description:"Name for this worker type"`
	Namespace           string `description:"Kubernetes namespace to create pods in"`
	Image               string `description:"Container image to use for the worker pod"`
	Command             string `description:"Command to run in the container (overrides entrypoint)"`
	Params              string `description:"Command-line parameters to pass to the entrypoint"`
	AuthMethod          string `description:"One of: kubeconfig, incluster" default:"incluster"`
	KubeConfig          string `description:"Kubeconfig filename (for authmethod=kubeconfig)"`
	Pod                 string `description:"Pod definition filename, in json or yaml format"`
	KubeVerifyTLS       bool   `description:"verify server TLS certificate/hostname" default:"true"`
	KubeTLSCAData       string `description:"CA certificate PEM data to verify against"`
	AllowRuntimeAuth    bool   `description:"Allow passing API parameters at runtime" default:"false"`
	AllowRuntimeTLS     bool   `description:"Allow passing TLS parameters at runtime" default:"false"`
	AllowRuntimeCommand bool   `description:"Allow specifying image & command at runtime" default:"false"`
	AllowRuntimeParams  bool   `description:"Allow adding command parameters at runtime" default:"false"`
	AllowRuntimePod     bool   `description:"Allow passing Pod at runtime" default:"false"`
	DeletePodOnRestart  bool   `description:"On restart, delete the pod if in pending state" default:"true"`
	StreamMethod        string `description:"Method for connecting to worker pods: logger or tcp" default:"logger"`
}

// newWorker is a factory to produce worker instances.
func (cfg workKubeCfg) newWorker(w *Workceptor, unitID string, workType string) WorkUnit {
	ku := &kubeUnit{
		BaseWorkUnit: BaseWorkUnit{
			status: StatusFileData{
				ExtraData: &kubeExtraData{
					Image:         cfg.Image,
					Command:       cfg.Command,
					KubeNamespace: cfg.Namespace,
					KubePod:       cfg.Pod,
					KubeConfig:    cfg.KubeConfig,
					KubeVerifyTLS: cfg.KubeVerifyTLS,
					KubeTLSCAData: cfg.KubeTLSCAData,
				},
			},
		},
		authMethod:          strings.ToLower(cfg.AuthMethod),
		streamMethod:        strings.ToLower(cfg.StreamMethod),
		baseParams:          cfg.Params,
		allowRuntimeAuth:    cfg.AllowRuntimeAuth,
		allowRuntimeTLS:     cfg.AllowRuntimeTLS,
		allowRuntimeCommand: cfg.AllowRuntimeCommand,
		allowRuntimeParams:  cfg.AllowRuntimeParams,
		allowRuntimePod:     cfg.AllowRuntimePod,
		deletePodOnRestart:  cfg.DeletePodOnRestart,
		namePrefix:          fmt.Sprintf("%s-", strings.ToLower(cfg.WorkType)),
	}
	ku.BaseWorkUnit.Init(w, unitID, workType)

	return ku
}

// Prepare inspects the configuration for validity.
func (cfg workKubeCfg) Prepare() error {
	lcAuth := strings.ToLower(cfg.AuthMethod)
	if lcAuth != "kubeconfig" && lcAuth != "incluster" && lcAuth != "runtime" {
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
	if cfg.Pod != "" && (cfg.Image != "" || cfg.Command != "" || cfg.Params != "") {
		return fmt.Errorf("can only provide Pod when Image, Command, and Params are empty")
	}
	if cfg.KubeTLSCAData != "" {
		block, _ := pem.Decode([]byte(cfg.KubeTLSCAData))
		if block == nil || block.Type != "BEGIN CERTIFICATE" {
			return fmt.Errorf("could not decode KubeTLSCAData as a PEM formatted certificate")
		}
	}
	if cfg.Image == "" && !cfg.AllowRuntimeCommand && !cfg.AllowRuntimePod {
		return fmt.Errorf("must specify a container image to run")
	}
	method := strings.ToLower(cfg.StreamMethod)
	if method != "logger" && method != "tcp" {
		return fmt.Errorf("stream mode must be logger or tcp")
	}

	return nil
}

// Run runs the action.
func (cfg workKubeCfg) Run() error {
	err := MainInstance.RegisterWorker(cfg.WorkType, cfg.newWorker)

	return err
}

func init() {
	cmdline.RegisterConfigTypeForApp("receptor-workers",
		"work-kubernetes", "Run a worker using Kubernetes", workKubeCfg{}, cmdline.Section(workersSection))
}

// Kubernetes allows receptor interfacing with a k8s cluster.
type Kubernetes struct {
	// Name for this worker type.
	WorkType string `mapstructure:"work-type"`
	// Kubernetes namespace to create pods in.
	Namespace *string `mapstructure:"namespace"`
	// Container image to use for the worker pod.
	Image string `mapstructure:"image"`
	// Command to run in the container (overrides entrypoint).
	Command string `mapstructure:"command"`
	// Command-line parameters to pass to the entrypoint.
	Params string `mapstructure:"parameters"`
	// One of: kubeconfig, incluster.
	AuthMethod *string `mapstructure:"auth-method"` // default:"incluster"`
	// Kubeconfig filename (for authmethod=kubeconfig).
	KubeConfig *string `mapstructure:"kube-config"`
	// Pod definition filename, in json or yaml format.
	Pod string `mapstructure:"pod"`
	// Do not verify server TLS certificate/hostname.
	KubeNoVerifyTLS bool `mapstructure:"insecure-kube-no-verify-tls"`
	// CA certificate PEM data to verify against.
	KubeTLSCAData string `mapstructure:"kube-tls-ca-data"`
	// Allow passing API parameters at runtime.
	AllowRuntimeAuth bool `mapstructure:"allow-runtime-auth"`
	// Allow passing TLS parameters at runtime.
	AllowRuntimeTLS bool `mapstructure:"allow-runtime-tls"`
	// Allow specifying image & command at runtime.
	AllowRuntimeCommand bool `mapstructure:"allow-runtime-command"`
	// Allow adding command parameters at runtime.
	AllowRuntimeParams bool `mapstructure:"allow-runtime-parameters"`
	// Allow passing Pod at runtime.
	AllowRuntimePod bool `mapstructure:"allow-runtime-pod"`
	// On restart, keep the pod if in pending state instead of deleting it.
	KeepPodOnRestart bool `mapstructure:"keep-pod-on-restart"`
	// Method for connecting to worker pods: logger or tcp.
	StreamMethod *string `mapstructure:""`
}

func (k Kubernetes) setup(wc *Workceptor) error {
	authMethod := "incluster"
	if k.AuthMethod != nil {
		authMethod = *k.AuthMethod
	}
	if authMethod != "kubeconfig" && authMethod != "incluster" && authMethod != "runtime" {
		return fmt.Errorf("invalid AuthMethod: %s", authMethod)
	}
	namespace := ""
	if k.Namespace != nil {
		namespace = *k.Namespace
	} else if !(authMethod == "kubeconfig" || k.AllowRuntimeAuth) {
		return fmt.Errorf("must provide namespace when AuthMethod is not kubeconfig")
	}
	kubeConfig := ""
	if k.KubeConfig != nil {
		if authMethod != "kubeconfig" {
			return fmt.Errorf("can only provide KubeConfig when AuthMethod=kubeconfig")
		}

		if _, err := os.Stat(*k.KubeConfig); err != nil {
			return fmt.Errorf("error accessing kubeconfig file: %s", err)
		}
		kubeConfig = *k.KubeConfig
	}

	if k.Pod != "" && (k.Image != "" || k.Command != "" || k.Params != "") {
		return fmt.Errorf("can only provide Pod when Image, Command, and Params are empty")
	}
	if k.KubeTLSCAData != "" {
		if block, _ := pem.Decode([]byte(k.KubeTLSCAData)); block == nil || block.Type != "BEGIN CERTIFICATE" {
			return fmt.Errorf("could not decode KubeTLSCAData as a PEM formatted certificate")
		}
	}
	if k.Image == "" && !k.AllowRuntimeCommand && !k.AllowRuntimePod {
		return fmt.Errorf("must specify a container image to run")
	}
	streamMethod := "logger"
	if k.StreamMethod != nil {
		streamMethod = *k.StreamMethod
	}
	if streamMethod != "logger" && streamMethod != "tcp" {
		return fmt.Errorf("stream mode must be logger or tcp")
	}

	factory := func(w *Workceptor, unitID string, workType string) WorkUnit {
		ku := &kubeUnit{
			BaseWorkUnit: BaseWorkUnit{
				status: StatusFileData{
					ExtraData: &kubeExtraData{
						Image:         k.Image,
						Command:       k.Command,
						KubeNamespace: namespace,
						KubePod:       k.Pod,
						KubeConfig:    kubeConfig,
						KubeVerifyTLS: !k.KubeNoVerifyTLS,
						KubeTLSCAData: k.KubeTLSCAData,
					},
				},
			},
			authMethod:          authMethod,
			streamMethod:        streamMethod,
			baseParams:          k.Params,
			allowRuntimeAuth:    k.AllowRuntimeAuth,
			allowRuntimeTLS:     k.AllowRuntimeTLS,
			allowRuntimeCommand: k.AllowRuntimeCommand,
			allowRuntimeParams:  k.AllowRuntimeParams,
			allowRuntimePod:     k.AllowRuntimePod,
			deletePodOnRestart:  !k.KeepPodOnRestart,
			namePrefix:          fmt.Sprintf("%s-", strings.ToLower(k.WorkType)),
		}
		ku.BaseWorkUnit.Init(w, unitID, workType)

		return ku
	}

	return wc.RegisterWorker(k.WorkType, factory)
}
