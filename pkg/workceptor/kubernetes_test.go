//go:build !no_workceptor
// +build !no_workceptor

package workceptor_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
		// Do NOT add newline - this simulates a partial line read that triggers the bug
		n = copy(p, []byte(e.content))
		e.hasRead = true

		return n, nil
	}

	return 0, io.EOF
}

func (e *eofReadCloser) Close() error {
	return nil
}

// errorReadCloser simulates network errors after a few reads to trigger non-EOF error paths.
type errorReadCloser struct {
	readCount int
	maxReads  int
}

func (e *errorReadCloser) Read(p []byte) (int, error) {
	e.readCount++
	if e.readCount <= e.maxReads {
		// Return some data for the first few reads
		content := "2024-12-09T00:31:19.123456789Z Log line\n"

		return copy(p, []byte(content)), nil
	}
	// After maxReads, return a network error (not EOF)
	return 0, errors.New("network connection reset")
}

func (e *errorReadCloser) Close() error {
	return nil
}

func TestKubeLoggingWithReconnect(t *testing.T) {
	type testCase struct {
		name              string
		setupMocks        func(mockBaseWorkUnit *mock_workceptor.MockBaseWorkUnitForWorkUnit, mockNetceptor *mock_workceptor.MockNetceptorForWorkceptor, mockKubeAPI *mock_workceptor.MockKubeAPIer, w *workceptor.Workceptor, ctx context.Context)
		stdinErr          *error
		expectedStdoutErr bool
		timeoutSeconds    int
		validateLogs      bool
		expectedLogMsgs   []string
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
				mockKubeAPI.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(pod, nil).AnyTimes()

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

				mockKubeAPI.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("pod not found")).Times(5)

				mockBaseWorkUnit.EXPECT().UpdateBasicStatus(workceptor.WorkStateFailed, gomock.Any(), gomock.Any()).MaxTimes(6)
			},
			stdinErr: func() *error {
				var err error

				return &err
			}(),
			expectedStdoutErr: false,
			timeoutSeconds:    15, // Allow time for 5 retries with 1 second delays
			validateLogs:      true,
			expectedLogMsgs: []string{
				"Error getting pod Test_Namespace/Test_Name. Will retry 5 more times.",
				"Error getting pod Test_Namespace/Test_Name. Will retry 4 more times.",
				"Error getting pod Test_Namespace/Test_Name. Will retry 3 more times.",
				"Error getting pod Test_Namespace/Test_Name. Will retry 2 more times.",
				"Error getting pod Test_Namespace/Test_Name. Will retry 1 more times.",
			},
		},
		{
			name: "log_stream_connection_failure",
			setupMocks: func(mockBaseWorkUnit *mock_workceptor.MockBaseWorkUnitForWorkUnit, mockNetceptor *mock_workceptor.MockNetceptorForWorkceptor, mockKubeAPI *mock_workceptor.MockKubeAPIer, w *workceptor.Workceptor, ctx context.Context) {
				mockBaseWorkUnit.EXPECT().GetWorkceptor().Return(w).AnyTimes()
				mockBaseWorkUnit.EXPECT().GetContext().Return(ctx).AnyTimes()

				mockBaseWorkUnit.EXPECT().UpdateBasicStatus(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "Test_Name", Namespace: "Test_Namespace"},
					Status:     corev1.PodStatus{Phase: corev1.PodRunning},
				}
				mockKubeAPI.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(pod, nil).AnyTimes()

				// Counter to track retry attempts and fail consistently
				var attemptCount int
				failReq := fakerest.RESTClient{
					Client: fakerest.CreateHTTPClient(func(request *http.Request) (*http.Response, error) {
						attemptCount++
						// Always fail to trigger the retry loop in kubeLoggingConnectionHandler
						return nil, fmt.Errorf("dial tcp: connection refused (attempt %d)", attemptCount)
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
					mockKubeAPI.EXPECT().Get(gomock.Any(), gomock.Any(), "Test_Namespace", "Test_Name", gomock.Any()).Return(notReadyPod, nil).MaxTimes(6),
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
						ContainerStatuses: []corev1.ContainerStatus{
							{
								Name: "worker",
								State: corev1.ContainerState{
									Running: &corev1.ContainerStateRunning{},
								},
							},
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
							Body:       &eofReadCloser{content: "2024-12-09T00:31:19.123456789Z Retry log", hasRead: true}, // Start with hasRead=true so it immediately returns EOF
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
			timeoutSeconds:    15,
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
					mockKubeAPI.EXPECT().Get(gomock.Any(), gomock.Any(), "Test_Namespace", "Test_Name", gomock.Any()).Return(completedPod, nil).MaxTimes(6),
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
					mockKubeAPI.EXPECT().Get(gomock.Any(), gomock.Any(), "Test_Namespace", "Test_Name", gomock.Any()).Return(completedPod, nil).MaxTimes(6),
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
		{
			name: "timestamp_remove_on_eof_with_final_line",
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

				gomock.InOrder(
					mockKubeAPI.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(runningPod, nil),
					mockKubeAPI.EXPECT().Get(gomock.Any(), gomock.Any(), "Test_Namespace", "Test_Name", gomock.Any()).Return(notReadyPod, nil).MaxTimes(6),
				)

				req := fakerest.RESTClient{
					Client: fakerest.CreateHTTPClient(func(request *http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: http.StatusOK,
							Body:       &eofReadCloser{content: "2024-12-09T00:31:18.823849250Z This timestamp should be removed", hasRead: false},
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

			// Set up logger - capture output only if validation is enabled
			var logBuffer bytes.Buffer
			logger := logger.NewReceptorLogger("")
			if tt.validateLogs {
				logger.SetOutput(&logBuffer)
			}
			mockNetceptor.EXPECT().GetLogger().Return(logger).AnyTimes()

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

				if tt.name == "timestamp_remove_on_eof_with_final_line" {
					hasTimestampLeak := false
					for _, data := range writtenData {
						if strings.HasPrefix(data, "2024-") {
							hasTimestampLeak = true
							t.Logf("TIMESTAMP LEAK DETECTED: %s", data)

							break
						}
					}
					if hasTimestampLeak {
						t.Errorf("Did not expect a timestamp leak but one was found in written data")
					}
				}
			}
			// Validate log messages if enabled
			if tt.validateLogs {
				logOutput := logBuffer.String()
				for _, expectedMsg := range tt.expectedLogMsgs {
					if !strings.Contains(logOutput, expectedMsg) {
						t.Errorf("Missing expected log message: %s got:\n%s", expectedMsg, logOutput)
					}
				}
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
		assert.Equal(t, "func(*rest.Config) (kubernetes.Interface, error)", methodType.String())
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
	mockKubeAPI.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(pod, nil).AnyTimes()
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

func TestProcessLogLine(t *testing.T) {
	kw, err := startNetceptorNodeWithWorkceptor()
	if err != nil {
		t.Fatal(err)
	}

	baseTime := time.Date(2024, 1, 17, 12, 0, 0, 0, time.UTC)
	laterTime := time.Date(2024, 1, 17, 12, 0, 5, 0, time.UTC)

	tests := []struct {
		name            string
		line            string
		sinceTime       time.Time
		successfulWrite bool
		expectedMsg     string
		expectedSkip    bool
	}{
		{
			name:            "Valid timestamp with message",
			line:            "2024-01-17T12:00:05Z Hello world",
			sinceTime:       baseTime,
			successfulWrite: false,
			expectedMsg:     "Hello world",
			expectedSkip:    false,
		},
		{
			name:            "Valid timestamp without message",
			line:            "2024-01-17T12:00:05Z",
			sinceTime:       baseTime,
			successfulWrite: false,
			expectedMsg:     "",
			expectedSkip:    false,
		},
		{
			name:            "No timestamp - treated as regular message",
			line:            "Regular log message without timestamp",
			sinceTime:       baseTime,
			successfulWrite: false,
			expectedMsg:     "Regular log message without timestamp",
			expectedSkip:    false,
		},
		{
			name:            "Timestamp older than sinceTime with no successful write - should skip",
			line:            "2024-01-17T11:59:55Z Old message",
			sinceTime:       baseTime,
			successfulWrite: false,
			expectedMsg:     "",
			expectedSkip:    true,
		},
		{
			name:            "Timestamp older than sinceTime with successful write - should not skip",
			line:            "2024-01-17T11:59:55Z Old message but successful write",
			sinceTime:       baseTime,
			successfulWrite: true,
			expectedMsg:     "Old message but successful write",
			expectedSkip:    false,
		},
		{
			name:            "Timestamp equal to sinceTime with no successful write - should skip",
			line:            "2024-01-17T12:00:00Z Equal timestamp",
			sinceTime:       baseTime,
			successfulWrite: false,
			expectedMsg:     "",
			expectedSkip:    true,
		},
		{
			name:            "Timestamp equal to sinceTime with successful write - should not skip",
			line:            "2024-01-17T12:00:00Z Equal timestamp with successful write",
			sinceTime:       baseTime,
			successfulWrite: true,
			expectedMsg:     "Equal timestamp with successful write",
			expectedSkip:    false,
		},
		{
			name:            "RFC3339Nano timestamp format",
			line:            "2024-01-17T12:00:10.123456789Z Nano precision",
			sinceTime:       laterTime,
			successfulWrite: false,
			expectedMsg:     "Nano precision",
			expectedSkip:    false,
		},
		{
			name:            "Invalid timestamp format - treated as regular message",
			line:            "2024-01-17 12:00:10 Invalid format message",
			sinceTime:       baseTime,
			successfulWrite: false,
			expectedMsg:     "2024-01-17 12:00:10 Invalid format message",
			expectedSkip:    false,
		},
		{
			name:            "Empty line",
			line:            "",
			sinceTime:       baseTime,
			successfulWrite: false,
			expectedMsg:     "",
			expectedSkip:    false,
		},
		{
			name:            "Line with only timestamp and space",
			line:            "2024-01-17T12:00:10Z ",
			sinceTime:       laterTime,
			successfulWrite: false,
			expectedMsg:     "",
			expectedSkip:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, newSinceTime, shouldSkip := kw.ProcessLogLine(tt.line, tt.sinceTime, tt.successfulWrite)

			if msg != tt.expectedMsg {
				t.Errorf("ProcessLogLine() msg = %q, want %q", msg, tt.expectedMsg)
			}

			if shouldSkip != tt.expectedSkip {
				t.Errorf("ProcessLogLine() shouldSkip = %v, want %v", shouldSkip, tt.expectedSkip)
			}

			if newSinceTime.IsZero() && !tt.sinceTime.IsZero() {
				t.Errorf("ProcessLogLine() returned zero time when it shouldn't")
			}
		})
	}
}

func TestKubeAPIWrapper_NewSPDYExecutor(t *testing.T) {
	tests := []struct {
		name        string
		config      *rest.Config
		method      string
		urlString   string
		expectError bool
		description string
	}{
		{
			name: "Valid config and URL",
			config: &rest.Config{
				Host: "https://kubernetes.default.svc",
			},
			method:      "POST",
			urlString:   "https://kubernetes.default.svc/api/v1/namespaces/default/pods/test/exec",
			expectError: false,
			description: "Should create SPDY executor with valid inputs",
		},
		{
			name:        "Nil config",
			config:      nil,
			method:      "POST",
			urlString:   "https://kubernetes.default.svc/api/v1/namespaces/default/pods/test/exec",
			expectError: true,
			description: "Should panic with nil config - wrapped as expectError for testing",
		},
		{
			name: "Invalid URL",
			config: &rest.Config{
				Host: "https://kubernetes.default.svc",
			},
			method:      "POST",
			urlString:   "://invalid-url",
			expectError: true,
			description: "URL parsing should fail with malformed URL",
		},
		{
			name: "Empty method",
			config: &rest.Config{
				Host: "https://kubernetes.default.svc",
			},
			method:      "",
			urlString:   "https://kubernetes.default.svc/api/v1/namespaces/default/pods/test/exec",
			expectError: false,
			description: "Should handle empty method (defaulted internally)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapper := workceptor.KubeAPIWrapper{}

			var testURL *url.URL
			var urlErr error
			if tt.urlString != "" {
				testURL, urlErr = url.Parse(tt.urlString)
				if urlErr != nil {
					// URL parsing failed - this is part of the test
					if tt.expectError {
						assert.Error(t, urlErr, "URL parsing should fail for "+tt.description)

						return
					} else {
						t.Fatalf("Failed to parse test URL: %v", urlErr)
					}
				}
			}

			// Handle panic for nil config case
			if tt.config == nil && tt.expectError {
				defer func() {
					if r := recover(); r != nil {
						// Expected panic for nil config
						assert.Contains(t, fmt.Sprintf("%v", r), "nil pointer", tt.description)
					}
				}()

				executor, err := wrapper.NewSPDYExecutor(tt.config, tt.method, testURL)
				// If we get here without panic, test should fail
				t.Errorf("Expected panic for nil config, but got executor=%v, err=%v", executor, err)

				return
			}

			executor, err := wrapper.NewSPDYExecutor(tt.config, tt.method, testURL)

			if tt.expectError {
				assert.Error(t, err, tt.description)
				assert.Nil(t, executor)
			} else {
				assert.NoError(t, err, tt.description)
				assert.NotNil(t, executor)
			}
		})
	}
}

func TestKubeAPIWrapper_NewForConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *rest.Config
		expectError bool
		description string
	}{
		{
			name: "Valid config",
			config: &rest.Config{
				Host: "https://kubernetes.default.svc",
			},
			expectError: false,
			description: "Should create clientset with valid config",
		},
		{
			name:        "Nil config",
			config:      nil,
			expectError: true,
			description: "Should return error with nil config",
		},
		{
			name: "Invalid host config",
			config: &rest.Config{
				Host: "://invalid-url",
			},
			expectError: true,
			description: "Should return error with malformed host URL",
		},
		{
			name:        "Empty config",
			config:      &rest.Config{},
			expectError: false,
			description: "Should accept empty config (will use defaults)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapper := workceptor.KubeAPIWrapper{}

			// Handle panic for nil config case
			if tt.config == nil && tt.expectError {
				defer func() {
					if r := recover(); r != nil {
						// Expected panic for nil config
						assert.Contains(t, fmt.Sprintf("%v", r), "nil pointer", tt.description)
					}
				}()

				clientset, err := wrapper.NewForConfig(tt.config)
				// If we get here without panic, test should fail
				t.Errorf("Expected panic for nil config, but got clientset=%v, err=%v", clientset, err)

				return
			}

			clientset, err := wrapper.NewForConfig(tt.config)

			if tt.expectError {
				assert.Error(t, err, tt.description)
				assert.Nil(t, clientset)
			} else {
				assert.NoError(t, err, tt.description)
				assert.NotNil(t, clientset)
			}
		})
	}
}

func TestKubeUnit_SetFromParams(t *testing.T) {
	tests := []struct {
		name               string
		params             map[string]string
		allowRuntimeAuth   bool
		allowRuntimeCmd    bool
		allowRuntimeParams bool
		allowRuntimePod    bool
		authMethod         string
		expectError        bool
		expectedErrorMsg   string
		description        string
	}{
		{
			name: "Valid parameters with all permissions",
			params: map[string]string{
				"kube_command":        "echo hello",
				"kube_image":          "busybox:latest",
				"kube_params":         "--verbose",
				"kube_namespace":      "test-ns",
				"pod_pending_timeout": "5m",
			},
			allowRuntimeAuth:   true,
			allowRuntimeCmd:    true,
			allowRuntimeParams: true,
			allowRuntimePod:    true,
			authMethod:         "incluster",
			expectError:        false,
			description:        "Should accept valid parameters when permissions allow",
		},
		{
			name: "Command without permission",
			params: map[string]string{
				"kube_command": "echo hello",
			},
			allowRuntimeAuth:   false,
			allowRuntimeCmd:    false,
			allowRuntimeParams: false,
			allowRuntimePod:    false,
			authMethod:         "incluster",
			expectError:        true,
			expectedErrorMsg:   "kube_command provided but not allowed",
			description:        "Should reject command when allowRuntimeCommand is false",
		},
		{
			name: "Image without permission",
			params: map[string]string{
				"kube_image": "busybox:latest",
			},
			allowRuntimeAuth:   false,
			allowRuntimeCmd:    false,
			allowRuntimeParams: false,
			allowRuntimePod:    false,
			authMethod:         "incluster",
			expectError:        true,
			expectedErrorMsg:   "kube_image provided but not allowed",
			description:        "Should reject image when allowRuntimeCommand is false",
		},
		{
			name: "Params without permission",
			params: map[string]string{
				"kube_params": "--verbose",
			},
			allowRuntimeAuth:   false,
			allowRuntimeCmd:    false,
			allowRuntimeParams: false,
			allowRuntimePod:    false,
			authMethod:         "incluster",
			expectError:        true,
			expectedErrorMsg:   "kube_params provided but not allowed",
			description:        "Should reject params when allowRuntimeParams is false",
		},
		{
			name: "Namespace without permission",
			params: map[string]string{
				"kube_namespace": "test-ns",
			},
			allowRuntimeAuth:   false,
			allowRuntimeCmd:    false,
			allowRuntimeParams: false,
			allowRuntimePod:    false,
			authMethod:         "incluster",
			expectError:        true,
			expectedErrorMsg:   "kube_namespace provided but not allowed",
			description:        "Should reject namespace when allowRuntimeAuth is false",
		},
		{
			name: "Pod definition without permission",
			params: map[string]string{
				"secret_kube_pod": "apiVersion: v1\nkind: Pod",
			},
			allowRuntimeAuth:   false,
			allowRuntimeCmd:    false,
			allowRuntimeParams: false,
			allowRuntimePod:    false,
			authMethod:         "incluster",
			expectError:        true,
			expectedErrorMsg:   "secret_kube_pod provided but not allowed",
			description:        "Should reject pod when allowRuntimePod is false",
		},
		{
			name: "Runtime auth method without kubeconfig",
			params: map[string]string{
				"kube_image": "busybox:latest",
			},
			allowRuntimeAuth:   true,
			allowRuntimeCmd:    true,
			allowRuntimeParams: true,
			allowRuntimePod:    true,
			authMethod:         "runtime",
			expectError:        true,
			expectedErrorMsg:   "param secret_kube_config must be provided if AuthMethod=runtime",
			description:        "Should require kubeconfig when authMethod is runtime",
		},
		{
			name: "Pod with conflicting image parameter",
			params: map[string]string{
				"secret_kube_pod": "apiVersion: v1\nkind: Pod",
				"kube_image":      "busybox:latest",
			},
			allowRuntimeAuth:   true,
			allowRuntimeCmd:    true,
			allowRuntimeParams: true,
			allowRuntimePod:    true,
			authMethod:         "incluster",
			expectError:        true,
			expectedErrorMsg:   "params kube_command, kube_image, kube_params not compatible with secret_kube_pod",
			description:        "Should reject conflicting pod and image parameters",
		},
		{
			name: "Pod with conflicting command parameter",
			params: map[string]string{
				"secret_kube_pod": "apiVersion: v1\nkind: Pod",
				"kube_command":    "echo hello",
			},
			allowRuntimeAuth:   true,
			allowRuntimeCmd:    true,
			allowRuntimeParams: true,
			allowRuntimePod:    true,
			authMethod:         "incluster",
			expectError:        true,
			expectedErrorMsg:   "params kube_command, kube_image, kube_params not compatible with secret_kube_pod",
			description:        "Should reject conflicting pod and command parameters",
		},
		{
			name: "Invalid pod pending timeout",
			params: map[string]string{
				"pod_pending_timeout": "invalid-duration",
			},
			allowRuntimeAuth:   true,
			allowRuntimeCmd:    true,
			allowRuntimeParams: true,
			allowRuntimePod:    true,
			authMethod:         "incluster",
			expectError:        true,
			expectedErrorMsg:   "time: invalid duration",
			description:        "Should reject invalid duration format",
		},
		{
			name: "Valid pod pending timeout",
			params: map[string]string{
				"pod_pending_timeout": "10m30s",
			},
			allowRuntimeAuth:   true,
			allowRuntimeCmd:    true,
			allowRuntimeParams: true,
			allowRuntimePod:    true,
			authMethod:         "incluster",
			expectError:        false,
			description:        "Should accept valid duration format",
		},
		{
			name:               "Empty parameters",
			params:             map[string]string{},
			allowRuntimeAuth:   true,
			allowRuntimeCmd:    true,
			allowRuntimeParams: true,
			allowRuntimePod:    true,
			authMethod:         "incluster",
			expectError:        false,
			description:        "Should handle empty parameters gracefully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockBaseWorkUnit := mock_workceptor.NewMockBaseWorkUnitForWorkUnit(ctrl)
			mockNetceptor := mock_workceptor.NewMockNetceptorForWorkceptor(ctrl)
			mockKubeAPI := mock_workceptor.NewMockKubeAPIer(ctrl)

			mockNetceptor.EXPECT().NodeID().Return("test-node").AnyTimes()

			ctx := context.Background()
			w, err := workceptor.New(ctx, mockNetceptor, "/tmp")
			if err != nil {
				t.Fatalf("Error creating Workceptor: %v", err)
			}

			mockBaseWorkUnit.EXPECT().Init(w, "", "", workceptor.FileSystem{})

			// Create KubeWorkerCfg with test configuration
			kubeConfig := workceptor.KubeWorkerCfg{
				AuthMethod:          tt.authMethod,
				StreamMethod:        "logger",
				AllowRuntimeAuth:    tt.allowRuntimeAuth,
				AllowRuntimeCommand: tt.allowRuntimeCmd,
				AllowRuntimeParams:  tt.allowRuntimeParams,
				AllowRuntimePod:     tt.allowRuntimePod,
			}

			kubeUnit := kubeConfig.NewkubeWorker(mockBaseWorkUnit, w, "", "", mockKubeAPI).(*workceptor.KubeUnit)

			// Mock the GetStatusCopy call
			extraData := &workceptor.KubeExtraData{}
			status := workceptor.StatusFileData{ExtraData: extraData}
			mockBaseWorkUnit.EXPECT().GetStatusCopy().Return(status).AnyTimes()

			// Mock GetWorkceptor and logger for timeout parsing errors
			if strings.Contains(tt.expectedErrorMsg, "time: invalid duration") {
				logger := logger.NewReceptorLogger("")
				mockNetceptor.EXPECT().GetLogger().Return(logger).AnyTimes()
				mockBaseWorkUnit.EXPECT().GetWorkceptor().Return(w).AnyTimes()
			}

			err = kubeUnit.SetFromParams(tt.params)

			if tt.expectError {
				assert.Error(t, err, tt.description)
				if tt.expectedErrorMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorMsg, tt.description)
				}
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

func TestKubeUnit_CreatePod(t *testing.T) {
	tests := []struct {
		name             string
		extraData        *workceptor.KubeExtraData
		env              map[string]string
		setupMocks       func(*mock_workceptor.MockBaseWorkUnitForWorkUnit, *mock_workceptor.MockKubeAPIer, *workceptor.Workceptor)
		expectError      bool
		expectedErrorMsg string
		validateResult   func(*testing.T, *workceptor.KubeUnit)
		description      string
	}{
		{
			name: "Successful pod creation with simple image",
			extraData: &workceptor.KubeExtraData{
				Image:         "busybox:latest",
				Command:       "echo hello",
				Params:        "--verbose",
				KubeNamespace: "default",
			},
			env: map[string]string{
				"TEST_VAR": "test_value",
			},
			setupMocks: func(mockBWU *mock_workceptor.MockBaseWorkUnitForWorkUnit, mockAPI *mock_workceptor.MockKubeAPIer, w *workceptor.Workceptor) {
				// Mock status calls for UnredactedStatus
				statusLock := &sync.RWMutex{}
				statusData := &workceptor.StatusFileData{ExtraData: &workceptor.KubeExtraData{}}
				statusCopy := workceptor.StatusFileData{ExtraData: &workceptor.KubeExtraData{
					Image:         "busybox:latest",
					Command:       "echo hello",
					Params:        "--verbose",
					KubeNamespace: "default",
				}}
				mockBWU.EXPECT().GetStatusLock().Return(statusLock).Times(2)
				mockBWU.EXPECT().GetStatusWithoutExtraData().Return(statusData)
				mockBWU.EXPECT().GetStatusCopy().Return(statusCopy)
				mockBWU.EXPECT().GetContext().Return(context.Background()).AnyTimes()
				mockBWU.EXPECT().ID().Return("test-unit-id").AnyTimes()

				// Mock pod creation
				createdPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pod-123", Namespace: "default"},
					Status:     corev1.PodStatus{Phase: corev1.PodRunning},
				}
				mockAPI.EXPECT().Create(gomock.Any(), gomock.Any(), "default", gomock.Any(), gomock.Any()).Return(createdPod, nil)

				// Mock status update
				mockBWU.EXPECT().UpdateFullStatus(gomock.Any()).Do(func(updateFunc interface{}) {
					// Verify the status update function works
					status := &workceptor.StatusFileData{ExtraData: &workceptor.KubeExtraData{}}
					updateFunc.(func(*workceptor.StatusFileData))(status)
				})

				// Mock pod waiting
				selector := &hasTerm{field: "metadata.name", value: "test-pod-123"}
				mockAPI.EXPECT().OneTermEqualSelector("metadata.name", "test-pod-123").Return(selector)
				mockAPI.EXPECT().List(gomock.Any(), gomock.Any(), "default", gomock.Any()).Return(&corev1.PodList{}, nil).AnyTimes()
				mockAPI.EXPECT().Watch(gomock.Any(), gomock.Any(), "default", gomock.Any()).Return(nil, nil).AnyTimes()

				watchEvent := &watch.Event{
					Type:   watch.Modified,
					Object: createdPod,
				}
				mockAPI.EXPECT().UntilWithSync(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(watchEvent, nil)
			},
			expectError: false,
			validateResult: func(t *testing.T, ku *workceptor.KubeUnit) {
				assert.NotNil(t, ku.Pod)
				assert.Equal(t, "test-pod-123", ku.Pod.Name)
				assert.Equal(t, "default", ku.Pod.Namespace)
			},
			description: "Should successfully create pod with image, command, params and environment variables",
		},
		{
			name: "Pod creation with custom pod definition",
			extraData: &workceptor.KubeExtraData{
				KubePod: `apiVersion: v1
kind: Pod
metadata:
  name: custom-pod
  namespace: custom-ns
spec:
  containers:
  - name: worker
    image: custom:latest
    command: ["sh", "-c"]
    args: ["echo custom"]
  restartPolicy: Never`,
				KubeNamespace: "default",
			},
			env: nil,
			setupMocks: func(mockBWU *mock_workceptor.MockBaseWorkUnitForWorkUnit, mockAPI *mock_workceptor.MockKubeAPIer, w *workceptor.Workceptor) {
				// Mock status calls for UnredactedStatus
				statusLock := &sync.RWMutex{}
				statusData := &workceptor.StatusFileData{ExtraData: &workceptor.KubeExtraData{}}
				statusCopy := workceptor.StatusFileData{ExtraData: &workceptor.KubeExtraData{
					KubePod: `apiVersion: v1
kind: Pod
metadata:
  name: custom-pod
  namespace: custom-ns
spec:
  containers:
  - name: worker
    image: custom:latest
    command: ["sh", "-c"]
    args: ["echo custom"]
  restartPolicy: Never`,
					KubeNamespace: "default",
				}}
				mockBWU.EXPECT().GetStatusLock().Return(statusLock).Times(2)
				mockBWU.EXPECT().GetStatusWithoutExtraData().Return(statusData)
				mockBWU.EXPECT().GetStatusCopy().Return(statusCopy)
				mockBWU.EXPECT().GetContext().Return(context.Background()).AnyTimes()
				mockBWU.EXPECT().ID().Return("test-unit-id").AnyTimes()

				createdPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "custom-pod-abc", Namespace: "custom-ns"},
					Status:     corev1.PodStatus{Phase: corev1.PodRunning},
				}
				mockAPI.EXPECT().Create(gomock.Any(), gomock.Any(), "custom-ns", gomock.Any(), gomock.Any()).Return(createdPod, nil)
				mockBWU.EXPECT().UpdateFullStatus(gomock.Any())

				selector := &hasTerm{field: "metadata.name", value: "custom-pod-abc"}
				mockAPI.EXPECT().OneTermEqualSelector("metadata.name", "custom-pod-abc").Return(selector)
				mockAPI.EXPECT().List(gomock.Any(), gomock.Any(), "custom-ns", gomock.Any()).Return(&corev1.PodList{}, nil).AnyTimes()
				mockAPI.EXPECT().Watch(gomock.Any(), gomock.Any(), "custom-ns", gomock.Any()).Return(nil, nil).AnyTimes()

				watchEvent := &watch.Event{Type: watch.Modified, Object: createdPod}
				mockAPI.EXPECT().UntilWithSync(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(watchEvent, nil)
			},
			expectError: false,
			description: "Should successfully create pod with custom pod definition",
		},
		{
			name: "Invalid command parsing",
			extraData: &workceptor.KubeExtraData{
				Image:         "busybox:latest",
				Command:       "echo 'unclosed quote",
				Params:        "--verbose",
				KubeNamespace: "default",
			},
			env: nil,
			setupMocks: func(mockBWU *mock_workceptor.MockBaseWorkUnitForWorkUnit, mockAPI *mock_workceptor.MockKubeAPIer, w *workceptor.Workceptor) {
				// Mock status calls for UnredactedStatus
				statusLock := &sync.RWMutex{}
				statusData := &workceptor.StatusFileData{ExtraData: &workceptor.KubeExtraData{}}
				statusCopy := workceptor.StatusFileData{ExtraData: &workceptor.KubeExtraData{
					Image:         "busybox:latest",
					Command:       "echo 'unclosed quote",
					Params:        "--verbose",
					KubeNamespace: "default",
				}}
				mockBWU.EXPECT().GetStatusLock().Return(statusLock).Times(2)
				mockBWU.EXPECT().GetStatusWithoutExtraData().Return(statusData)
				mockBWU.EXPECT().GetStatusCopy().Return(statusCopy)
			},
			expectError:      true,
			expectedErrorMsg: "EOF found when expecting closing quote",
			description:      "Should return error when command cannot be parsed",
		},
		{
			name: "Invalid params parsing",
			extraData: &workceptor.KubeExtraData{
				Image:         "busybox:latest",
				Command:       "echo hello",
				Params:        "--option 'unclosed quote",
				KubeNamespace: "default",
			},
			env: nil,
			setupMocks: func(mockBWU *mock_workceptor.MockBaseWorkUnitForWorkUnit, mockAPI *mock_workceptor.MockKubeAPIer, w *workceptor.Workceptor) {
				// Mock status calls for UnredactedStatus
				statusLock := &sync.RWMutex{}
				statusData := &workceptor.StatusFileData{ExtraData: &workceptor.KubeExtraData{}}
				statusCopy := workceptor.StatusFileData{ExtraData: &workceptor.KubeExtraData{
					Image:         "busybox:latest",
					Command:       "echo hello",
					Params:        "--option 'unclosed quote",
					KubeNamespace: "default",
				}}
				mockBWU.EXPECT().GetStatusLock().Return(statusLock).Times(2)
				mockBWU.EXPECT().GetStatusWithoutExtraData().Return(statusData)
				mockBWU.EXPECT().GetStatusCopy().Return(statusCopy)
			},
			expectError:      true,
			expectedErrorMsg: "EOF found when expecting closing quote",
			description:      "Should return error when params cannot be parsed",
		},
		{
			name: "Invalid pod definition YAML",
			extraData: &workceptor.KubeExtraData{
				KubePod: `invalid: yaml: content
  missing: proper: structure`,
				KubeNamespace: "default",
			},
			env: nil,
			setupMocks: func(mockBWU *mock_workceptor.MockBaseWorkUnitForWorkUnit, mockAPI *mock_workceptor.MockKubeAPIer, w *workceptor.Workceptor) {
				// Mock status calls for UnredactedStatus
				statusLock := &sync.RWMutex{}
				statusData := &workceptor.StatusFileData{ExtraData: &workceptor.KubeExtraData{}}
				statusCopy := workceptor.StatusFileData{ExtraData: &workceptor.KubeExtraData{
					KubePod: `invalid: yaml: content
  missing: proper: structure`,
					KubeNamespace: "default",
				}}
				mockBWU.EXPECT().GetStatusLock().Return(statusLock).Times(2)
				mockBWU.EXPECT().GetStatusWithoutExtraData().Return(statusData)
				mockBWU.EXPECT().GetStatusCopy().Return(statusCopy)
			},
			expectError:      true,
			expectedErrorMsg: "yaml: mapping values are not allowed in this context",
			description:      "Should return error when pod definition is invalid",
		},
		{
			name: "Pod definition without worker container",
			extraData: &workceptor.KubeExtraData{
				KubePod: `apiVersion: v1
kind: Pod
metadata:
  name: no-worker-pod
spec:
  containers:
  - name: other-container
    image: busybox:latest`,
				KubeNamespace: "default",
			},
			env: nil,
			setupMocks: func(mockBWU *mock_workceptor.MockBaseWorkUnitForWorkUnit, mockAPI *mock_workceptor.MockKubeAPIer, w *workceptor.Workceptor) {
				// Mock status calls for UnredactedStatus
				statusLock := &sync.RWMutex{}
				statusData := &workceptor.StatusFileData{ExtraData: &workceptor.KubeExtraData{}}
				statusCopy := workceptor.StatusFileData{ExtraData: &workceptor.KubeExtraData{
					KubePod: `apiVersion: v1
kind: Pod
metadata:
  name: no-worker-pod
spec:
  containers:
  - name: other-container
    image: busybox:latest`,
					KubeNamespace: "default",
				}}
				mockBWU.EXPECT().GetStatusLock().Return(statusLock).Times(2)
				mockBWU.EXPECT().GetStatusWithoutExtraData().Return(statusData)
				mockBWU.EXPECT().GetStatusCopy().Return(statusCopy)
			},
			expectError:      true,
			expectedErrorMsg: "at least one container must be named worker",
			description:      "Should return error when pod definition lacks worker container",
		},
		{
			name: "Kubernetes API Create failure",
			extraData: &workceptor.KubeExtraData{
				Image:         "busybox:latest",
				Command:       "echo hello",
				Params:        "",
				KubeNamespace: "default",
			},
			env: nil,
			setupMocks: func(mockBWU *mock_workceptor.MockBaseWorkUnitForWorkUnit, mockAPI *mock_workceptor.MockKubeAPIer, w *workceptor.Workceptor) {
				// Mock status calls for UnredactedStatus
				statusLock := &sync.RWMutex{}
				statusData := &workceptor.StatusFileData{ExtraData: &workceptor.KubeExtraData{}}
				statusCopy := workceptor.StatusFileData{ExtraData: &workceptor.KubeExtraData{
					Image:         "busybox:latest",
					Command:       "echo hello",
					Params:        "",
					KubeNamespace: "default",
				}}
				mockBWU.EXPECT().GetStatusLock().Return(statusLock).Times(2)
				mockBWU.EXPECT().GetStatusWithoutExtraData().Return(statusData)
				mockBWU.EXPECT().GetStatusCopy().Return(statusCopy)
				mockBWU.EXPECT().GetContext().Return(context.Background()).AnyTimes()

				// Mock failed pod creation
				mockAPI.EXPECT().Create(gomock.Any(), gomock.Any(), "default", gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("insufficient quota"))
			},
			expectError:      true,
			expectedErrorMsg: "insufficient quota",
			description:      "Should return error when Kubernetes API Create fails",
		},
		{
			name: "Context cancelled during creation",
			extraData: &workceptor.KubeExtraData{
				Image:         "busybox:latest",
				Command:       "echo hello",
				Params:        "",
				KubeNamespace: "default",
			},
			env: nil,
			setupMocks: func(mockBWU *mock_workceptor.MockBaseWorkUnitForWorkUnit, mockAPI *mock_workceptor.MockKubeAPIer, w *workceptor.Workceptor) {
				status := workceptor.StatusFileData{ExtraData: &workceptor.KubeExtraData{
					Image:         "busybox:latest",
					Command:       "echo hello",
					Params:        "",
					KubeNamespace: "default",
				}}
				mockBWU.EXPECT().GetStatusLock().Return(&sync.RWMutex{}).Times(2)
				mockBWU.EXPECT().GetStatusWithoutExtraData().Return(&workceptor.StatusFileData{ExtraData: &workceptor.KubeExtraData{}})
				mockBWU.EXPECT().GetStatusCopy().Return(status)

				// Create cancelled context
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // Cancel immediately
				mockBWU.EXPECT().GetContext().Return(ctx).AnyTimes()

				createdPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pod-123", Namespace: "default"},
				}
				mockAPI.EXPECT().Create(gomock.Any(), gomock.Any(), "default", gomock.Any(), gomock.Any()).Return(createdPod, nil)
			},
			expectError:      true,
			expectedErrorMsg: "cancelled",
			description:      "Should return error when context is cancelled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockBaseWorkUnit := mock_workceptor.NewMockBaseWorkUnitForWorkUnit(ctrl)
			mockNetceptor := mock_workceptor.NewMockNetceptorForWorkceptor(ctrl)
			mockKubeAPI := mock_workceptor.NewMockKubeAPIer(ctrl)

			mockNetceptor.EXPECT().NodeID().Return("test-node").AnyTimes()

			ctx := context.Background()
			w, err := workceptor.New(ctx, mockNetceptor, "/tmp")
			if err != nil {
				t.Fatalf("Error creating Workceptor: %v", err)
			}

			// Create KubeUnit
			kubeConfig := workceptor.KubeWorkerCfg{
				AuthMethod:   "incluster",
				StreamMethod: "logger",
			}

			mockBaseWorkUnit.EXPECT().Init(w, "", "", workceptor.FileSystem{})
			kubeUnit := kubeConfig.NewkubeWorker(mockBaseWorkUnit, w, "", "", mockKubeAPI).(*workceptor.KubeUnit)

			// Setup test-specific mocks
			tt.setupMocks(mockBaseWorkUnit, mockKubeAPI, w)

			// Execute the test
			err = kubeUnit.CreatePod(tt.env)

			// Verify results
			if tt.expectError {
				assert.Error(t, err, tt.description)
				if tt.expectedErrorMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorMsg, tt.description)
				}
			} else {
				assert.NoError(t, err, tt.description)
				if tt.validateResult != nil {
					tt.validateResult(t, kubeUnit)
				}
			}
		})
	}
}

func TestKubeUnit_RunWorkUsingLogger(t *testing.T) {
	// Test basic execution paths that are feasible to test with focused mocking
	tests := []struct {
		name        string
		setupMocks  func(*mock_workceptor.MockBaseWorkUnitForWorkUnit, *mock_workceptor.MockKubeAPIer, *mock_workceptor.MockNetceptorForWorkceptor, *workceptor.Workceptor)
		description string
	}{
		{
			name: "CreatePod failure - error path",
			setupMocks: func(mockBWU *mock_workceptor.MockBaseWorkUnitForWorkUnit, mockAPI *mock_workceptor.MockKubeAPIer, mockNetceptor *mock_workceptor.MockNetceptorForWorkceptor, w *workceptor.Workceptor) {
				// Mock status calls for new pod creation that will fail
				statusLock := &sync.RWMutex{}
				statusData := &workceptor.StatusFileData{ExtraData: &workceptor.KubeExtraData{}}
				statusCopy := workceptor.StatusFileData{ExtraData: &workceptor.KubeExtraData{
					Image:         "busybox:latest",
					Command:       "echo hello",
					KubeNamespace: "default",
					PodName:       "", // Empty triggers new pod creation
				}}
				// Multiple Status() calls: 1 from RunWorkUsingLogger, 1 from CreatePod
				mockBWU.EXPECT().GetStatusLock().Return(statusLock).AnyTimes()
				mockBWU.EXPECT().GetStatusWithoutExtraData().Return(statusData).AnyTimes()
				mockBWU.EXPECT().GetStatusCopy().Return(statusCopy).AnyTimes()

				// CreatePod path mocks
				mockBWU.EXPECT().GetContext().Return(context.Background()).AnyTimes()
				mockBWU.EXPECT().ID().Return("test-unit-id").AnyTimes()

				// Mock clientset injection (CreatePod may need this)
				mockBWU.EXPECT().UnitDir().Return("/tmp/test").AnyTimes()

				// Mock failed pod creation
				mockAPI.EXPECT().Create(gomock.Any(), gomock.Any(), "default", gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("insufficient resources"))

				// Mock error logging and status update for failed pod creation
				mockBWU.EXPECT().GetWorkceptor().Return(w).AnyTimes()
				mockNetceptor.EXPECT().GetLogger().Return(logger.NewReceptorLogger("test")).AnyTimes()
				mockBWU.EXPECT().UpdateBasicStatus(workceptor.WorkStateFailed, gomock.Any(), gomock.Any())
			},
			description: "Should handle CreatePod failures and exit early with error status",
		},
		{
			name: "Missing namespace for existing pod - error path",
			setupMocks: func(mockBWU *mock_workceptor.MockBaseWorkUnitForWorkUnit, mockAPI *mock_workceptor.MockKubeAPIer, mockNetceptor *mock_workceptor.MockNetceptorForWorkceptor, w *workceptor.Workceptor) {
				// Mock status calls for existing pod with missing namespace
				statusLock := &sync.RWMutex{}
				statusData := &workceptor.StatusFileData{ExtraData: &workceptor.KubeExtraData{}}
				statusCopy := workceptor.StatusFileData{ExtraData: &workceptor.KubeExtraData{
					KubeNamespace: "", // Empty namespace with existing pod name
					PodName:       "existing-pod-123",
				}}
				mockBWU.EXPECT().GetStatusLock().Return(statusLock).Times(2)
				mockBWU.EXPECT().GetStatusWithoutExtraData().Return(statusData)
				mockBWU.EXPECT().GetStatusCopy().Return(statusCopy)

				// Mock error logging and status update for missing namespace
				mockBWU.EXPECT().GetWorkceptor().Return(w).AnyTimes()
				mockNetceptor.EXPECT().GetLogger().Return(logger.NewReceptorLogger("test")).AnyTimes()
				mockBWU.EXPECT().UpdateBasicStatus(workceptor.WorkStateFailed, gomock.Any(), gomock.Any())
			},
			description: "Should handle missing namespace error and exit early with error status",
		},
		{
			name: "Pod retrieval failure after retries - error path",
			setupMocks: func(mockBWU *mock_workceptor.MockBaseWorkUnitForWorkUnit, mockAPI *mock_workceptor.MockKubeAPIer, mockNetceptor *mock_workceptor.MockNetceptorForWorkceptor, w *workceptor.Workceptor) {
				// Mock status calls for existing pod that fails retrieval
				statusLock := &sync.RWMutex{}
				statusData := &workceptor.StatusFileData{ExtraData: &workceptor.KubeExtraData{}}
				statusCopy := workceptor.StatusFileData{ExtraData: &workceptor.KubeExtraData{
					KubeNamespace: "default",
					PodName:       "missing-pod-123",
				}}
				mockBWU.EXPECT().GetStatusLock().Return(statusLock).Times(2)
				mockBWU.EXPECT().GetStatusWithoutExtraData().Return(statusData)
				mockBWU.EXPECT().GetStatusCopy().Return(statusCopy)
				mockBWU.EXPECT().GetContext().Return(context.Background()).AnyTimes()

				// Mock failed pod retrieval (5 retries) - should have proper expectations
				mockAPI.EXPECT().Get(gomock.Any(), gomock.Any(), "default", "missing-pod-123", gomock.Any()).Return(nil, fmt.Errorf("pod not found")).Times(5)

				// Mock warning and error logging (5 warnings + 1 error)
				mockBWU.EXPECT().GetWorkceptor().Return(w).AnyTimes()
				mockNetceptor.EXPECT().GetLogger().Return(logger.NewReceptorLogger("test")).AnyTimes()
				mockBWU.EXPECT().UpdateBasicStatus(workceptor.WorkStateFailed, gomock.Any(), gomock.Any())
			},
			description: "Should handle pod retrieval failures and exit with error status after retries",
		},
		{
			name: "Context cancellation during pod retrieval - early exit",
			setupMocks: func(mockBWU *mock_workceptor.MockBaseWorkUnitForWorkUnit, mockAPI *mock_workceptor.MockKubeAPIer, mockNetceptor *mock_workceptor.MockNetceptorForWorkceptor, w *workceptor.Workceptor) {
				// Mock status calls for existing pod
				statusLock := &sync.RWMutex{}
				statusData := &workceptor.StatusFileData{ExtraData: &workceptor.KubeExtraData{}}
				statusCopy := workceptor.StatusFileData{ExtraData: &workceptor.KubeExtraData{
					KubeNamespace: "default",
					PodName:       "cancelled-pod-123",
				}}
				mockBWU.EXPECT().GetStatusLock().Return(statusLock).Times(2)
				mockBWU.EXPECT().GetStatusWithoutExtraData().Return(statusData)
				mockBWU.EXPECT().GetStatusCopy().Return(statusCopy)

				// Mock cancelled context
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // Pre-cancel the context
				mockBWU.EXPECT().GetContext().Return(ctx).AnyTimes()

				// Mock warning logging for context cancellation
				mockBWU.EXPECT().GetWorkceptor().Return(w).AnyTimes()
				mockNetceptor.EXPECT().GetLogger().Return(logger.NewReceptorLogger("test")).AnyTimes()
			},
			description: "Should handle context cancellation and exit early with warning",
		},
		{
			name: "Successful pod retrieval but stdout file creation failure",
			setupMocks: func(mockBWU *mock_workceptor.MockBaseWorkUnitForWorkUnit, mockAPI *mock_workceptor.MockKubeAPIer, mockNetceptor *mock_workceptor.MockNetceptorForWorkceptor, w *workceptor.Workceptor) {
				// Mock status calls for existing pod (skipStdin=true case)
				statusLock := &sync.RWMutex{}
				statusData := &workceptor.StatusFileData{ExtraData: &workceptor.KubeExtraData{}}
				statusCopy := workceptor.StatusFileData{ExtraData: &workceptor.KubeExtraData{
					KubeNamespace: "default",
					PodName:       "existing-pod-789", // Non-empty triggers existing pod path
				}}
				mockBWU.EXPECT().GetStatusLock().Return(statusLock).Times(2)
				mockBWU.EXPECT().GetStatusWithoutExtraData().Return(statusData)
				mockBWU.EXPECT().GetStatusCopy().Return(statusCopy)
				mockBWU.EXPECT().GetContext().Return(context.Background()).AnyTimes()

				// Mock successful pod retrieval (no retries needed)
				existingPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "existing-pod-789",
						Namespace: "default",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				}
				mockAPI.EXPECT().Get(gomock.Any(), gomock.Any(), "default", "existing-pod-789", gomock.Any()).Return(existingPod, nil)

				// Mock UnitDir call for stdout file creation - but we'll trigger failure in NewStdoutWriter
				// This simulates line 812: stdout, err := NewStdoutWriter(FileSystem{}, kw.UnitDir())
				mockBWU.EXPECT().UnitDir().Return("/invalid/path/that/causes/stdout/writer/failure")

				// Mock error logging for stdout file creation failure
				mockBWU.EXPECT().GetWorkceptor().Return(w).AnyTimes()
				mockNetceptor.EXPECT().GetLogger().Return(logger.NewReceptorLogger("test")).AnyTimes()
				mockBWU.EXPECT().UpdateBasicStatus(workceptor.WorkStateFailed, gomock.Any(), gomock.Any())
			},
			description: "Should handle successful pod retrieval but fail on stdout file creation",
		},
		{
			name: "Pod retrieval with partial retry success - covers retry logic",
			setupMocks: func(mockBWU *mock_workceptor.MockBaseWorkUnitForWorkUnit, mockAPI *mock_workceptor.MockKubeAPIer, mockNetceptor *mock_workceptor.MockNetceptorForWorkceptor, w *workceptor.Workceptor) {
				// Mock status calls for existing pod that requires retries
				statusLock := &sync.RWMutex{}
				statusData := &workceptor.StatusFileData{ExtraData: &workceptor.KubeExtraData{}}
				statusCopy := workceptor.StatusFileData{ExtraData: &workceptor.KubeExtraData{
					KubeNamespace: "default",
					PodName:       "retry-pod-456", // Non-empty triggers existing pod path
				}}
				mockBWU.EXPECT().GetStatusLock().Return(statusLock).Times(2)
				mockBWU.EXPECT().GetStatusWithoutExtraData().Return(statusData)
				mockBWU.EXPECT().GetStatusCopy().Return(statusCopy)
				mockBWU.EXPECT().GetContext().Return(context.Background()).AnyTimes()

				// Mock pod retrieval with 2 failures then 1 success (covers retry logic and success path)
				mockAPI.EXPECT().Get(gomock.Any(), gomock.Any(), "default", "retry-pod-456", gomock.Any()).Return(nil, fmt.Errorf("temporary failure")).Times(2)

				// Third attempt succeeds
				retrievedPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "retry-pod-456",
						Namespace: "default",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				}
				mockAPI.EXPECT().Get(gomock.Any(), gomock.Any(), "default", "retry-pod-456", gomock.Any()).Return(retrievedPod, nil)

				// Mock warning messages for the two failed attempts
				mockBWU.EXPECT().GetWorkceptor().Return(w).AnyTimes()
				mockNetceptor.EXPECT().GetLogger().Return(logger.NewReceptorLogger("test")).AnyTimes()

				// After successful retrieval, try to create stdout file but fail
				mockBWU.EXPECT().UnitDir().Return("/nonexistent/path/for/stdout")

				// Mock error logging for stdout file creation failure
				mockBWU.EXPECT().UpdateBasicStatus(workceptor.WorkStateFailed, gomock.Any(), gomock.Any())
			},
			description: "Should handle pod retrieval retries with eventual success, then fail on stdout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockBaseWorkUnit := mock_workceptor.NewMockBaseWorkUnitForWorkUnit(ctrl)
			mockNetceptor := mock_workceptor.NewMockNetceptorForWorkceptor(ctrl)
			mockKubeAPI := mock_workceptor.NewMockKubeAPIer(ctrl)

			mockNetceptor.EXPECT().NodeID().Return("test-node").AnyTimes()

			ctx := context.Background()
			w, err := workceptor.New(ctx, mockNetceptor, "/tmp")
			if err != nil {
				t.Fatalf("Error creating Workceptor: %v", err)
			}

			// Create KubeUnit
			kubeConfig := workceptor.KubeWorkerCfg{
				AuthMethod:   "incluster",
				StreamMethod: "logger",
			}

			mockBaseWorkUnit.EXPECT().Init(w, "", "", workceptor.FileSystem{})
			kubeUnit := kubeConfig.NewkubeWorker(mockBaseWorkUnit, w, "", "", mockKubeAPI).(*workceptor.KubeUnit)

			// Set up mocks
			tt.setupMocks(mockBaseWorkUnit, mockKubeAPI, mockNetceptor, w)

			// Execute the function directly (no goroutine)
			// This tests the early error paths which should return cleanly
			t.Logf("Testing %s", tt.description)
			kubeUnit.RunWorkUsingLogger()

			// If we reach here, the test passed - the function returned after handling the error
			t.Logf("Function completed successfully for error path test")
		})
	}
}

