package workceptor_test

import (
	"context"
	"fmt"
	"math"
	"os"
	"path"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/ansible/receptor/pkg/workceptor"
	"github.com/ansible/receptor/pkg/workceptor/mock_workceptor"
	"go.uber.org/mock/gomock"
)

func createPythonUnitTestSetup(t *testing.T) (workceptor.WorkUnit, *mock_workceptor.MockBaseWorkUnitForWorkUnit, *mock_workceptor.MockNetceptorForWorkceptor, *workceptor.Workceptor) {
	ctrl := gomock.NewController(t)
	ctx := context.Background()

	mockBaseWorkUnit := mock_workceptor.NewMockBaseWorkUnitForWorkUnit(ctrl)
	mockNetceptor := mock_workceptor.NewMockNetceptorForWorkceptor(ctrl)
	mockNetceptor.EXPECT().NodeID().Return("NodeID")
	mockNetceptor.EXPECT().GetLogger()

	w, err := workceptor.New(ctx, mockNetceptor, "/tmp")
	if err != nil {
		t.Errorf("Error while creating Workceptor: %v", err)
	}

	mockBaseWorkUnit.EXPECT().Init(w, "", "", workceptor.FileSystem{}, nil)
	mockBaseWorkUnit.EXPECT().SetStatusExtraData(gomock.Any())
	workUnit := workceptor.NewRemoteWorker(mockBaseWorkUnit, w, "", "")

	return workUnit, mockBaseWorkUnit, mockNetceptor, w
}

// Creates a no-op script that can be run during tests.
func createReceptorPythonWorkerScript() error {
	tmpDir := "/tmp"
	filename := "receptor-python-worker"
	absoluteFilename := path.Join(tmpDir, filename)
	if workDir, err := os.Getwd(); err != nil {
		fmt.Printf("os.Getwd=%s", workDir)
	}

	tmpInPath := slices.Contains(strings.Split(os.Getenv("PATH"), ":"), tmpDir)
	if !tmpInPath {
		newPath := os.Getenv("PATH") + ":" + tmpDir
		err := os.Setenv("PATH", newPath)
		if err != nil {
			return fmt.Errorf("Error setting PATH: %v", err)
		}
	}

	f, err := os.Create(absoluteFilename)
	if err != nil {
		return fmt.Errorf("Error creating %s: %v", filename, err)
	}
	defer f.Close()

	_, err = f.WriteString("#!/usr/bin/env /bin/sh\necho \"\" > /dev/null")
	if err != nil {
		return fmt.Errorf("Error writing to %s: %v", filename, err)
	}

	err = os.Chmod(absoluteFilename, 0o755)
	if err != nil {
		return fmt.Errorf("Error making %s executable: %v", absoluteFilename, err)
	}

	return nil
}

func TestPythonUnitStartRunsToSuccess(t *testing.T) {
	_, mockBaseWorkUnit, _, _ := createPythonUnitTestSetup(t) //nolint:dogsled
	pw := workceptor.NewPythonUnit(mockBaseWorkUnit, "", "", nil)
	statusLock := &sync.RWMutex{}

	mockBaseWorkUnit.EXPECT().UpdateBasicStatus(gomock.Any(), gomock.Any(), gomock.Any()).Times(2)
	mockBaseWorkUnit.EXPECT().GetStatusLock().Return(statusLock).Times(2)
	mockBaseWorkUnit.EXPECT().GetStatusWithoutExtraData().Return(&workceptor.StatusFileData{})
	mockBaseWorkUnit.EXPECT().GetStatusCopy().Return(workceptor.StatusFileData{
		ExtraData: &workceptor.CommandExtraData{},
	})
	mockBaseWorkUnit.EXPECT().UnitDir()
	mockBaseWorkUnit.EXPECT().UpdateFullStatus(gomock.Any())
	mockBaseWorkUnit.EXPECT().MonitorLocalStatus().AnyTimes()
	mockBaseWorkUnit.EXPECT().UpdateFullStatus(gomock.Any()).AnyTimes()

	createReceptorPythonWorkerScript()

	err := pw.Start()
	if err != nil {
		t.Errorf("Error when testing Start method of pythonUnit: %v", err)
	}
}

func TestPythonUnitStartFailsOnInvalidConfig(t *testing.T) {
	_, mockBaseWorkUnit, _, _ := createPythonUnitTestSetup(t) //nolint:dogsled

	statusLock := &sync.RWMutex{}

	mockBaseWorkUnit.EXPECT().UpdateBasicStatus(gomock.Any(), gomock.Any(), gomock.Any()).Times(2)
	mockBaseWorkUnit.EXPECT().GetStatusLock().Return(statusLock).Times(2)
	mockBaseWorkUnit.EXPECT().GetStatusWithoutExtraData().Return(&workceptor.StatusFileData{})
	mockBaseWorkUnit.EXPECT().GetStatusCopy().Return(workceptor.StatusFileData{
		ExtraData: &workceptor.CommandExtraData{},
	})

	badConfig := map[string]any{
		"badOption": math.Inf(1),
	}
	pw := workceptor.NewPythonUnit(mockBaseWorkUnit, "", "", badConfig)

	createReceptorPythonWorkerScript()

	err := pw.Start()
	if err == nil {
		t.Errorf("Expected json marshal error for configuration.")
	}
}

func TestWorkPythonConfigNewWorkerRunsToSuccess(t *testing.T) {
	_, mockBaseWorkUnitForWorkUnit, mockNetceptorForWorkceptor, _ := createPythonUnitTestSetup(t)
	mockNetceptorForWorkceptor.EXPECT().NodeID().AnyTimes()

	wpc := &workceptor.WorkPythonCfg{}
	w, err := workceptor.New(context.TODO(), mockNetceptorForWorkceptor, "")
	if err != nil {
		t.Errorf("Error when creating workceptor for test: %v", err)
	}
	workUnit := wpc.NewWorker(mockBaseWorkUnitForWorkUnit, w, "", "")
	if workUnit == nil {
		t.Error("Returned WorkUnit was nil")
	}
}

func TestWorkPythonConfigRunRunsToSuccess(t *testing.T) {
	_, _, mockNetceptorForWorkceptor, _ := createPythonUnitTestSetup(t) //nolint:dogsled
	mockNetceptorForWorkceptor.EXPECT().NodeID().AnyTimes()
	mockNetceptorForWorkceptor.EXPECT().AddWorkCommand("", false)

	wpc := &workceptor.WorkPythonCfg{}
	w, err := workceptor.New(context.TODO(), mockNetceptorForWorkceptor, "")
	if err != nil {
		t.Errorf("Error when creating workceptor for test: %v", err)
	}
	workceptor.MainInstance = w
	err = wpc.Run()
	if err == nil {
		t.Errorf("Expected deprecation warning but received none.")
	}
}
