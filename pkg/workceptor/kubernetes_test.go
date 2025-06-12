package workceptor_test

import (
	"context"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/workceptor"
	"github.com/ansible/receptor/pkg/workceptor/mock_workceptor"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	fakeapi "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	fakerest "k8s.io/client-go/rest/fake"
	"k8s.io/client-go/tools/remotecommand"
)

func startNetceptorNodeWithWorkceptor() (*workceptor.KubeUnit, error) {
	kw := &workceptor.KubeUnit{
		BaseWorkUnitForWorkUnit: &workceptor.BaseWorkUnit{},
	}

	// Create Netceptor node using external backends
	n1 := netceptor.New(context.Background(), "node1")
	b1, err := netceptor.NewExternalBackend()
	if err != nil {
		return kw, err
	}

	err = n1.AddBackend(b1)
	if err != nil {
		return kw, err
	}

	w, err := workceptor.New(context.Background(), n1, "")
	if err != nil {
		return kw, err
	}

	kw.SetWorkceptor(w)

	return kw, nil
}

func TestShouldUseReconnect(t *testing.T) {
	const envVariable string = "RECEPTOR_KUBE_SUPPORT_RECONNECT"

	kw, err := startNetceptorNodeWithWorkceptor()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		envValue string
		want     bool
	}{
		{
			name:     "Enabled test",
			envValue: "enabled",
			want:     true,
		},
		{
			name:     "Disabled test",
			envValue: "disabled",
			want:     false,
		},
		{
			name:     "Auto test",
			envValue: "auto",
			want:     true,
		},
		{
			name:     "Default test",
			envValue: "default",
			want:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(envVariable, tt.envValue)
				defer os.Unsetenv(envVariable)
			} else {
				os.Unsetenv(envVariable)
			}

			if got := workceptor.ShouldUseReconnect(kw); got != tt.want {
				t.Errorf("shouldUseReconnect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetTimeoutOpenLogstream(t *testing.T) {
	const envVariable string = "RECEPTOR_OPEN_LOGSTREAM_TIMEOUT"

	kw, err := startNetceptorNodeWithWorkceptor()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		envValue string
		want     int
	}{
		{
			name:     "No env value set",
			envValue: "",
			want:     1,
		},
		{
			name:     "Env value set incorrectly to text",
			envValue: "text instead of int",
			want:     1,
		},
		{
			name:     "Env value set incorrectly to negative",
			envValue: "-1",
			want:     1,
		},
		{
			name:     "Env value set incorrectly to zero",
			envValue: "0",
			want:     1,
		},
		{
			name:     "Env value set correctly",
			envValue: "2",
			want:     2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(envVariable, tt.envValue)
				defer os.Unsetenv(envVariable)
			} else {
				os.Unsetenv(envVariable)
			}

			if got := workceptor.GetTimeoutOpenLogstream(kw); got != tt.want {
				t.Errorf("GetTimeoutOpenLogstream() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseTime(t *testing.T) {
	type args struct {
		s string
	}

	desiredTimeString := "2024-01-17T00:00:00Z"
	desiredTime, _ := time.Parse(time.RFC3339, desiredTimeString)

	tests := []struct {
		name    string
		args    args
		want    *time.Time
		wantErr bool
	}{
		{
			name: "Positive test",
			args: args{
				s: desiredTimeString,
			},
			want: &desiredTime,
		},
		{
			name: "Error test",
			args: args{
				s: "Invalid time",
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := workceptor.ParseTime(tt.args.s); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseTime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func createKubernetesTestSetup(t *testing.T) (workceptor.WorkUnit, *mock_workceptor.MockBaseWorkUnitForWorkUnit, *mock_workceptor.MockNetceptorForWorkceptor, *workceptor.Workceptor, *mock_workceptor.MockKubeAPIer, *gomock.Controller, context.Context) {
	ctrl := gomock.NewController(t)
	ctx := context.Background()

	mockBaseWorkUnit := mock_workceptor.NewMockBaseWorkUnitForWorkUnit(ctrl)
	mockNetceptor := mock_workceptor.NewMockNetceptorForWorkceptor(ctrl)
	mockNetceptor.EXPECT().NodeID().Return("NodeID")
	mockKubeAPI := mock_workceptor.NewMockKubeAPIer(ctrl)

	w, err := workceptor.New(ctx, mockNetceptor, "/tmp")
	if err != nil {
		t.Errorf("Error while creating Workceptor: %v", err)
	}

	mockBaseWorkUnit.EXPECT().Init(w, "", "", workceptor.FileSystem{})
	kubeConfig := workceptor.KubeWorkerCfg{AuthMethod: "incluster"}
	ku := kubeConfig.NewkubeWorker(mockBaseWorkUnit, w, "", "", mockKubeAPI)

	return ku, mockBaseWorkUnit, mockNetceptor, w, mockKubeAPI, ctrl, ctx
}

type hasTerm struct {
	field, value string
}

func (h *hasTerm) DeepCopySelector() fields.Selector { return h }
func (h *hasTerm) Empty() bool                       { return true }
func (h *hasTerm) Matches(_ fields.Fields) bool      { return true }
func (h *hasTerm) Requirements() fields.Requirements {
	return []fields.Requirement{{
		Field:    h.field,
		Operator: selection.Equals,
		Value:    h.value,
	}}
}
func (h *hasTerm) RequiresExactMatch(_ string) (value string, found bool)    { return "", true }
func (h *hasTerm) String() string                                            { return "Test" }
func (h *hasTerm) Transform(_ fields.TransformFunc) (fields.Selector, error) { return h, nil }

type ex struct{}

func (e *ex) Stream(_ remotecommand.StreamOptions) error {
	return nil
}

func (e *ex) StreamWithContext(_ context.Context, _ remotecommand.StreamOptions) error {
	return nil
}

func TestKubeStart(t *testing.T) {
	ku, mockbwu, mockNet, w, mockKubeAPI, _, ctx := createKubernetesTestSetup(t)

	startTestCases := []struct {
		name          string
		expectedCalls func()
	}{
		{
			name: "test1",
			expectedCalls: func() {
				mockbwu.EXPECT().UpdateBasicStatus(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				config := rest.Config{}
				mockKubeAPI.EXPECT().InClusterConfig().Return(&config, nil)
				mockbwu.EXPECT().GetWorkceptor().Return(w).AnyTimes()
				logger := logger.NewReceptorLogger("")
				mockNet.EXPECT().GetLogger().Return(logger).AnyTimes()
				clientset := kubernetes.Clientset{}
				mockKubeAPI.EXPECT().NewForConfig(gomock.Any()).Return(&clientset, nil)
				mockbwu.EXPECT().MonitorLocalStatus().AnyTimes()
				lock := &sync.RWMutex{}
				mockbwu.EXPECT().GetStatusLock().Return(lock).AnyTimes()
				kubeExtraData := workceptor.KubeExtraData{}
				status := workceptor.StatusFileData{ExtraData: &kubeExtraData}
				mockbwu.EXPECT().GetStatusWithoutExtraData().Return(&status).AnyTimes()
				mockbwu.EXPECT().GetStatusCopy().Return(status).AnyTimes()
				mockbwu.EXPECT().GetContext().Return(ctx).AnyTimes()
				pod := corev1.Pod{TypeMeta: metav1.TypeMeta{}, ObjectMeta: metav1.ObjectMeta{Name: "Test Name"}, Spec: corev1.PodSpec{}, Status: corev1.PodStatus{}}

				mockKubeAPI.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&pod, nil).AnyTimes()
				mockbwu.EXPECT().UpdateFullStatus(gomock.Any()).AnyTimes()

				field := hasTerm{}
				mockKubeAPI.EXPECT().OneTermEqualSelector(gomock.Any(), gomock.Any()).Return(&field).AnyTimes()
				ev := watch.Event{Object: &pod}
				mockKubeAPI.EXPECT().UntilWithSync(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&ev, nil).AnyTimes()
				apierr := apierrors.StatusError{}
				mockKubeAPI.EXPECT().NewNotFound(gomock.Any(), gomock.Any()).Return(&apierr).AnyTimes()
				mockbwu.EXPECT().MonitorLocalStatus().AnyTimes()

				c := rest.RESTClient{}
				req := rest.NewRequest(&c)
				mockKubeAPI.EXPECT().SubResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(req).AnyTimes()
				exec := ex{}
				mockKubeAPI.EXPECT().NewSPDYExecutor(gomock.Any(), gomock.Any(), gomock.Any()).Return(&exec, nil).AnyTimes()
				mockbwu.EXPECT().UnitDir().Return("TestDir").AnyTimes()
			},
		},
	}

	for _, testCase := range startTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.expectedCalls()

			err := ku.Start()
			if err != nil {
				t.Error(err)
			}
		})
	}
}

func Test_IsCompatibleK8S(t *testing.T) {
	type args struct {
		kw         *workceptor.KubeUnit
		versionStr string
	}

	kw, err := startNetceptorNodeWithWorkceptor()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Kubernetes X stream negative test",
			args: args{
				versionStr: "v0.0.0",
			},
			want: false,
		},
		{
			name: "Kubernetes Y stream negative test",
			args: args{
				versionStr: "v1.22.9998",
			},
			want: false,
		},
		{
			name: "Kubernetes 1.23 Z stream negative test",
			args: args{
				versionStr: "v1.23.13",
			},
			want: false,
		},
		{
			name: "Kubernetes 1.23 exact positive test",
			args: args{
				versionStr: "v1.23.14",
			},
			want: true,
		},
		{
			name: "Kubernetes 1.23 Z stream positive test",
			args: args{
				versionStr: "v1.23.15",
			},
			want: true,
		},
		{
			name: "Kubernetes 1.24 Z stream negative test",
			args: args{
				versionStr: "v1.24.7",
			},
			want: false,
		},
		{
			name: "Kubernetes 1.24 exact positive test",
			args: args{
				versionStr: "v1.24.8",
			},
			want: true,
		},
		{
			name: "Kubernetes 1.24 Z stream positive test",
			args: args{
				versionStr: "v1.24.9",
			},
			want: true,
		},
		{
			name: "Kubernetes 1.25 Z stream negative test",
			args: args{
				versionStr: "v1.25.3",
			},
			want: false,
		},
		{
			name: "Kuberentes 1.25 exact positive test",
			args: args{
				versionStr: "v1.25.4",
			},
			want: true,
		},
		{
			name: "Kubernetes 1.25 Z stream positive test",
			args: args{
				versionStr: "v1.25.99",
			},
			want: true,
		},
		{
			name: "Kubernetes Y stream positive test",
			args: args{
				versionStr: "v1.26.0",
			},
			want: true,
		},
		{
			name: "Kubernetes X stream positive test 1",
			args: args{
				versionStr: "v2.0.0",
			},
			want: false,
		},
		{
			name: "Kubernetes X stream positive test 2",
			args: args{
				versionStr: "v2.23.14",
			},
			want: true,
		},
		{
			name: "Kubernetes X stream positive test 3",
			args: args{
				versionStr: "v2.24.8",
			},
			want: true,
		},
		{
			name: "Kubernetes X stream positive test 4",
			args: args{
				versionStr: "v2.25.4",
			},
			want: true,
		},
		{
			name: "Kubernetes X stream positive test 5",
			args: args{
				versionStr: "v2.26.0",
			},
			want: true,
		},
		{
			name: "Missing Kubernetes version negative test",
			args: args{
				versionStr: "yoloswag",
			},
			want: false,
		},
		{
			name: "Prerelease Kubernetes version positive test 1",
			args: args{
				versionStr: "v1.32.14+sadfasdf",
			},
			want: true,
		},
		{
			name: "Prerelease Kubernetes version positive test 2",
			args: args{
				versionStr: "v1.32.14-asdfasdf+12131",
			},
			want: true,
		},
		{
			name: "Prerelease Kubernetes version positive test 3",
			args: args{
				versionStr: "v1.32.15-asdfasdf+12131",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		tt.args.kw = kw
		t.Run(tt.name, func(t *testing.T) {
			if got := workceptor.IsCompatibleK8S(tt.args.kw, tt.args.versionStr); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("IsCompatibleK8S() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKubeLoggingWithReconnect(t *testing.T) {
	var stdinErr error
	var stdoutErr error
	_, mockBaseWorkUnit, mockNetceptor, w, mockKubeAPI, ctrl, ctx := createKubernetesTestSetup(t)

	pod := &corev1.Pod{TypeMeta: metav1.TypeMeta{}, ObjectMeta: metav1.ObjectMeta{Name: "Test_Name", Namespace: "Test_Namespace"}, Spec: corev1.PodSpec{}, Status: corev1.PodStatus{Phase: corev1.PodRunning}}

	kw := &workceptor.KubeUnit{
		BaseWorkUnitForWorkUnit: mockBaseWorkUnit,
		KubeAPIWrapperInstance:  mockKubeAPI,
		Pod:                     pod,
	}

	tests := []struct {
		name          string
		expectedCalls func()
	}{
		{
			name: "Kube error should be read",
			expectedCalls: func() {
				mockBaseWorkUnit.EXPECT().GetWorkceptor().Return(w)
				mockBaseWorkUnit.EXPECT().GetContext().Return(ctx).Times(3)
				mockKubeAPI.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(pod, nil)
				logger := logger.NewReceptorLogger("")
				mockNetceptor.EXPECT().GetLogger().Return(logger)
				req := fakerest.RESTClient{
					Client: fakerest.CreateHTTPClient(func(request *http.Request) (*http.Response, error) {
						resp := &http.Response{
							StatusCode: http.StatusOK,
							Body:       io.NopCloser(strings.NewReader("2024-12-09T00:31:18.823849250Z HI\n kube error")),
						}

						return resp, nil
					}),
					NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
				}
				mockKubeAPI.EXPECT().GetLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(req.Request())
			},
		},
		{
			name: "Kube error 503",
			expectedCalls: func() {
				mockBaseWorkUnit.EXPECT().GetWorkceptor().Return(w).MinTimes(1)
				mockBaseWorkUnit.EXPECT().GetContext().Return(ctx).MinTimes(3)
				mockKubeAPI.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(pod, nil)
				logger := logger.NewReceptorLogger("")
				mockNetceptor.EXPECT().GetLogger().Return(logger).MinTimes(1)
				mockBaseWorkUnit.EXPECT().UpdateBasicStatus(gomock.Any(), gomock.Any(), gomock.Any())
				req := fakerest.RESTClient{
					Client: fakerest.CreateHTTPClient(func(request *http.Request) (*http.Response, error) {
						resp := &http.Response{
							StatusCode: http.StatusServiceUnavailable, // 503
							Body:       io.NopCloser(strings.NewReader("kube error")),
						}

						return resp, nil
					}),
					NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
				}
				mockKubeAPI.EXPECT().GetLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(req.Request())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.expectedCalls()
			wg := &sync.WaitGroup{}
			wg.Add(1)
			mockfilesystemer := mock_workceptor.NewMockFileSystemer(ctrl)
			mockfilesystemer.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(&os.File{}, nil)
			stdout, _ := workceptor.NewStdoutWriter(mockfilesystemer, "")
			mockFileWC := mock_workceptor.NewMockFileWriteCloser(ctrl)
			stdout.SetWriter(mockFileWC)
			mockFileWC.EXPECT().Write(gomock.AnyOf([]byte("HI\n"), []byte(" kube error\n"))).Return(0, nil).AnyTimes()
			kw.KubeLoggingWithReconnect(wg, stdout, &stdinErr, &stdoutErr)
		})
	}
}
func createTestPods() (*corev1.Pod, *corev1.Pod, *corev1.Pod, *corev1.Pod) {
	podSuccess := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "test-pod",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
		},
	}

	podInfraError := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "infra-error-pod",
		},
		Status: corev1.PodStatus{
			Phase:  corev1.PodFailed,
			Reason: "PodMockOOMKilled",
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "main",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 1,
							Reason:   "MockOOMKill",
						},
					},
				},
			},
		},
	}

	podAppError := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "app-error-pod",
		},
		Status: corev1.PodStatus{
			Phase:  corev1.PodFailed,
			Reason: "AppFailure",
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "main",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 1,
							Reason:   "ContainerFailure",
						},
					},
				},
			},
		},
	}

	podPending := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "pending-pod",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "main",
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

	return podSuccess, podInfraError, podAppError, podPending
}

func TestGetPodStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockKubeAPI := mock_workceptor.NewMockKubeAPIer(ctrl)

	// Create real implementation instance
	realImplementation := &workceptor.KubeAPIWrapper{mockKubeAPI}

	podSuccess, infraErrorPod, applicationErrorPod, _ := createTestPods()

	tests := []struct {
		name       string
		pod        *corev1.Pod
		wantOk     bool
		wantReason string
		wantErr    bool
	}{
		{
			name:       "nil pod",
			pod:        nil,
			wantOk:     false,
			wantReason: "pod is nil",
			wantErr:    true,
		},
		{
			name:       "infrastructure failure",
			pod:        infraErrorPod,
			wantOk:     false,
			wantReason: "pod default/infra-error-pod infrastructure pod reason PodMockOOMKilled container main MockOOMKill",
			wantErr:    false,
		},
		{
			name:       "application failure",
			pod:        applicationErrorPod,
			wantOk:     false,
			wantReason: "pod default/app-error-pod infrastructure pod reason AppFailure container main ContainerFailure",
			wantErr:    false,
		},
		{
			name:       "success case",
			pod:        podSuccess,
			wantOk:     true,
			wantReason: "",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, reason, err := realImplementation.GetPodStatus(tt.pod)
			if ok != tt.wantOk || reason != tt.wantReason || (err != nil) != tt.wantErr {
				t.Errorf("Failed %s case: ok=%v reason=%q err=%v", tt.name, ok, reason, err)
			}
			if err != nil && !strings.Contains(err.Error(), tt.wantReason) {
				t.Errorf("Expected error message '%s', got '%s'", tt.wantReason, err.Error())
			}
		})
	}

}