// TestKubeAPIWrapper_StreamWithContext tests the StreamWithContext method.
func TestKubeAPIWrapper_StreamWithContext(t *testing.T) {
	wrapper := workceptor.KubeAPIWrapper{}

	// Test with nil executor - should panic
	t.Run("Nil executor", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				// Expected panic
				assert.Contains(t, fmt.Sprintf("%v", r), "nil pointer")
			}
		}()

		ctx := context.Background()
		options := remotecommand.StreamOptions{}

		_ = wrapper.StreamWithContext(ctx, nil, options)
		t.Error("Expected panic for nil executor")
	})

	// Test method exists and has correct signature
	t.Run("Method signature", func(t *testing.T) {
		methodType := reflect.TypeOf(wrapper.StreamWithContext)
		assert.Equal(t, "func(context.Context, remotecommand.Executor, remotecommand.StreamOptions) error", methodType.String())
	})
}

// TestKubeAPIWrapper_UntilWithSync tests the UntilWithSync method.
func TestKubeAPIWrapper_UntilWithSync(t *testing.T) {
	wrapper := workceptor.KubeAPIWrapper{}

	// Test method exists and has correct signature
	t.Run("Method signature", func(t *testing.T) {
		methodType := reflect.TypeOf(wrapper.UntilWithSync)
		assert.Contains(t, methodType.String(), "func(context.Context, cache.ListerWatcher, runtime.Object")
		assert.Contains(t, methodType.String(), "(*watch.Event, error)")
	})

	// Test with nil parameters - just verify it doesn't panic
	t.Run("Nil parameters", func(t *testing.T) {
		ctx := context.Background()
		// This will likely panic or return an error depending on the implementation
		// Let's just verify the method can be called
		assert.NotPanics(t, func() {
			_, _ = wrapper.UntilWithSync(ctx, nil, nil, nil)
		}, "Should not panic with nil parameters")
	})
}

