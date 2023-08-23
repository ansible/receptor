package workceptor_test

import (
	"context"
	"errors"
	"path"
	"testing"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/workceptor"
	"github.com/ansible/receptor/pkg/workceptor/mock_workceptor"
	"github.com/golang/mock/gomock"
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
	bwu.Init(w, "test", "test", workceptor.FileSystem{})
	ctrl.Finish()
}

func TestErrorLog(t *testing.T) {
	ctrl, bwu, w, _ := setUp(t)
	bwu.Init(w, "test", "test", workceptor.FileSystem{})
	bwu.Error("test error")
	ctrl.Finish()
}

func TestWarningLog(t *testing.T) {
	ctrl, bwu, w, _ := setUp(t)
	bwu.Init(w, "test", "test", workceptor.FileSystem{})
	bwu.Warning("test warning")
	ctrl.Finish()
}

func TestInfoLog(t *testing.T) {
	ctrl, bwu, w, _ := setUp(t)
	bwu.Init(w, "test", "test", workceptor.FileSystem{})
	bwu.Info("test info")
	ctrl.Finish()
}

func TestDebugLog(t *testing.T) {
	ctrl, bwu, w, _ := setUp(t)
	bwu.Init(w, "test", "test", workceptor.FileSystem{})
	bwu.Error("test debug")
	ctrl.Finish()
}

func TestSetFromParams(t *testing.T) {
	ctrl, bwu, w, _ := setUp(t)
	bwu.Init(w, "test", "test", workceptor.FileSystem{})
	err := bwu.SetFromParams(nil)
	if err != nil {
		t.Errorf("SetFromParams should return nil: got %v", err)
	}
	ctrl.Finish()
}

func TestUnitDir(t *testing.T) {
	ctrl, bwu, w, _ := setUp(t)
	bwu.Init(w, "test", "test", workceptor.FileSystem{})
	expectedUnitDir := path.Join("/tmp", "NodeID/test")
	if unitDir := bwu.UnitDir(); unitDir != expectedUnitDir {
		t.Errorf("UnitDir returned wrong value: got %s, want %s", unitDir, expectedUnitDir)
	}
	ctrl.Finish()
}

func TestID(t *testing.T) {
	ctrl, bwu, w, _ := setUp(t)
	bwu.Init(w, "test", "test", workceptor.FileSystem{})
	if id := bwu.ID(); id != "test" {
		t.Errorf("ID returned wrong value: got %s, want %s", id, "test")
	}
	ctrl.Finish()
}

func TestStatusFileName(t *testing.T) {
	ctrl, bwu, w, _ := setUp(t)
	bwu.Init(w, "test", "", workceptor.FileSystem{})
	expectedUnitDir := path.Join("/tmp", "NodeID/test")
	expectedStatusFileName := path.Join(expectedUnitDir, "status")
	if statusFileName := bwu.StatusFileName(); statusFileName != expectedStatusFileName {
		t.Errorf("StatusFileName returned wrong value: got %s, want %s", statusFileName, expectedStatusFileName)
	}
	ctrl.Finish()
}

func TestStdoutFileName(t *testing.T) {
	ctrl, bwu, w, _ := setUp(t)
	bwu.Init(w, "test", "", workceptor.FileSystem{})
	expectedUnitDir := path.Join("/tmp", "NodeID/test")
	expectedStdoutFileName := path.Join(expectedUnitDir, "stdout")
	if stdoutFileName := bwu.StdoutFileName(); stdoutFileName != expectedStdoutFileName {
		t.Errorf("StdoutFileName returned wrong value: got %s, want %s", stdoutFileName, expectedStdoutFileName)
	}
	ctrl.Finish()
}

func TestSaveBaseWorkUnit(t *testing.T) {
	ctrl, bwu, w, _ := setUp(t)
	mockFileSystem := mock_workceptor.NewMockFileSystemer(ctrl)
	//mockFileWriter := mock_workceptor.NewMockFileWriter(ctrl)
	//mockFileWriter.EXPECT().Write(gomock.Any()).Return(0, nil)
	mockStatusFileData := mock_workceptor.NewMockStatusFileDataer(ctrl)
	bwu.Init(w, "test", "", mockFileSystem)
	bwu.SetStatusFileData(mockStatusFileData)
	mockStatusFileData.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
	//mockFileSystem.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(&os.File{}, nil)
	err := bwu.Save()
	if err != nil {
		t.Errorf("Base work unit Save returned wrong value: got %s, want nil", err.Error())
	}
}
