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
	if foundContainer.State.Terminated == nil { // means it is waiting or running, so application logic has not completed yet. Normal behavior when job completes successfully.
		return true, nil
	}

	if foundContainer.State.Terminated.ExitCode != 0 { // exit code of 0 means success
		return false, fmt.Errorf("container %s exited with code %d: %s", containerName, foundContainer.State.Terminated.ExitCode, foundContainer.State.Terminated.Reason)
	}

	return true, nil // container terminated with exit code of 0
}

// PodInfrastructureSuccess checks if the pod has either successfully started, is pending or is running, or has is successfully terminated.
// Any other state is considered an infrastructure failure.
func (kw KubeUnit) PodHealthy(pod *corev1.Pod, containerName string) (bool, error) {
	if pod == nil {
		return false, fmt.Errorf("pod is nil")
	}

	containerDiag := fmt.Sprintf("container %s is healthy", containerName)
	containerOk, containerError := kw.PodContainerHealthy(pod, containerName)
	if containerError != nil {
		containerDiag = fmt.Sprintf("%v", containerError)
	}

	switch pod.Status.Phase {
	case corev1.PodFailed:
		podError := fmt.Errorf("pod failed with reason: %s", pod.Status.Reason)
		if pod.Status.Message != "" {
			podError = fmt.Errorf("%s message: %s", podError, pod.Status.Message)
		}

		return false, fmt.Errorf("%s %v", podError, containerDiag)

	case corev1.PodSucceeded, corev1.PodRunning, corev1.PodPending:
		return containerOk, containerError
	default:
		return false, fmt.Errorf("unknown phase: %s %s", pod.Status.Phase, containerDiag)
	}
}

func (kw KubeUnit) WaitForPodCompleted(ctx context.Context, pod *corev1.Pod, clientset kubernetes.Interface, timeoutSeconds *int64) (*corev1.Pod, error) {
	if pod == nil {
		return nil, fmt.Errorf("pod is nil")
	}

	originalPhase := pod.Status.Phase

	watcher, err := clientset.CoreV1().Pods(pod.Namespace).Watch(ctx, metav1.ListOptions{
		TimeoutSeconds: timeoutSeconds,
		FieldSelector:  "involvedObject.kind=Pod,involvedObject.name=" + pod.Name,
	})
	defer watcher.Stop()
	if err != nil {
		return pod, err
	}

	for event := range watcher.ResultChan() {
		switch event.Type {
		case watch.Error:
			kw.GetWorkceptor().nc.GetLogger().Debug("Pod %s/%s event %s phase %s (error)", pod.Namespace, pod.Name, event.Type, pod.Status.Phase)

			return pod, event.Object.(error)
		default:
			pod = event.Object.(*corev1.Pod)
			if pod.Status.Phase != originalPhase {
				kw.GetWorkceptor().nc.GetLogger().Debug("Pod %s/%s phase changed from %s to %s", pod.Namespace, pod.Name, originalPhase, pod.Status.Phase)

				return pod, nil
			}
			kw.GetWorkceptor().nc.GetLogger().Debug("Pod %s/%s event %s phase %s (no change)", pod.Namespace, pod.Name, event.Type, pod.Status.Phase)

			return pod, nil
		}
	}

	kw.GetWorkceptor().nc.GetLogger().Debug("Pod %s/%s phase %s timeout (no change)", pod.Namespace, pod.Name, pod.Status.Phase)

	return pod, nil
}
