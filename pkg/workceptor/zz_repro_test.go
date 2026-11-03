package workceptor

// Reproductions for AAP-97861 Path A. These encode the behaviour the fix must
// provide: releasing a work unit must not make an in-flight stdout reader or a
// status writer emit ERRORs, and a failed release must not corrupt the unit.

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
)

type reproNetceptor struct {
	log *logger.ReceptorLogger
}

func (n *reproNetceptor) NodeID() string                    { return "testnode" }
func (n *reproNetceptor) AddWorkCommand(string, bool) error { return nil }
func (n *reproNetceptor) GetClientTLSConfig(string, string, netceptor.ExpectedHostnameType) (*tls.Config, error) {
	return nil, nil
}
func (n *reproNetceptor) GetLogger() *logger.ReceptorLogger { return n.log }
func (n *reproNetceptor) DialContext(context.Context, string, string, *tls.Config) (*netceptor.Conn, error) {
	return nil, nil
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
	FileSystem
	err error
}

func (f failingFS) RemoveAll(string) error { return f.err }

// reproUnit adapts BaseWorkUnit to the WorkUnit interface for the reproduction.
type reproUnit struct {
	*BaseWorkUnit
}

func (u *reproUnit) Cancel() error {
	u.CancelContext()

	return nil
}
func (u *reproUnit) Restart() error { return nil }
func (u *reproUnit) Start() error   { return nil }

func newReproUnit(t *testing.T, unitID string, stdoutBytes int, state int, createDir bool) (*Workceptor, *reproUnit, *syncBuf) {
	t.Helper()
	logBuf := &syncBuf{}
	nc := &reproNetceptor{log: logger.NewReceptorLogger("")}
	nc.log.SetOutput(logBuf)

	w, err := New(context.Background(), nc, t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := os.MkdirAll(w.dataDir, 0o700); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}

	bwu := &BaseWorkUnit{}
	bwu.Init(w, unitID, "ansible-runner", FileSystem{})
	unit := &reproUnit{BaseWorkUnit: bwu}
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
	bwu.UpdateFullStatus(func(s *StatusFileData) {
		s.State = state
		s.Detail = "repro"
		s.StdoutSize = int64(len(stdout))
	})

	w.activeUnitsLock.Lock()
	w.activeUnits[unitID] = unit
	w.activeUnitsLock.Unlock()

	return w, unit, logBuf
}

// TestReproReleaseWhileStdoutStreaming releases a *running* unit while
// GetResults is mid-stream. The reader must stop cleanly instead of logging
// "Exceeded retries for reading stdout".
func TestReproReleaseWhileStdoutStreaming(t *testing.T) {
	w, _, logBuf := newReproUnit(t, "unit1", 4<<20, WorkStateRunning, true)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := w.GetResults(ctx, "unit1", 0)
	if err != nil {
		t.Fatalf("GetResults: %v", err)
	}

	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first stdout chunk")
	}

	if err := w.ReleaseUnit("unit1", false); err != nil {
		t.Fatalf("ReleaseUnit: %v", err)
	}

	// Drain slowly so the reader is still mid-stream when the stat watcher
	// would trip, rather than racing to EOF first.
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		for range ch { //nolint:revive // intentionally draining
			time.Sleep(200 * time.Millisecond)
		}
	}()
	select {
	case <-drained:
	case <-time.After(10 * time.Second):
		t.Fatal("stdout reader did not exit after release")
	}

	if strings.Contains(logBuf.String(), "Exceeded retries for reading stdout") {
		t.Fatalf("release produced stdout-retry ERROR spam:\n%s", logBuf.String())
	}
}

// TestReproReleaseWhileStatusWriterActive runs the in-process status writer
// (the path used by the remote proxy monitor) against a running unit while it
// is released. It must not surface an ENOENT on status.lock.
func TestReproReleaseWhileStatusWriterActive(t *testing.T) {
	w, unit, logBuf := newReproUnit(t, "unit2", 0, WorkStateRunning, true)

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
			unit.UpdateBasicStatus(WorkStateRunning, "running", 0)
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
	if err := w.ReleaseUnit("unit2", false); err != nil {
		t.Fatalf("ReleaseUnit: %v", err)
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
}

// TestReleaseFailureRestoresState guards the retry path: a failed RemoveAll
// must not leave a live unit marked canceled/released.
func TestReleaseFailureRestoresState(t *testing.T) {
	_, unit, _ := newReproUnit(t, "unit3", 0, WorkStateSucceeded, true)
	unit.fs = failingFS{err: errors.New("device busy")}

	if err := unit.Release(false); err == nil {
		t.Fatal("expected Release to fail")
	}
	if got := unit.Status().State; got != WorkStateSucceeded {
		t.Fatalf("state after failed release = %d, want %d", got, WorkStateSucceeded)
	}
	if unit.released {
		t.Fatal("released flag left set after failed release")
	}
}

// TestMissingDirIsNotReportedAsReleased ensures a stray ENOENT (bad data dir,
// filesystem fault) is still a real error, not silently treated as a release.
func TestMissingDirIsNotReportedAsReleased(t *testing.T) {
	_, unit, _ := newReproUnit(t, "unit4", 0, WorkStateRunning, false)

	unit.UpdateBasicStatus(WorkStateRunning, "running", 0)
	err := unit.LastUpdateError()
	if err == nil || !strings.Contains(err.Error(), "no such file or directory") {
		t.Fatalf("expected ENOENT for a unit that was never released, got %v", err)
	}
}

// TestUpdateFullStatusReleasedMapsENOENT covers the full-status update variant
// of the released-unit ENOENT mapping (UpdateBasicStatus is covered above).
func TestUpdateFullStatusReleasedMapsENOENT(t *testing.T) {
	w, unit, _ := newReproUnit(t, "unit5", 0, WorkStateRunning, true)

	if err := w.ReleaseUnit("unit5", false); err != nil {
		t.Fatalf("ReleaseUnit: %v", err)
	}
	unit.UpdateFullStatus(func(s *StatusFileData) { s.Detail = "after release" })
	if err := unit.LastUpdateError(); !errors.Is(err, ErrUnitReleased) {
		t.Fatalf("LastUpdateError = %v, want ErrUnitReleased", err)
	}
}

// TestReleaseForceFailureRestoresState covers the force branch where RemoveAll
// still fails: force ignores the error but the unit is not actually released.
func TestReleaseForceFailureRestoresState(t *testing.T) {
	_, unit, _ := newReproUnit(t, "unit6", 0, WorkStateSucceeded, true)
	unit.fs = failingFS{err: errors.New("device busy")}

	if err := unit.Release(true); err != nil {
		t.Fatalf("force Release returned %v, want nil (pre-existing force semantics)", err)
	}
	if got := unit.Status().State; got != WorkStateSucceeded {
		t.Fatalf("state after failed force release = %d, want %d", got, WorkStateSucceeded)
	}
	if unit.released {
		t.Fatal("released flag left set after failed force release")
	}
}
