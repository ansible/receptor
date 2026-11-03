//go:build !no_workceptor
// +build !no_workceptor

package workceptor

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/utils"
)

type remoteWorkStatusWriterFake struct {
	BaseWorkUnitForWorkUnit
	writeErrors []error
	lastError   error
	writes      int
	persisted   string
}

func (f *remoteWorkStatusWriterFake) UpdateFullStatus(update func(*StatusFileData)) {
	f.writes++
	f.lastError = nil
	if f.writes <= len(f.writeErrors) {
		f.lastError = f.writeErrors[f.writes-1]
	}
	if f.lastError != nil {
		return
	}
	status := &StatusFileData{ExtraData: &RemoteExtraData{RemoteWorkType: f.persisted}}
	update(status)
	f.persisted = status.ExtraData.(*RemoteExtraData).RemoteWorkType
}

func (f *remoteWorkStatusWriterFake) LastUpdateError() error {
	return f.lastError
}

func TestRemoteWorkRetriesRemoteWorkTypeStatusWrite(t *testing.T) {
	writer := &remoteWorkStatusWriterFake{writeErrors: []error{errors.New("temporary write failure")}}
	rw := &remoteUnit{BaseWorkUnitForWorkUnit: writer}

	cachedType, err := rw.updateRemoteWorkType("", "echoint")
	if err == nil {
		t.Fatal("expected the first status write to fail")
	}
	if cachedType != "" || writer.persisted != "" {
		t.Fatalf("failed write cached %q and persisted %q; want both empty", cachedType, writer.persisted)
	}

	cachedType, err = rw.updateRemoteWorkType(cachedType, "echoint")
	if err != nil {
		t.Fatal(err)
	}
	if cachedType != "echoint" || writer.persisted != "echoint" {
		t.Fatalf("retry cached %q and persisted %q; want echoint", cachedType, writer.persisted)
	}
	if writer.writes != 2 {
		t.Fatalf("status writes = %d, want retry after first failure", writer.writes)
	}
}

func TestRemoteWorkKeepsKnownOrEmptyRemoteWorkType(t *testing.T) {
	tests := []struct {
		name        string
		currentType string
		remoteType  string
		wantType    string
	}{
		{name: "known type", currentType: "known", remoteType: "new", wantType: "known"},
		{name: "remote has no type", remoteType: "", wantType: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := &remoteWorkStatusWriterFake{}
			rw := &remoteUnit{BaseWorkUnitForWorkUnit: writer}
			got, err := rw.updateRemoteWorkType(tt.currentType, tt.remoteType)
			if err != nil || got != tt.wantType || writer.writes != 0 {
				t.Fatalf("got (%q, %v) after %d writes, want (%q, nil) and no write", got, err, writer.writes, tt.wantType)
			}
		})
	}
}

func TestRemoteWorkUsesPersistedRemoteWorkType(t *testing.T) {
	writer := &remoteWorkStatusWriterFake{persisted: "stored-type"}
	rw := &remoteUnit{BaseWorkUnitForWorkUnit: writer}
	got, err := rw.updateRemoteWorkType("", "remote-type")
	if err != nil {
		t.Fatal(err)
	}
	if got != "stored-type" {
		t.Fatalf("cached type = %q, want persisted value stored-type", got)
	}
}

type releasedStdoutNetceptor struct {
	NetceptorForWorkceptor
	log *logger.ReceptorLogger
}

func (n *releasedStdoutNetceptor) NodeID() string { return "testnode" }

func (n *releasedStdoutNetceptor) GetLogger() *logger.ReceptorLogger { return n.log }

// TestMonitorRemoteStdoutStopsQuietlyAfterRelease covers the stdout monitor's
// status-file reload: once the local proxy is released, the missing status file
// is expected and must not be logged as an error.
func TestMonitorRemoteStdoutStopsQuietlyAfterRelease(t *testing.T) {
	logBuf := &bytes.Buffer{}
	lg := logger.NewReceptorLogger("")
	lg.SetOutput(logBuf)

	w, err := New(context.Background(), &releasedStdoutNetceptor{log: lg}, t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	bwu := &BaseWorkUnit{}
	bwu.Init(w, "unit", "remote", FileSystem{})
	if err := os.MkdirAll(bwu.UnitDir(), 0o700); err != nil {
		t.Fatalf("mkdir unit dir: %v", err)
	}
	bwu.SetStatusExtraData(&RemoteExtraData{RemoteNode: "node", RemoteUnitID: "remote-unit"})
	bwu.UpdateFullStatus(func(s *StatusFileData) {
		s.State = WorkStateRunning
		s.Detail = "running"
	})
	if err := os.WriteFile(bwu.StdoutFileName(), nil, 0o600); err != nil {
		t.Fatalf("write stdout: %v", err)
	}

	rw := &remoteUnit{BaseWorkUnitForWorkUnit: bwu}
	mw := &utils.JobContext{}
	mw.NewJob(context.Background(), 1, false)

	done := make(chan struct{})
	go func() {
		rw.monitorRemoteStdout(mw)
		close(done)
	}()

	// Let the monitor open the stdout file and complete its first status load
	// before the directory disappears.
	time.Sleep(150 * time.Millisecond)
	if err := bwu.Release(false); err != nil {
		t.Fatalf("Release: %v", err)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("monitorRemoteStdout did not stop after release")
	}

	if strings.Contains(logBuf.String(), "Could not read status file") {
		t.Fatalf("release logged a status-file read error:\n%s", logBuf.String())
	}
}
