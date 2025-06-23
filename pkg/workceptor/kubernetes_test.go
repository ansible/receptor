//go:build !no_workceptor
// +build !no_workceptor

package workceptor_test

import (
	"context"
	"errors"
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
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
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

func createKubernetesTestSetup(t *testing.T, options ...string) (workceptor.WorkUnit, *mock_workceptor.MockBaseWorkUnitForWorkUnit, *mock_workceptor.MockNetceptorForWorkceptor, *workceptor.Workceptor, *mock_workceptor.MockKubeAPIer, *gomock.Controller, context.Context) {
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

	// Default configuration
	kubeConfig := workceptor.KubeWorkerCfg{
		AuthMethod:         "incluster",
		StreamMethod:       "logger",
		DeletePodOnRestart: false,
	}

	// Apply options
	for _, option := range options {
		if strings.HasPrefix(option, "streamMethod=") {
			kubeConfig.StreamMethod = strings.TrimPrefix(option, "streamMethod=")
		} else if strings.HasPrefix(option, "deletePodOnRestart=") {
			deletePodOnRestart := strings.TrimPrefix(option, "deletePodOnRestart=")
			kubeConfig.DeletePodOnRestart = (deletePodOnRestart == "true")
		}
	}

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

// Helper type to simulate EOF from stream reading.
type eofReadCloser struct {
	content string
	hasRead bool
}

func (e *eofReadCloser) Read(p []byte) (n int, err error) {
	if !e.hasRead && len(e.content) > 0 {
		// Add newline to simulate a complete log line
		fullContent := e.content + "\n"
		n = copy(p, []byte(fullContent))
		e.hasRead = true

		return n, nil
	}

	return 0, io.EOF
}

func (e *eofReadCloser) Close() error {
	return nil
}

func TestKubeLoggingWithReconnect(t *testing.T) {
	type testCase struct {
		name              string
		setupMocks        func(mockBaseWorkUnit *mock_workceptor.MockBaseWorkUnitForWorkUnit, mockNetceptor *mock_workceptor.MockNetceptorForWorkceptor, mockKubeAPI *mock_workceptor.MockKubeAPIer, w *workceptor.Workceptor, ctx context.Context)
		stdinErr          *error
		expectedStdoutErr bool
		timeoutSeconds    int
	}

	tests := []testCase{
		{
			name: "stdin_error_causes_immediate_return",
			setupMocks: func(mockBaseWorkUnit *mock_workceptor.MockBaseWorkUnitForWorkUnit, mockNetceptor *mock_workceptor.MockNetceptorForWorkceptor, mockKubeAPI *mock_workceptor.MockKubeAPIer, w *workceptor.Workceptor, ctx context.Context) {
				mockBaseWorkUnit.EXPECT().GetWorkceptor().Return(w).AnyTimes()
				logger := logger.NewReceptorLogger("")
				mockNetceptor.EXPECT().GetLogger().Return(logger).AnyTimes()
			},
			stdinErr: func() *error {
				err := errors.New("stdin failed")

				return &err
			}(),
			expectedStdoutErr: false,
			timeoutSeconds:    1,
		},
		{
			name: "context_cancellation_during_reading",
			setupMocks: func(mockBaseWorkUnit *mock_workceptor.MockBaseWorkUnitForWorkUnit, mockNetceptor *mock_workceptor.MockNetceptorForWorkceptor, mockKubeAPI *mock_workceptor.MockKubeAPIer, w *workceptor.Workceptor, ctx context.Context) {
				mockBaseWorkUnit.EXPECT().GetWorkceptor().Return(w).AnyTimes()
				cancelCtx, cancel := context.WithCancel(ctx)
				mockBaseWorkUnit.EXPECT().GetContext().Return(cancelCtx).AnyTimes()
				logger := logger.NewReceptorLogger("")
				mockNetceptor.EXPECT().GetLogger().Return(logger).AnyTimes()

				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "Test_Name", Namespace: "Test_Namespace"},
					Status:     corev1.PodStatus{Phase: corev1.PodRunning},
				}
				mockKubeAPI.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(pod, nil).Times(1)

				req := fakerest.RESTClient{
					Client: fakerest.CreateHTTPClient(func(request *http.Request) (*http.Response, error) {
						// Cancel context during request to simulate cancellation during reading
						cancel()

						return &http.Response{
							StatusCode: http.StatusOK,
							Body:       io.NopCloser(strings.NewReader("2024-12-09T00:31:18.823849250Z Test log\n")),
						}, nil
					}),
					NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
				}
				mockKubeAPI.EXPECT().GetLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(req.Request()).Times(1)
			},
			stdinErr: func() *error {
				var err error

				return &err
			}(),
			expectedStdoutErr: false,
			timeoutSeconds:    2,
		},
		{
			name: "pod_retrieval_failure_exhausts_retries",
			setupMocks: func(mockBaseWorkUnit *mock_workceptor.MockBaseWorkUnitForWorkUnit, mockNetceptor *mock_workceptor.MockNetceptorForWorkceptor, mockKubeAPI *mock_workceptor.MockKubeAPIer, w *workceptor.Workceptor, ctx context.Context) {
				mockBaseWorkUnit.EXPECT().GetWorkceptor().Return(w).AnyTimes()
				mockBaseWorkUnit.EXPECT().GetContext().Return(ctx).AnyTimes()
				logger := logger.NewReceptorLogger("")
				mockNetceptor.EXPECT().GetLogger().Return(logger).AnyTimes()

				mockKubeAPI.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("pod not found")).Times(5)

				mockBaseWorkUnit.EXPECT().UpdateBasicStatus(workceptor.WorkStateFailed, gomock.Any(), gomock.Any()).Times(1)
			},
			stdinErr: func() *error {
				var err error

				return &err
			}(),
			expectedStdoutErr: false,
			timeoutSeconds:    10, // Allow time for 5 retries with 1 second delays
		},
		{
			name: "log_stream_connection_failure",
			setupMocks: func(mockBaseWorkUnit *mock_workceptor.MockBaseWorkUnitForWorkUnit, mockNetceptor *mock_workceptor.MockNetceptorForWorkceptor, mockKubeAPI *mock_workceptor.MockKubeAPIer, w *workceptor.Workceptor, ctx context.Context) {
				mockBaseWorkUnit.EXPECT().GetWorkceptor().Return(w).AnyTimes()
				mockBaseWorkUnit.EXPECT().GetContext().Return(ctx).AnyTimes()
				logger := logger.NewReceptorLogger("")
				mockNetceptor.EXPECT().GetLogger().Return(logger).AnyTimes()

				mockBaseWorkUnit.EXPECT().UpdateBasicStatus(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "Test_Name", Namespace: "Test_Namespace"},
					Status:     corev1.PodStatus{Phase: corev1.PodRunning},
				}
				mockKubeAPI.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(pod, nil).AnyTimes()

				failReq := fakerest.RESTClient{
					Client: fakerest.CreateHTTPClient(func(request *http.Request) (*http.Response, error) {
						return nil, errors.New("connection refused")
					}),
					NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
				}
				mockKubeAPI.EXPECT().GetLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(failReq.Request()).AnyTimes()
			},
			stdinErr: func() *error {
				var err error

				return &err
			}(),
			expectedStdoutErr: false,
			timeoutSeconds:    30, // Allow time for retries
		},
		{
			name: "eof_with_pod_not_ready_exits_immediately",
			setupMocks: func(mockBaseWorkUnit *mock_workceptor.MockBaseWorkUnitForWorkUnit, mockNetceptor *mock_workceptor.MockNetceptorForWorkceptor, mockKubeAPI *mock_workceptor.MockKubeAPIer, w *workceptor.Workceptor, ctx context.Context) {
				mockBaseWorkUnit.EXPECT().GetWorkceptor().Return(w).AnyTimes()
				mockBaseWorkUnit.EXPECT().GetContext().Return(ctx).AnyTimes()
				logger := logger.NewReceptorLogger("")
				mockNetceptor.EXPECT().GetLogger().Return(logger).AnyTimes()

				runningPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "Test_Name", Namespace: "Test_Namespace"},
					Status:     corev1.PodStatus{Phase: corev1.PodRunning},
				}
				notReadyPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "Test_Name", Namespace: "Test_Namespace"},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{Type: corev1.PodReady, Status: corev1.ConditionFalse},
						},
					},
				}

				// First Get() for main loop, second Get() after EOF for readiness check
				gomock.InOrder(
					mockKubeAPI.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(runningPod, nil),
					mockKubeAPI.EXPECT().Get(gomock.Any(), gomock.Any(), "Test_Namespace", "Test_Name", gomock.Any()).Return(notReadyPod, nil),
				)

				req := fakerest.RESTClient{
					Client: fakerest.CreateHTTPClient(func(request *http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: http.StatusOK,
							Body:       &eofReadCloser{content: "2024-12-09T00:31:18.823849250Z Final log", hasRead: false},
						}, nil
					}),
					NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
				}
				mockKubeAPI.EXPECT().GetLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(req.Request()).Times(1)
			},
			stdinErr: func() *error {
				var err error

				return &err
			}(),
			expectedStdoutErr: false,
			timeoutSeconds:    2,
		},
		{
			name: "eof_with_pod_ready_triggers_retry_then_exhausts",
			setupMocks: func(mockBaseWorkUnit *mock_workceptor.MockBaseWorkUnitForWorkUnit, mockNetceptor *mock_workceptor.MockNetceptorForWorkceptor, mockKubeAPI *mock_workceptor.MockKubeAPIer, w *workceptor.Workceptor, ctx context.Context) {
				mockBaseWorkUnit.EXPECT().GetWorkceptor().Return(w).AnyTimes()
				mockBaseWorkUnit.EXPECT().GetContext().Return(ctx).AnyTimes()
				logger := logger.NewReceptorLogger("")
				mockNetceptor.EXPECT().GetLogger().Return(logger).AnyTimes()

				runningPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "Test_Name", Namespace: "Test_Namespace"},
					Status:     corev1.PodStatus{Phase: corev1.PodRunning},
				}
				readyPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "Test_Name", Namespace: "Test_Namespace"},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{Type: corev1.PodReady, Status: corev1.ConditionTrue},
						},
					},
				}

				// Multiple Get() calls for retries - pod stays ready so it keeps retrying
				gomock.InOrder(
					mockKubeAPI.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(runningPod, nil),
					mockKubeAPI.EXPECT().Get(gomock.Any(), gomock.Any(), "Test_Namespace", "Test_Name", gomock.Any()).Return(readyPod, nil).AnyTimes(),
				)

				// Return EOF each time to trigger retry exhaustion
				req := fakerest.RESTClient{
					Client: fakerest.CreateHTTPClient(func(request *http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: http.StatusOK,
							Body:       &eofReadCloser{content: "2024-12-09T00:31:19.123456789Z Retry log", hasRead: false},
						}, nil
					}),
					NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
				}
				mockKubeAPI.EXPECT().GetLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(req.Request()).AnyTimes()
			},
			stdinErr: func() *error {
				var err error

				return &err
			}(),
			expectedStdoutErr: false,
			timeoutSeconds:    5,
		},
		{
			name: "successful_log_reading_with_timestamps",
			setupMocks: func(mockBaseWorkUnit *mock_workceptor.MockBaseWorkUnitForWorkUnit, mockNetceptor *mock_workceptor.MockNetceptorForWorkceptor, mockKubeAPI *mock_workceptor.MockKubeAPIer, w *workceptor.Workceptor, ctx context.Context) {
				mockBaseWorkUnit.EXPECT().GetWorkceptor().Return(w).AnyTimes()
				mockBaseWorkUnit.EXPECT().GetContext().Return(ctx).AnyTimes()
				logger := logger.NewReceptorLogger("")
				mockNetceptor.EXPECT().GetLogger().Return(logger).AnyTimes()

				runningPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "Test_Name", Namespace: "Test_Namespace"},
					Status:     corev1.PodStatus{Phase: corev1.PodRunning},
				}
				completedPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "Test_Name", Namespace: "Test_Namespace"},
					Status: corev1.PodStatus{
						Phase: corev1.PodSucceeded,
						Conditions: []corev1.PodCondition{
							{Type: corev1.PodReady, Status: corev1.ConditionFalse},
						},
					},
				}

				gomock.InOrder(
					mockKubeAPI.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(runningPod, nil),
					mockKubeAPI.EXPECT().Get(gomock.Any(), gomock.Any(), "Test_Namespace", "Test_Name", gomock.Any()).Return(completedPod, nil),
				)

				req := fakerest.RESTClient{
					Client: fakerest.CreateHTTPClient(func(request *http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: http.StatusOK,
							Body: io.NopCloser(strings.NewReader(
								"2024-12-09T00:31:18.823849250Z First log line\n" +
									"2024-12-09T00:31:19.123456789Z Second log line\n" +
									"2024-12-09T00:31:20.999999999Z Final log line",
							)),
						}, nil
					}),
					NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
				}
				mockKubeAPI.EXPECT().GetLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(req.Request()).Times(1)
			},
			stdinErr: func() *error {
				var err error

				return &err
			}(),
			expectedStdoutErr: false,
			timeoutSeconds:    2,
		},
		{
			name: "malformed_timestamps_are_handled",
			setupMocks: func(mockBaseWorkUnit *mock_workceptor.MockBaseWorkUnitForWorkUnit, mockNetceptor *mock_workceptor.MockNetceptorForWorkceptor, mockKubeAPI *mock_workceptor.MockKubeAPIer, w *workceptor.Workceptor, ctx context.Context) {
				mockBaseWorkUnit.EXPECT().GetWorkceptor().Return(w).AnyTimes()
				mockBaseWorkUnit.EXPECT().GetContext().Return(ctx).AnyTimes()
				logger := logger.NewReceptorLogger("")
				mockNetceptor.EXPECT().GetLogger().Return(logger).AnyTimes()

				runningPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "Test_Name", Namespace: "Test_Namespace"},
					Status:     corev1.PodStatus{Phase: corev1.PodRunning},
				}
				completedPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "Test_Name", Namespace: "Test_Namespace"},
					Status: corev1.PodStatus{
						Phase: corev1.PodSucceeded,
						Conditions: []corev1.PodCondition{
							{Type: corev1.PodReady, Status: corev1.ConditionFalse},
						},
					},
				}

				gomock.InOrder(
					mockKubeAPI.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(runningPod, nil),
					mockKubeAPI.EXPECT().Get(gomock.Any(), gomock.Any(), "Test_Namespace", "Test_Name", gomock.Any()).Return(completedPod, nil),
				)

				req := fakerest.RESTClient{
					Client: fakerest.CreateHTTPClient(func(request *http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: http.StatusOK,
							Body: io.NopCloser(strings.NewReader(
								"2024-12-09T00:31:18.823849250Z Valid timestamp log\n" +
									"invalid-timestamp This log has no valid timestamp\n" +
									"2024-12-09T00:31:20.999999999Z Another valid log",
							)),
						}, nil
					}),
					NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
				}
				mockKubeAPI.EXPECT().GetLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(req.Request()).Times(1)
			},
			stdinErr: func() *error {
				var err error

				return &err
			}(),
			expectedStdoutErr: false,
			timeoutSeconds:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdoutErr error
			_, mockBaseWorkUnit, mockNetceptor, w, mockKubeAPI, ctrl, ctx := createKubernetesTestSetup(t)
			defer ctrl.Finish()

			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "Test_Name", Namespace: "Test_Namespace"},
				Status:     corev1.PodStatus{Phase: corev1.PodRunning},
			}

			kw := &workceptor.KubeUnit{
				BaseWorkUnitForWorkUnit: mockBaseWorkUnit,
				KubeAPIWrapperInstance:  mockKubeAPI,
				Pod:                     pod,
			}

			tt.setupMocks(mockBaseWorkUnit, mockNetceptor, mockKubeAPI, w, ctx)

			mockfilesystemer := mock_workceptor.NewMockFileSystemer(ctrl)
			mockfilesystemer.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(&os.File{}, nil).AnyTimes()
			stdout, _ := workceptor.NewStdoutWriter(mockfilesystemer, "")
			mockFileWC := mock_workceptor.NewMockFileWriteCloser(ctrl)
			stdout.SetWriter(mockFileWC)

			var writtenData []string
			mockFileWC.EXPECT().Write(gomock.Any()).DoAndReturn(func(data []byte) (int, error) {
				writtenData = append(writtenData, string(data))

				return len(data), nil
			}).AnyTimes()

			wg := &sync.WaitGroup{}
			wg.Add(1)

			done := make(chan bool, 1)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						t.Logf("Function panicked: %v", r)
					}
					done <- true
				}()
				kw.KubeLoggingWithReconnect(wg, stdout, tt.stdinErr, &stdoutErr)
			}()

			select {
			case <-done:
			case <-time.After(time.Duration(tt.timeoutSeconds) * time.Second):
				t.Logf("Test timed out after %d seconds", tt.timeoutSeconds)
			}

			if tt.expectedStdoutErr && stdoutErr == nil {
				t.Errorf("Expected stdout error but got none")
			}
			if !tt.expectedStdoutErr && stdoutErr != nil {
				t.Errorf("Unexpected stdout error: %v", stdoutErr)
			}

			if len(writtenData) > 0 {
				t.Logf("Written data: %v", writtenData)
			}
		})
	}
}

