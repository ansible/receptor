//go:build !no_workceptor
// +build !no_workceptor

package workceptor

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ghjm/cmdline"
	"github.com/google/shlex"
	configv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
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
	KubeConfig    string
	KubePod       string
	PodName       string
}

// ErrPodCompleted is returned when pod has already completed before we could attach.
var ErrPodCompleted = fmt.Errorf("pod ran to completion")

// ErrImagePullBackOff is returned when the image for the container in the Pod cannot be pulled.
var ErrImagePullBackOff = fmt.Errorf("container failed to start")

// podRunningAndReady is a completion criterion for pod ready to be attached to.
func podRunningAndReady() func(event watch.Event) (bool, error) {
	imagePullBackOffRetries := 3
	inner := func(event watch.Event) (bool, error) {
		if event.Type == watch.Deleted {
			return false, apierrors.NewNotFound(schema.GroupResource{Resource: "pods"}, "")
		}
		if t, ok := event.Object.(*corev1.Pod); ok {
			switch t.Status.Phase {
			case corev1.PodFailed, corev1.PodSucceeded:
				return false, ErrPodCompleted
			case corev1.PodRunning, corev1.PodPending:
				conditions := t.Status.Conditions
				if conditions == nil {
					return false, nil
				}
				for i := range conditions {
					if conditions[i].Type == corev1.PodReady &&
						conditions[i].Status == corev1.ConditionTrue {
						return true, nil
					}
					if conditions[i].Type == corev1.ContainersReady &&
						conditions[i].Status == corev1.ConditionFalse {
						statuses := t.Status.ContainerStatuses
						for j := range statuses {
							if statuses[j].State.Waiting != nil {
								if statuses[j].State.Waiting.Reason == "ImagePullBackOff" {
									if imagePullBackOffRetries == 0 {
										return false, ErrImagePullBackOff
									}
									imagePullBackOffRetries--
								}
							}
						}
					}
				}
			}
		}

		return false, nil
	}

	return inner
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

	// get pod and store to kw.pod
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

	ev, err := watch2.UntilWithSync(ctxPodReady, lw, &corev1.Pod{}, nil, podRunningAndReady())
	if ev == nil || ev.Object == nil {
		return fmt.Errorf("did not return an event while watching pod for work unit %s", kw.ID())
	}

	var ok bool
	kw.pod, ok = ev.Object.(*corev1.Pod)
	if !ok {
		return fmt.Errorf("watch did not return a pod")
	}

	if err == ErrPodCompleted {
		// Hao: shouldn't we also call kw.Cancel() in these cases?
		if len(kw.pod.Status.ContainerStatuses) != 1 {
			return fmt.Errorf("expected 1 container in pod but there were %d", len(kw.pod.Status.ContainerStatuses))
		}

		cstat := kw.pod.Status.ContainerStatuses[0]
		if cstat.State.Terminated != nil && cstat.State.Terminated.ExitCode != 0 {
			return fmt.Errorf("container failed with exit code %d: %s", cstat.State.Terminated.ExitCode, cstat.State.Terminated.Message)
		}

		return err
	} else if err != nil { // any other error besides ErrPodCompleted
		kw.Cancel()
		if len(kw.pod.Status.ContainerStatuses) == 1 {
			if kw.pod.Status.ContainerStatuses[0].State.Waiting != nil {
				return fmt.Errorf("%s, %s", err.Error(), kw.pod.Status.ContainerStatuses[0].State.Waiting.Reason)
			}
		}

		return err
	}

	return nil
}

