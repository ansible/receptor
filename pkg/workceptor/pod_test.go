package workceptor_test

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeapi "k8s.io/client-go/kubernetes/fake"
)

var podSuccess = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "test-pod",
	},
	Status: corev1.PodStatus{
		Phase: corev1.PodSucceeded,
		ContainerStatuses: []corev1.ContainerStatus{
			{
				Name: "worker",
				State: corev1.ContainerState{
					Waiting: nil,
					Running: nil,
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 0,
						Reason:   "Success",
					},
				},
			},
		},
	},
}

var podInfraError = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "infra-error-pod",
	},
	Status: corev1.PodStatus{
		Phase:  corev1.PodFailed,
		Reason: "OOMKilled",
		ContainerStatuses: []corev1.ContainerStatus{
			{
				Name: "worker",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 137,
						Reason:   "OOMKill",
					},
				},
			},
		},
	},
}

var podPending = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "pending-pod",
	},
	Status: corev1.PodStatus{
		Phase: corev1.PodPending,
		ContainerStatuses: []corev1.ContainerStatus{
			{
				Name: "worker",
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{
						Reason:  "ContainerCreating",
						Message: "Container is being created",
					},
				},
			},
		},
	},
}

func TestGetPodStatus(t *testing.T) {
	kw, err := startNetceptorNodeWithWorkceptor()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name         string
		pod          *corev1.Pod
		wantOk       bool
		wantErr      bool
		wantErrorMsg string
	}{
		{
			name:         "nil pod",
			pod:          nil,
			wantOk:       false,
			wantErr:      true,
			wantErrorMsg: "pod is nil",
		},
		{
			name:         "pod error",
			pod:          podInfraError,
			wantOk:       false,
			wantErr:      true,
			wantErrorMsg: "container worker exited with code 137: OOMKill",
		},
		{
			name:    "success case",
			pod:     podSuccess,
			wantOk:  true,
			wantErr: false,
		},
		{
			name:    "pending pod",
			pod:     podPending,
			wantOk:  true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := kw.GetPodStatus(tt.pod)
			if ok != tt.wantOk || (err != nil) != tt.wantErr {
				t.Errorf("Failed %s case: ok=%v wantok=%v err=%v", tt.name, ok, tt.wantOk, err)
			}
			if err != nil && tt.wantErr == false {
				t.Errorf("Expected error message got '%s'", err.Error())
			}
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error message '%s', got nil error", tt.wantErrorMsg)
				} else if !strings.Contains(err.Error(), tt.wantErrorMsg) {
					t.Errorf("Expected error message '%s', got '%s'", tt.wantErrorMsg, err.Error())
				}
			}
			if tt.wantErrorMsg == "" && err != nil {
				t.Errorf("Unexpected error for %s case: %v", tt.name, err)
			}
		})
	}
}

func TestWaitForPodCompleted(t *testing.T) {
	kw, err := startNetceptorNodeWithWorkceptor()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name            string
		initialPod      *corev1.Pod
		updatePhase     corev1.PodPhase
		wantErr         bool
		wantErrorString string
	}{
		{
			name:            "nil pod",
			initialPod:      nil,
			wantErr:         true,
			wantErrorString: "pod is nil",
		},
		{
			name:        "pending to success",
			initialPod:  podPending,
			updatePhase: corev1.PodSucceeded,
			wantErr:     false,
		},
		{
			name:        "pending to failed",
			initialPod:  podPending,
			updatePhase: corev1.PodFailed,
			wantErr:     false,
		},
		{
			name:        "pending to running",
			initialPod:  podPending,
			updatePhase: corev1.PodRunning,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			timeoutSeconds := int64(2)

			var clientset *fakeapi.Clientset
			if tt.initialPod != nil {
				clientset = fakeapi.NewSimpleClientset(tt.initialPod)
			} else {
				clientset = fakeapi.NewSimpleClientset(&corev1.Pod{})
			}

			if tt.updatePhase != "" && tt.initialPod != nil {
				go func() {
					updatedPod := tt.initialPod.DeepCopy()
					updatedPod.Status.Phase = tt.updatePhase
					_, _ = clientset.CoreV1().Pods("default").Update(context.Background(), updatedPod, metav1.UpdateOptions{})
				}()
			}

			_, err := kw.WaitForPodCompleted(ctx, tt.initialPod, clientset, &timeoutSeconds)
			if (err != nil) != tt.wantErr {
				t.Errorf("WaitForPodCompleted() error = %v, wantErr %v", err, tt.wantErr)
				if tt.wantErrorString != "" && err != nil && !strings.Contains(err.Error(), tt.wantErrorString) {
					t.Errorf("Expected error message '%s', got '%s'", tt.wantErrorString, err.Error())
				}
			}
			if tt.wantErr && err != nil {
				if !strings.Contains(err.Error(), tt.wantErrorString) {
					t.Errorf("Expected error message '%s', got '%s'", tt.wantErrorString, err.Error())
				}
			}
		})
	}
}
