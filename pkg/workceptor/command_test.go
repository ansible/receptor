package workceptor_test

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/workceptor"
	"github.com/ansible/receptor/pkg/workceptor/mock_workceptor"
	"github.com/golang/mock/gomock"
)

func statusExpectCalls(mockBaseWorkUnit *mock_workceptor.MockBaseWorkUnitForWorkUnit) {
	statusLock := &sync.RWMutex{}
	mockBaseWorkUnit.EXPECT().GetStatusLock().Return(statusLock).Times(2)
	mockBaseWorkUnit.EXPECT().GetStatusWithoutExtraData().Return(&workceptor.StatusFileData{})
	mockBaseWorkUnit.EXPECT().GetStatusCopy().Return(workceptor.StatusFileData{
		ExtraData: &workceptor.CommandExtraData{},
	})
}

func createTestSetup(t *testing.T) (workceptor.WorkUnit, *mock_workceptor.MockBaseWorkUnitForWorkUnit, *mock_workceptor.MockNetceptorForWorkceptor, *gomock.Controller, *workceptor.Workceptor) {
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
	return workUnit, mockBaseWorkUnit, mockNetceptor, ctrl, w
}

func TestSetFromParams2(t *testing.T) {
	wu, mockBaseWorkUnit, _, _, _ := createTestSetup(t)

	paramsTestCases := []struct {
		name          string
		params        map[string]string
		expectedCalls func()
		errorCatch    func(error, *testing.T)
	}{
		{
			name:   "one",
			params: map[string]string{"": ""},
			expectedCalls: func() {
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
			name:   "two",
			params: map[string]string{"params": "param"},
			expectedCalls: func() {

			},
			errorCatch: func(err error, t *testing.T) {
				if err == nil {
					t.Error(err)
				}
			},
		},
	}

	for _, testCase := range paramsTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.expectedCalls()
			err := wu.SetFromParams(testCase.params)
			testCase.errorCatch(err, t)
		})
	}

}

func TestUnredactedStatus(t *testing.T) {
	wu, mockBaseWorkUnit, _, _, _ := createTestSetup(t)
	statusLock := &sync.RWMutex{}
	mockBaseWorkUnit.EXPECT().GetStatusLock().Return(statusLock).Times(2)
	mockBaseWorkUnit.EXPECT().GetStatusWithoutExtraData().Return(&workceptor.StatusFileData{})
	mockBaseWorkUnit.EXPECT().GetStatusCopy().Return(workceptor.StatusFileData{
		ExtraData: &workceptor.CommandExtraData{},
	})

	wu.UnredactedStatus()
}

func TestStart(t *testing.T) {
	wu, mockBaseWorkUnit, mockNetceptor, _, w := createTestSetup(t)

	mockBaseWorkUnit.EXPECT().GetWorkceptor().Return(w).Times(2)
	mockNetceptor.EXPECT().GetLogger().Times(2)
	mockBaseWorkUnit.EXPECT().UpdateBasicStatus(gomock.Any(), gomock.Any(), gomock.Any())
	statusExpectCalls(mockBaseWorkUnit)

	mockBaseWorkUnit.EXPECT().UnitDir()
	mockBaseWorkUnit.EXPECT().UpdateFullStatus(gomock.Any())
	mockBaseWorkUnit.EXPECT().MonitorLocalStatus().AnyTimes()
	mockBaseWorkUnit.EXPECT().UpdateFullStatus(gomock.Any()).AnyTimes()
	wu.Start()
}

func TestRestart(t *testing.T) {
	wu, mockBaseWorkUnit, _, _, _ := createTestSetup(t)

	restartTestCases := []struct {
		name          string
		expectedCalls func()
		errorCatch    func(error, *testing.T)
	}{
		{
			name: "return error 1",
			expectedCalls: func() {
				mockBaseWorkUnit.EXPECT().Load().Return(errors.New("terminated"))
			},
			errorCatch: func(err error, t *testing.T) {
				if err.Error() != "terminated" {
					t.Error(err)
				}
			},
		},
		{
			name: "return nil 1",
			expectedCalls: func() {
				statusFile := &workceptor.StatusFileData{State: 2}
				mockBaseWorkUnit.EXPECT().Load().Return(nil)
				statusLock := &sync.RWMutex{}
				mockBaseWorkUnit.EXPECT().GetStatusLock().Return(statusLock).Times(2)
				mockBaseWorkUnit.EXPECT().GetStatusWithoutExtraData().Return(statusFile)
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
			name: "return nil 2",
			expectedCalls: func() {
				statusFile := &workceptor.StatusFileData{State: 0}
				mockBaseWorkUnit.EXPECT().Load().Return(nil)
				statusLock := &sync.RWMutex{}
				mockBaseWorkUnit.EXPECT().GetStatusLock().Return(statusLock).Times(2)
				mockBaseWorkUnit.EXPECT().GetStatusWithoutExtraData().Return(statusFile)
				mockBaseWorkUnit.EXPECT().GetStatusCopy().Return(workceptor.StatusFileData{
					ExtraData: &workceptor.CommandExtraData{},
				})
				mockBaseWorkUnit.EXPECT().UpdateBasicStatus(gomock.Any(), gomock.Any(), gomock.Any())
				mockBaseWorkUnit.EXPECT().UnitDir()
			},
			errorCatch: func(err error, t *testing.T) {
				if err != nil {
					t.Error(err)
				}
			},
		},
	}

	for _, testCase := range restartTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.expectedCalls()
			mockBaseWorkUnit.EXPECT().MonitorLocalStatus().AnyTimes()
			err := wu.Restart()
			testCase.errorCatch(err, t)
		})
	}
}