func (kw *kubeUnit) runWorkUsingLogger() {
	skipStdin := true

	status := kw.Status()
	ked := status.ExtraData.(*kubeExtraData)

	podName := ked.PodName
	podNamespace := ked.KubeNamespace

	if podName == "" {
		// create new pod if ked.PodName is empty
		if err := kw.createPod(nil); err != nil {
			if err != ErrPodCompleted {
				errMsg := fmt.Sprintf("Error creating pod: %s", err)
				kw.Error(errMsg)
				kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)

				return
			}
		} else {
			// for newly created pod we need to streaming stdin
			skipStdin = false
		}

		podName = kw.pod.Name
		podNamespace = kw.pod.Namespace
	} else {
		if podNamespace == "" {
			errMsg := fmt.Sprintf("Error creating pod: pod namespace is empty for pod %s",
				podName,
			)
			kw.Error(errMsg)
			kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)

			return
		}

		// resuming from a previously created pod
		var err error
		for retries := 5; retries > 0; retries-- {
			kw.pod, err = kw.clientset.CoreV1().Pods(podNamespace).Get(kw.ctx, podName, metav1.GetOptions{})
			if err == nil {
				break
			} else {
				time.Sleep(100 * time.Millisecond)
			}
		}
		if err != nil {
			errMsg := fmt.Sprintf("Error getting pod %s/%s: %s", podNamespace, podName, err)
			kw.Error(errMsg)
			kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)

			return
		}
	}

	// Attach stdin stream to the pod
	var exec remotecommand.Executor
	if !skipStdin {
		req := kw.clientset.CoreV1().RESTClient().Post().
			Resource("pods").
			Name(podName).
			Namespace(podNamespace).
			SubResource("attach")

		req.VersionedParams(
			&corev1.PodExecOptions{
				Container: "worker",
				Stdin:     true,
				Stdout:    false,
				Stderr:    false,
				TTY:       false,
			},
			scheme.ParameterCodec,
		)

		var err error
		exec, err = remotecommand.NewSPDYExecutor(kw.config, "POST", req.URL())
		if err != nil {
			errMsg := fmt.Sprintf("Error creating SPDY executor: %s", err)
			kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)

			return
		}
	}

	var stdinErr error
	var stdoutErr error

	// finishedChan signal the stdin and stdout monitoring goroutine to stop
	finishedChan := make(chan struct{})

	// this will signal the stdin and stdout monitoring goroutine to stop when this function returns
	defer close(finishedChan)

	stdinErrChan := make(chan struct{}) // signal that stdin goroutine have errored and stop stdout goroutine

	// open stdin reader that reads from the work unit's data directory
	var stdin *stdinReader
	if !skipStdin {
		var err error
		stdin, err = newStdinReader(kw.UnitDir())
		if err != nil {
			if errors.Is(err, errFileSizeZero) {
				skipStdin = true
			} else {
				errMsg := fmt.Sprintf("Error opening stdin file: %s", err)
				kw.Error(errMsg)
				kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)

				return
			}
		} else {
			// goroutine to cancel stdin reader
			go func() {
				select {
				case <-kw.ctx.Done():
					stdin.reader.Close()

					return
				case <-finishedChan:
				case <-stdin.Done():
					return
				}
			}()
		}
	}

	// open stdout writer that writes to work unit's data directory
	stdout, err := newStdoutWriter(kw.UnitDir())
	if err != nil {
		errMsg := fmt.Sprintf("Error opening stdout file: %s", err)
		kw.Error(errMsg)
		kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)

		return
	}

	// goroutine to cancel stdout stream
	go func() {
		select {
		case <-kw.ctx.Done():
			stdout.writer.Close()

			return
		case <-stdinErrChan:
			stdout.writer.Close()

			return
		case <-finishedChan:
			return
		}
	}()

	streamWait := sync.WaitGroup{}
	streamWait.Add(2)

	if skipStdin {
		kw.UpdateBasicStatus(WorkStateRunning, "Pod Running", stdout.Size())
		streamWait.Done()
	} else {
		go func() {
			defer streamWait.Done()

			kw.UpdateFullStatus(func(status *StatusFileData) {
				status.State = WorkStatePending
				status.Detail = "Sending stdin to pod"
			})

			var err error
			for retries := 5; retries > 0; retries-- {
				err = exec.Stream(remotecommand.StreamOptions{
					Stdin: stdin,
					Tty:   false,
				})
				if err != nil {
					// NOTE: io.EOF for stdin is handled by remotecommand and will not trigger this
					kw.Warning("Error streaming stdin to pod %s/%s. Retrying: %s",
						podNamespace,
						podName,
						err,
					)
					time.Sleep(100 * time.Millisecond)
				} else {
					break
				}
			}

			if err != nil {
				stdinErr = err
				errMsg := fmt.Sprintf("Error streaming stdin to pod %s/%s: %s",
					podNamespace,
					podName,
					err,
				)
				kw.Error(errMsg)
				kw.UpdateBasicStatus(WorkStateFailed, errMsg, stdout.Size())

				close(stdinErrChan) // signal STDOUT goroutine to stop
			} else {
				if stdin.Error() == io.EOF {
					kw.UpdateBasicStatus(WorkStateRunning, "Pod Running", stdout.Size())
				} else {
					// this is probably not possible...
					errMsg := fmt.Sprintf("Error reading stdin: %s", stdin.Error())
					kw.Error(errMsg)
					kw.UpdateBasicStatus(WorkStateFailed, errMsg, stdout.Size())

					close(stdinErrChan) // signal STDOUT goroutine to stop
				}
			}
		}()
	}

	noReconnect := func() {
		// Legacy method, for use on k8s < v1.23.14
		// uses io.Copy to stream data from pod to stdout file
		// known issues around this, as logstream can terminate due to log rotation
		// or 4 hr timeout
		defer streamWait.Done()
		var logStream io.ReadCloser
		logReq := kw.clientset.CoreV1().Pods(podNamespace).GetLogs(
			podName, &corev1.PodLogOptions{
				Container: "worker",
				Follow:    true,
			},
		)
		// get logstream, with retry
		for retries := 5; retries > 0; retries-- {
			logStream, err = logReq.Stream(kw.ctx)
			if err == nil {
				break
			} else {
				errMsg := fmt.Sprintf("Error opening log stream for pod %s/%s. Will retry %d more times.", podNamespace, podName, retries)
				kw.Warning(errMsg)
				time.Sleep(time.Second)
			}
		}
		if err != nil {
			errMsg := fmt.Sprintf("Error opening log stream for pod %s/%s: %s", podNamespace, podName, err)
			kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)
			kw.Error(errMsg)

			return
		}

		_, stdoutErr = io.Copy(stdout, logStream)
	}

	withReconnect := func() {
		// preferred method for k8s >= 1.23.14
		defer streamWait.Done()
		var sinceTime time.Time
		var logStream io.ReadCloser
		eofRetries := 5
		successfulWrite := false
		remainingEOFAttempts := eofRetries // resets on each successful read from pod stdout

		for {
			if stdinErr != nil {
				break
			}

			// get pod, with retry
			for retries := 5; retries > 0; retries-- {
				kw.pod, err = kw.clientset.CoreV1().Pods(podNamespace).Get(kw.ctx, podName, metav1.GetOptions{})
				if err == nil {
					break
				} else {
					errMsg := fmt.Sprintf("Error getting pod %s/%s. Will retry %d more times.", podNamespace, podName, retries)
					kw.Warning(errMsg)
					time.Sleep(time.Second)
				}
			}
			if err != nil {
				errMsg := fmt.Sprintf("Error getting pod %s/%s: %s", podNamespace, podName, err)
				kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)
				kw.Error(errMsg)

				break
			}

			logReq := kw.clientset.CoreV1().Pods(podNamespace).GetLogs(
				podName, &corev1.PodLogOptions{
					Container:  "worker",
					Follow:     true,
					Timestamps: true,
					SinceTime:  &metav1.Time{Time: sinceTime},
				},
			)
			// get logstream, with retry
			for retries := 5; retries > 0; retries-- {
				logStream, err = logReq.Stream(kw.ctx)
				if err == nil {
					break
				} else {
					errMsg := fmt.Sprintf("Error opening log stream for pod %s/%s. Will retry %d more times.", podNamespace, podName, retries)
					kw.Warning(errMsg)
					time.Sleep(time.Second)
				}
			}
			if err != nil {
				errMsg := fmt.Sprintf("Error opening log stream for pod %s/%s: %s", podNamespace, podName, err)
				kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)
				kw.Error(errMsg)

				break
			}

			// read from logstream
			streamReader := bufio.NewReader(logStream)
			for stdinErr == nil { // check between every line read to see if we need to stop reading
				line, err := streamReader.ReadString('\n')
				if err == io.EOF {
					kw.Debug("Detected EOF for pod %s/%s. Will retry %d more times.", podNamespace, podName, remainingEOFAttempts)
					successfulWrite = false
					remainingEOFAttempts--
					if remainingEOFAttempts > 0 {
						time.Sleep(100 * time.Millisecond)

						break
					}

					return
				}
				if err != nil {
					stdoutErr = err

					return
				}
				split := strings.SplitN(line, " ", 2)
				timeStamp := parseTime(split[0])
				if !timeStamp.After(sinceTime) && !successfulWrite {
					continue
				}
				msg := split[1]

				_, err = stdout.Write([]byte(msg))
				if err != nil {
					stdoutErr = fmt.Errorf("writing to stdout: %s", err)

					return
				}
				remainingEOFAttempts = eofRetries // each time we read successfully, reset this counter
				sinceTime = *timeStamp
				successfulWrite = true
			}

			logStream.Close()
		}
	}

	stdoutWithReconnect := shouldUseReconnect(kw)
	if stdoutWithReconnect && stdoutErr == nil {
		kw.Debug("streaming stdout with reconnect support")
		go withReconnect()
	} else {
		kw.Debug("streaming stdout with no reconnect support")
		go noReconnect()
	}

	streamWait.Wait()

	if stdinErr != nil || stdoutErr != nil {
		var errDetail string
		switch {
		case stdinErr == nil:
			errDetail = fmt.Sprintf("%s", stdoutErr)
		case stdoutErr == nil:
			errDetail = fmt.Sprintf("%s", stdinErr)
		default:
			errDetail = fmt.Sprintf("stdin: %s, stdout: %s", stdinErr, stdoutErr)
		}
		kw.UpdateBasicStatus(WorkStateFailed, fmt.Sprintf("Stream error running pod: %s", errDetail), stdout.Size())

		return
	}
	kw.UpdateBasicStatus(WorkStateSucceeded, "Finished", stdout.Size())
}

