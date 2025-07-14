package workceptor

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

type KubePodStateHelper interface {
	PodHealthy(pod *corev1.Pod, containerName string) (bool, error)
	PodContainerHealthy(pod *corev1.Pod, containerName string) (bool, error)
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

// PodHealthy checks if the pod and container are in a healthy state.
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