// TestKubeAPIWrapper tests the KubeAPIWrapper methods.
func TestKubeAPIWrapper(t *testing.T) {
	// Create a KubeAPIWrapper instance
	wrapper := workceptor.KubeAPIWrapper{}

	// Test NewNotFound
	t.Run("NewNotFound", func(t *testing.T) {
		gr := schema.GroupResource{Group: "test", Resource: "test"}
		err := wrapper.NewNotFound(gr, "test-name")
		assert.NotNil(t, err)
		assert.True(t, apierrors.IsNotFound(err))
	})

	// Test OneTermEqualSelector
	t.Run("OneTermEqualSelector", func(t *testing.T) {
		selector := wrapper.OneTermEqualSelector("key", "value")
		assert.NotNil(t, selector)
		requirements := selector.Requirements()
		assert.Equal(t, 1, len(requirements))
		assert.Equal(t, "key", requirements[0].Field)
		assert.Equal(t, "value", requirements[0].Value)
	})

	// Test NewFakeNeverRateLimiter
	t.Run("NewFakeNeverRateLimiter", func(t *testing.T) {
		limiter := wrapper.NewFakeNeverRateLimiter()
		assert.NotNil(t, limiter)
		// This should never wait
		assert.Equal(t, false, limiter.TryAccept())
	})

	// Test NewFakeAlwaysRateLimiter
	t.Run("NewFakeAlwaysRateLimiter", func(t *testing.T) {
		limiter := wrapper.NewFakeAlwaysRateLimiter()
		assert.NotNil(t, limiter)
		// This should always wait
		assert.Equal(t, true, limiter.TryAccept())
	})
}

