package workceptor_test

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/workceptor"
	"github.com/ansible/receptor/pkg/workceptor/mock_workceptor"
	"github.com/fsnotify/fsnotify"
	
	gomock "go.uber.org/mock/gomock"
)

func TestIsComplete(t *testing.T) {
	testCases := []struct {
		name       string
		workState  int
		isComplete bool
	}{
		{"Pending Work is Incomplete", workceptor.WorkStatePending, false},
		{"Running Work is Incomplete", workceptor.WorkStateRunning, false},
		{"Succeeded Work is Complete", workceptor.WorkStateSucceeded, true},
		{"Failed Work is Complete", workceptor.WorkStateFailed, true},
		{"Unknown Work is Incomplete", 999, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if result := workceptor.IsComplete(tc.workState); result != tc.isComplete {
				t.Errorf("expected %v, got %v", tc.isComplete, result)
			}
		})
	}
}

func TestWorkStateToString(t *testing.T) {
	testCases := []struct {
		name        string
		workState   int
		description string
	}{
		{"Pending Work Description", workceptor.WorkStatePending, "Pending"},
		{"Running Work Description", workceptor.WorkStateRunning, "Running"},
		{"Succeeded Work Description", workceptor.WorkStateSucceeded, "Succeeded"},
		{"Failed Work Description", workceptor.WorkStateFailed, "Failed"},
		{"Unknown Work Description", 999, "Unknown"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if result := workceptor.WorkStateToString(tc.workState); result != tc.description {
				t.Errorf("expected %s, got %s", tc.description, result)
			}
		})
	}
}

func TestIsPending(t *testing.T) {
	testCases := []struct {
		name      string
		err       error
		isPending bool
	}{
		{"Pending Error", workceptor.ErrPending, true},
		{"Non-pending Error", errors.New("test error"), false},
		{"Nil Error", nil, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if result := workceptor.IsPending(tc.err); result != tc.isPending {
				t.Errorf("expected %v, got %v", tc.isPending, result)
			}
		})
	}
}

func setUp(t *testing.T) (*gomock.Controller, workceptor.BaseWorkUnit, *workceptor.Workceptor, *mock_workceptor.MockNetceptorForWorkceptor) {
	ctrl := gomock.NewController(t)

	mockNetceptor := mock_workceptor.NewMockNetceptorForWorkceptor(ctrl)

	// attach logger to the mock netceptor and return any number of times
	logger := logger.NewReceptorLogger("")
	mockNetceptor.EXPECT().GetLogger().AnyTimes().Return(logger)
	mockNetceptor.EXPECT().NodeID().Return("NodeID")
	ctx := context.Background()
	w, err := workceptor.New(ctx, mockNetceptor, "/tmp")
	if err != nil {
		t.Errorf("Error while creating Workceptor: %v", err)
	}

	bwu := workceptor.BaseWorkUnit{}

	return ctrl, bwu, w, mockNetceptor
}

func TestInit(t *testing.T) {
	ctrl, bwu, w, _ := setUp(t)
	bwu.Init(w, "test", "test", workceptor.FileSystem{}, nil)
	ctrl.Finish()
}

func TestErrorLog(t *testing.T) {
	ctrl, bwu, w, _ := setUp(t)
	bwu.Init(w, "test", "test", workceptor.FileSystem{}, &workceptor.RealWatcher{})
	bwu.Error("test error")
	ctrl.Finish()
}

func TestWarningLog(t *testing.T) {
	ctrl, bwu, w, _ := setUp(t)
	bwu.Init(w, "test", "test", workceptor.FileSystem{}, &workceptor.RealWatcher{})
	bwu.Warning("test warning")
	ctrl.Finish()
}

func TestInfoLog(t *testing.T) {
	ctrl, bwu, w, _ := setUp(t)
	bwu.Init(w, "test", "test", workceptor.FileSystem{}, &workceptor.RealWatcher{})
	bwu.Info("test info")
	ctrl.Finish()
}

func TestDebugLog(t *testing.T) {
	ctrl, bwu, w, _ := setUp(t)
	bwu.Init(w, "test", "test", workceptor.FileSystem{}, &workceptor.RealWatcher{})
	bwu.Error("test debug")
	ctrl.Finish()
}

func TestSetFromParams(t *testing.T) {
	ctrl, bwu, w, _ := setUp(t)
	bwu.Init(w, "test", "test", workceptor.FileSystem{}, &workceptor.RealWatcher{})
	err := bwu.SetFromParams(nil)
	if err != nil {
		t.Errorf("SetFromParams should return nil: got %v", err)
	}
	ctrl.Finish()
}

