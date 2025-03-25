package workceptor

import (
	"context"
	"crypto/tls"
	"fmt"
	"math"
	"os"
	"path"
	"strings"
	"sync"
	"testing"

	"slices"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
	// "github.com/ansible/receptor/pkg/workceptor/mock_workceptor"
)

type baseWorkUnitForWorkUnitMock struct {
	statusLock *sync.RWMutex
}

func (mock *baseWorkUnitForWorkUnitMock) CancelContext() {

}

func (mock *baseWorkUnitForWorkUnitMock) ID() string {
	return ""
}

func (mock *baseWorkUnitForWorkUnitMock) Init(w *Workceptor, unitID string, workType string, fs FileSystemer, watcher WatcherWrapper) {
}

func (mock *baseWorkUnitForWorkUnitMock) LastUpdateError() error {
	return nil
}

func (mock *baseWorkUnitForWorkUnitMock) Load() error {
	return nil
}

func (mock *baseWorkUnitForWorkUnitMock) MonitorLocalStatus() {

}

func (mock *baseWorkUnitForWorkUnitMock) Release(force bool) error {
	return nil
}

func (mock *baseWorkUnitForWorkUnitMock) Save() error {
	return nil
}

func (mock *baseWorkUnitForWorkUnitMock) SetFromParams(_ map[string]string) error {
	return nil
}

func (mock *baseWorkUnitForWorkUnitMock) Status() *StatusFileData {
	return nil
}

func (mock *baseWorkUnitForWorkUnitMock) StatusFileName() string {
	return ""
}

func (mock *baseWorkUnitForWorkUnitMock) StdoutFileName() string {
	return ""
}

func (mock *baseWorkUnitForWorkUnitMock) UnitDir() string {
	return ""
}

func (mock *baseWorkUnitForWorkUnitMock) UnredactedStatus() *StatusFileData {
	return nil
}

func (mock *baseWorkUnitForWorkUnitMock) UpdateBasicStatus(state int, detail string, stdoutSize int64) {
}

func (mock *baseWorkUnitForWorkUnitMock) UpdateFullStatus(statusFunc func(*StatusFileData)) {

}

func (mock *baseWorkUnitForWorkUnitMock) GetStatusCopy() StatusFileData {
	return StatusFileData{
		ExtraData: &CommandExtraData{},
	}
}

func (mock *baseWorkUnitForWorkUnitMock) GetStatusWithoutExtraData() *StatusFileData {
	return &StatusFileData{}
}

func (mock *baseWorkUnitForWorkUnitMock) SetStatusExtraData(interface{}) {

}

func (mock *baseWorkUnitForWorkUnitMock) GetStatusLock() *sync.RWMutex {
	return mock.statusLock
}

func (mock *baseWorkUnitForWorkUnitMock) GetWorkceptor() *Workceptor {
	return nil
}

func (mock *baseWorkUnitForWorkUnitMock) SetWorkceptor(*Workceptor) {

}

func (mock *baseWorkUnitForWorkUnitMock) GetContext() context.Context {
	return nil
}

func (mock *baseWorkUnitForWorkUnitMock) GetCancel() context.CancelFunc {
	return nil
}

// ============================================================

type netceptorForWorkceptorMock struct {
}

func (mock *netceptorForWorkceptorMock) NodeID() string {
	return ""
}

func (mock *netceptorForWorkceptorMock) AddWorkCommand(typeName string, verifySignature bool) error {
	return nil
}

func (mock *netceptorForWorkceptorMock) GetClientTLSConfig(name string, expectedHostName string, expectedHostNameType netceptor.ExpectedHostnameType) (*tls.Config, error) {
	return nil, nil
}

func (mock *netceptorForWorkceptorMock) GetLogger() *logger.ReceptorLogger {
	return nil
}

func (mock *netceptorForWorkceptorMock) DialContext(ctx context.Context, node string, service string, tlscfg *tls.Config) (*netceptor.Conn, error) {
	return nil, nil
}

// ============================================================

// Creates a no-op script that can be run during tests.
// Assumes that /tmp is in PATH which is configured by .vscode/settings.json file.
func createReceptorPythonWorkerScript() error {
	// tmpDir := os.TempDir()
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

	// _, err = f.WriteString("#!/usr/bin/env /bin/sh\necho \"\" > /dev/")
	_, err = f.WriteString("#!/usr/bin/env /bin/sh\necho \"\" > /dev/null")
	if err != nil {
		return fmt.Errorf("Error writing to %s: %v", filename, err)
	}

	err = os.Chmod(absoluteFilename, 0755)
	if err != nil {
		return fmt.Errorf("Error making %s executable: %v", absoluteFilename, err)
	}

	return nil
}

func TestPythonUnitStartRunsToSuccess(t *testing.T) {
	pw := &pythonUnit{
		commandUnit: commandUnit{
			BaseWorkUnitForWorkUnit: &baseWorkUnitForWorkUnitMock{
				statusLock: &sync.RWMutex{},
			},
		},
		plugin:   "",
		function: "",
		config: map[string]any{
			"test": true,
		},
	}

	// Create an executable script /tmp/receptor-python-worker that echoes
	// "Hello Receptor" to satisfy the exec.Command() call which hard codes
	// the command in the pythonUnit.Start() method.
	createReceptorPythonWorkerScript()

	err := pw.Start()
	if err != nil {
		t.Errorf("Error when testing Start method of pythonUnit: %v", err)
	}
}

func TestPythonUnitStartFailsOnInvalidConfig(t *testing.T) {
	pw := &pythonUnit{
		commandUnit: commandUnit{
			BaseWorkUnitForWorkUnit: &baseWorkUnitForWorkUnitMock{
				statusLock: &sync.RWMutex{},
			},
		},
		plugin:   "",
		function: "",
		config: map[string]any{
			"test":      true,
			"badConfig": math.Inf(1),
		},
	}

	// Create an executable script /tmp/receptor-python-worker that echoes
	// "Hello Receptor" to satisfy the exec.Command() call which hard codes
	// the command in the pythonUnit.Start() method.
	createReceptorPythonWorkerScript()

	err := pw.Start()
	if err == nil {
		t.Errorf("Expected json marshal error for configuration.")
	}
}

func TestWorkPythonConfigNewWorkerRunsToSuccess(t *testing.T) {
	wpc := &WorkPythonCfg{}
	baseUnit := BaseWorkUnit{}
	workceptor := &Workceptor{
		ctx: context.TODO(),
	}
	workUnit := wpc.NewWorker(&baseUnit, workceptor, "", "")
	if workUnit == nil {
		t.Error("Returned WorkUnit was nil")
	}
}

func TestWorkPythonConfigRunRunsToSuccess(t *testing.T) {
	wpc := &WorkPythonCfg{}
	// baseUnit := BaseWorkUnit{}
	MainInstance = &Workceptor{
		ctx:             context.TODO(),
		workTypesLock:   &sync.RWMutex{},
		workTypes:       map[string]*workType{},
		nc:              &netceptorForWorkceptorMock{},
		activeUnitsLock: &sync.RWMutex{},
	}
	err := wpc.Run()
	if err == nil {
		t.Errorf("Expected deprecation warning but received none.")
	}
}
