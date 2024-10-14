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
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ghjm/cmdline"
	"github.com/google/shlex"
	"github.com/spf13/viper"
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

// KubeUnit implements the WorkUnit interface.
type KubeUnit struct {
	BaseWorkUnitForWorkUnit
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
type KubeExtraData struct {
	Image         string
	Command       string
	Params        string
	KubeNamespace string
	KubeConfig    string
	KubePod       string
	PodName       string
}

type KubeAPIer interface {
	NewNotFound(schema.GroupResource, string) *apierrors.StatusError
	OneTermEqualSelector(string, string) fields.Selector
	NewForConfig(*rest.Config) (*kubernetes.Clientset, error)
	GetLogs(*kubernetes.Clientset, string, string, *corev1.PodLogOptions) *rest.Request
	Get(context.Context, *kubernetes.Clientset, string, string, metav1.GetOptions) (*corev1.Pod, error)
	Create(context.Context, *kubernetes.Clientset, string, *corev1.Pod, metav1.CreateOptions) (*corev1.Pod, error)
	List(context.Context, *kubernetes.Clientset, string, metav1.ListOptions) (*corev1.PodList, error)
	Watch(context.Context, *kubernetes.Clientset, string, metav1.ListOptions) (watch.Interface, error)
	Delete(context.Context, *kubernetes.Clientset, string, string, metav1.DeleteOptions) error
	SubResource(*kubernetes.Clientset, string, string) *rest.Request
	InClusterConfig() (*rest.Config, error)
	NewDefaultClientConfigLoadingRules() *clientcmd.ClientConfigLoadingRules
	BuildConfigFromFlags(string, string) (*rest.Config, error)
	NewClientConfigFromBytes([]byte) (clientcmd.ClientConfig, error)
	NewSPDYExecutor(*rest.Config, string, *url.URL) (remotecommand.Executor, error)
	StreamWithContext(context.Context, remotecommand.Executor, remotecommand.StreamOptions) error
	UntilWithSync(context.Context, cache.ListerWatcher, runtime.Object, watch2.PreconditionFunc, ...watch2.ConditionFunc) (*watch.Event, error)
	NewFakeNeverRateLimiter() flowcontrol.RateLimiter
	NewFakeAlwaysRateLimiter() flowcontrol.RateLimiter
}

type KubeAPIWrapper struct{}

func (ku KubeAPIWrapper) NewNotFound(qualifiedResource schema.GroupResource, name string) *apierrors.StatusError {
	return apierrors.NewNotFound(qualifiedResource, name)
}

func (ku KubeAPIWrapper) OneTermEqualSelector(k string, v string) fields.Selector {
	return fields.OneTermEqualSelector(k, v)
}

func (ku KubeAPIWrapper) NewForConfig(c *rest.Config) (*kubernetes.Clientset, error) {
	return kubernetes.NewForConfig(c)
}

func (ku KubeAPIWrapper) GetLogs(clientset *kubernetes.Clientset, namespace string, name string, opts *corev1.PodLogOptions) *rest.Request {
	return clientset.CoreV1().Pods(namespace).GetLogs(name, opts)
}

func (ku KubeAPIWrapper) Get(ctx context.Context, clientset *kubernetes.Clientset, namespace string, name string, opts metav1.GetOptions) (*corev1.Pod, error) {
	return clientset.CoreV1().Pods(namespace).Get(ctx, name, opts)
}

func (ku KubeAPIWrapper) Create(ctx context.Context, clientset *kubernetes.Clientset, namespace string, pod *corev1.Pod, opts metav1.CreateOptions) (*corev1.Pod, error) {
	return clientset.CoreV1().Pods(namespace).Create(ctx, pod, opts)
}

func (ku KubeAPIWrapper) List(ctx context.Context, clientset *kubernetes.Clientset, namespace string, opts metav1.ListOptions) (*corev1.PodList, error) {
	return clientset.CoreV1().Pods(namespace).List(ctx, opts)
}

func (ku KubeAPIWrapper) Watch(ctx context.Context, clientset *kubernetes.Clientset, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return clientset.CoreV1().Pods(namespace).Watch(ctx, opts)
}

func (ku KubeAPIWrapper) Delete(ctx context.Context, clientset *kubernetes.Clientset, namespace string, name string, opts metav1.DeleteOptions) error {
	return clientset.CoreV1().Pods(namespace).Delete(ctx, name, opts)
}

func (ku KubeAPIWrapper) SubResource(clientset *kubernetes.Clientset, podName string, podNamespace string) *rest.Request {
	return clientset.CoreV1().RESTClient().Post().Resource("pods").Name(podName).Namespace(podNamespace).SubResource("attach")
}

func (ku KubeAPIWrapper) InClusterConfig() (*rest.Config, error) {
	return rest.InClusterConfig()
}

func (ku KubeAPIWrapper) NewDefaultClientConfigLoadingRules() *clientcmd.ClientConfigLoadingRules {
	return clientcmd.NewDefaultClientConfigLoadingRules()
}

func (ku KubeAPIWrapper) BuildConfigFromFlags(masterURL string, kubeconfigPath string) (*rest.Config, error) {
	return clientcmd.BuildConfigFromFlags(masterURL, kubeconfigPath)
}

func (ku KubeAPIWrapper) NewClientConfigFromBytes(configBytes []byte) (clientcmd.ClientConfig, error) {
	return clientcmd.NewClientConfigFromBytes(configBytes)
}

func (ku KubeAPIWrapper) NewSPDYExecutor(config *rest.Config, method string, url *url.URL) (remotecommand.Executor, error) {
	return remotecommand.NewSPDYExecutor(config, method, url)
}

func (ku KubeAPIWrapper) StreamWithContext(ctx context.Context, exec remotecommand.Executor, options remotecommand.StreamOptions) error {
	return exec.StreamWithContext(ctx, options)
}

func (ku KubeAPIWrapper) UntilWithSync(ctx context.Context, lw cache.ListerWatcher, objType runtime.Object, precondition watch2.PreconditionFunc, conditions ...watch2.ConditionFunc) (*watch.Event, error) {
	return watch2.UntilWithSync(ctx, lw, objType, precondition, conditions...)
}

func (ku KubeAPIWrapper) NewFakeNeverRateLimiter() flowcontrol.RateLimiter {
	return flowcontrol.NewFakeNeverRateLimiter()
}

func (ku KubeAPIWrapper) NewFakeAlwaysRateLimiter() flowcontrol.RateLimiter {
	return flowcontrol.NewFakeAlwaysRateLimiter()
}

// KubeAPIWrapperInstance is a package level var that wraps all required kubernetes API calls.
// It is instantiated in the NewkubeWorker function and available throughout the package.
var KubeAPIWrapperInstance KubeAPIer

var KubeAPIWrapperLock *sync.RWMutex

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
			return false, KubeAPIWrapperInstance.NewNotFound(schema.GroupResource{Resource: "pods"}, "")
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

func (kw *KubeUnit) kubeLoggingConnectionHandler(timestamps bool, sinceTime time.Time) (io.ReadCloser, error) {
	var logStream io.ReadCloser
	var err error
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

	logReq := KubeAPIWrapperInstance.GetLogs(kw.clientset, podNamespace, podName, podOptions)
	// get logstream, with retry
	for retries := 5; retries > 0; retries-- {
		logStream, err = logReq.Stream(kw.GetContext())
		if err == nil {
			break
		}
		kw.GetWorkceptor().nc.GetLogger().Warning(
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
		kw.GetWorkceptor().nc.GetLogger().Error(errMsg) //nolint:govet
		kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)

		return nil, err
	}

	return logStream, nil
}

func (kw *KubeUnit) kubeLoggingNoReconnect(streamWait *sync.WaitGroup, stdout *STDoutWriter, stdoutErr *error) {
	// Legacy method, for use on k8s < v1.23.14
	// uses io.Copy to stream data from pod to stdout file
	// known issues around this, as logstream can terminate due to log rotation
	// or 4 hr timeout
	defer streamWait.Done()
	var sinceTime time.Time
	podNamespace := kw.pod.Namespace
	podName := kw.pod.Name
	logStream, err := kw.kubeLoggingConnectionHandler(true, sinceTime)
	if err != nil {
		return
	}

	_, *stdoutErr = io.Copy(stdout, logStream)
	if *stdoutErr != nil {
		kw.GetWorkceptor().nc.GetLogger().Error(
			"Error streaming pod logs to stdout for pod %s/%s. Error: %s",
			podNamespace,
			podName,
			*stdoutErr,
		)
	}
	// After primary log streaming, retrieve remaining logs
	err = kw.retrieveRemainingLogs(stdout, &sinceTime)
	if err != nil {
		kw.GetWorkceptor().nc.GetLogger().Error(
			"Error retrieving remaining logs for pod %s/%s. Error: %s",
			podNamespace,
			podName,
			err,
		)
	}
}

func (kw *KubeUnit) kubeLoggingWithReconnect(streamWait *sync.WaitGroup, stdout *STDoutWriter, stdinErr *error, stdoutErr *error) {
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
			// fail to send stdin to pod, no need to continue
			return
		}

		// get pod, with retry
		for retries := 5; retries > 0; retries-- {
			kw.pod, err = KubeAPIWrapperInstance.Get(kw.GetContext(), kw.clientset, podNamespace, podName, metav1.GetOptions{})
			if err == nil {
				break
			}
			kw.GetWorkceptor().nc.GetLogger().Warning(
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
			kw.GetWorkceptor().nc.GetLogger().Error(errMsg) //nolint:govet
			kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)

			// fail to get pod, no need to continue
			return
		}

		logStream, err := kw.kubeLoggingConnectionHandler(true, sinceTime)
		if err != nil {
			// fail to get log stream, no need to continue
			return
		}

		// read from logstream
		streamReader := bufio.NewReader(logStream)
		for *stdinErr == nil { // check between every line read to see if we need to stop reading
			line, err := streamReader.ReadString('\n')
			if err != nil {
				if kw.GetContext().Err() == context.Canceled {
					kw.GetWorkceptor().nc.GetLogger().Info(
						"Context was canceled while reading logs for pod %s/%s. Assuming pod has finished",
						podNamespace,
						podName,
					)

					return
				}
				kw.GetWorkceptor().nc.GetLogger().Info(
					"Detected Error: %s for pod %s/%s. Will retry %d more times.",
					err,
					podNamespace,
					podName,
					remainingRetries,
				)

				successfulWrite = false
				remainingRetries--
				if remainingRetries > 0 {
					time.Sleep(200 * time.Millisecond)

					break
				}

				kw.GetWorkceptor().nc.GetLogger().Error("Error reading from pod %s/%s: %s", podNamespace, podName, err)

				// At this point we exausted all retries, every retry we either failed to read OR we read but did not get newer msg
				// If we got a EOF on the last retry we assume that we read everything and we can stop the loop
				// we ASSUME this is the happy path.
				if err != io.EOF {
					*stdoutErr = err
				}

				return
			}

			split := strings.SplitN(line, " ", 2)
			timeStamp := ParseTime(split[0])
			if !timeStamp.After(sinceTime) && !successfulWrite {
				continue
			}
			msg := split[1]

			_, err = stdout.Write([]byte(msg))
			if err != nil {
				*stdoutErr = fmt.Errorf("writing to stdout: %s", err)
				kw.GetWorkceptor().nc.GetLogger().Error("Error writing to stdout: %s", err)

				return
			}
			remainingRetries = retries // each time we read successfully, reset this counter
			sinceTime = *timeStamp
			successfulWrite = true
		}

		logStream.Close()

		// Check if the pod has terminated
		podIsTerminated, err := kw.isPodTerminated()
		if err != nil {
			kw.GetWorkceptor().nc.GetLogger().Error("Error checking pod status for %s/%s: %s", podNamespace, podName, err)
			*stdoutErr = err

			return
		}
		if podIsTerminated {
			// Retrieve any remaining logs
			err = kw.retrieveRemainingLogs(stdout, &sinceTime)
			if err != nil {
				kw.GetWorkceptor().nc.GetLogger().Error(
					"Error retrieving remaining logs for pod %s/%s. Error: %s",
					podNamespace,
					podName,
					err,
				)
			}

			break
		}
	}
}

