package workceptor_test

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

var podInfraErrorWithMessage = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "infra-error-pod",
	},
	Status: corev1.PodStatus{
		Phase:   corev1.PodFailed,
		Reason:  "Pod OOMKilled",
		Message: "The pod was killed because it ran out of memory",
		ContainerStatuses: []corev1.ContainerStatus{
			{
				Name: "worker",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 137,
						Reason:   "Container OOMKill",
					},
				},
			},
		},
	},
}

var podAppError = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "app-error-pod",
	},
	Status: corev1.PodStatus{
		Phase:  corev1.PodFailed,
		Reason: "Error",
		ContainerStatuses: []corev1.ContainerStatus{
			{
				Name: "worker",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 1,
						Reason:   "Error",
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

var podUnknownPhase = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "unknown-phase-pod",
	},
	Status: corev1.PodStatus{
		Phase: "NotARealPhase",
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

var podMultipleContainers = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "multi-container-pod",
	},
	Status: corev1.PodStatus{
		Phase: corev1.PodRunning,
		ContainerStatuses: []corev1.ContainerStatus{
			{
				Name: "worker",
				State: corev1.ContainerState{
					Running: &corev1.ContainerStateRunning{
						StartedAt: metav1.Now(),
					},
				},
				Ready: true,
			},
			{
				Name: "helper",
				State: corev1.ContainerState{
					Running: &corev1.ContainerStateRunning{
						StartedAt: metav1.Now(),
					},
				},
				Ready: true,
			},
		},
	},
}

var podImagePullBackOff = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "pod-image-pull-backoff",
	},
	Status: corev1.PodStatus{
		Phase: corev1.PodPending,
		ContainerStatuses: []corev1.ContainerStatus{
			{
				Name: "worker",
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{
						Reason:  "ImagePullBackOff",
						Message: "Back-off pulling image",
					},
				},
			},
		},
	},
}

var podCreateContainerConfigError = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "pod-Create-Container-ConfigErrorf",
	},
	Status: corev1.PodStatus{
		Phase: corev1.PodPending,
		ContainerStatuses: []corev1.ContainerStatus{
			{
				Name: "worker",
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{
						Reason:  "CreateContainerConfigError",
						Message: "Create container error",
					},
				},
			},
		},
	},
}

var podErrImagePull = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "pod-error-image-pull",
	},
	Status: corev1.PodStatus{
		Phase: corev1.PodPending,
		ContainerStatuses: []corev1.ContainerStatus{
			{
				Name: "worker",
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{
						Reason:  "ErrImagePull",
						Message: "Error when pulling image",
					},
				},
			},
		},
	},
}

var podCrashLoopBackOff = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "pod-crash-loop-backoff",
	},
	Status: corev1.PodStatus{
		Phase: corev1.PodPending,
		ContainerStatuses: []corev1.ContainerStatus{
			{
				Name: "worker",
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{
						Reason:  "CrashLoopBackOff",
						Message: "Error when starting image",
					},
				},
			},
		},
	},
}

var podCreating = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "pod-creating",
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