func TestWaitForPodCompleted(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockKubeAPI := mock_workceptor.NewMockKubeAPIer(ctrl)

	// Create real implementation instance
	realImplementation := &workceptor.KubeAPIWrapper{mockKubeAPI}

	tests := []struct {
		name         string
		initialPhase corev1.PodPhase
		updatePhase  corev1.PodPhase
		timeout      time.Duration
		updateDelay  time.Duration
		wantPhase    corev1.PodPhase
		wantErr      bool
		wantError    string
	}{
		{
			name:         "successful completion",
			initialPhase: corev1.PodPending,
			updatePhase:  corev1.PodSucceeded,
			timeout:      2 * time.Second,
			wantErr:      false,
			wantPhase:    corev1.PodSucceeded,
		},
		{
			name:         "timeout handling",
			initialPhase: corev1.PodPending,
			timeout:      1 * time.Second,
			wantErr:      true,
			wantPhase:    corev1.PodPending,
			wantError:    "timeout: pod default/timeout handling-pod did not complete within 1s",
		},
		{
			name:         "immediate failure",
			initialPhase: corev1.PodFailed,
			timeout:      2 * time.Second,
			wantPhase:    corev1.PodFailed,
			wantErr:      false,
		},
		{
			name:         "pending to failed",
			initialPhase: corev1.PodPending,
			updatePhase:  corev1.PodFailed,
			updateDelay:  100 * time.Millisecond,
			timeout:      2 * time.Second,
			wantPhase:    corev1.PodFailed,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh clientset for each test case
			initialPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      tt.name + "-pod",
				},
				Status: corev1.PodStatus{
					Phase: tt.initialPhase,
				},
			}
			clientset := fakeapi.NewSimpleClientset(initialPod)

			if tt.updateDelay == 0 {
				tt.updateDelay = 500 * time.Millisecond // Default update delay if not specified
			}

			if tt.updatePhase != "" {
				go func() {
					time.Sleep(tt.updateDelay)
					updatedPod := initialPod.DeepCopy()
					updatedPod.Status.Phase = tt.updatePhase
					_, _ = clientset.CoreV1().Pods("default").Update(context.Background(), updatedPod, metav1.UpdateOptions{})
				}()
			}

			_, err := realImplementation.WaitForPodCompleted(initialPod, clientset, tt.timeout)

			if (err != nil) != tt.wantErr {
				t.Errorf("WaitForPodCompleted() error = %v, wantErr %v", err, tt.wantErr)
				if tt.wantError != "" && err != nil && !strings.Contains(err.Error(), tt.wantError) {
					t.Errorf("Expected error message '%s', got '%s'", tt.wantError, err.Error())
				}
			}
			if tt.wantErr && err != nil {
				if !strings.Contains(err.Error(), tt.wantError) {
					t.Errorf("Expected error message '%s', got '%s'", tt.wantError, err.Error())
				}
			}
		})
	}
}