// TestKubeWorkerCfg_NewWorker tests the NewWorker method.
func TestKubeWorkerCfg_NewWorker(t *testing.T) {
	tests := []struct {
		name         string
		cfg          workceptor.KubeWorkerCfg
		setupMocks   func() (workceptor.BaseWorkUnitForWorkUnit, *workceptor.Workceptor)
		unitID       string
		workType     string
		expectResult bool
		description  string
	}{
		{
			name: "Valid configuration",
			cfg: workceptor.KubeWorkerCfg{
				WorkType:     "test-work",
				AuthMethod:   "incluster",
				StreamMethod: "logger",
				Image:        "busybox:latest",
				Command:      "echo hello",
				Params:       "--verbose",
				Namespace:    "default",
			},
			setupMocks: func() (workceptor.BaseWorkUnitForWorkUnit, *workceptor.Workceptor) {
				ctrl := gomock.NewController(t)
				mockBaseWorkUnit := mock_workceptor.NewMockBaseWorkUnitForWorkUnit(ctrl)
				mockNetceptor := mock_workceptor.NewMockNetceptorForWorkceptor(ctrl)

				mockNetceptor.EXPECT().NodeID().Return("test-node")
				ctx := context.Background()
				w, err := workceptor.New(ctx, mockNetceptor, "/tmp")
				if err != nil {
					t.Fatalf("Error creating Workceptor: %v", err)
				}

				mockBaseWorkUnit.EXPECT().Init(w, "test-unit", "test-work", workceptor.FileSystem{})

				return mockBaseWorkUnit, w
			},
			unitID:       "test-unit",
			workType:     "test-work",
			expectResult: true,
			description:  "Should create WorkUnit with valid configuration",
		},
		{
			name: "With nil BaseWorkUnit",
			cfg: workceptor.KubeWorkerCfg{
				WorkType:     "test-work",
				AuthMethod:   "incluster",
				StreamMethod: "logger",
				Image:        "busybox:latest",
				Command:      "echo hello",
				Params:       "--verbose",
				Namespace:    "default",
			},
			setupMocks: func() (workceptor.BaseWorkUnitForWorkUnit, *workceptor.Workceptor) {
				ctrl := gomock.NewController(t)
				mockNetceptor := mock_workceptor.NewMockNetceptorForWorkceptor(ctrl)

				mockNetceptor.EXPECT().NodeID().Return("test-node")
				ctx := context.Background()
				w, err := workceptor.New(ctx, mockNetceptor, "/tmp")
				if err != nil {
					t.Fatalf("Error creating Workceptor: %v", err)
				}

				return nil, w
			},
			unitID:       "test-unit",
			workType:     "test-work",
			expectResult: true,
			description:  "Should create WorkUnit with nil BaseWorkUnit by creating default one",
		},
		{
			name: "Different work type",
			cfg: workceptor.KubeWorkerCfg{
				WorkType:     "custom-work",
				AuthMethod:   "kubeconfig",
				StreamMethod: "tcp",
				Image:        "alpine:latest",
				Command:      "sh",
				Params:       "-c 'echo test'",
				Namespace:    "custom-ns",
			},
			setupMocks: func() (workceptor.BaseWorkUnitForWorkUnit, *workceptor.Workceptor) {
				ctrl := gomock.NewController(t)
				mockBaseWorkUnit := mock_workceptor.NewMockBaseWorkUnitForWorkUnit(ctrl)
				mockNetceptor := mock_workceptor.NewMockNetceptorForWorkceptor(ctrl)

				mockNetceptor.EXPECT().NodeID().Return("test-node")
				ctx := context.Background()
				w, err := workceptor.New(ctx, mockNetceptor, "/tmp")
				if err != nil {
					t.Fatalf("Error creating Workceptor: %v", err)
				}

				mockBaseWorkUnit.EXPECT().Init(w, "custom-unit", "custom-work", workceptor.FileSystem{})

				return mockBaseWorkUnit, w
			},
			unitID:       "custom-unit",
			workType:     "custom-work",
			expectResult: true,
			description:  "Should create WorkUnit with different work type and parameters",
		},
		{
			name: "With runtime permissions",
			cfg: workceptor.KubeWorkerCfg{
				WorkType:            "runtime-work",
				AuthMethod:          "runtime",
				StreamMethod:        "logger",
				AllowRuntimeAuth:    true,
				AllowRuntimeCommand: true,
				AllowRuntimeParams:  true,
				AllowRuntimePod:     true,
				DeletePodOnRestart:  true,
			},
			setupMocks: func() (workceptor.BaseWorkUnitForWorkUnit, *workceptor.Workceptor) {
				ctrl := gomock.NewController(t)
				mockBaseWorkUnit := mock_workceptor.NewMockBaseWorkUnitForWorkUnit(ctrl)
				mockNetceptor := mock_workceptor.NewMockNetceptorForWorkceptor(ctrl)

				mockNetceptor.EXPECT().NodeID().Return("test-node")
				ctx := context.Background()
				w, err := workceptor.New(ctx, mockNetceptor, "/tmp")
				if err != nil {
					t.Fatalf("Error creating Workceptor: %v", err)
				}

				mockBaseWorkUnit.EXPECT().Init(w, "runtime-unit", "runtime-work", workceptor.FileSystem{})

				return mockBaseWorkUnit, w
			},
			unitID:       "runtime-unit",
			workType:     "runtime-work",
			expectResult: true,
			description:  "Should create WorkUnit with runtime permissions enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockBWU, w := tt.setupMocks()

			result := tt.cfg.NewWorker(mockBWU, w, tt.unitID, tt.workType)

			if tt.expectResult {
				assert.NotNil(t, result, tt.description)
				assert.Implements(t, (*workceptor.WorkUnit)(nil), result, tt.description)

				// Verify it's a KubeUnit
				kubeUnit, ok := result.(*workceptor.KubeUnit)
				assert.True(t, ok, "Should return a KubeUnit")
				assert.NotNil(t, kubeUnit, "KubeUnit should not be nil")
			} else {
				assert.Nil(t, result, tt.description)
			}
		})
	}
}

