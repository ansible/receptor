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
	PodApplicationSuccess(pod *corev1.Pod, containerName string) (bool, string, error)
	PodInfrastructureSuccess(pod *corev1.Pod, containerName string) (bool, string, error)
	WaitForPodCompleted(pod *corev1.Pod, clientset kubernetes.Interface, timeoutSeconds *int64) (*corev1.Pod, error)
}

func (kw *KubeUnit) CapturePodStatus(pod *corev1.Pod, stdoutSize int64, timeoutSeconds *int64) (ok bool, err error) {
	if pod == nil {
		return false, fmt.Errorf("pod is nil")
	}

	if pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodPending {
		pod, err = kw.WaitForPodCompleted(kw.GetContext(), pod, kw.clientset, timeoutSeconds)
		if err != nil {
			kw.GetWorkceptor().nc.GetLogger().Debug("Job complete and pod is still running: %v", err)
		}
	}

	ok, reason, err := kw.GetPodStatus(pod)
	if err != nil {
		reasonDetail := fmt.Sprintf("Pod diagnostics failed: %v reason %s", err, reason)
		kw.GetWorkceptor().nc.GetLogger().Warning("%s", reasonDetail)

		return false, err

	} else if !ok {
		kw.GetWorkceptor().nc.GetLogger().Warning("Pod did not succeed: %s", reason)
		kw.UpdateBasicStatus(WorkStateFailed, reason, stdoutSize)

		return false, fmt.Errorf("pod did not succeed: %s", reason)
	}
	kw.GetWorkceptor().nc.GetLogger().Debug("Pod status captured: %s", pod.Status.String())

	return true, nil
}

// GetPodStatus checks if the pod has successfully completed its application logic and infrastructure is healthy.
func (ku KubeUnit) GetPodStatus(pod *corev1.Pod) (bool, string, error) {
	if pod == nil {
		return false, "pod is nil", fmt.Errorf("pod is nil")
	}

	podRef := fmt.Sprintf("pod %s/%s", pod.Namespace, pod.Name)

	infraOK, reason, err := ku.PodInfrastructureSuccess(pod, containerName)

	if !infraOK || err != nil {
		return infraOK, fmt.Sprintf("%s infrastructure %s", podRef, reason), err
	}

	appOK, reason, err := ku.PodApplicationSuccess(pod, containerName)
	if !appOK || err != nil {
		return appOK, fmt.Sprintf("%s application %s", podRef, reason), err
	}

	return true, "", nil
}

// PodInfrastructureSuccess checks if the pod has either successfully started, is pending or is running, or has is successfully terminated.
// Any other state is considered an infrastructure failure.
func (ku KubeUnit) PodInfrastructureSuccess(pod *corev1.Pod, containerName string) (bool, string, error) {
	for _, cs := range pod.Status.ContainerStatuses {
		// Check if this is the container we care about first
		if cs.Name == containerName {
			switch pod.Status.Phase {
			case corev1.PodFailed:
				if cs.State.Terminated != nil {
					switch cs.State.Terminated.ExitCode {
					case 0:
						break
					default:

						return false, fmt.Sprintf("pod reason %s container %s %s", pod.Status.Reason, cs.Name, cs.State.Terminated.Reason), nil
					}
				}

				return true, "", nil

			case corev1.PodSucceeded:

				return true, "", nil

			case corev1.PodRunning, corev1.PodPending:

				return true, fmt.Sprintf("pod phase: %s", pod.Status.Phase), nil

			default:

				return false, fmt.Sprintf("unknown phase: %s", pod.Status.Phase), fmt.Errorf("invalid pod phase")
			}
		}
	}

	return false, fmt.Sprintf("pod %s/%s does not contain container %s", pod.Namespace, pod.Name, containerName), fmt.Errorf("pod does not contain container")
}

// PodApplicationSuccess checks if the pod has successfully completed its application logic.
// this is called after podInfrastructureSuccess has confirmed the pod is in a terminal state.
func (ku KubeUnit) PodApplicationSuccess(pod *corev1.Pod, containerName string) (bool, string, error) {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name == containerName {
			if cs.State.Terminated == nil { // means it is waiting or running, so application logic has not completed yet. Normal behavior when job completes successfully.

				return true, "container has not terminated", nil
			}

			if cs.State.Terminated.ExitCode != 0 { // exit code of 0 means success

				return false, fmt.Sprintf("container %s exited with code %d: %s", cs.Name, cs.State.Terminated.ExitCode, cs.State.Terminated.Reason), nil
			}

			return true, "", nil // container terminated with exit code of 0
		}
	}

	return false, fmt.Sprintf("pod %s/%s does not contain container %s", pod.Namespace, pod.Name, containerName), fmt.Errorf("pod does not contain container")
}

func (ku KubeUnit) WaitForPodCompleted(ctx context.Context, pod *corev1.Pod, clientset kubernetes.Interface, timeoutSeconds *int64) (*corev1.Pod, error) {
	if pod == nil {
		return nil, fmt.Errorf("pod is nil")
	}

	watcher, err := clientset.CoreV1().Pods(pod.Namespace).Watch(ctx, metav1.ListOptions{
		TimeoutSeconds: timeoutSeconds,
		FieldSelector:  "involvedObject.kind=Pod,involvedObject.name=" + pod.Name})
	if err != nil {
		return pod, err
	}

	for event := range watcher.ResultChan() {
		switch event.Type {
		case watch.Error:
			return pod, event.Object.(error)
		default:
			pod = event.Object.(*corev1.Pod)
			fmt.Printf("%s: %s/%s (Phase: %s)\n", event.Type, pod.Namespace, pod.Name, pod.Status.Phase)
			return pod, nil
		}
	}

	return pod, nil
}