func TestPodHeathy(t *testing.T) {
	kw, err := startNetceptorNodeWithWorkceptor()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		pod       *corev1.Pod
		container string
		wantOk    bool
		wantErr   bool
		wantError string
	}{
		{
			name:      "pod err image pull",
			pod:       podErrImagePull,
			container: "worker",
			wantOk:    false,
			wantErr:   true,
			wantError: "container worker in error state: ErrImagePull Error when pulling image",
		},
		{
			name:      "backoff image backoff pod",
			pod:       podImagePullBackOff,
			container: "worker",
			wantOk:    false,
			wantErr:   true,
			wantError: "container worker in error state: ImagePullBackOff Back-off pulling image",
		},
		{
			name:      "Create Container Config Error pod",
			pod:       podCreateContainerConfigError,
			container: "worker",
			wantOk:    false,
			wantErr:   true,
			wantError: "container worker in error state: CreateContainerConfigError Create container error",
		},
		{
			name:      "crash loop backoff pod",
			pod:       podCrashLoopBackOff,
			container: "worker",
			wantOk:    false,
			wantErr:   true,
			wantError: "container worker in error state: CrashLoopBackOff Error when starting image",
		},
		{
			name:      "pod creating",
			pod:       podCreating,
			container: "worker",
			wantOk:    false,
			wantErr:   false,
		},
		{
			name:      "nil pod",
			pod:       nil,
			container: "worker",
			wantOk:    false,
			wantErr:   true,
			wantError: "pod is nil",
		},
		{
			name:      "pod not terminated",
			pod:       podPending,
			container: "worker",
			wantOk:    false,
			wantErr:   false,
		},
		{
			name:      "container missing",
			pod:       &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "no-container-pod", Namespace: "default"}},
			container: "worker",
			wantOk:    false,
			wantErr:   true,
			wantError: "pod does not contain container worker",
		},
		{
			name:      "container healthy",
			pod:       podSuccess,
			container: "worker",
			wantOk:    true,
			wantErr:   false,
		},
		{
			name:      "pod unknown phase",
			pod:       podUnknownPhase,
			container: "worker",
			wantOk:    false,
			wantErr:   true,
			wantError: "unknown phase: NotARealPhase",
		},
		{
			name:      "pod with multiple containers",
			pod:       podMultipleContainers,
			container: "worker",
			wantOk:    true,
			wantErr:   false,
		},
		{
			name:      "pod with multiple containers, different container",
			pod:       podMultipleContainers,
			container: "helper",
			wantOk:    true,
			wantErr:   false,
		},
		{
			name:      "pod with oomkill error",
			pod:       podInfraError,
			container: "worker",
			wantOk:    false,
			wantErr:   true,
			wantError: "pod failed with reason: OOMKilled container worker failed with exit code 137: OOMKill",
		},
		{
			name:      "pod with application error",
			pod:       podAppError,
			container: "worker",
			wantOk:    false,
			wantErr:   true,
			wantError: "pod failed with reason: Error container worker failed with exit code 1: Error ",
		},
		{
			name:      "pod with oomkill error and message",
			pod:       podInfraErrorWithMessage,
			container: "worker",
			wantOk:    false,
			wantErr:   true,
			wantError: "pod failed with reason: Pod OOMKilled message: The pod was killed because it ran out of memory container worker failed with exit code 137: Container OOMKill ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := kw.PodHealthy(tt.pod, tt.container)
			if ok != tt.wantOk || (err != nil) != tt.wantErr {
				t.Errorf("Failed %s case: ok=%v wantok=%v err=%v", tt.name, ok, tt.wantOk, err)
			}
			if err != nil && tt.wantErr == false {
				t.Errorf("Expected error message got '%s'", err.Error())
			}
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error message '%s', got nil error", tt.wantError)
				} else if !strings.Contains(err.Error(), tt.wantError) {
					t.Errorf("Expected error message '%s', got '%s'", tt.wantError, err.Error())
				}
			}
			if tt.wantError == "" && err != nil {
				t.Errorf("Unexpected error for %s case: %v", tt.name, err)
			}
		})
	}
}

func TestPodContainerHealthy(t *testing.T) {
	kw, err := startNetceptorNodeWithWorkceptor()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		pod       *corev1.Pod
		container string
		wantOk    bool
		wantErr   bool
		wantError string
	}{
		{
			name:      "nil pod",
			pod:       nil,
			container: "worker",
			wantOk:    false,
			wantErr:   true,
			wantError: "pod is nil",
		},
		{
			name:      "pod not terminated",
			pod:       podPending,
			container: "worker",
			wantOk:    false,
			wantErr:   false,
		},
		{
			name:      "container missing",
			pod:       &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "no-container-pod", Namespace: "default"}},
			container: "worker",
			wantOk:    false,
			wantErr:   true,
			wantError: "pod does not contain container worker",
		},
		{
			name:      "container healthy",
			pod:       podSuccess,
			container: "worker",
			wantOk:    true,
			wantErr:   false,
		},
		{
			name:      "pod ImagePullBackOff",
			pod:       podImagePullBackOff,
			container: "worker",
			wantOk:    false,
			wantErr:   true,
			wantError: "container worker in error state: ImagePullBackOff Back-off pulling image",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := kw.PodContainerHealthy(tt.pod, tt.container)
			if ok != tt.wantOk || (err != nil) != tt.wantErr {
				t.Errorf("Failed %s case: ok=%v wantok=%v err=%v", tt.name, ok, tt.wantOk, err)
			}
			if err != nil && tt.wantErr == false {
				t.Errorf("Expected error message got '%s'", err.Error())
			}
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error message '%s', got nil error", tt.wantError)
				} else if !strings.Contains(err.Error(), tt.wantError) {
					t.Errorf("Expected error message '%s', got '%s'", tt.wantError, err.Error())
				}
			}
			if tt.wantError == "" && err != nil {
				t.Errorf("Unexpected error for %s case: %v", tt.name, err)
			}
		})
	}
}
