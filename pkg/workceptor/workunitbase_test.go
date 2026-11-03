package workceptor_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/randstr"
	"github.com/ansible/receptor/pkg/workceptor"
	"github.com/ansible/receptor/pkg/workceptor/mock_workceptor"
	"github.com/fsnotify/fsnotify"
	"go.uber.org/mock/gomock"
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
		{"Canceled Work Description", workceptor.WorkStateCanceled, "Canceled"},
		{"Unknown Work Description", 999, "Unknown: 999"},
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

func setUp(t *testing.T) (*gomock.Controller, workceptor.BaseWorkUnit, *workceptor.Workceptor, *mock_workceptor.MockNetceptorForWorkceptor, *logger.ReceptorLogger) {
	ctrl := gomock.NewController(t)
	mockNetceptor := mock_workceptor.NewMockNetceptorForWorkceptor(ctrl)
	logger := logger.NewReceptorLogger("")

	mockNetceptor.EXPECT().GetLogger().AnyTimes().Return(logger)
	mockNetceptor.EXPECT().NodeID().Return("NodeID")
	ctx := context.Background()
	w, err := workceptor.New(ctx, mockNetceptor, "/tmp")
	if err != nil {
		t.Errorf("Error while creating Workceptor: %v", err)
	}

	bwu := workceptor.BaseWorkUnit{}

	return ctrl, bwu, w, mockNetceptor, logger
}

func TestInit(t *testing.T) {
	ctrl, bwu, w, _, _ := setUp(t)
	bwu.Init(w, "test", "test", workceptor.FileSystem{})
	ctrl.Finish()
}

func TestErrorLog(t *testing.T) {
	ctrl, bwu, w, _, _ := setUp(t)
	bwu.Init(w, "test", "test", workceptor.FileSystem{})
	bwu.Error("test error")
	ctrl.Finish()
}

func TestWarningLog(t *testing.T) {
	ctrl, bwu, w, _, _ := setUp(t)
	bwu.Init(w, "test", "test", workceptor.FileSystem{})
	bwu.Warning("test warning")
	ctrl.Finish()
}

func TestInfoLog(t *testing.T) {
	ctrl, bwu, w, _, _ := setUp(t)
	bwu.Init(w, "test", "test", workceptor.FileSystem{})
	bwu.Info("test info")
	ctrl.Finish()
}

func TestDebugLog(t *testing.T) {
	ctrl, bwu, w, _, _ := setUp(t)
	bwu.Init(w, "test", "test", workceptor.FileSystem{})
	bwu.Error("test debug")
	ctrl.Finish()
}

func TestSetFromParams(t *testing.T) {
	ctrl, bwu, w, _, _ := setUp(t)
	bwu.Init(w, "test", "test", workceptor.FileSystem{})
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
	ctrl, bwu, w, _, _ := setUp(t)
	bwu.Init(w, "test", "test", workceptor.FileSystem{})
	expectedUnitDir := path.Join(rootDir, testDir)
	if unitDir := bwu.UnitDir(); unitDir != expectedUnitDir {
		t.Errorf("UnitDir returned wrong value: got %s, want %s", unitDir, expectedUnitDir)
	}
	ctrl.Finish()
}

func TestID(t *testing.T) {
	ctrl, bwu, w, _, _ := setUp(t)
	bwu.Init(w, "test", "test", workceptor.FileSystem{})
	if id := bwu.ID(); id != "test" {
		t.Errorf("ID returned wrong value: got %s, want %s", id, "test")
	}
	ctrl.Finish()
}

func TestStatusFileName(t *testing.T) {
	ctrl, bwu, w, _, _ := setUp(t)
	bwu.Init(w, "test", "", workceptor.FileSystem{})
	expectedUnitDir := path.Join(rootDir, testDir)
	expectedStatusFileName := path.Join(expectedUnitDir, "status")
	if statusFileName := bwu.StatusFileName(); statusFileName != expectedStatusFileName {
		t.Errorf("StatusFileName returned wrong value: got %s, want %s", statusFileName, expectedStatusFileName)
	}
	ctrl.Finish()
}