func (kw *KubeUnit) createPod(env map[string]string) error {
	ked := kw.UnredactedStatus().ExtraData.(*KubeExtraData)
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
	kw.pod, err = KubeAPIWrapperInstance.Create(kw.GetContext(), kw.clientset, ked.KubeNamespace, pod, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	select {
	case <-kw.GetContext().Done():
		return fmt.Errorf("cancelled")
	default:
	}

	kw.UpdateFullStatus(func(status *StatusFileData) {
		status.State = WorkStatePending
		status.Detail = "Pod created"
		status.StdoutSize = 0
		status.ExtraData.(*KubeExtraData).PodName = kw.pod.Name
	})

	// Wait for the pod to be running
	fieldSelector := KubeAPIWrapperInstance.OneTermEqualSelector("metadata.name", kw.pod.Name).String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector

			return KubeAPIWrapperInstance.List(kw.GetContext(), kw.clientset, ked.KubeNamespace, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = fieldSelector

			return KubeAPIWrapperInstance.Watch(kw.GetContext(), kw.clientset, ked.KubeNamespace, options)
		},
	}

	ctxPodReady := kw.GetContext()
	if kw.podPendingTimeout != time.Duration(0) {
		var ctxPodCancel context.CancelFunc
		ctxPodReady, ctxPodCancel = context.WithTimeout(kw.GetContext(), kw.podPendingTimeout)
		defer ctxPodCancel()
	}

	time.Sleep(2 * time.Second)
	ev, err := KubeAPIWrapperInstance.UntilWithSync(ctxPodReady, lw, &corev1.Pod{}, nil, podRunningAndReady())
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
			kw.GetWorkceptor().nc.GetLogger().Error(errMsg) //nolint:govet
			kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)

			return fmt.Errorf(errMsg) //nolint:govet,staticcheck
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

