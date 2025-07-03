package workceptor

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

type KubePodStateHelper interface {
	GetPodStatus(pod *corev1.Pod) (bool, error)
	PodHealthy(pod *corev1.Pod, containerName string) (bool, error)
	PodContainerHealthy(pod *corev1.Pod, containerName string) (bool, error)
}

// PodContainerHealthy checks if the pod has successfully completed its application logic.
// this is called after podInfrastructureSuccess has confirmed the pod is in a terminal state.
func (kw KubeUnit) PodContainerHealthy(pod *corev1.Pod, containerName string) (bool, error) {
	if pod == nil {
		return false, fmt.Errorf("pod is nil")
	}

	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name == containerName {
			if cs.State.Terminated == nil { // means it is waiting or running, so application logic has not completed yet. Normal behavior when job completes successfully.
				return true, nil
			}

			if cs.State.Terminated.ExitCode != 0 { // exit code of 0 means success
				return false, fmt.Errorf("container %s exited with code %d: %s", cs.Name, cs.State.Terminated.ExitCode, cs.State.Terminated.Reason)
			}

			return true, nil // container terminated with exit code of 0
		}
	}

	return false, fmt.Errorf("pod does not contain container %s", containerName)
}

// PodInfrastructureSuccess checks if the pod has either successfully started, is pending or is running, or has is successfully terminated.
// Any other state is considered an infrastructure failure.
func (kw KubeUnit) PodHealthy(pod *corev1.Pod, containerName string) (bool, error) {
	if pod == nil {
		return false, fmt.Errorf("pod is nil")
	}

	for _, cs := range pod.Status.ContainerStatuses {
		// Check if this is the container we care about first
		if cs.Name == containerName {
			switch pod.Status.Phase {
			case corev1.PodFailed:
				ok, err := kw.PodContainerHealthy(pod, containerName)
				if !ok || err != nil {
					return false, err
				}

				return true, nil
			case corev1.PodSucceeded:
				return true, nil
			case corev1.PodRunning, corev1.PodPending:
				return true, nil
			default:
				return false, fmt.Errorf("unknown phase: %s", pod.Status.Phase)
			}
		}
	}

	return false, fmt.Errorf("pod does not contain container %s", containerName)
}

// GetPodStatus checks if the pod has successfully completed its application logic and infrastructure is healthy.
func (kw KubeUnit) GetPodStatus(pod *corev1.Pod) (bool, error) {
	if pod == nil {
		return false, fmt.Errorf("pod is nil")
	}

	ok, err := kw.PodHealthy(pod, WorkerContainerName)
	if !ok || err != nil {
		return ok, err
	}

	return ok, nil
}
