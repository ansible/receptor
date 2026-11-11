//go:build !no_workceptor
// +build !no_workceptor

package workceptor

import (
	"bufio"
	"context"
	"net"
	"os"

	"github.com/ansible/receptor/pkg/controlsvc"
	"github.com/ansible/receptor/pkg/utils"
)

// NewWorkceptorCommandTypeForTest exposes the unexported work command type to
// tests in the workceptor_test package.
func NewWorkceptorCommandTypeForTest(w *Workceptor) controlsvc.ControlCommandType {
	return &workceptorCommandType{w: w}
}

// AddActiveUnitForTest inserts unit into the active-unit map so tests in the
// workceptor_test package can drive method lookups such as ReleaseUnit and
// GetResults without allocating a worker.
func (w *Workceptor) AddActiveUnitForTest(unitID string, unit WorkUnit) {
	w.activeUnitsLock.Lock()
	defer w.activeUnitsLock.Unlock()

	w.activeUnits[unitID] = unit
}

// StartRemoteUnitForTest invokes the unexported remote unit start attempt on a
// unit built from the given base work unit, for tests in the workceptor_test
// package.
func StartRemoteUnitForTest(b BaseWorkUnitForWorkUnit, ctx context.Context, conn net.Conn, reader *bufio.Reader) error {
	return (&remoteUnit{BaseWorkUnitForWorkUnit: b}).StartRemoteUnit(ctx, conn, reader)
}

// MonitorRemoteStatusForTest runs the unexported remote status monitor on a
// unit built from the given base work unit, for tests in the workceptor_test
// package.
func MonitorRemoteStatusForTest(b BaseWorkUnitForWorkUnit, mw *utils.JobContext, forRelease bool) {
	(&remoteUnit{BaseWorkUnitForWorkUnit: b}).monitorRemoteStatus(mw, forRelease)
}

// MonitorRemoteStdoutForTest runs the unexported remote stdout monitor on a
// unit built from the given base work unit, for tests in the workceptor_test
// package.
func MonitorRemoteStdoutForTest(b BaseWorkUnitForWorkUnit, mw *utils.JobContext) {
	(&remoteUnit{BaseWorkUnitForWorkUnit: b}).monitorRemoteStdout(mw)
}

// SetCloseFileForTest replaces the file-close hook for the duration of a test
// and returns a function that restores the previous hook.
func SetCloseFileForTest(fn func(*os.File) error) func() {
	old := closeFile
	closeFile = fn

	return func() { closeFile = old }
}
