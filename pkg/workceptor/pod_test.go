package workceptor_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	testcore "k8s.io/client-go/testing"
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
			},
			{
				Name: "helper",
				State: corev1.ContainerState{
					Running: &corev1.ContainerStateRunning{
						StartedAt: metav1.Now(),
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
			wantOk:    true,
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
			wantError: "pod failed with reason: OOMKilled container worker exited with code 137: OOMKill",
		},
		{
			name:      "pod with application error",
			pod:       podAppError,
			container: "worker",
			wantOk:    false,
			wantErr:   true,
			wantError: "pod failed with reason: Error container worker exited with code 1: Error",
		},
		{
			name:      "pod with oomkill error and message",
			pod:       podInfraErrorWithMessage,
			container: "worker",
			wantOk:    false,
			wantErr:   true,
			wantError: "pod failed with reason: Pod OOMKilled message: The pod was killed because it ran out of memory container worker exited with code 137: Container OOMKill",
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
			wantOk:    true,
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

type statusErrorForTesting struct {
	*metav1.Status
}

func (s *statusErrorForTesting) Error() string {
	return s.Message
}

func TestWaitForPodCompleted(t *testing.T) {
	var logBuffer bytes.Buffer
	logger.SetGlobalLogLevel(logger.DebugLevel)
	testLogger := logger.NewReceptorLogger("")
	testLogger.SetOutput(&logBuffer)

	kw, err := startNetceptorNodeWithWorkceptorWithLogger(testLogger)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name            string
		initialPod      *corev1.Pod
		updatePhase     corev1.PodPhase
		wantErr         bool
		wantErrorString string
		debugLine       string
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
			debugLine:   "Pod default/pending-pod phase changed from Pending to Succeeded",
		},
		{
			name:        "pending to failed",
			initialPod:  podPending,
			updatePhase: corev1.PodFailed,
			wantErr:     false,
			debugLine:   "Pod default/pending-pod phase changed from Pending to Failed",
		},
		{
			name:        "pending to running",
			initialPod:  podPending,
			updatePhase: corev1.PodRunning,
			wantErr:     false,
			debugLine:   "Pod default/pending-pod phase changed from Pending to Running",
		},
		{
			name:        "pending to pending", // This simulates a pod that remains pending despite an update.
			initialPod:  podPending,
			updatePhase: corev1.PodPending,
			wantErr:     false,
			debugLine:   "Pod default/pending-pod event MODIFIED phase Pending (no change)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			timeoutSeconds := int64(2)
			logBuffer.Reset()

			var clientset *fake.Clientset
			if tt.initialPod != nil {
				clientset = fake.NewSimpleClientset(tt.initialPod)
			} else {
				clientset = fake.NewSimpleClientset(&corev1.Pod{})
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
			if tt.debugLine != "" {
				logOutput := logBuffer.String()
				if !strings.Contains(logOutput, tt.debugLine) {
					t.Errorf("Expected debug log '%s', got '%s'", tt.debugLine, logOutput)
				}
			} else {
				assert.NotContains(t, logBuffer.String(), "Pod diagnostics failed")
			}
		})
	}
}

func TestWaitForPodCompleted_HandlesTimeoutAndErrorEvents(t *testing.T) {
	// Create error event with custom type
	errStatus := &statusErrorForTesting{
		Status: &metav1.Status{
			Status:  metav1.StatusFailure,
			Message: "simulated error for unit test",
			Reason:  metav1.StatusReasonUnknown,
			Code:    500,
		},
	}

	errorEvent := watch.Event{
		Type:   watch.Error,
		Object: errStatus,
	}

	tests := []struct {
		name               string
		pod                *corev1.Pod
		watchEvents        []watch.Event
		expectedDebugLogs  []string
		expectedError      string
		cancelContextAfter time.Duration
		timeoutSeconds     int64
	}{
		{
			name: "Timeout",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
				Status:     corev1.PodStatus{Phase: corev1.PodPending},
			},
			watchEvents: []watch.Event{},
			expectedDebugLogs: []string{
				"Pod default/test-pod phase Pending timeout (no change)\n",
			},
			timeoutSeconds: 1,
		},
		{
			name: "Error Event",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
				Status:     corev1.PodStatus{Phase: corev1.PodPending},
			},
			watchEvents: []watch.Event{
				errorEvent,
			},
			expectedDebugLogs: []string{
				"Pod default/test-pod event ERROR phase Pending (error)",
			},
			expectedError:      "simulated error for unit test",
			timeoutSeconds:     2,
			cancelContextAfter: 0, // No cancellation needed for this test
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a buffer to capture log output
			var logBuffer bytes.Buffer
			logger.SetGlobalLogLevel(logger.DebugLevel)
			testLogger := logger.NewReceptorLogger("")
			testLogger.SetOutput(&logBuffer)

			// Create a KubeUnit with a mock logger
			kw, err := startNetceptorNodeWithWorkceptorWithLogger(testLogger)
			if err != nil {
				t.Fatal(err)
			}

			clientset := fake.NewSimpleClientset(tt.pod)
			watcher := watch.NewFake()
			clientset.PrependWatchReactor("pods", testcore.DefaultWatchReactor(watcher, nil))

			// Use a WaitGroup to ensure the goroutine is done
			var wg sync.WaitGroup
			wg.Add(1)

			go func() {
				defer wg.Done()
				for _, event := range tt.watchEvents {
					watcher.Action(event.Type, event.Object)
				}
				// Close the watcher to terminate the loop in WaitForPodCompleted
				watcher.Stop()
			}()

			ctx, cancel := context.WithCancel(context.Background())
			if tt.cancelContextAfter > 0 {
				go func() {
					time.Sleep(tt.cancelContextAfter)
					cancel()
				}()
			} else {
				defer cancel()
			}
			timeout := tt.timeoutSeconds
			_, err = kw.WaitForPodCompleted(ctx, tt.pod, clientset, &timeout)

			wg.Wait()

			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			logOutput := logBuffer.String()
			for _, expectedLog := range tt.expectedDebugLogs {
				assert.Contains(t, logOutput, expectedLog)
			}
			fmt.Println(logOutput)
		})
	}
}