func (kw *KubeUnit) retrieveRemainingLogs(stdout io.Writer, sinceTime *time.Time) error {
	podNamespace := kw.pod.Namespace
	podName := kw.pod.Name

	// Set PodLogOptions to retrieve logs since the last timestamp
	podOptions := &corev1.PodLogOptions{
		Container:  "worker",
		Follow:     false,
		Previous:   false,
		Timestamps: true,
	}

	if sinceTime != nil {
		podOptions.SinceTime = &metav1.Time{Time: *sinceTime}
	}

	// Get the logs
	logReq := KubeAPIWrapperInstance.GetLogs(kw.clientset, podNamespace, podName, podOptions)
	logStream, err := logReq.Stream(kw.GetContext())
	if err != nil {
		// Handle case where no logs are available
		if apierrors.IsNotFound(err) {
			kw.GetWorkceptor().nc.GetLogger().Info("No additional logs to retrieve for pod %s/%s", podNamespace, podName)

			return nil
		}
		kw.GetWorkceptor().nc.GetLogger().Error("Error retrieving remaining logs for pod %s/%s: %s", podNamespace, podName, err)

		return err
	}
	defer logStream.Close()

	// Read and process the logs
	streamReader := bufio.NewReader(logStream)
	logsRetrieved := false

	for {
		line, err := streamReader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			kw.GetWorkceptor().nc.GetLogger().Error("Error reading remaining logs for pod %s/%s: %s", podNamespace, podName, err)
			return err
		}

		// Process the log line
		split := strings.SplitN(line, " ", 2)
		if len(split) != 2 {
			continue // Skip malformed lines
		}

		timeStamp := ParseTime(split[0])
		if timeStamp.After(*sinceTime) {
			msg := split[1]

			// Write the log message to stdout
			_, err = stdout.Write([]byte(msg))
			if err != nil {
				kw.GetWorkceptor().nc.GetLogger().Error("Error writing remaining logs for pod %s/%s: %s", podNamespace, podName, err)
				return err
			}
			logsRetrieved = true
		}
	}

	if !logsRetrieved {
		kw.GetWorkceptor().nc.GetLogger().Info("No new logs retrieved for pod %s/%s", podNamespace, podName)
	}

	return nil
}