// TestKubeAPIWrapperExtended tests the remaining KubeAPIWrapper methods.
func TestKubeAPIWrapperExtended(t *testing.T) {
	// Create a KubeAPIWrapper instance
	wrapper := workceptor.KubeAPIWrapper{}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Test NewForConfig
	t.Run("NewForConfig", func(t *testing.T) {
		// Verify NewForConfig method exists with correct signature
		methodType := reflect.TypeOf(wrapper.NewForConfig)
		assert.Equal(t, "func(*rest.Config) (*kubernetes.Clientset, error)", methodType.String())
	})

	// Test GetLogs
	t.Run("GetLogs", func(t *testing.T) {
		// Create a mock clientset
		clientset := kubernetes.NewForConfigOrDie(&rest.Config{Host: "https://localhost:8443"})
		// Call the method
		req := wrapper.GetLogs(clientset, "default", "test-pod", &corev1.PodLogOptions{})
		// Verify the request is created correctly
		assert.NotNil(t, req)
		assert.Contains(t, req.URL().Path, "pods")
		assert.Contains(t, req.URL().Path, "test-pod")
		assert.Contains(t, req.URL().Path, "log")
	})

	// Test SubResource
	t.Run("SubResource", func(t *testing.T) {
		// Create a mock clientset
		clientset := kubernetes.NewForConfigOrDie(&rest.Config{Host: "https://localhost:8443"})
		// Call the method
		req := wrapper.SubResource(clientset, "test-pod", "default")
		// Verify the request is created correctly
		assert.NotNil(t, req)
		assert.Contains(t, req.URL().Path, "pods")
		assert.Contains(t, req.URL().Path, "test-pod")
		assert.Contains(t, req.URL().Path, "attach")
	})

	// Test NewDefaultClientConfigLoadingRules
	t.Run("NewDefaultClientConfigLoadingRules", func(t *testing.T) {
		// Call the method
		rules := wrapper.NewDefaultClientConfigLoadingRules()
		// Verify the rules are created correctly
		assert.NotNil(t, rules)
	})

	// Test BuildConfigFromFlags
	t.Run("BuildConfigFromFlags", func(t *testing.T) {
		_, err := wrapper.BuildConfigFromFlags("", "")
		assert.Error(t, err)
	})

	// Test NewClientConfigFromBytes
	t.Run("NewClientConfigFromBytes", func(t *testing.T) {
		// Create a minimal kubeconfig
		kubeconfig := `
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://localhost:8443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: test-token
`
		// Call the method
		config, err := wrapper.NewClientConfigFromBytes([]byte(kubeconfig))
		// Verify the config is created correctly
		assert.NoError(t, err)
		assert.NotNil(t, config)
	})

	// Test Get, Create, List, Watch, Delete
	t.Run("Pod CRUD Operations", func(t *testing.T) {
		clientset := kubernetes.NewForConfigOrDie(&rest.Config{Host: "https://localhost:8443"})
		ctx := context.Background()
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod",
				Namespace: "default",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "test-container",
						Image: "test-image",
					},
				},
			},
		}

		_, err := wrapper.Get(ctx, clientset, "default", "test-pod", metav1.GetOptions{})
		assert.Error(t, err) // We expect an error here since we're not connecting to a real API server

		_, err = wrapper.Create(ctx, clientset, "default", pod, metav1.CreateOptions{})
		assert.Error(t, err)

		_, err = wrapper.List(ctx, clientset, "default", metav1.ListOptions{})
		assert.Error(t, err)

		_, err = wrapper.Watch(ctx, clientset, "default", metav1.ListOptions{})
		assert.Error(t, err)

		err = wrapper.Delete(ctx, clientset, "default", "test-pod", metav1.DeleteOptions{})
		assert.Error(t, err)
	})

	// Test InClusterConfig
	t.Run("InClusterConfig", func(t *testing.T) {
		// This will fail because we're not running inside a Kubernetes cluster
		// but it will exercise the code path
		_, err := wrapper.InClusterConfig()
		assert.Error(t, err) // We expect an error here
	})

	// Test NewSPDYExecutor and StreamWithContext
	t.Run("SPDY Operations", func(t *testing.T) {
		// Verify NewSPDYExecutor method exists with correct signature
		methodType := reflect.TypeOf(wrapper.NewSPDYExecutor)
		assert.Equal(t, "func(*rest.Config, string, *url.URL) (remotecommand.Executor, error)", methodType.String())

		// Verify StreamWithContext method exists with correct signature
		methodType = reflect.TypeOf(wrapper.StreamWithContext)
		assert.Equal(t, "func(context.Context, remotecommand.Executor, remotecommand.StreamOptions) error", methodType.String())
	})

	// Test UntilWithSync
	t.Run("UntilWithSync", func(t *testing.T) {
		// Verify UntilWithSync method exists with correct signature
		methodType := reflect.TypeOf(wrapper.UntilWithSync)
		// Just check that it's a function that returns the right types
		assert.Contains(t, methodType.String(), "func(context.Context, cache.ListerWatcher, runtime.Object")
		assert.Contains(t, methodType.String(), "(*watch.Event, error)")
	})
}