func TestStdoutFileName(t *testing.T) {
	ctrl, bwu, w, _, _ := setUp(t)
	bwu.Init(w, "test", "", workceptor.FileSystem{})
	expectedUnitDir := path.Join(rootDir, testDir)
	expectedStdoutFileName := path.Join(expectedUnitDir, "stdout")
	if stdoutFileName := bwu.StdoutFileName(); stdoutFileName != expectedStdoutFileName {
		t.Errorf("StdoutFileName returned wrong value: got %s, want %s", stdoutFileName, expectedStdoutFileName)
	}
	ctrl.Finish()
}

func TestBaseSave(t *testing.T) {
	ctrl, bwu, w, _, _ := setUp(t)
	bwu.Init(w, "test", "", workceptor.FileSystem{})
	err := bwu.Save()
	if !strings.Contains(err.Error(), dirError) {
		t.Errorf("Base Work Unit Save, no such file or directory expected, instead %s", err.Error())
	}
	ctrl.Finish()
}

func TestBaseLoad(t *testing.T) {
	ctrl, bwu, w, _, _ := setUp(t)
	bwu.Init(w, "test", "", workceptor.FileSystem{})
	err := bwu.Load()
	if !strings.Contains(err.Error(), dirError) {
		t.Errorf("TestBaseLoad, no such file or directory expected, instead %s", err.Error())
	}
	ctrl.Finish()
}

func TestBaseUpdateFullStatus(t *testing.T) {
	ctrl, bwu, w, _, _ := setUp(t)
	bwu.Init(w, "test", "", workceptor.FileSystem{})
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
	ctrl, bwu, w, _, _ := setUp(t)
	bwu.Init(w, "test", "", workceptor.FileSystem{})
	bwu.UpdateBasicStatus(1, "Details", 0)
	err := bwu.LastUpdateError()
	if !strings.Contains(err.Error(), dirError) {
		t.Errorf("TestBaseUpdateBasicStatus, no such file or directory expected, instead %s", err.Error())
	}
	ctrl.Finish()
}

func TestBaseStatus(t *testing.T) {
	ctrl, bwu, w, _, _ := setUp(t)
	bwu.Init(w, "test", "", workceptor.FileSystem{})
	status := bwu.Status()
	if status.State != workceptor.WorkStatePending {
		t.Errorf("TestBaseStatus, expected work state pending, received %d", status.State)
	}
	ctrl.Finish()
}

func TestBaseRelease(t *testing.T) {
	ctrl, bwu, w, _, _ := setUp(t)
	mockFileSystem := mock_workceptor.NewMockFileSystemer(ctrl)
	bwu.Init(w, "test12345", "", mockFileSystem)

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
		logOutput     string
		sleepDuration time.Duration
	}{
		{
			name:          "Error Reading Status",
			statObj:       nil,
			addWatcherErr: fmt.Errorf("error adding watcher"),
			statErr:       fmt.Errorf("stat error"),
			fsNotifyEvent: nil,
			logOutput:     "",
			sleepDuration: 100 * time.Millisecond,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl, bwu, w, _, l := setUp(t)
			defer ctrl.Finish()
			randstring := randstr.RandomString(4)
			logFilePath := fmt.Sprintf("/tmp/monitorLocalStatusLog%s", randstring)

			mockFileSystem := mock_workceptor.NewMockFileSystemer(ctrl)
			bwu.Init(w, "test", "", mockFileSystem)

			mockFileSystem.EXPECT().Stat(gomock.Any()).Return(tc.statObj, tc.statErr).AnyTimes()
			if tc.statObjLater != nil {
				mockFileSystem.EXPECT().Stat(gomock.Any()).Return(tc.statObjLater, nil).AnyTimes()
			}

			logFile, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
			if err != nil {
				t.Error("error creating monitorLocalStatusLog file")
			}
			l.SetOutput(logFile)

			go bwu.MonitorLocalStatus()
			time.Sleep(tc.sleepDuration)
			logOutput, err := os.ReadFile(logFilePath)
			if err != nil && len(logOutput) == 0 {
				t.Errorf("error reading %s file", logFilePath)
			}
			if !bytes.Contains(logOutput, []byte(tc.logOutput)) {
				t.Errorf("expected log to be: %s, got %s", tc.logOutput, string(logOutput))
			}

			bwu.CancelContext()
		})
	}
}

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.b.String()
}

