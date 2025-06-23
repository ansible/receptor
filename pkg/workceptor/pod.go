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
	CapturePodStatus(pod *corev1.Pod, stdoutSize int64, timeoutSeconds *int64) (ok bool, err error)
	GetPodStatus(pod *corev1.Pod) (bool, string, error)
	PodContainerHealthy(pod *corev1.Pod, containerName string) (bool, error)
	PodHealthy(pod *corev1.Pod, containerName string) (bool, string, error)
	WaitForPodCompleted(pod *corev1.Pod, clientset kubernetes.Interface, timeoutSeconds *int64) (*corev1.Pod, error)
}

func (kw *KubeUnit) CapturePodStatus(pod *corev1.Pod, stdoutSize int64, timeoutSeconds *int64) (bool, error) {
	if pod == nil {
		return false, fmt.Errorf("pod is nil")
	}

	if pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodPending {
		err := kw.WaitForPodCompleted(kw.GetContext(), pod, kw.clientset, timeoutSeconds)
		if err != nil {
			kw.GetWorkceptor().nc.GetLogger().Debug("Pod error detected while waiting for completion: %v", err)
		}
	}

	ok, err := kw.GetPodStatus(pod)
	if !ok || err != nil {
		kw.GetWorkceptor().nc.GetLogger().Warning("Pod did not succeed:  %v", err)
		kw.UpdateBasicStatus(WorkStateFailed, err.Error(), stdoutSize)
		return false, err

	}

	kw.GetWorkceptor().nc.GetLogger().Debug("Pod status: %s", pod.Status.String())

	return true, nil
}

// GetPodStatus checks if the pod has successfully completed its application logic and infrastructure is healthy.
func (ku KubeUnit) GetPodStatus(pod *corev1.Pod) (bool, error) {
	if pod == nil {
		return false, fmt.Errorf("pod is nil")
	}

	podRef := fmt.Sprintf("pod %s/%s", pod.Namespace, pod.Name)

	ok, err := ku.PodHealthy(pod, containerName)
	if !ok || err != nil {
		return ok, fmt.Errorf("%s %s", podRef, err)
	}

	return ok, nil
}

// PodContainerHealthy checks if the pod has successfully completed its application logic.
// this is called after podInfrastructureSuccess has confirmed the pod is in a terminal state.
func (ku KubeUnit) PodContainerHealthy(pod *corev1.Pod, containerName string) (bool, error) {
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

	return false, fmt.Errorf("pod %s/%s does not contain container %s", pod.Namespace, pod.Name, containerName)
}

// PodInfrastructureSuccess checks if the pod has either successfully started, is pending or is running, or has is successfully terminated.
// Any other state is considered an infrastructure failure.
func (ku KubeUnit) PodHealthy(pod *corev1.Pod, containerName string) (bool, error) {
	if pod == nil {
		return false, fmt.Errorf("pod is nil")
	}

	for _, cs := range pod.Status.ContainerStatuses {
		// Check if this is the container we care about first
		if cs.Name == containerName {
			switch pod.Status.Phase {
			case corev1.PodFailed:
				ok, err := ku.PodContainerHealthy(pod, containerName)
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

	return false, fmt.Errorf("pod %s/%s does not contain container %s", pod.Namespace, pod.Name, containerName)
}

func (ku KubeUnit) WaitForPodCompleted(ctx context.Context, pod *corev1.Pod, clientset kubernetes.Interface, timeoutSeconds *int64) error {
	if pod == nil {
		return fmt.Errorf("pod is nil")
	}

	watcher, err := clientset.CoreV1().Pods(pod.Namespace).Watch(ctx, metav1.ListOptions{
		TimeoutSeconds: timeoutSeconds,
		FieldSelector:  "involvedObject.kind=Pod,involvedObject.name=" + pod.Name})
	if err != nil {
		return err
	}

	for event := range watcher.ResultChan() {
		switch event.Type {
		case watch.Error:
			return event.Object.(error)
		default:
			pod = event.Object.(*corev1.Pod)
			fmt.Printf("%s: %s/%s (Phase: %s)\n", event.Type, pod.Namespace, pod.Name, pod.Status.Phase)
			return nil
		}
	}

	return nil
}