func (kw *KubeUnit) isPodTerminated() (bool, error) {
	podNamespace := kw.pod.Namespace
	podName := kw.pod.Name

	// Get the latest pod status
	pod, err := KubeAPIWrapperInstance.Get(kw.GetContext(), kw.clientset, podNamespace, podName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	// Check if the pod has terminated
	if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		return true, nil
	}

	return false, nil
}

func (kw *KubeUnit) runWorkUsingLogger() {
	skipStdin := true

	status := kw.Status()
	ked := status.ExtraData.(*KubeExtraData)

	podName := ked.PodName
	podNamespace := ked.KubeNamespace

	if podName == "" {
		// create new pod if ked.PodName is empty
		// TODO: add retry logic to make this more resilient to transient errors
		if err := kw.createPod(nil); err != nil {
			if err != ErrPodCompleted {
				errMsg := fmt.Sprintf("Error creating pod: %s", err)
				kw.GetWorkceptor().nc.GetLogger().Error(errMsg) //nolint:govet
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
			kw.GetWorkceptor().nc.GetLogger().Error(errMsg) //nolint:govet
			kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)

			return
		}

		// resuming from a previously created pod
		var err error
		for retries := 5; retries > 0; retries-- {
			// check if the kw.ctx is already cancel
			select {
			case <-kw.GetContext().Done():
				errMsg := fmt.Sprintf("Context Done while getting pod %s/%s. Error: %s", podNamespace, podName, kw.GetContext().Err())
				kw.GetWorkceptor().nc.GetLogger().Warning(errMsg) //nolint:govet

				return
			default:
			}

			kw.pod, err = KubeAPIWrapperInstance.Get(kw.GetContext(), kw.clientset, podNamespace, podName, metav1.GetOptions{})
			if err == nil {
				break
			}
			kw.GetWorkceptor().nc.GetLogger().Warning(
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
			kw.GetWorkceptor().nc.GetLogger().Error(errMsg) //nolint:govet
			kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)

			return
		}
	}

	// Attach stdin stream to the pod
	var exec remotecommand.Executor
	if !skipStdin {
		req := KubeAPIWrapperInstance.SubResource(kw.clientset, podName, podNamespace)

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
		exec, err = KubeAPIWrapperInstance.NewSPDYExecutor(kw.config, "POST", req.URL())
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
				kw.GetWorkceptor().nc.GetLogger().Error(errMsg) //nolint:govet
				kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)

				return
			}
		} else {
			// goroutine to cancel stdin reader
			go func() {
				select {
				case <-kw.GetContext().Done():
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
		kw.GetWorkceptor().nc.GetLogger().Error(errMsg) //nolint:govet
		kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)

		return
	}

	// goroutine to cancel stdout stream
	go func() {
		select {
		case <-kw.GetContext().Done():
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
				err = KubeAPIWrapperInstance.StreamWithContext(kw.GetContext(), exec, remotecommand.StreamOptions{
					Stdin: stdin,
					Tty:   false,
				})
				if err != nil {
					// NOTE: io.EOF for stdin is handled by remotecommand and will not trigger this
					kw.GetWorkceptor().nc.GetLogger().Warning(
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
				kw.GetWorkceptor().nc.GetLogger().Error(errMsg) //nolint:govet
				kw.UpdateBasicStatus(WorkStateFailed, errMsg, stdout.Size())

				close(stdinErrChan) // signal STDOUT goroutine to stop
			} else {
				if stdin.Error() == io.EOF {
					kw.UpdateBasicStatus(WorkStateRunning, "Pod Running", stdout.Size())
				} else {
					// this is probably not possible...
					errMsg := fmt.Sprintf("Error reading stdin: %s", stdin.Error())
					kw.GetWorkceptor().nc.GetLogger().Error(errMsg) //nolint:govet
					kw.GetWorkceptor().nc.GetLogger().Error("Pod status at time of error %s", kw.pod.Status.String())
					kw.UpdateBasicStatus(WorkStateFailed, errMsg, stdout.Size())

					close(stdinErrChan) // signal STDOUT goroutine to stop
				}
			}
		}()
	}

	stdoutWithReconnect := ShouldUseReconnect(kw)
	if stdoutWithReconnect && stdoutErr == nil {
		kw.GetWorkceptor().nc.GetLogger().Debug("streaming stdout with reconnect support")
		go kw.kubeLoggingWithReconnect(&streamWait, stdout, &stdinErr, &stdoutErr)
	} else {
		kw.GetWorkceptor().nc.GetLogger().Debug("streaming stdout with no reconnect support")
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

		if kw.GetContext().Err() != context.Canceled {
			kw.UpdateBasicStatus(WorkStateFailed, errDetail, stdout.Size())
		}

		return
	}

	// only transition from WorkStateRunning to WorkStateSucceeded if WorkStateFailed is set we do not override
	if kw.GetContext().Err() != context.Canceled && kw.Status().State == WorkStateRunning {
		kw.UpdateBasicStatus(WorkStateSucceeded, "Finished", stdout.Size())
	}
}

func IsCompatibleK8S(kw *KubeUnit, versionStr string) bool {
	semver, err := version.ParseSemantic(versionStr)
	if err != nil {
		kw.GetWorkceptor().nc.GetLogger().Warning("could parse Kubernetes server version %s, will not use reconnect support", versionStr)

		return false
	}

	// ignore pre-release in version comparison
	semver = semver.WithPreRelease("")

	// The patch was backported to minor version 23, 24 and 25
	// We check z stream based on the minor version
	// if minor versions < 23, set to high value (e.g. v1.22.9999)
	// if minor versions == 23, compare with v1.23.14
	// if minor version == 24, compare with v1.24.8
	// if minor version == 25, compare with v1.25.4
	// if minor versions > 23, compare with low value (e.g. v1.26.0)
	var compatibleVer string
	switch {
	case semver.Minor() == 23:
		compatibleVer = "v1.23.14"
	case semver.Minor() == 24:
		compatibleVer = "v1.24.8"
	case semver.Minor() == 25:
		compatibleVer = "v1.25.4"
	case semver.Minor() > 25:
		compatibleVer = fmt.Sprintf("%d.%d.0", semver.Major(), semver.Minor())
	default:
		compatibleVer = fmt.Sprintf("%d.%d.9999", semver.Major(), semver.Minor())
	}

	if semver.AtLeast(version.MustParseSemantic(compatibleVer)) {
		kw.GetWorkceptor().nc.GetLogger().Debug("Kubernetes version %s is at least %s, using reconnect support", semver, compatibleVer)

		return true
	}

	kw.GetWorkceptor().nc.GetLogger().Debug("Kubernetes version %s not at least %s, not using reconnect support", semver, compatibleVer)

	return false
}

func ShouldUseReconnect(kw *KubeUnit) bool {
	// Support for streaming from pod with timestamps using reconnect method is in all current versions
	// Can override the detection by setting the RECEPTOR_KUBE_SUPPORT_RECONNECT
	// accepted values: "enabled", "disabled", "auto".  The default is "enabled"
	// all invalid values will assume to be "disabled"

	version := viper.GetInt("version")
	var env string
	ok := false
	switch version {
	case 2:
		env = viper.GetString("node.ReceptorKubeSupportReconnect")
		if env != "" {
			ok = true
		}
	default:
		env, ok = os.LookupEnv("RECEPTOR_KUBE_SUPPORT_RECONNECT")
	}
	if ok {
		switch env {
		case "enabled":
			return true
		case "disabled":
			return false
		case "auto":
			return true
		default:
			return false
		}
	}

	serverVerInfo, err := kw.clientset.ServerVersion()
	if err != nil {
		kw.GetWorkceptor().nc.GetLogger().Warning("could not detect Kubernetes server version, will not use reconnect support")

		return false
	}

	return IsCompatibleK8S(kw, serverVerInfo.String())
}

func ParseTime(s string) *time.Time {
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

func (kw *KubeUnit) runWorkUsingTCP() {
	// Create local cancellable context
	ctx, cancel := kw.GetContext(), kw.GetCancel()
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
		kw.GetWorkceptor().nc.GetLogger().Error(errMsg) //nolint:govet

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
			kw.GetWorkceptor().nc.GetLogger().Error(errMsg) //nolint:govet
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
		kw.GetWorkceptor().nc.GetLogger().Error(errMsg) //nolint:govet
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
		kw.GetWorkceptor().nc.GetLogger().Error(errMsg) //nolint:govet
		kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)
		cancel()

		return
	}

	// Open stdout writer
	stdout, err := NewStdoutWriter(FileSystem{}, kw.UnitDir())
	if err != nil {
		errMsg := fmt.Sprintf("Error opening stdout file: %s", err)
		kw.GetWorkceptor().nc.GetLogger().Error(errMsg) //nolint:govet
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
			kw.GetWorkceptor().nc.GetLogger().Error(errMsg) //nolint:govet
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
		kw.GetWorkceptor().nc.GetLogger().Error(errMsg) //nolint:govet
		kw.UpdateBasicStatus(WorkStateFailed, errMsg, 0)
		cancel()

		return
	}

	if ctx.Err() == nil {
		kw.UpdateBasicStatus(WorkStateSucceeded, "Finished", stdout.Size())
	}
}

