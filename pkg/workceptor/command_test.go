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

func TestCommandSetFromParams(t *testing.T) {
	wu, mockBaseWorkUnit, _, _, _ := createTestSetup(t) //nolint:dogsled

	paramsTestCases := []struct {
		name          string
		params        map[string]string
		expectedCalls func()
		errorCatch    func(error, *testing.T)
	}{
		{
			name:   "no params with no error",
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
			name:   "params with error",
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
	wu, mockBaseWorkUnit, _, _, _ := createTestSetup(t) //nolint:dogsled
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
	wu, mockBaseWorkUnit, _, _, _ := createTestSetup(t) //nolint:dogsled

	restartTestCases := []struct {
		name          string
		expectedCalls func()
		errorCatch    func(error, *testing.T)
	}{
		{
			name: "load error",
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
			name: "job complete with no error",
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
			name: "restart successful",
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
	wu, mockBaseWorkUnit, _, _, _ := createTestSetup(t) //nolint:dogsled

	paramsTestCases := []struct {
		name          string
		expectedCalls func()
		errorCatch    func(error, *testing.T)
	}{
		{
			name: "not a valid pid no error",
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
			name: "process interrupt error",
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
			name: "process already finished",
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
			name: "cancelled process successfully",
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
	wu, mockBaseWorkUnit, _, _, _ := createTestSetup(t) //nolint:dogsled

	releaseTestCases := []struct {
		name          string
		expectedCalls func()
		errorCatch    func(error, *testing.T)
		force         bool
	}{
		{
			name:          "cancel error",
			expectedCalls: func() {},
			errorCatch: func(err error, t *testing.T) {
				if err == nil {
					t.Error(err)
				}
			},
			force: false,
		},
		{
			name: "released successfully",
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

func TestSigningKeyPrepare(t *testing.T) {
	privateKey := workceptor.SigningKeyPrivateCfg{}
	err := privateKey.Prepare()

	if err == nil {
		t.Error(err)
	}
}

func TestPrepareSigningKeyPrivateCfg(t *testing.T) {
	signingKeyTestCases := []struct {
		name            string
		errorCatch      func(error, *testing.T)
		privateKey      string
		tokenExpiration string
	}{
		{
			name:            "file does not exist error",
			privateKey:      "does_not_exist.txt",
			tokenExpiration: "",
			errorCatch: func(err error, t *testing.T) {
				if err == nil {
					t.Error(err)
				}
			},
		},
		{
			name:            "failed to parse token expiration",
			privateKey:      "/etc/hosts",
			tokenExpiration: "random_input",
			errorCatch: func(err error, t *testing.T) {
				if err == nil {
					t.Error(err)
				}
			},
		},
		{
			name:            "duration no error",
			privateKey:      "/etc/hosts",
			tokenExpiration: "3h",
			errorCatch: func(err error, t *testing.T) {
				if err != nil {
					t.Error(err)
				}
			},
		},
		{
			name:            "no duration no error",
			privateKey:      "/etc/hosts",
			tokenExpiration: "",
			errorCatch: func(err error, t *testing.T) {
				if err != nil {
					t.Error(err)
				}
			},
		},
	}

	for _, testCase := range signingKeyTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			privateKey := workceptor.SigningKeyPrivateCfg{
				PrivateKey:      testCase.privateKey,
				TokenExpiration: testCase.tokenExpiration,
			}
			_, err := privateKey.PrepareSigningKeyPrivateCfg()
			testCase.errorCatch(err, t)
		})
	}
}

func TestVerifyingKeyPrepare(t *testing.T) {
	publicKey := workceptor.VerifyingKeyPublicCfg{}
	err := publicKey.Prepare()

	if err == nil {
		t.Error(err)
	}
}

func TestPrepareVerifyingKeyPrivateCfg(t *testing.T) {
	verifyingKeyTestCases := []struct {
		name       string
		errorCatch func(error, *testing.T)
		publicKey  string
	}{
		{
			name:      "file does not exist",
			publicKey: "does_not_exist.txt",
			errorCatch: func(err error, t *testing.T) {
				if err == nil {
					t.Error(err)
				}
			},
		},
		{
			name:      "prepared successfully",
			publicKey: "/etc/hosts",
			errorCatch: func(err error, t *testing.T) {
				if err != nil {
					t.Error(err)
				}
			},
		},
	}

	for _, testCase := range verifyingKeyTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			publicKey := workceptor.VerifyingKeyPublicCfg{
				PublicKey: testCase.publicKey,
			}
			err := publicKey.PrepareVerifyingKeyPublicCfg()
			testCase.errorCatch(err, t)
		})
	}
}