func shouldUseReconnectOCP(kw *kubeUnit) (bool, bool) {
	// isOCP should remain false until it is confirmed that OpenShift is being
	// used
	isOCP := false

	clientset, err := dynamic.NewForConfig(kw.config)
	if err != nil {
		kw.Warning("error getting K8S dynamic clientset")

		return false, isOCP
	}

	gvr := schema.GroupVersionResource{
		Group:    "config.openshift.io",
		Version:  "v1",
		Resource: "clusteroperators",
	}

	resp, err := clientset.Resource(gvr).Get(kw.ctx, "openshift-apiserver", metav1.GetOptions{})
	if err != nil {
		kw.Debug("error getting K8s openshift-apiserver")

		return false, isOCP
	}

	unstructured := resp.UnstructuredContent()
	var data configv1.ClusterOperator
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured, &data)
	if err != nil {
		kw.Debug("cannot unmarshal into ClusterOperator")

		return false, isOCP
	}

	isOCP = true // have confirmed that OCP is being used

	var ocpVersion string
	for _, verItem := range data.Status.Versions {
		if verItem.Name == "openshift-apiserver" {
			ocpVersion = verItem.Version
		}
	}
	if ocpVersion == "" {
		kw.Debug("did not find openshift-apiserver")

		return false, isOCP
	}

	semver, err := version.ParseSemantic(ocpVersion)
	if err != nil {
		kw.Warning("could not parse OCP server version %s, not using reconnect support", ocpVersion)

		return false, isOCP
	}

	// The patch was backported to minor version 10, 11 and 12. We must check z stream
	// based on the minor version
	// if minor version == 12, compare with v4.12.0
	// if minor version == 11, compare with v4.11.16
	// all other minor versions compare with v4.10.42
	var compatibleVer string
	switch semver.Minor() {
	case 12:
		compatibleVer = "v4.12.0"
	case 11:
		compatibleVer = "v4.11.16"
	default:
		compatibleVer = "v4.10.44"
	}

	if semver.AtLeast(version.MustParseSemantic(compatibleVer)) {
		kw.Info("OCP version %s is at least %s, using reconnect support", semver, compatibleVer)

		return true, isOCP
	}
	kw.Warning("OCP version %s not at least %s, not using reconnect support", semver, compatibleVer)

	return false, isOCP
}