// failingFS lets a release fail the way a busy/NFS-backed data dir can.
type failingFS struct {
	workceptor.FileSystem
	err error
}

func (f failingFS) RemoveAll(string) error { return f.err }

// testUnit adapts BaseWorkUnit to the WorkUnit interface for release tests.
type testUnit struct {
	*workceptor.BaseWorkUnit
}

func (u *testUnit) Cancel() error {
	u.CancelContext()

	return nil
}

func (u *testUnit) Restart() error { return nil }

func (u *testUnit) Start() error { return nil }

// newTestUnit builds a Workceptor plus a unit registered in its active-unit map,
// so tests can release the unit or read its results without a real worker.
func newTestUnit(t *testing.T, unitID string, stdoutBytes int, state int, createDir bool, unitFS workceptor.FileSystemer) (*workceptor.Workceptor, *testUnit, *syncBuf) {
	t.Helper()
	if unitFS == nil {
		unitFS = workceptor.FileSystem{}
	}
	ctrl := gomock.NewController(t)
	nc := mock_workceptor.NewMockNetceptorForWorkceptor(ctrl)
	nc.EXPECT().NodeID().Return("testnode").AnyTimes()
	logBuf := &syncBuf{}
	lg := logger.NewReceptorLogger("")
	lg.SetOutput(logBuf)
	nc.EXPECT().GetLogger().Return(lg).AnyTimes()

	w, err := workceptor.New(context.Background(), nc, t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	bwu := &workceptor.BaseWorkUnit{}
	bwu.Init(w, unitID, "ansible-runner", unitFS)
	unit := &testUnit{BaseWorkUnit: bwu}
	if createDir {
		if err := os.MkdirAll(bwu.UnitDir(), 0o700); err != nil {
			t.Fatalf("mkdir unit dir: %v", err)
		}
	}

	stdout := bytes.Repeat([]byte("x"), stdoutBytes)
	if stdoutBytes > 0 {
		if err := os.WriteFile(bwu.StdoutFileName(), stdout, 0o600); err != nil {
			t.Fatalf("write stdout: %v", err)
		}
	}
	bwu.UpdateFullStatus(func(s *workceptor.StatusFileData) {
		s.State = state
		s.Detail = "test unit"
		s.StdoutSize = int64(len(stdout))
	})

	w.AddActiveUnitForTest(unitID, unit)

	return w, unit, logBuf
}

// TestReleaseWhileStatusWriterActive runs the in-process status writer (the
// path used by the remote proxy monitor) against a running unit while it is
// released. It must not surface an ENOENT on status.lock.
func TestReleaseWhileStatusWriterActive(t *testing.T) {
	_, unit, logBuf := newTestUnit(t, "unit2", 0, workceptor.WorkStateRunning, true, nil)

	var wg sync.WaitGroup
	stop := make(chan struct{})
	errCh := make(chan error, 512)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			unit.UpdateBasicStatus(workceptor.WorkStateRunning, "running", 0)
			if err := unit.LastUpdateError(); err != nil {
				select {
				case errCh <- err:
				default:
				}
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()

	time.Sleep(50 * time.Millisecond)
	if err := unit.Release(false); err != nil {
		t.Fatalf("Release: %v", err)
	}
	time.Sleep(250 * time.Millisecond)
	close(stop)
	wg.Wait()
	close(errCh)

	var errs []string
	for err := range errCh {
		errs = append(errs, err.Error())
	}
	for _, e := range errs {
		if strings.Contains(e, "no such file or directory") || strings.Contains(e, "status.lock") {
			t.Fatalf("release raced the status writer (%d errors), e.g.: %s", len(errs), e)
		}
	}
	if strings.Contains(logBuf.String(), "Error updating status file") {
		t.Fatalf("release logged status-write errors:\n%s", logBuf.String())
	}

	unit.UpdateBasicStatus(workceptor.WorkStateRunning, "after release", 0)
	if err := unit.LastUpdateError(); !errors.Is(err, workceptor.ErrUnitReleased) {
		t.Fatalf("post-release basic status error = %v, want ErrUnitReleased", err)
	}
}

// TestReleaseFailureRestoresState guards the retry path: a failed RemoveAll
// must not leave a live unit marked canceled/released.
func TestReleaseFailureRestoresState(t *testing.T) {
	_, unit, _ := newTestUnit(t, "unit3", 0, workceptor.WorkStateSucceeded, true, failingFS{err: errors.New("device busy")})

	if err := unit.Release(false); err == nil {
		t.Fatal("expected Release to fail")
	}
	if got := unit.Status().State; got != workceptor.WorkStateSucceeded {
		t.Fatalf("state after failed release = %d, want %d", got, workceptor.WorkStateSucceeded)
	}
	if unit.IsReleased() {
		t.Fatal("released flag left set after failed release")
	}
}

// TestMissingDirIsNotReportedAsReleased ensures a stray ENOENT (bad data dir,
// filesystem fault) is still a real error, not silently treated as a release.
func TestMissingDirIsNotReportedAsReleased(t *testing.T) {
	_, unit, _ := newTestUnit(t, "unit4", 0, workceptor.WorkStateRunning, false, nil)

	unit.UpdateBasicStatus(workceptor.WorkStateRunning, "running", 0)
	err := unit.LastUpdateError()
	if err == nil || !strings.Contains(err.Error(), "no such file or directory") {
		t.Fatalf("expected ENOENT for a unit that was never released, got %v", err)
	}
}

// TestUpdateFullStatusReleasedMapsENOENT covers the full-status update variant
// of the released-unit ENOENT mapping (UpdateBasicStatus is covered above).
func TestUpdateFullStatusReleasedMapsENOENT(t *testing.T) {
	_, unit, _ := newTestUnit(t, "unit5", 0, workceptor.WorkStateRunning, true, nil)

	if err := unit.Release(false); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if !unit.IsReleased() {
		t.Fatal("Release did not record the released state")
	}
	unit.UpdateFullStatus(func(s *workceptor.StatusFileData) { s.Detail = "after release" })
	if err := unit.LastUpdateError(); !errors.Is(err, workceptor.ErrUnitReleased) {
		t.Fatalf("LastUpdateError = %v, want ErrUnitReleased", err)
	}
}

// TestForceReleaseFailureMarksReleased covers the force branch where RemoveAll
// still fails: force-release is best-effort, so the unit is reported released
// even though the files could not be removed.
func TestForceReleaseFailureMarksReleased(t *testing.T) {
	_, unit, _ := newTestUnit(t, "unit6", 0, workceptor.WorkStateSucceeded, true, failingFS{err: errors.New("device busy")})

	if err := unit.Release(true); err != nil {
		t.Fatalf("force Release returned %v, want nil (pre-existing force semantics)", err)
	}
	if got := unit.Status().State; got != workceptor.WorkStateCanceled {
		t.Fatalf("state after force release = %d, want %d", got, workceptor.WorkStateCanceled)
	}
	if !unit.IsReleased() {
		t.Fatal("force release did not record the released state")
	}
}