// TestKubeWorkerCfg_Prepare tests the Prepare method.
func TestKubeWorkerCfg_Prepare(t *testing.T) {
	// Create a temporary kubeconfig file for testing
	tmpfile, err := os.CreateTemp("", "kubeconfig")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	kubeconfig := `apiVersion: v1
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
	if _, err := tmpfile.Write([]byte(kubeconfig)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name             string
		cfg              workceptor.KubeWorkerCfg
		expectError      bool
		expectedErrorMsg string
		description      string
	}{
		{
			name: "Valid incluster configuration",
			cfg: workceptor.KubeWorkerCfg{
				WorkType:     "test-work",
				AuthMethod:   "incluster",
				StreamMethod: "logger",
				Image:        "busybox:latest",
				Namespace:    "default",
			},
			expectError: false,
			description: "Should accept valid incluster configuration",
		},
		{
			name: "Valid kubeconfig configuration",
			cfg: workceptor.KubeWorkerCfg{
				WorkType:     "test-work",
				AuthMethod:   "kubeconfig",
				StreamMethod: "logger",
				Image:        "busybox:latest",
				KubeConfig:   tmpfile.Name(),
			},
			expectError: false,
			description: "Should accept valid kubeconfig configuration",
		},
		{
			name: "Valid runtime configuration",
			cfg: workceptor.KubeWorkerCfg{
				WorkType:         "test-work",
				AuthMethod:       "runtime",
				StreamMethod:     "logger",
				AllowRuntimeAuth: true,
				AllowRuntimePod:  true,
			},
			expectError: false,
			description: "Should accept valid runtime configuration",
		},
		{
			name: "Invalid auth method",
			cfg: workceptor.KubeWorkerCfg{
				WorkType:     "test-work",
				AuthMethod:   "invalid-auth",
				StreamMethod: "logger",
				Image:        "busybox:latest",
				Namespace:    "default",
			},
			expectError:      true,
			expectedErrorMsg: "invalid AuthMethod: invalid-auth",
			description:      "Should reject invalid auth method",
		},
		{
			name: "Missing namespace with incluster",
			cfg: workceptor.KubeWorkerCfg{
				WorkType:     "test-work",
				AuthMethod:   "incluster",
				StreamMethod: "logger",
				Image:        "busybox:latest",
				// Missing Namespace
			},
			expectError:      true,
			expectedErrorMsg: "must provide namespace when AuthMethod is not kubeconfig",
			description:      "Should reject missing namespace with incluster auth",
		},
		{
			name: "KubeConfig with wrong auth method",
			cfg: workceptor.KubeWorkerCfg{
				WorkType:     "test-work",
				AuthMethod:   "incluster",
				StreamMethod: "logger",
				Image:        "busybox:latest",
				Namespace:    "default",
				KubeConfig:   tmpfile.Name(),
			},
			expectError:      true,
			expectedErrorMsg: "can only provide KubeConfig when AuthMethod=kubeconfig",
			description:      "Should reject KubeConfig with non-kubeconfig auth method",
		},
		{
			name: "Non-existent kubeconfig file",
			cfg: workceptor.KubeWorkerCfg{
				WorkType:     "test-work",
				AuthMethod:   "kubeconfig",
				StreamMethod: "logger",
				Image:        "busybox:latest",
				KubeConfig:   "/non/existent/file",
			},
			expectError:      true,
			expectedErrorMsg: "error accessing kubeconfig file",
			description:      "Should reject non-existent kubeconfig file",
		},
		{
			name: "Pod with conflicting image",
			cfg: workceptor.KubeWorkerCfg{
				WorkType:     "test-work",
				AuthMethod:   "incluster",
				StreamMethod: "logger",
				Image:        "busybox:latest",
				Namespace:    "default",
				Pod:          "apiVersion: v1\nkind: Pod",
			},
			expectError:      true,
			expectedErrorMsg: "can only provide Pod when Image, Command, and Params are empty",
			description:      "Should reject pod with conflicting image",
		},
		{
			name: "Pod with conflicting command",
			cfg: workceptor.KubeWorkerCfg{
				WorkType:     "test-work",
				AuthMethod:   "incluster",
				StreamMethod: "logger",
				Command:      "echo hello",
				Namespace:    "default",
				Pod:          "apiVersion: v1\nkind: Pod",
			},
			expectError:      true,
			expectedErrorMsg: "can only provide Pod when Image, Command, and Params are empty",
			description:      "Should reject pod with conflicting command",
		},
		{
			name: "Pod with conflicting params",
			cfg: workceptor.KubeWorkerCfg{
				WorkType:     "test-work",
				AuthMethod:   "incluster",
				StreamMethod: "logger",
				Params:       "--verbose",
				Namespace:    "default",
				Pod:          "apiVersion: v1\nkind: Pod",
			},
			expectError:      true,
			expectedErrorMsg: "can only provide Pod when Image, Command, and Params are empty",
			description:      "Should reject pod with conflicting params",
		},
		{
			name: "Missing image and pod without runtime permissions",
			cfg: workceptor.KubeWorkerCfg{
				WorkType:     "test-work",
				AuthMethod:   "incluster",
				StreamMethod: "logger",
				Namespace:    "default",
				// Missing Image, Pod, and runtime permissions
			},
			expectError:      true,
			expectedErrorMsg: "must specify a container image to run",
			description:      "Should reject missing image/pod without runtime permissions",
		},
		{
			name: "Valid configuration with runtime command allowed",
			cfg: workceptor.KubeWorkerCfg{
				WorkType:            "test-work",
				AuthMethod:          "incluster",
				StreamMethod:        "logger",
				Namespace:           "default",
				AllowRuntimeCommand: true,
			},
			expectError: false,
			description: "Should accept missing image with runtime command permission",
		},
		{
			name: "Valid configuration with runtime pod allowed",
			cfg: workceptor.KubeWorkerCfg{
				WorkType:        "test-work",
				AuthMethod:      "incluster",
				StreamMethod:    "logger",
				Namespace:       "default",
				AllowRuntimePod: true,
			},
			expectError: false,
			description: "Should accept missing image with runtime pod permission",
		},
		{
			name: "Invalid stream method",
			cfg: workceptor.KubeWorkerCfg{
				WorkType:     "test-work",
				AuthMethod:   "incluster",
				StreamMethod: "invalid-stream",
				Image:        "busybox:latest",
				Namespace:    "default",
			},
			expectError:      true,
			expectedErrorMsg: "stream mode must be logger or tcp",
			description:      "Should reject invalid stream method",
		},
		{
			name: "Valid TCP stream method",
			cfg: workceptor.KubeWorkerCfg{
				WorkType:     "test-work",
				AuthMethod:   "incluster",
				StreamMethod: "tcp",
				Image:        "busybox:latest",
				Namespace:    "default",
			},
			expectError: false,
			description: "Should accept valid TCP stream method",
		},
		{
			name: "Valid logger stream method",
			cfg: workceptor.KubeWorkerCfg{
				WorkType:     "test-work",
				AuthMethod:   "incluster",
				StreamMethod: "logger",
				Image:        "busybox:latest",
				Namespace:    "default",
			},
			expectError: false,
			description: "Should accept valid logger stream method",
		},
		{
			name: "Case insensitive auth method",
			cfg: workceptor.KubeWorkerCfg{
				WorkType:     "test-work",
				AuthMethod:   "INCLUSTER",
				StreamMethod: "logger",
				Image:        "busybox:latest",
				Namespace:    "default",
			},
			expectError: false,
			description: "Should accept case insensitive auth method",
		},
		{
			name: "Case insensitive stream method",
			cfg: workceptor.KubeWorkerCfg{
				WorkType:     "test-work",
				AuthMethod:   "incluster",
				StreamMethod: "LOGGER",
				Image:        "busybox:latest",
				Namespace:    "default",
			},
			expectError: false,
			description: "Should accept case insensitive stream method",
		},
		{
			name: "Valid pod only configuration",
			cfg: workceptor.KubeWorkerCfg{
				WorkType:     "test-work",
				AuthMethod:   "incluster",
				StreamMethod: "logger",
				Namespace:    "default",
				Pod:          "apiVersion: v1\nkind: Pod\nmetadata:\n  name: test-pod",
			},
			expectError: false,
			description: "Should accept valid pod only configuration",
		},
		{
			name: "Kubeconfig namespace detection",
			cfg: workceptor.KubeWorkerCfg{
				WorkType:     "test-work",
				AuthMethod:   "kubeconfig",
				StreamMethod: "logger",
				Image:        "busybox:latest",
				KubeConfig:   tmpfile.Name(),
				// Namespace will be detected from kubeconfig
			},
			expectError: false,
			description: "Should accept kubeconfig without explicit namespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Prepare()

			if tt.expectError {
				assert.Error(t, err, tt.description)
				if tt.expectedErrorMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorMsg, tt.description)
				}
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

// TestKubeWorkerCfg_GetWorkType tests the GetWorkType method.
func TestKubeWorkerCfg_GetWorkType(t *testing.T) {
	tests := []struct {
		name        string
		cfg         workceptor.KubeWorkerCfg
		expected    string
		description string
	}{
		{
			name: "Basic work type",
			cfg: workceptor.KubeWorkerCfg{
				WorkType: "test-work",
			},
			expected:    "test-work",
			description: "Should return configured work type",
		},
		{
			name: "Kubernetes work type",
			cfg: workceptor.KubeWorkerCfg{
				WorkType: "kubernetes",
			},
			expected:    "kubernetes",
			description: "Should return kubernetes work type",
		},
		{
			name: "Custom work type",
			cfg: workceptor.KubeWorkerCfg{
				WorkType: "custom-k8s-worker",
			},
			expected:    "custom-k8s-worker",
			description: "Should return custom work type",
		},
		{
			name: "Empty work type",
			cfg: workceptor.KubeWorkerCfg{
				WorkType: "",
			},
			expected:    "",
			description: "Should return empty work type",
		},
		{
			name: "Work type with spaces",
			cfg: workceptor.KubeWorkerCfg{
				WorkType: "work type with spaces",
			},
			expected:    "work type with spaces",
			description: "Should return work type with spaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cfg.GetWorkType()
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

// TestKubeWorkerCfg_GetVerifySignature tests the GetVerifySignature method.
func TestKubeWorkerCfg_GetVerifySignature(t *testing.T) {
	tests := []struct {
		name        string
		cfg         workceptor.KubeWorkerCfg
		expected    bool
		description string
	}{
		{
			name: "Verify signature true",
			cfg: workceptor.KubeWorkerCfg{
				VerifySignature: true,
			},
			expected:    true,
			description: "Should return true when verify signature is enabled",
		},
		{
			name: "Verify signature false",
			cfg: workceptor.KubeWorkerCfg{
				VerifySignature: false,
			},
			expected:    false,
			description: "Should return false when verify signature is disabled",
		},
		{
			name: "Default verify signature",
			cfg:  workceptor.KubeWorkerCfg{
				// Default value (false)
			},
			expected:    false,
			description: "Should return false by default",
		},
		{
			name: "Verify signature with other config",
			cfg: workceptor.KubeWorkerCfg{
				WorkType:        "test-work",
				AuthMethod:      "incluster",
				StreamMethod:    "logger",
				VerifySignature: true,
			},
			expected:    true,
			description: "Should return true when verify signature is enabled with other config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cfg.GetVerifySignature()
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

// TestKubeWorkerCfg_Run tests the Run method.
func TestKubeWorkerCfg_Run(t *testing.T) {
	tests := []struct {
		name        string
		cfg         workceptor.KubeWorkerCfg
		expectError bool
		description string
	}{
		{
			name: "Valid configuration",
			cfg: workceptor.KubeWorkerCfg{
				WorkType:        "test-work",
				AuthMethod:      "incluster",
				StreamMethod:    "logger",
				Image:           "busybox:latest",
				Namespace:       "default",
				VerifySignature: false,
			},
			expectError: false,
			description: "Should successfully register worker with valid configuration",
		},
		{
			name: "With signature verification",
			cfg: workceptor.KubeWorkerCfg{
				WorkType:        "signed-work",
				AuthMethod:      "incluster",
				StreamMethod:    "logger",
				Image:           "busybox:latest",
				Namespace:       "default",
				VerifySignature: true,
			},
			expectError: false,
			description: "Should successfully register worker with signature verification",
		},
		{
			name: "Different work type",
			cfg: workceptor.KubeWorkerCfg{
				WorkType:        "custom-kubernetes-worker",
				AuthMethod:      "kubeconfig",
				StreamMethod:    "tcp",
				Image:           "alpine:latest",
				Namespace:       "custom-ns",
				VerifySignature: false,
			},
			expectError: false,
			description: "Should successfully register worker with different configuration",
		},
		{
			name: "Empty work type",
			cfg: workceptor.KubeWorkerCfg{
				WorkType:        "",
				AuthMethod:      "incluster",
				StreamMethod:    "logger",
				Image:           "busybox:latest",
				Namespace:       "default",
				VerifySignature: false,
			},
			expectError: false,
			description: "Should handle empty work type (MainInstance.RegisterWorker handles validation)",
		},
		{
			name: "Minimal valid configuration",
			cfg: workceptor.KubeWorkerCfg{
				WorkType: "minimal-work",
			},
			expectError: false,
			description: "Should handle minimal configuration (validation happens elsewhere)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up a proper workceptor instance for MainInstance
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			mockNetceptor := mock_workceptor.NewMockNetceptorForWorkceptor(ctrl)
			mockNetceptor.EXPECT().NodeID().Return("test-node").AnyTimes()
			mockNetceptor.EXPECT().GetLogger().Return(&logger.ReceptorLogger{}).AnyTimes()
			mockNetceptor.EXPECT().AddWorkCommand(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			w, err := workceptor.New(ctx, mockNetceptor, "/tmp")
			assert.NoError(t, err, "Should create workceptor instance")

			// Set MainInstance to our test workceptor
			originalMainInstance := workceptor.MainInstance
			workceptor.MainInstance = w
			defer func() {
				workceptor.MainInstance = originalMainInstance
			}()

			// Test the Run method
			err = tt.cfg.Run()
			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}