func shouldUseReconnectK8S(kw *kubeUnit) bool {
	serverVerInfo, err := kw.clientset.ServerVersion()
	if err != nil {
		kw.Warning("could not detect Kubernetes server version, will not use reconnect support")

		return false
	}

	semver, err := version.ParseSemantic(serverVerInfo.String())
	if err != nil {
		kw.Warning("could parse Kubernetes server version %s, will not use reconnect support", serverVerInfo.String())

		return false
	}

	// The patch was backported to minor version 23, 24 and 25. We must check z stream
	// based on the minor version
	// if minor version == 24, compare with v1.24.8
	// if minor version == 25, compare with v1.25.4
	// all other minor versions compare with v1.23.14
	var compatibleVer string
	switch serverVerInfo.Minor {
	case "24":
		compatibleVer = "v1.24.8"
	case "25":
		compatibleVer = "v1.25.4"
	default:
		compatibleVer = "v1.23.14"
	}

	if semver.AtLeast(version.MustParseSemantic(compatibleVer)) {
		kw.Debug("Kubernetes version %s is at least %s, using reconnect support", serverVerInfo.GitVersion, compatibleVer)

		return true
	}
	kw.Debug("Kubernetes version %s not at least %s, not using reconnect support", serverVerInfo.GitVersion, compatibleVer)

	return false
}