const (
	rootDir  = "/tmp"
	testDir  = "NodeID/test"
	dirError = "no such file or directory"
)

func TestUnitDir(t *testing.T) {
	ctrl, bwu, w, _ := setUp(t)
	bwu.Init(w, "test", "test", workceptor.FileSystem{}, &workceptor.RealWatcher{})
	expectedUnitDir := path.Join(rootDir, testDir)
	if unitDir := bwu.UnitDir(); unitDir != expectedUnitDir {
		t.Errorf("UnitDir returned wrong value: got %s, want %s", unitDir, expectedUnitDir)
	}
	ctrl.Finish()
}

func TestID(t *testing.T) {
	ctrl, bwu, w, _ := setUp(t)
	bwu.Init(w, "test", "test", workceptor.FileSystem{}, &workceptor.RealWatcher{})
	if id := bwu.ID(); id != "test" {
		t.Errorf("ID returned wrong value: got %s, want %s", id, "test")
	}
	ctrl.Finish()
}

func TestStatusFileName(t *testing.T) {
	ctrl, bwu, w, _ := setUp(t)
	bwu.Init(w, "test", "", workceptor.FileSystem{}, &workceptor.RealWatcher{})
	expectedUnitDir := path.Join(rootDir, testDir)
	expectedStatusFileName := path.Join(expectedUnitDir, "status")
	if statusFileName := bwu.StatusFileName(); statusFileName != expectedStatusFileName {
		t.Errorf("StatusFileName returned wrong value: got %s, want %s", statusFileName, expectedStatusFileName)
	}
	ctrl.Finish()
}

func TestStdoutFileName(t *testing.T) {
	ctrl, bwu, w, _ := setUp(t)
	bwu.Init(w, "test", "", workceptor.FileSystem{}, &workceptor.RealWatcher{})
	expectedUnitDir := path.Join(rootDir, testDir)
	expectedStdoutFileName := path.Join(expectedUnitDir, "stdout")
	if stdoutFileName := bwu.StdoutFileName(); stdoutFileName != expectedStdoutFileName {
		t.Errorf("StdoutFileName returned wrong value: got %s, want %s", stdoutFileName, expectedStdoutFileName)
	}
	ctrl.Finish()
}

func TestBaseSave(t *testing.T) {
	ctrl, bwu, w, _ := setUp(t)
	bwu.Init(w, "test", "", workceptor.FileSystem{}, &workceptor.RealWatcher{})
	err := bwu.Save()
	if !strings.Contains(err.Error(), dirError) {
		t.Errorf("Base Work Unit Save, no such file or directory expected, instead %s", err.Error())
	}
	ctrl.Finish()
}

func TestBaseLoad(t *testing.T) {
	ctrl, bwu, w, _ := setUp(t)
	bwu.Init(w, "test", "", workceptor.FileSystem{}, &workceptor.RealWatcher{})
	err := bwu.Load()
	if !strings.Contains(err.Error(), dirError) {
		t.Errorf("TestBaseLoad, no such file or directory expected, instead %s", err.Error())
	}
	ctrl.Finish()
}

func TestBaseUpdateFullStatus(t *testing.T) {
	ctrl, bwu, w, _ := setUp(t)
	bwu.Init(w, "test", "", workceptor.FileSystem{}, &workceptor.RealWatcher{})
	sf := func(sfd *workceptor.StatusFileData) {
		// Do nothing
	}
	bwu.UpdateFullStatus(sf)
	err := bwu.LastUpdateError()
	if !strings.Contains(err.Error(), dirError) {
		t.Errorf("TestBaseUpdateFullStatus, no such file or directory expected, instead %s", err.Error())
	}
	ctrl.Finish()
}

func TestBaseUpdateBasicStatus(t *testing.T) {
	ctrl, bwu, w, _ := setUp(t)
	bwu.Init(w, "test", "", workceptor.FileSystem{}, &workceptor.RealWatcher{})
	bwu.UpdateBasicStatus(1, "Details", 0)
	err := bwu.LastUpdateError()
	if !strings.Contains(err.Error(), dirError) {
		t.Errorf("TestBaseUpdateBasicStatus, no such file or directory expected, instead %s", err.Error())
	}
	ctrl.Finish()
}

func TestBaseStatus(t *testing.T) {
	ctrl, bwu, w, _ := setUp(t)
	bwu.Init(w, "test", "", workceptor.FileSystem{}, &workceptor.RealWatcher{})
	status := bwu.Status()
	if status.State != workceptor.WorkStatePending {
		t.Errorf("TestBaseStatus, expected work state pending, received %d", status.State)
	}
	ctrl.Finish()
}