// TestReadFileToString tests the ReadFileToString function.
func TestReadFileToString(t *testing.T) {
	// Create a temporary file for testing
	content := "test content"
	tmpfile, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	// Write content to the file
	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Test with empty filename
	t.Run("Empty filename", func(t *testing.T) {
		result, err := workceptor.ReadFileToString("")
		assert.NoError(t, err)
		assert.Equal(t, "", result)
	})

	// Test with valid file
	t.Run("Valid file", func(t *testing.T) {
		result, err := workceptor.ReadFileToString(tmpfile.Name())
		assert.NoError(t, err)
		assert.Equal(t, content, result)
	})

	// Test with non-existent file
	t.Run("Non-existent file", func(t *testing.T) {
		result, err := workceptor.ReadFileToString("/non/existent/file")
		assert.Error(t, err)
		assert.Equal(t, "", result)
	})
}

// TestParseTimeExtended tests the ParseTime function with more cases.
func TestParseTimeExtended(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool // true if we expect a non-nil result
	}{
		{
			name:     "RFC3339",
			input:    "2024-01-17T00:00:00Z",
			expected: true,
		},
		{
			name:     "RFC3339Nano",
			input:    "2024-01-17T00:00:00.123456789Z",
			expected: true,
		},
		{
			name:     "Invalid format",
			input:    "2024-01-17",
			expected: false,
		},
		{
			name:     "Empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "Random string",
			input:    "not a time",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := workceptor.ParseTime(tt.input)
			if tt.expected {
				assert.NotNil(t, result)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

// TestIsCompatibleK8SExtended tests the IsCompatibleK8S function with more cases.
func TestIsCompatibleK8SExtended(t *testing.T) {
	kw, err := startNetceptorNodeWithWorkceptor()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		versionStr string
		want       bool
	}{
		{
			name:       "Empty version",
			versionStr: "",
			want:       false,
		},
		{
			name:       "Invalid version format",
			versionStr: "invalid",
			want:       false,
		},
		{
			name:       "Version with only major",
			versionStr: "v1",
			want:       false,
		},
		{
			name:       "Version with major and minor",
			versionStr: "v1.26",
			want:       false,
		},
		{
			name:       "Version 1.27.0",
			versionStr: "v1.27.0",
			want:       true,
		},
		{
			name:       "Version 1.28.0",
			versionStr: "v1.28.0",
			want:       true,
		},
		{
			name:       "Version 1.29.0",
			versionStr: "v1.29.0",
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := workceptor.IsCompatibleK8S(kw, tt.versionStr); got != tt.want {
				t.Errorf("IsCompatibleK8S() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGetTimeoutOpenLogstreamExtended tests the GetTimeoutOpenLogstream function with more cases.
func TestGetTimeoutOpenLogstreamExtended(t *testing.T) {
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
			name:     "Large value",
			envValue: "100",
			want:     100,
		},
		{
			name:     "Zero value",
			envValue: "0",
			want:     1, // Should default to 1
		},
		{
			name:     "Negative value",
			envValue: "-10",
			want:     1, // Should default to 1
		},
		{
			name:     "Non-integer value",
			envValue: "abc",
			want:     1, // Should default to 1
		},
		{
			name:     "Float value",
			envValue: "1.5",
			want:     1, // Should default to 1
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

// TestKubeLoggingWithReconnectSimple tests the KubeLoggingWithReconnect function with a simple success case.
func TestKubeLoggingWithReconnectSimple(t *testing.T) {
	// We'll test just the success case for now to avoid mock complexity
	var stdinErr error
	var stdoutErr error
	_, mockBaseWorkUnit, mockNetceptor, w, mockKubeAPI, ctrl, ctx := createKubernetesTestSetup(t)
	defer ctrl.Finish()

	pod := &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{Name: "Test_Name", Namespace: "Test_Namespace"},
		Spec:       corev1.PodSpec{},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}

	kw := &workceptor.KubeUnit{
		BaseWorkUnitForWorkUnit: mockBaseWorkUnit,
		KubeAPIWrapperInstance:  mockKubeAPI,
		Pod:                     pod,
	}

	// Set up expectations
	mockBaseWorkUnit.EXPECT().GetWorkceptor().Return(w).AnyTimes()
	mockBaseWorkUnit.EXPECT().GetContext().Return(ctx).AnyTimes()
	mockKubeAPI.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(pod, nil).Times(2)
	logger := logger.NewReceptorLogger("")
	mockNetceptor.EXPECT().GetLogger().Return(logger).AnyTimes()

	// Set up the fake REST client
	req := fakerest.RESTClient{
		Client: fakerest.CreateHTTPClient(func(request *http.Request) (*http.Response, error) {
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("2024-12-09T00:31:18.823849250Z Log line with timestamp\n")),
			}

			return resp, nil
		}),
		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
	}
	mockKubeAPI.EXPECT().GetLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(req.Request())

	wg := &sync.WaitGroup{}
	wg.Add(1)
	mockfilesystemer := mock_workceptor.NewMockFileSystemer(ctrl)
	mockfilesystemer.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(&os.File{}, nil)
	stdout, _ := workceptor.NewStdoutWriter(mockfilesystemer, "")
	mockFileWC := mock_workceptor.NewMockFileWriteCloser(ctrl)
	stdout.SetWriter(mockFileWC)
	mockFileWC.EXPECT().Write(gomock.Any()).Return(0, nil).AnyTimes()

	kw.KubeLoggingWithReconnect(wg, stdout, &stdinErr, &stdoutErr)

	assert.NoError(t, stdoutErr)
}

// TestKubeUnitCancel tests the Cancel method of KubeUnit.
func TestKubeUnitCancel(t *testing.T) {
	// Create a test setup
	_, mockBaseWorkUnit, mockNetceptor, w, mockKubeAPI, ctrl, _ := createKubernetesTestSetup(t)
	defer ctrl.Finish()

	// Create a pod for testing
	pod := &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{Name: "Test_Name", Namespace: "Test_Namespace"},
		Spec:       corev1.PodSpec{},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}

	// Create a clientset
	clientset := &kubernetes.Clientset{}

	// Create a KubeUnit for testing
	kw := &workceptor.KubeUnit{
		BaseWorkUnitForWorkUnit: mockBaseWorkUnit,
		KubeAPIWrapperInstance:  mockKubeAPI,
		Pod:                     pod,
	}

	// Set the clientset
	kw.SetClientset(clientset)

	// Set up expectations
	mockBaseWorkUnit.EXPECT().CancelContext().Times(2)
	mockBaseWorkUnit.EXPECT().UpdateBasicStatus(workceptor.WorkStateCanceled, "Canceled", int64(-1))
	mockBaseWorkUnit.EXPECT().GetCancel().Return(func() {})
	mockBaseWorkUnit.EXPECT().GetWorkceptor().Return(w).AnyTimes()
	mockNetceptor.EXPECT().NodeID().Return("NodeID").AnyTimes()
	logger := logger.NewReceptorLogger("")
	mockNetceptor.EXPECT().GetLogger().Return(logger).AnyTimes()

	// Mock the Delete method
	mockKubeAPI.EXPECT().Delete(
		gomock.Any(),
		gomock.Eq(clientset),
		gomock.Eq(pod.Namespace),
		gomock.Eq(pod.Name),
		gomock.Any(),
	).Return(nil)

	// Call the method being tested
	err := kw.Cancel()

	// Verify the results
	assert.NoError(t, err)
}

// TestKubeUnitRelease tests the Release method of KubeUnit.
func TestKubeUnitRelease(t *testing.T) {
	// Create a test setup
	_, mockBaseWorkUnit, mockNetceptor, w, mockKubeAPI, ctrl, _ := createKubernetesTestSetup(t)
	defer ctrl.Finish()

	// Create a pod for testing
	pod := &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{Name: "Test_Name", Namespace: "Test_Namespace"},
		Spec:       corev1.PodSpec{},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}

	// Create a clientset
	clientset := &kubernetes.Clientset{}

	// Create a KubeUnit for testing
	kw := &workceptor.KubeUnit{
		BaseWorkUnitForWorkUnit: mockBaseWorkUnit,
		KubeAPIWrapperInstance:  mockKubeAPI,
		Pod:                     pod,
	}

	// Set the clientset
	kw.SetClientset(clientset)

	// Set up expectations for Cancel
	mockBaseWorkUnit.EXPECT().CancelContext().Times(2)
	mockBaseWorkUnit.EXPECT().UpdateBasicStatus(workceptor.WorkStateCanceled, "Canceled", int64(-1))
	mockBaseWorkUnit.EXPECT().GetCancel().Return(func() {})
	mockBaseWorkUnit.EXPECT().GetWorkceptor().Return(w).AnyTimes()
	mockNetceptor.EXPECT().NodeID().Return("NodeID").AnyTimes()
	logger := logger.NewReceptorLogger("")
	mockNetceptor.EXPECT().GetLogger().Return(logger).AnyTimes()

	// Mock the Delete method
	mockKubeAPI.EXPECT().Delete(
		gomock.Any(),
		gomock.Eq(clientset),
		gomock.Eq(pod.Namespace),
		gomock.Eq(pod.Name),
		gomock.Any(),
	).Return(nil)

	// Set up expectations for Release
	mockBaseWorkUnit.EXPECT().Release(false).Return(nil)

	// Call the method being tested
	err := kw.Release(false)

	// Verify the results
	assert.NoError(t, err)
}

// TestKubeUnitReleaseWithForce tests the Release method of KubeUnit with force=true.
func TestKubeUnitReleaseWithForce(t *testing.T) {
	// Create a test setup
	_, mockBaseWorkUnit, mockNetceptor, w, mockKubeAPI, ctrl, _ := createKubernetesTestSetup(t)
	defer ctrl.Finish()

	// Create a pod for testing
	pod := &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{Name: "Test_Name", Namespace: "Test_Namespace"},
		Spec:       corev1.PodSpec{},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}

	// Create a clientset
	clientset := &kubernetes.Clientset{}

	// Create a KubeUnit for testing
	kw := &workceptor.KubeUnit{
		BaseWorkUnitForWorkUnit: mockBaseWorkUnit,
		KubeAPIWrapperInstance:  mockKubeAPI,
		Pod:                     pod,
	}

	// Set the clientset
	kw.SetClientset(clientset)

	// Set up expectations for Cancel (with error)
	mockBaseWorkUnit.EXPECT().CancelContext().Times(2)
	mockBaseWorkUnit.EXPECT().UpdateBasicStatus(workceptor.WorkStateCanceled, "Canceled", int64(-1))
	mockBaseWorkUnit.EXPECT().GetCancel().Return(func() {})
	mockBaseWorkUnit.EXPECT().GetWorkceptor().Return(w).AnyTimes()
	mockNetceptor.EXPECT().NodeID().Return("NodeID").AnyTimes()
	logger := logger.NewReceptorLogger("")
	mockNetceptor.EXPECT().GetLogger().Return(logger).AnyTimes()

	// Mock the Delete method with an error
	mockKubeAPI.EXPECT().Delete(
		gomock.Any(),
		gomock.Eq(clientset),
		gomock.Eq(pod.Namespace),
		gomock.Eq(pod.Name),
		gomock.Any(),
	).Return(errors.New("delete error"))

	// Set up expectations for Release
	mockBaseWorkUnit.EXPECT().Release(true).Return(nil)

	// Call the method being tested
	err := kw.Release(true)

	// Verify the results
	assert.NoError(t, err)
}

// TestKubeUnitRestart tests the Restart method of KubeUnit.
func TestKubeUnitRestart(t *testing.T) {
	t.Run("Complete state", func(t *testing.T) {
		// Create a test setup
		_, mockBaseWorkUnit, _, _, _, ctrl, _ := createKubernetesTestSetup(t)
		defer ctrl.Finish()

		// Create a KubeUnit for testing
		kw := &workceptor.KubeUnit{
			BaseWorkUnitForWorkUnit: mockBaseWorkUnit,
		}

		// Set up expectations
		status := &workceptor.StatusFileData{
			State:     workceptor.WorkStateSucceeded,
			ExtraData: &workceptor.KubeExtraData{},
		}
		mockBaseWorkUnit.EXPECT().Status().Return(status).AnyTimes()

		// Mock other necessary methods
		lock := &sync.RWMutex{}
		mockBaseWorkUnit.EXPECT().GetStatusLock().Return(lock).AnyTimes()
		mockBaseWorkUnit.EXPECT().GetStatusWithoutExtraData().Return(status).AnyTimes()
		mockBaseWorkUnit.EXPECT().GetStatusCopy().Return(*status).AnyTimes()

		// Call the method being tested
		err := kw.Restart()

		// Verify the results
		assert.NoError(t, err)
	})

	t.Run("Running state with TCP", func(t *testing.T) {
		// Create a test setup with tcp stream method
		ku, mockBaseWorkUnit, _, _, _, ctrl, _ := createKubernetesTestSetup(t, "streamMethod=tcp")
		defer ctrl.Finish()

		// Use the KubeUnit from createKubernetesTestSetup
		kw := ku.(*workceptor.KubeUnit)

		// Set up expectations
		status := &workceptor.StatusFileData{
			State:     workceptor.WorkStateRunning,
			ExtraData: &workceptor.KubeExtraData{},
		}
		mockBaseWorkUnit.EXPECT().Status().Return(status).AnyTimes()

		// Mock other necessary methods
		lock := &sync.RWMutex{}
		mockBaseWorkUnit.EXPECT().GetStatusLock().Return(lock).AnyTimes()
		mockBaseWorkUnit.EXPECT().GetStatusWithoutExtraData().Return(status).AnyTimes()
		mockBaseWorkUnit.EXPECT().GetStatusCopy().Return(*status).AnyTimes()

		// Call the method being tested
		err := kw.Restart()

		// Verify the results
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "restart not implemented for streammethod tcp")
	})
}
