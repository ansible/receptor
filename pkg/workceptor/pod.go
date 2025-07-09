package workceptor

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

type KubePodStateHelper interface {
	PodHealthy(pod *corev1.Pod, containerName string) (bool, error)
	PodContainerHealthy(pod *corev1.Pod, containerName string) (bool, error)
	WaitForPodCompleted(ctx context.Context, pod *corev1.Pod, clientset kubernetes.Interface, timeoutSeconds *int64) (*corev1.Pod, error)
	GetPodStatus(ctx context.Context, pod *corev1.Pod, clientset kubernetes.Interface, containerName string, timeoutSeconds *int64) (bool, error)
}

// PodContainerHealthy checks if the pod has successfully completed its application logic.
// this is called after podInfrastructureSuccess has confirmed the pod is in a terminal state.
func (kw KubeUnit) PodContainerHealthy(pod *corev1.Pod, containerName string) (bool, error) {
	if pod == nil {
		return false, fmt.Errorf("pod is nil")
	}

	var foundContainer *corev1.ContainerStatus = nil

	for i, cs := range pod.Status.ContainerStatuses {
		if cs.Name == containerName {
			foundContainer = &pod.Status.ContainerStatuses[i]

			break
		}
	}
	if foundContainer == nil {
		return false, fmt.Errorf("pod does not contain container %s", containerName)
	}

	state := foundContainer.State

	// Check if container is running and ready
	if state.Running != nil {
		return foundContainer.Ready, nil // Use Ready field for health
	}

	// Check if container terminated successfully
	if state.Terminated != nil {
		if state.Terminated.ExitCode == 0 {
			return true, nil // Successfully completed
		}

		return false, fmt.Errorf("container %s failed with exit code %d: %s %s",
			containerName, state.Terminated.ExitCode, state.Terminated.Reason, state.Terminated.Message)
	}

	// Container is waiting - usually not healthy yet
	if state.Waiting != nil {
		// Check if it's a problematic waiting state
		reason := state.Waiting.Reason
		if reason == "ImagePullBackOff" || reason == "ErrImagePull" ||
			reason == "CrashLoopBackOff" || reason == "CreateContainerConfigError" {
			return false, fmt.Errorf("container %s in error state: %s %s", containerName, reason, state.Waiting.Message)
		}
		// Normal waiting states like "ContainerCreating", "PodInitializing"
		return false, nil // Not healthy yet, but not an error
	}

	return false, fmt.Errorf("container %s in unknown state: %v", containerName, state)
}

// PodHealthy checks if the pod and container are in a healthy state.WaitForPodCompleted.
func (kw KubeUnit) PodHealthy(pod *corev1.Pod, containerName string) (bool, error) {
	if pod == nil {
		return false, fmt.Errorf("pod is nil")
	}

	var containerDiag string = ""
	containerOk, containerError := kw.PodContainerHealthy(pod, containerName)
	if containerError != nil {
		containerDiag = fmt.Sprintf(" %v", containerError)
	}

	switch pod.Status.Phase {
	case corev1.PodFailed:
		podError := fmt.Errorf("pod failed with reason: %s", pod.Status.Reason)
		if pod.Status.Message != "" {
			podError = fmt.Errorf("%s message: %s", podError, pod.Status.Message)
		}

		return false, fmt.Errorf("%s%s", podError, containerDiag)

	case corev1.PodSucceeded, corev1.PodRunning, corev1.PodPending:
		return containerOk, containerError
	default:
		return false, fmt.Errorf("unknown phase: %s%s", pod.Status.Phase, containerDiag)
	}
}

func (kw KubeUnit) WaitForPodCompleted(ctx context.Context, pod *corev1.Pod, clientset kubernetes.Interface, timeoutSeconds *int64) (*corev1.Pod, error) {
	if pod == nil {
		return nil, fmt.Errorf("pod is nil")
	}

	originalPhase := pod.Status.Phase
	if originalPhase == corev1.PodSucceeded {
		return pod, nil // Pod already completed successfully
	}

	if originalPhase == corev1.PodFailed {
		return pod, fmt.Errorf("pod already failed with reason: %s %s", pod.Status.Reason, pod.Status.Message)
	}

	// Create a watcher for the pod
	watcher, err := clientset.CoreV1().Pods(pod.Namespace).Watch(ctx, metav1.ListOptions{
		TimeoutSeconds: timeoutSeconds,
		FieldSelector:  "metadata.name=" + pod.Name, // Use metadata.name instead of involvedObject
	})
	if err != nil {
		return pod, fmt.Errorf("failed to watch pod: %w", err)
	}
	defer watcher.Stop()

	for {
		select {
		case event, ok := <-watcher.ResultChan():
			if !ok {
				// The watcher channel was closed, which is the expected behavior on timeout.
				// Log this event and return the current pod state without an error.
				kw.GetWorkceptor().nc.GetLogger().Debug("Pod %s/%s phase %s timeout (no change)", pod.Namespace, pod.Name, pod.Status.Phase)

				return pod, nil
			}

			switch event.Type {
			case watch.Error:
				// Handle error events
				if err, ok := event.Object.(error); ok {
					kw.GetWorkceptor().nc.GetLogger().Debug("Pod %s/%s event %s phase %s (error)", pod.Namespace, pod.Name, event.Type, pod.Status.Phase)

					return pod, err
				}

				return pod, fmt.Errorf("received error event without error object")

			case watch.Added, watch.Modified:
				// Update the pod object
				updatedPod, ok := event.Object.(*corev1.Pod)
				if !ok {
					return pod, fmt.Errorf("unexpected object type: %T", event.Object)
				}
				pod = updatedPod

				// Check if the phase has changed
				if pod.Status.Phase != originalPhase {
					kw.GetWorkceptor().nc.GetLogger().Debug("Pod %s/%s phase changed from %s to %s", pod.Namespace, pod.Name, originalPhase, pod.Status.Phase)

					return pod, nil
				}

				kw.GetWorkceptor().nc.GetLogger().Debug("Pod %s/%s event %s phase %s (no change)", pod.Namespace, pod.Name, event.Type, pod.Status.Phase)

			default:
				// Handle other event types if necessary
				kw.GetWorkceptor().nc.GetLogger().Debug("Pod %s/%s received unexpected event type: %s", pod.Namespace, pod.Name, event.Type)
			}

		case <-ctx.Done():
			// Handle context cancellation
			kw.GetWorkceptor().nc.GetLogger().Debug("Pod %s/%s watch cancelled due to context: %s", pod.Namespace, pod.Name, ctx.Err())

			return pod, ctx.Err()
		}
	}
}

func (kw KubeUnit) GetPodStatus(ctx context.Context, pod *corev1.Pod, clientset kubernetes.Interface, containerName string, timeout_seconds int64) (bool, error) {
	pod, err := kw.WaitForPodCompleted(ctx, pod, clientset, &timeout_seconds)
	if err != nil {
		return false, err
	}

	return kw.PodHealthy(pod, containerName)
}