func TestBaseRelease(t *testing.T) {
	ctrl, bwu, w, _ := setUp(t)
	mockFileSystem := mock_workceptor.NewMockFileSystemer(ctrl)
	bwu.Init(w, "test", "", mockFileSystem, &workceptor.RealWatcher{})

	const removeError = "RemoveAll Error"
	testCases := []struct {
		name  string
		err   error
		force bool
		calls func()
	}{
		{
			name:  removeError,
			err:   errors.New(removeError),
			force: false,
			calls: func() { mockFileSystem.EXPECT().RemoveAll(gomock.Any()).Return(errors.New(removeError)).Times(3) },
		},
		{
			name:  "No remote error without force",
			err:   nil,
			force: false,
			calls: func() { mockFileSystem.EXPECT().RemoveAll(gomock.Any()).Return(nil) },
		},
		{
			name:  "No remote error with force",
			err:   nil,
			force: true,
			calls: func() { mockFileSystem.EXPECT().RemoveAll(gomock.Any()).Return(nil) },
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.calls()
			err := bwu.Release(tc.force)
			if err != nil && err.Error() != tc.err.Error() {
				t.Errorf("Error returned dosent match, err received %s, expected %s", err, tc.err)
			}
		})
	}

	ctrl.Finish()
}

func TestMonitorLocalStatus(t *testing.T) {
	tests := []struct {
		name          string
		statObj       *Info
		statObjLater  *Info
		addWatcherErr error
		statErr       error
		fsNotifyEvent *fsnotify.Event // using pointer to allow nil
		sleepDuration time.Duration
	}{
		{
			name:          "Handle Write Event",
			statObj:       NewInfo("test", 1, 0, time.Now()),
			addWatcherErr: nil,
			statErr:       nil,
			fsNotifyEvent: &fsnotify.Event{Op: fsnotify.Write},
			sleepDuration: 100 * time.Millisecond,
		},
		{
			name:          "Error Adding Watcher",
			statObj:       NewInfo("test", 1, 0, time.Now()),
			addWatcherErr: fmt.Errorf("error adding watcher"),
			statErr:       nil,
			fsNotifyEvent: nil,
			sleepDuration: 100 * time.Millisecond,
		},
		{
			name:          "Error Reading Status",
			statObj:       nil,
			addWatcherErr: fmt.Errorf("error adding watcher"),
			statErr:       fmt.Errorf("stat error"),
			fsNotifyEvent: nil,
			sleepDuration: 100 * time.Millisecond,
		},
		{
			name:          "Handle Context Cancellation",
			statObj:       NewInfo("test", 1, 0, time.Now()),
			addWatcherErr: nil,
			statErr:       nil,
			fsNotifyEvent: &fsnotify.Event{Op: fsnotify.Write},
			sleepDuration: 100 * time.Millisecond,
		},
		{
			name:          "Handle File Update Without Event",
			statObj:       NewInfo("test", 1, 0, time.Now()),
			statObjLater:  NewInfo("test", 1, 0, time.Now().Add(10*time.Second)),
			addWatcherErr: nil,
			statErr:       nil,
			fsNotifyEvent: &fsnotify.Event{Op: fsnotify.Write},
			sleepDuration: 500 * time.Millisecond,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl, bwu, w, _ := setUp(t)
			defer ctrl.Finish()

			mockWatcher := mock_workceptor.NewMockWatcherWrapper(ctrl)
			mockFileSystem := mock_workceptor.NewMockFileSystemer(ctrl)
			bwu.Init(w, "test", "", mockFileSystem, mockWatcher)

			mockFileSystem.EXPECT().Stat(gomock.Any()).Return(tc.statObj, tc.statErr).AnyTimes()
			if tc.statObjLater != nil {
				mockFileSystem.EXPECT().Stat(gomock.Any()).Return(tc.statObjLater, nil).AnyTimes()
			}
			mockWatcher.EXPECT().Add(gomock.Any()).Return(tc.addWatcherErr)
			mockWatcher.EXPECT().Close().AnyTimes()

			if tc.fsNotifyEvent != nil {
				eventCh := make(chan fsnotify.Event, 1)
				mockWatcher.EXPECT().EventChannel().Return(eventCh).AnyTimes()
				go func() { eventCh <- *tc.fsNotifyEvent }()
			}

			go bwu.MonitorLocalStatus()

			time.Sleep(tc.sleepDuration)
			bwu.CancelContext()
		})
	}
}
