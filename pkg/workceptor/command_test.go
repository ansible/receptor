package workceptor_test

import (
	"context"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/workceptor"
	"github.com/ansible/receptor/pkg/workceptor/mock_workceptor"
	"github.com/golang/mock/gomock"
)

func createTestSetup(t *testing.T) (workceptor.WorkUnit, *mock_workceptor.MockBaseWorkUnitForWorkUnit, *mock_workceptor.MockNetceptorForWorkceptor, *gomock.Controller) {
	ctrl := gomock.NewController(t)
	ctx := context.Background()

	mockBaseWorkUnit := mock_workceptor.NewMockBaseWorkUnitForWorkUnit(ctrl)
	mockNetceptor := mock_workceptor.NewMockNetceptorForWorkceptor(ctrl)
	mockNetceptor.EXPECT().NodeID().Return("NodeID")

	w, err := workceptor.New(ctx, mockNetceptor, "/tmp")
	if err != nil {
		t.Errorf("Error while creating Workceptor: %v", err)
	}

	cwc := &workceptor.CommandWorkerCfg{}
	mockBaseWorkUnit.EXPECT().Init(w, "", "", workceptor.FileSystem{}, nil)
	workUnit := cwc.NewWorker(mockBaseWorkUnit, w, "", "")
	return workUnit, mockBaseWorkUnit, mockNetceptor, ctrl
}

func TestSetFromParams2(t *testing.T) {
	wu, _, _, _ := createTestSetup(t)

	paramsTestCases := []struct {
		name       string
		params     map[string]string
		errorCatch func(error, *testing.T)
	}{
		{
			name:   "one",
			params: map[string]string{"": ""},
			errorCatch: func(err error, t *testing.T) {
				if err != nil {
					t.Error(err)
				}
			},
		},
		{
			name:   "two",
			params: map[string]string{"params": "param"},
			errorCatch: func(err error, t *testing.T) {
				if err == nil {
					t.Error(err)
				}
			},
		},
	}

	for _, testCase := range paramsTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := wu.SetFromParams(testCase.params)
			testCase.errorCatch(err, t)
		})
	}

}

func TestCancel(t *testing.T) {
	wu, mockBaseWorkUnit, _, _ := createTestSetup(t)

	paramsTestCases := []struct {
		name          string
		expectedCalls func()
		errorCatch    func(error, *testing.T)
	}{
		{
			name: "return no error 1",
			expectedCalls: func() {
				mockBaseWorkUnit.EXPECT().CancelContext()
				mockBaseWorkUnit.EXPECT().GetStatusLock().Return(&sync.RWMutex{}).Times(2)
				mockBaseWorkUnit.EXPECT().GetStatusWithoutExtraData().Return(&workceptor.StatusFileData{})
				mockBaseWorkUnit.EXPECT().GetStatusCopy().Return(workceptor.StatusFileData{
					ExtraData: &workceptor.CommandExtraData{},
				})

			},
			errorCatch: func(err error, t *testing.T) {
				if err != nil {
					t.Error(err)
				}
			},
		},
		{
			name: "return err 2",
			expectedCalls: func() {
				mockBaseWorkUnit.EXPECT().CancelContext()
				mockBaseWorkUnit.EXPECT().GetStatusLock().Return(&sync.RWMutex{}).Times(2)
				mockBaseWorkUnit.EXPECT().GetStatusWithoutExtraData().Return(&workceptor.StatusFileData{})
				mockBaseWorkUnit.EXPECT().GetStatusCopy().Return(workceptor.StatusFileData{
					ExtraData: &workceptor.CommandExtraData{
						Pid: 1,
					},
				})
			},
			errorCatch: func(err error, t *testing.T) {
				if err == nil {
					t.Error(err)
				}
			},
		},
		{
			name: "return nil already finished",
			expectedCalls: func() {
				mockBaseWorkUnit.EXPECT().CancelContext()
				mockBaseWorkUnit.EXPECT().GetStatusLock().Return(&sync.RWMutex{}).Times(2)
				mockBaseWorkUnit.EXPECT().GetStatusWithoutExtraData().Return(&workceptor.StatusFileData{})

				c := exec.Command("ls", "/tmp")
				go func() {
					c.Run()
				}()
				time.Sleep(200 * time.Millisecond)

				mockBaseWorkUnit.EXPECT().GetStatusCopy().Return(workceptor.StatusFileData{
					ExtraData: &workceptor.CommandExtraData{
						Pid: c.Process.Pid,
					},
				})
				mockBaseWorkUnit.EXPECT().UpdateBasicStatus(gomock.Any(), gomock.Any(), gomock.Any())
			},
			errorCatch: func(err error, t *testing.T) {
				if err != nil {
					t.Error(err)
				}
			},
		},
		{
			name: "happy day",
			expectedCalls: func() {
				mockBaseWorkUnit.EXPECT().CancelContext()
				mockBaseWorkUnit.EXPECT().GetStatusLock().Return(&sync.RWMutex{}).Times(2)
				mockBaseWorkUnit.EXPECT().GetStatusWithoutExtraData().Return(&workceptor.StatusFileData{})

				c := exec.Command("sleep", "30")
				go func() {
					c.Run()
				}()
				time.Sleep(200 * time.Millisecond)

				mockBaseWorkUnit.EXPECT().GetStatusCopy().Return(workceptor.StatusFileData{
					ExtraData: &workceptor.CommandExtraData{
						Pid: c.Process.Pid,
					},
				})
			},
			errorCatch: func(err error, t *testing.T) {
				if err != nil {
					t.Error(err)
				}
			},
		},
	}

	for _, testCase := range paramsTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.expectedCalls()
			err := wu.Cancel()
			testCase.errorCatch(err, t)
		})
	}

}