func (kw *KubeUnit) connectUsingKubeconfig() error {
	var err error
	ked := kw.UnredactedStatus().ExtraData.(*KubeExtraData)
	if ked.KubeConfig == "" {
		clr := KubeAPIWrapperInstance.NewDefaultClientConfigLoadingRules()
		kw.config, err = KubeAPIWrapperInstance.BuildConfigFromFlags("", clr.GetDefaultFilename())
		if ked.KubeNamespace == "" {
			c, err := clr.Load()
			if err != nil {
				return err
			}
			curContext, ok := c.Contexts[c.CurrentContext]
			if ok && curContext != nil {
				kw.UpdateFullStatus(func(sfd *StatusFileData) {
					sfd.ExtraData.(*KubeExtraData).KubeNamespace = curContext.Namespace
				})
			} else {
				return fmt.Errorf("could not determine namespace")
			}
		}
	} else {
		cfg, err := KubeAPIWrapperInstance.NewClientConfigFromBytes([]byte(ked.KubeConfig))
		if err != nil {
			return err
		}
		if ked.KubeNamespace == "" {
			namespace, _, err := cfg.Namespace()
			if err != nil {
				return err
			}
			kw.UpdateFullStatus(func(sfd *StatusFileData) {
				sfd.ExtraData.(*KubeExtraData).KubeNamespace = namespace
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

func (kw *KubeUnit) connectUsingIncluster() error {
	var err error
	kw.config, err = KubeAPIWrapperInstance.InClusterConfig()
	if err != nil {
		return err
	}

	return nil
}

func (kw *KubeUnit) connectToKube() error {
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
	version := viper.GetInt("version")
	var envQPS string
	ok := false
	switch version {
	case 2:
		envQPS = viper.GetString("node.ReceptorKubeClientsetQPS")
		if envQPS != "" {
			ok = true
		}
	default:
		envQPS, ok = os.LookupEnv("RECEPTOR_KUBE_CLIENTSET_QPS")
	}
	if ok {
		qps, err := strconv.Atoi(envQPS)
		if err != nil {
			// ignore error, use default
			kw.GetWorkceptor().nc.GetLogger().Warning("Invalid value for RECEPTOR_KUBE_CLIENTSET_QPS: %s. Ignoring", envQPS)
		} else {
			kw.config.QPS = float32(qps)
			kw.config.Burst = qps * 10
		}
	}

	kw.GetWorkceptor().nc.GetLogger().Debug("RECEPTOR_KUBE_CLIENTSET_QPS: %s", envQPS)

	// RECEPTOR_KUBE_CLIENTSET_BURST
	// default: 10 x QPS
	var envBurst string
	switch version {
	case 2:
		envBurst = viper.GetString("node.ReceptorKubeClientsetBurst")
		if envBurst != "" {
			ok = true
		}
	default:
		envBurst, ok = os.LookupEnv("RECEPTOR_KUBE_CLIENTSET_BURST")
	}
	if ok {
		burst, err := strconv.Atoi(envBurst)
		if err != nil {
			kw.GetWorkceptor().nc.GetLogger().Warning("Invalid value for RECEPTOR_KUBE_CLIENTSET_BURST: %s. Ignoring", envQPS)
		} else {
			kw.config.Burst = burst
		}
	}

	kw.GetWorkceptor().nc.GetLogger().Debug("RECEPTOR_KUBE_CLIENTSET_BURST: %s", envBurst)

	kw.GetWorkceptor().nc.GetLogger().Debug("Initializing Kubernetes clientset")
	// RECEPTOR_KUBE_CLIENTSET_RATE_LIMITER
	// default: tokenbucket
	// options: never, always, tokenbucket
	var envRateLimiter string
	switch version {
	case 2:
		envRateLimiter = viper.GetString("node.ReceptorKubeClientsetRateLimiter")
		if envRateLimiter != "" {
			ok = true
		}
	default:
		envRateLimiter, ok = os.LookupEnv("RECEPTOR_KUBE_CLIENTSET_RATE_LIMITER")
	}
	if ok {
		switch envRateLimiter {
		case "never":
			kw.config.RateLimiter = KubeAPIWrapperInstance.NewFakeNeverRateLimiter()
		case "always":
			kw.config.RateLimiter = KubeAPIWrapperInstance.NewFakeAlwaysRateLimiter()
		default:
		}
		kw.GetWorkceptor().nc.GetLogger().Debug("RateLimiter: %s", envRateLimiter)
	}

	kw.GetWorkceptor().nc.GetLogger().Debug("QPS: %f, Burst: %d", kw.config.QPS, kw.config.Burst)
	kw.clientset, err = KubeAPIWrapperInstance.NewForConfig(kw.config)
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
func (kw *KubeUnit) SetFromParams(params map[string]string) error {
	ked := kw.GetStatusCopy().ExtraData.(*KubeExtraData)
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
			kw.GetWorkceptor().nc.GetLogger().Error("Failed to parse pod_pending_timeout -- valid examples include '1.5h', '30m', '30m10s'")

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
func (kw *KubeUnit) Status() *StatusFileData {
	status := kw.UnredactedStatus()
	ed, ok := status.ExtraData.(*KubeExtraData)
	if ok {
		ed.KubeConfig = ""
		ed.KubePod = ""
	}

	return status
}

// Status returns a copy of the status currently loaded in memory.
func (kw *KubeUnit) UnredactedStatus() *StatusFileData {
	kw.GetStatusLock().RLock()
	defer kw.GetStatusLock().RUnlock()
	status := kw.GetStatusWithoutExtraData()
	ked, ok := kw.GetStatusCopy().ExtraData.(*KubeExtraData)
	if ok {
		kedCopy := *ked
		status.ExtraData = &kedCopy
	}

	return status
}

// startOrRestart is a shared implementation of Start() and Restart().
func (kw *KubeUnit) startOrRestart() error {
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
	go kw.MonitorLocalStatus()

	return nil
}

// Restart resumes monitoring a job after a Receptor restart.
func (kw *KubeUnit) Restart() error {
	status := kw.Status()
	ked := status.ExtraData.(*KubeExtraData)
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
			kw.GetWorkceptor().nc.GetLogger().Warning("Pod %s could not be deleted: %s", ked.PodName, err.Error())
		} else {
			err := KubeAPIWrapperInstance.Delete(context.Background(), kw.clientset, ked.KubeNamespace, ked.PodName, metav1.DeleteOptions{})
			if err != nil {
				kw.GetWorkceptor().nc.GetLogger().Warning("Pod %s could not be deleted: %s", ked.PodName, err.Error())
			}
		}
	}
	if isTCP {
		return fmt.Errorf("restart not implemented for streammethod tcp")
	}

	return fmt.Errorf("work unit is not in running state, cannot be restarted")
}

// Start launches a job with given parameters.
func (kw *KubeUnit) Start() error {
	kw.UpdateBasicStatus(WorkStatePending, "Connecting to Kubernetes", 0)

	return kw.startOrRestart()
}

// Cancel releases resources associated with a job, including cancelling it if running.
func (kw *KubeUnit) Cancel() error {
	kw.CancelContext()
	kw.UpdateBasicStatus(WorkStateCanceled, "Canceled", -1)
	if kw.pod != nil {
		err := KubeAPIWrapperInstance.Delete(context.Background(), kw.clientset, kw.pod.Namespace, kw.pod.Name, metav1.DeleteOptions{})
		if err != nil {
			kw.GetWorkceptor().nc.GetLogger().Error("Error deleting pod %s: %s", kw.pod.Name, err)
		}
	}
	if kw.GetCancel() != nil {
		kw.CancelContext()
	}

	return nil
}

// Release releases resources associated with a job.  Implies Cancel.
func (kw *KubeUnit) Release(force bool) error {
	err := kw.Cancel()
	if err != nil && !force {
		return err
	}

	return kw.BaseWorkUnitForWorkUnit.Release(force)
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
func (cfg KubeWorkerCfg) NewWorker(bwu BaseWorkUnitForWorkUnit, w *Workceptor, unitID string, workType string) WorkUnit {
	return cfg.NewkubeWorker(bwu, w, unitID, workType, nil)
}

func (cfg KubeWorkerCfg) NewkubeWorker(bwu BaseWorkUnitForWorkUnit, w *Workceptor, unitID string, workType string, kawi KubeAPIer) WorkUnit {
	if bwu == nil {
		bwu = &BaseWorkUnit{
			status: StatusFileData{
				ExtraData: &KubeExtraData{
					Image:         cfg.Image,
					Command:       cfg.Command,
					KubeNamespace: cfg.Namespace,
					KubePod:       cfg.Pod,
					KubeConfig:    cfg.KubeConfig,
				},
			},
		}
	}

	KubeAPIWrapperLock = &sync.RWMutex{}
	KubeAPIWrapperLock.Lock()
	KubeAPIWrapperInstance = KubeAPIWrapper{}
	if kawi != nil {
		KubeAPIWrapperInstance = kawi
	}
	KubeAPIWrapperLock.Unlock()

	ku := &KubeUnit{
		BaseWorkUnitForWorkUnit: bwu,
		authMethod:              strings.ToLower(cfg.AuthMethod),
		streamMethod:            strings.ToLower(cfg.StreamMethod),
		baseParams:              cfg.Params,
		allowRuntimeAuth:        cfg.AllowRuntimeAuth,
		allowRuntimeCommand:     cfg.AllowRuntimeCommand,
		allowRuntimeParams:      cfg.AllowRuntimeParams,
		allowRuntimePod:         cfg.AllowRuntimePod,
		deletePodOnRestart:      cfg.DeletePodOnRestart,
		namePrefix:              fmt.Sprintf("%s-", strings.ToLower(cfg.WorkType)),
	}
	ku.BaseWorkUnitForWorkUnit.Init(w, unitID, workType, FileSystem{}, nil)

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
	version := viper.GetInt("version")
	if version > 1 {
		return
	}
	cmdline.RegisterConfigTypeForApp("receptor-workers",
		"work-kubernetes", "Run a worker using Kubernetes", KubeWorkerCfg{}, cmdline.Section(workersSection))
}
