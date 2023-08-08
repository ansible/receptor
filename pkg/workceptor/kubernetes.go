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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ghjm/cmdline"
	"github.com/google/shlex"
	"golang.org/x/net/http2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	watch2 "k8s.io/client-go/tools/watch"
	"k8s.io/client-go/util/flowcontrol"
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

// ErrPodFailed is returned when pod has failed before we could attach.
var ErrPodFailed = fmt.Errorf("pod failed to start")

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
			case corev1.PodFailed:
				return false, ErrPodFailed
			case corev1.PodSucceeded:
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

func (kw *kubeUnit) kubeLoggingConnectionHandler(timestamps bool) (io.ReadCloser, error) {
	var logStream io.ReadCloser
	var err error
	var sinceTime time.Time
	podNamespace := kw.pod.Namespace
	podName := kw.pod.Name
	podOptions := &corev1.PodLogOptions{
		Container: "worker",
		Follow:    true,
	}
	if timestamps {
		podOptions.Timestamps = true
		podOptions.SinceTime = &metav1.Time{Time: sinceTime}
	}

	logReq := kw.clientset.CoreV1().Pods(podNamespace).GetLogs(
		podName, podOptions,
	)
	// get logstream, with retry
	for retries := 5; retries > 0; retries-- {
		logStream, err = logReq.Stream(kw.ctx)
		if err == nil {
			break
		}
		kw.Warning(
			"Error opening log stream for pod %s/%s. Will retry %d more times. Error: %s",
			podNamespace,
			podName,
			retries,
			err,
		)
		time.Sleep(time.Second)
	}
	if err != nil {
		errMsg := fmt.Sprintf("Error opening log stream for pod %s/%s. Error: %s", podNamespace, podName, err)
		kw.Error(errMsg)
		kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)

		return nil, err
	}

	return logStream, nil
}

func (kw *kubeUnit) kubeLoggingNoReconnect(streamWait *sync.WaitGroup, stdout *STDoutWriter, stdoutErr *error) {
	// Legacy method, for use on k8s < v1.23.14
	// uses io.Copy to stream data from pod to stdout file
	// known issues around this, as logstream can terminate due to log rotation
	// or 4 hr timeout
	defer streamWait.Done()
	podNamespace := kw.pod.Namespace
	podName := kw.pod.Name
	logStream, err := kw.kubeLoggingConnectionHandler(false)
	if err != nil {
		return
	}

	_, *stdoutErr = io.Copy(stdout, logStream)
	if *stdoutErr != nil {
		kw.Error(
			"Error streaming pod logs to stdout for pod %s/%s. Error: %s",
			podNamespace,
			podName,
			*stdoutErr,
		)
	}
}