func TestCancel(t *testing.T) {
	wu, mockBaseWorkUnit, _, _, _ := createTestSetup(t)

	paramsTestCases := []struct {
		name          string
		expectedCalls func()
		errorCatch    func(error, *testing.T)
	}{
		{
			name: "return no error 1",
			expectedCalls: func() {
				mockBaseWorkUnit.EXPECT().CancelContext()
				statusExpectCalls(mockBaseWorkUnit)
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
				processPid := make(chan int)

				go func(c *exec.Cmd, processPid chan int) {
					c.Run()
					processPid <- c.Process.Pid
				}(c, processPid)

				time.Sleep(200 * time.Millisecond)

				mockBaseWorkUnit.EXPECT().GetStatusCopy().Return(workceptor.StatusFileData{
					ExtraData: &workceptor.CommandExtraData{
						Pid: <-processPid,
					},
				})
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
				mockBaseWorkUnit.EXPECT().UpdateBasicStatus(gomock.Any(), gomock.Any(), gomock.Any())

				c := exec.Command("sleep", "30")
				processPid := make(chan int)

				go func(c *exec.Cmd, processPid chan int) {
					err := c.Start()
					if err != nil {
						fmt.Println(err)
					}
					processPid <- c.Process.Pid
				}(c, processPid)
				time.Sleep(200 * time.Millisecond)

				mockBaseWorkUnit.EXPECT().GetStatusCopy().Return(workceptor.StatusFileData{
					ExtraData: &workceptor.CommandExtraData{
						Pid: <-processPid,
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

func TestRelease(t *testing.T) {
	wu, mockBaseWorkUnit, _, _, _ := createTestSetup(t)

	releaseTestCases := []struct {
		name          string
		expectedCalls func()
		errorCatch    func(error, *testing.T)
		force         bool
	}{
		{
			name:          "error",
			expectedCalls: func() {},
			errorCatch: func(err error, t *testing.T) {
				if err == nil {
					t.Error(err)
				}
			},
			force: false,
		},
		{
			name: "happy day",
			expectedCalls: func() {
				mockBaseWorkUnit.EXPECT().Release(gomock.Any())
			},
			errorCatch: func(err error, t *testing.T) {
				if err != nil {
					t.Error(err)
				}
			},
			force: true,
		},
	}
	for _, testCase := range releaseTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			mockBaseWorkUnit.EXPECT().CancelContext()
			mockBaseWorkUnit.EXPECT().GetStatusLock().Return(&sync.RWMutex{}).Times(2)
			mockBaseWorkUnit.EXPECT().GetStatusWithoutExtraData().Return(&workceptor.StatusFileData{})
			mockBaseWorkUnit.EXPECT().GetStatusCopy().Return(workceptor.StatusFileData{
				ExtraData: &workceptor.CommandExtraData{
					Pid: 1,
				},
			})
			testCase.expectedCalls()
			err := wu.Release(testCase.force)
			testCase.errorCatch(err, t)
		})
	}
}

// func TestCommandWorkerCfgRun(t *testing.T) {
// 	cfgTestCases := []struct {
// 		name            string
// 		verifySignature bool
// 	}{
// 		{
// 			name:            "error 1",
// 			verifySignature: true,
// 		},
// 	}
// 	for _, testCase := range cfgTestCases {
// 		t.Run(testCase.name, func(t *testing.T) {
// 			commandWorkerCfg := workceptor.CommandWorkerCfg{
// 				VerifySignature: testCase.verifySignature,
// 			}
// 			commandWorkerCfg.Run()
// 		})
// 	}

// }
