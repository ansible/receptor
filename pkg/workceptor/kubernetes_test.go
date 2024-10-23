package workceptor_test

import (
	"context"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/workceptor"
	"github.com/ansible/receptor/pkg/workceptor/mock_workceptor"
	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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
			name:     "Env value set incorrectly",
			envValue: "text instead of int",
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

func createKubernetesTestSetup(t *testing.T) (workceptor.WorkUnit, *mock_workceptor.MockBaseWorkUnitForWorkUnit, *mock_workceptor.MockNetceptorForWorkceptor, *workceptor.Workceptor, *mock_workceptor.MockKubeAPIer, context.Context) {
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

	mockBaseWorkUnit.EXPECT().Init(w, "", "", workceptor.FileSystem{}, nil)
	kubeConfig := workceptor.KubeWorkerCfg{AuthMethod: "incluster"}
	ku := kubeConfig.NewkubeWorker(mockBaseWorkUnit, w, "", "", mockKubeAPI)

	return ku, mockBaseWorkUnit, mockNetceptor, w, mockKubeAPI, ctx
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
	ku, mockbwu, mockNet, w, mockKubeAPI, ctx := createKubernetesTestSetup(t)

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
				versionStr: "v1.23.14+sadfasdf",
			},
			want: true,
		},
		{
			name: "Prerelease Kubernetes version positive test 2",
			args: args{
				versionStr: "v1.23.14-asdfasdf+12131",
			},
			want: true,
		},
		{
			name: "Prerelease Kubernetes version positive test 3",
			args: args{
				versionStr: "v1.23.15-asdfasdf+12131",
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