func (kw *kubeUnit) kubeLoggingWithReconnect(streamWait *sync.WaitGroup, stdout *STDoutWriter, stdinErr *error, stdoutErr *error) {
	// preferred method for k8s >= 1.23.14
	defer streamWait.Done()
	var sinceTime time.Time
	var err error
	podNamespace := kw.pod.Namespace
	podName := kw.pod.Name

	retries := 5
	successfulWrite := false
	remainingRetries := retries // resets on each successful read from pod stdout

	for {
		if *stdinErr != nil {
			break
		}

		// get pod, with retry
		for retries := 5; retries > 0; retries-- {
			kw.pod, err = kw.clientset.CoreV1().Pods(podNamespace).Get(kw.ctx, podName, metav1.GetOptions{})
			if err == nil {
				break
			}
			kw.Warning(
				"Error getting pod %s/%s. Will retry %d more times. Error: %s",
				podNamespace,
				podName,
				retries,
				err,
			)
			time.Sleep(time.Second)
		}
		if err != nil {
			errMsg := fmt.Sprintf("Error getting pod %s/%s. Error: %s", podNamespace, podName, err)
			kw.Error(errMsg)
			kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)

			break
		}

		logStream, err := kw.kubeLoggingConnectionHandler(true)
		if err != nil {
			break
		}

		// read from logstream
		streamReader := bufio.NewReader(logStream)
		for *stdinErr == nil { // check between every line read to see if we need to stop reading
			line, err := streamReader.ReadString('\n')
			if err == io.EOF {
				kw.Debug(
					"Detected EOF for pod %s/%s. Will retry %d more times. Error: %s",
					podNamespace,
					podName,
					remainingRetries,
					err,
				)
				successfulWrite = false
				remainingRetries--
				if remainingRetries > 0 {
					time.Sleep(200 * time.Millisecond)

					break
				}

				return
			} else if _, ok := err.(http2.GoAwayError); ok {
				// GOAWAY is sent by the server to indicate that the server is gracefully shutting down
				// this happens if the kube API server we are connected to is being restarted or is shutting down
				// for example during a cluster upgrade and rolling restart of the master node
				kw.Info(
					"Detected http2.GoAwayError for pod %s/%s. Will retry %d more times. Error: %s",
					podNamespace,
					podName,
					remainingRetries,
					err,
				)
				successfulWrite = false
				remainingRetries--
				if remainingRetries > 0 {
					time.Sleep(200 * time.Millisecond)

					break
				}
			}
			if err != nil {
				*stdoutErr = err
				kw.Error("Error reading from pod %s/%s: %s", podNamespace, podName, err)

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
				*stdoutErr = fmt.Errorf("writing to stdout: %s", err)
				kw.Error("Error writing to stdout: %s", err)

				return
			}
			remainingRetries = retries // each time we read successfully, reset this counter
			sinceTime = *timeStamp
			successfulWrite = true
		}

		logStream.Close()
	}
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

	time.Sleep(2 * time.Second)
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
		for _, cstat := range kw.pod.Status.ContainerStatuses {
			if cstat.Name == "worker" {
				if cstat.State.Terminated != nil && cstat.State.Terminated.ExitCode != 0 {
					return fmt.Errorf("container failed with exit code %d: %s", cstat.State.Terminated.ExitCode, cstat.State.Terminated.Message)
				}

				break
			}
		}

		return err
	} else if err != nil { // any other error besides ErrPodCompleted
		stdout, err2 := NewStdoutWriter(FileSystem{}, kw.UnitDir())
		if err2 != nil {
			errMsg := fmt.Sprintf("Error opening stdout file: %s", err2)
			kw.Error(errMsg)
			kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)

			return fmt.Errorf(errMsg)
		}
		var stdoutErr error
		var streamWait sync.WaitGroup
		streamWait.Add(1)
		go kw.kubeLoggingNoReconnect(&streamWait, stdout, &stdoutErr)
		streamWait.Wait()
		kw.Cancel()
		if len(kw.pod.Status.ContainerStatuses) == 1 {
			if kw.pod.Status.ContainerStatuses[0].State.Waiting != nil {
				return fmt.Errorf("%s, %s", err.Error(), kw.pod.Status.ContainerStatuses[0].State.Waiting.Reason)
			}

			for _, cstat := range kw.pod.Status.ContainerStatuses {
				if cstat.Name == "worker" {
					if cstat.State.Waiting != nil {
						return fmt.Errorf("%s, %s", err.Error(), cstat.State.Waiting.Reason)
					}

					if cstat.State.Terminated != nil && cstat.State.Terminated.ExitCode != 0 {
						return fmt.Errorf("%s, exit code %d: %s", err.Error(), cstat.State.Terminated.ExitCode, cstat.State.Terminated.Message)
					}

					break
				}
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
			// for newly created pod we need to stream stdin
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
			// check if the kw.ctx is already cancel
			select {
			case <-kw.ctx.Done():
				errMsg := fmt.Sprintf("Context Done while getting pod %s/%s. Error: %s", podNamespace, podName, kw.ctx.Err())
				kw.Warning(errMsg)

				return
			default:
			}

			kw.pod, err = kw.clientset.CoreV1().Pods(podNamespace).Get(kw.ctx, podName, metav1.GetOptions{})
			if err == nil {
				break
			}
			kw.Warning(
				"Error getting pod %s/%s. Will retry %d more times. Retrying: %s",
				podNamespace,
				podName,
				retries,
				err,
			)
			time.Sleep(200 * time.Millisecond)
		}
		if err != nil {
			errMsg := fmt.Sprintf("Error getting pod %s/%s. Error: %s", podNamespace, podName, err)
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
	var stdin *STDinReader
	if !skipStdin {
		var err error
		stdin, err = NewStdinReader(FileSystem{}, kw.UnitDir())
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
	stdout, err := NewStdoutWriter(FileSystem{}, kw.UnitDir())
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
				err = exec.StreamWithContext(kw.ctx, remotecommand.StreamOptions{
					Stdin: stdin,
					Tty:   false,
				})
				if err != nil {
					// NOTE: io.EOF for stdin is handled by remotecommand and will not trigger this
					kw.Warning(
						"Error streaming stdin to pod %s/%s. Will retry %d more times. Error: %s",
						podNamespace,
						podName,
						retries,
						err,
					)
					time.Sleep(200 * time.Millisecond)
				} else {
					break
				}
			}

			if err != nil {
				stdinErr = err
				errMsg := fmt.Sprintf(
					"Error streaming stdin to pod %s/%s. Error: %s",
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

	stdoutWithReconnect := shouldUseReconnect(kw)
	if stdoutWithReconnect && stdoutErr == nil {
		kw.Debug("streaming stdout with reconnect support")
		go kw.kubeLoggingWithReconnect(&streamWait, stdout, &stdinErr, &stdoutErr)
	} else {
		kw.Debug("streaming stdout with no reconnect support")
		go kw.kubeLoggingNoReconnect(&streamWait, stdout, &stdoutErr)
	}

	streamWait.Wait()

	if stdinErr != nil || stdoutErr != nil {
		var errDetail string
		switch {
		case stdinErr == nil:
			errDetail = fmt.Sprintf("Error with pod's stdout: %s", stdoutErr)
		case stdoutErr == nil:

			errDetail = fmt.Sprintf("Error with pod's stdin: %s", stdinErr)
		default:
			errDetail = fmt.Sprintf("Error running pod. stdin: %s, stdout: %s", stdinErr, stdoutErr)
		}

		if kw.ctx.Err() != context.Canceled {
			kw.UpdateBasicStatus(WorkStateFailed, errDetail, stdout.Size())
		}

		return
	}

	if kw.ctx.Err() != context.Canceled {
		kw.UpdateBasicStatus(WorkStateSucceeded, "Finished", stdout.Size())
	}
}

func isCompatibleK8S(kw *kubeUnit, versionStr string) bool {
	semver, err := version.ParseSemantic(versionStr)
	if err != nil {
		kw.w.nc.GetLogger().Warning("could parse Kubernetes server version %s, will not use reconnect support", versionStr)

		return false
	}

	// ignore pre-release in version comparison
	semver = semver.WithPreRelease("")

	// The patch was backported to minor version 23, 24 and 25. We must check z stream
	// based on the minor version
	// if minor version == 24, compare with v1.24.8
	// if minor version == 25, compare with v1.25.4
	// all other minor versions compare with v1.23.14
	var compatibleVer string
	switch semver.Minor() {
	case 24:
		compatibleVer = "v1.24.8"
	case 25:
		compatibleVer = "v1.25.4"
	default:
		compatibleVer = "v1.23.14"
	}

	if semver.AtLeast(version.MustParseSemantic(compatibleVer)) {
		kw.w.nc.GetLogger().Debug("Kubernetes version %s is at least %s, using reconnect support", semver, compatibleVer)

		return true
	}
	kw.w.nc.GetLogger().Debug("Kubernetes version %s not at least %s, not using reconnect support", semver, compatibleVer)

	return false
}

func shouldUseReconnect(kw *kubeUnit) bool {
	// Attempt to detect support for streaming from pod with timestamps based on
	// Kubernetes server version
	// In order to use reconnect method, Kubernetes server must be at least
	//   v1.23.14
	//   v1.24.8
	//   v1.25.4
	// These versions contain a critical patch that permits connecting to the
	// logstream with timestamps enabled.
	// Without the patch, stdout lines would be split after 4K characters into a
	// new line, which will cause issues in Receptor.
	// https://github.com/kubernetes/kubernetes/issues/77603
	// Can override the detection by setting the RECEPTOR_KUBE_SUPPORT_RECONNECT
	// accepted values: "enabled", "disabled", "auto" with "disabled" being the default
	// all invalid value will assume to be "disabled"

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
			return false
		}
	}

	serverVerInfo, err := kw.clientset.ServerVersion()
	if err != nil {
		kw.w.nc.GetLogger().Warning("could not detect Kubernetes server version, will not use reconnect support")

		return false
	}

	return isCompatibleK8S(kw, serverVerInfo.String())
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
		kw.w.nc.GetLogger().Error(errMsg)

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
			kw.w.nc.GetLogger().Error(errMsg)
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
		kw.w.nc.GetLogger().Error(errMsg)
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
	var stdin *STDinReader
	stdin, err = NewStdinReader(FileSystem{}, kw.UnitDir())
	if err != nil {
		errMsg := fmt.Sprintf("Error opening stdin file: %s", err)
		kw.w.nc.GetLogger().Error(errMsg)
		kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)
		cancel()

		return
	}

	// Open stdout writer
	stdout, err := NewStdoutWriter(FileSystem{}, kw.UnitDir())
	if err != nil {
		errMsg := fmt.Sprintf("Error opening stdout file: %s", err)
		kw.w.nc.GetLogger().Error(errMsg)
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
			kw.w.nc.GetLogger().Error(errMsg)
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
		kw.w.nc.GetLogger().Error(errMsg)
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

	kw.config.QPS = float32(100)
	kw.config.Burst = 1000

	// RECEPTOR_KUBE_CLIENTSET_QPS
	// default: 100
	envQPS, ok := os.LookupEnv("RECEPTOR_KUBE_CLIENTSET_QPS")
	if ok {
		qps, err := strconv.Atoi(envQPS)
		if err != nil {
			// ignore error, use default
			kw.Warning("Invalid value for RECEPTOR_KUBE_CLIENTSET_QPS: %s. Ignoring", envQPS)
		} else {
			kw.config.QPS = float32(qps)
			kw.config.Burst = qps * 10
		}
	}

	kw.Debug("RECEPTOR_KUBE_CLIENTSET_QPS: %s", envQPS)

	// RECEPTOR_KUBE_CLIENTSET_BURST
	// default: 10 x QPS
	envBurst, ok := os.LookupEnv("RECEPTOR_KUBE_CLIENTSET_BURST")
	if ok {
		burst, err := strconv.Atoi(envBurst)
		if err != nil {
			kw.Warning("Invalid value for RECEPTOR_KUBE_CLIENTSET_BURST: %s. Ignoring", envQPS)
		} else {
			kw.config.Burst = burst
		}
	}

	kw.Debug("RECEPTOR_KUBE_CLIENTSET_BURST: %s", envBurst)

	kw.Debug("Initializing Kubernetes clientset")
	// RECEPTOR_KUBE_CLIENTSET_RATE_LIMITER
	// default: tokenbucket
	// options: never, always, tokenbucket
	envRateLimiter, ok := os.LookupEnv("RECEPTOR_KUBE_CLIENTSET_RATE_LIMITER")
	if ok {
		switch envRateLimiter {
		case "never":
			kw.config.RateLimiter = flowcontrol.NewFakeNeverRateLimiter()
		case "always":
			kw.config.RateLimiter = flowcontrol.NewFakeAlwaysRateLimiter()
		default:
		}
		kw.Debug("RateLimiter: %s", envRateLimiter)
	}

	kw.Debug("QPS: %f, Burst: %d", kw.config.QPS, kw.config.Burst)
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
			kw.w.nc.GetLogger().Error("Failed to parse pod_pending_timeout -- valid examples include '1.5h', '30m', '30m10s'")

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
			kw.w.nc.GetLogger().Warning("Pod %s could not be deleted: %s", ked.PodName, err.Error())
		} else {
			err := kw.clientset.CoreV1().Pods(ked.KubeNamespace).Delete(context.Background(), ked.PodName, metav1.DeleteOptions{})
			if err != nil {
				kw.w.nc.GetLogger().Warning("Pod %s could not be deleted: %s", ked.PodName, err.Error())
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
	kw.UpdateBasicStatus(WorkStateCanceled, "Canceled", -1)
	if kw.pod != nil {
		err := kw.clientset.CoreV1().Pods(kw.pod.Namespace).Delete(context.Background(), kw.pod.Name, metav1.DeleteOptions{})
		if err != nil {
			kw.w.nc.GetLogger().Error("Error deleting pod %s: %s", kw.pod.Name, err)
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

// KubeWorkerCfg is the cmdline configuration object for a Kubernetes worker plugin.
type KubeWorkerCfg struct {
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

// NewWorker is a factory to produce worker instances.
func (cfg KubeWorkerCfg) NewWorker(w *Workceptor, unitID string, workType string) WorkUnit {
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
func (cfg KubeWorkerCfg) Prepare() error {
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
	if cfg.Pod == "" && cfg.Image == "" && !cfg.AllowRuntimeCommand && !cfg.AllowRuntimePod {
		return fmt.Errorf("must specify a container image to run")
	}
	method := strings.ToLower(cfg.StreamMethod)
	if method != "logger" && method != "tcp" {
		return fmt.Errorf("stream mode must be logger or tcp")
	}

	return nil
}

func (cfg KubeWorkerCfg) GetWorkType() string {
	return cfg.WorkType
}

func (cfg KubeWorkerCfg) GetVerifySignature() bool {
	return cfg.VerifySignature
}

// Run runs the action.
func (cfg KubeWorkerCfg) Run() error {
	err := MainInstance.RegisterWorker(cfg.WorkType, cfg.NewWorker, cfg.VerifySignature)

	return err
}

func init() {
	cmdline.RegisterConfigTypeForApp("receptor-workers",
		"work-kubernetes", "Run a worker using Kubernetes", KubeWorkerCfg{}, cmdline.Section(workersSection))
}