func shouldUseReconnect(kw *kubeUnit) bool {
	// Attempt to detect support for streaming from pod with timestamps based on
	// Kubernetes / OpenShift server versions
	// To use reconnect method, OpenShift server must be at least
	//   v4.10.42
	//   v4.11.16
	//   v4.12.0
	// If not on OpenShift, Kubernetes server must be at least
	//   v1.23.14
	//   v1.24.8
	//   v1.25.4
	// These versions contain a critical patch that permits connecting to the
	// logstream with timestamps enabled.
	// Without the patch, stdout lines would be split after 4K characters into a
	// new line, which will cause issues in Receptor.
	// https://github.com/kubernetes/kubernetes/issues/77603
	// Can override the detection by setting the RECEPTOR_KUBE_SUPPORT_RECONNECT
	// accepted values: "enabled", "disabled", "auto" with "auto" being the default
	// all invalid value will assume to be "auto"

	env, ok := os.LookupEnv("RECEPTOR_KUBE_SUPPORT_RECONNECT")
	if ok {
		switch env {
		case "enabled":
			return true
		case "disabled":
			return false
		case "auto":
			// continue
		default:
			// continue
		}
	}

	shouldReconnect, isOCP := shouldUseReconnectOCP(kw)
	if isOCP {
		return shouldReconnect
	}

	return shouldUseReconnectK8S(kw)
}

func parseTime(s string) *time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return &t
	}

	t, err = time.Parse(time.RFC3339Nano, s)
	if err == nil {
		return &t
	}

	return nil
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
	content, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// SetFromParams sets the in-memory state from parameters.
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
	AllowRuntimeAuth    bool   `description:"Allow passing API parameters at runtime" default:"false"`
	AllowRuntimeCommand bool   `description:"Allow specifying image & command at runtime" default:"false"`
	AllowRuntimeParams  bool   `description:"Allow adding command parameters at runtime" default:"false"`
	AllowRuntimePod     bool   `description:"Allow passing Pod at runtime" default:"false"`
	DeletePodOnRestart  bool   `description:"On restart, delete the pod if in pending state" default:"true"`
	StreamMethod        string `description:"Method for connecting to worker pods: logger or tcp" default:"logger"`
	VerifySignature     bool   `description:"Verify a signed work submission" default:"false"`
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
				},
			},
		},
		authMethod:          strings.ToLower(cfg.AuthMethod),
		streamMethod:        strings.ToLower(cfg.StreamMethod),
		baseParams:          cfg.Params,
		allowRuntimeAuth:    cfg.AllowRuntimeAuth,
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
	err := MainInstance.RegisterWorker(cfg.WorkType, cfg.newWorker, cfg.VerifySignature)

	return err
}

func init() {
	cmdline.RegisterConfigTypeForApp("receptor-workers",
		"work-kubernetes", "Run a worker using Kubernetes", workKubeCfg{}, cmdline.Section(workersSection))
}
