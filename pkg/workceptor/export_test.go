//go:build !no_workceptor
// +build !no_workceptor

package workceptor

import (
	"bufio"
	"context"
	"net"

	"github.com/ansible/receptor/pkg/controlsvc"
	"github.com/ansible/receptor/pkg/utils"
)

// NewWorkceptorCommandTypeForTest exposes the unexported work command type to
// tests in the workceptor_test package.
func NewWorkceptorCommandTypeForTest(w *Workceptor) controlsvc.ControlCommandType {
	return &workceptorCommandType{w: w}
}

// MonitorRemoteStdoutForTest runs the unexported remoteUnit stdout monitor on
// a unit built from the given base work unit, for tests in the workceptor_test
// package.
func MonitorRemoteStdoutForTest(b BaseWorkUnitForWorkUnit, mw *utils.JobContext) {
	(&remoteUnit{BaseWorkUnitForWorkUnit: b}).monitorRemoteStdout(mw)
}

// MonitorRemoteStatusForTest runs the unexported remoteUnit status monitor on
// a unit built from the given base work unit, for tests in the workceptor_test
// package.
func MonitorRemoteStatusForTest(b BaseWorkUnitForWorkUnit, mw *utils.JobContext, forRelease bool) {
	(&remoteUnit{BaseWorkUnitForWorkUnit: b}).monitorRemoteStatus(mw, forRelease)
}

// HandleConnectionErrorForTest invokes the unexported connection error handler,
// for tests in the workceptor_test package.
func HandleConnectionErrorForTest(b BaseWorkUnitForWorkUnit, err error) bool {
	return (&remoteUnit{BaseWorkUnitForWorkUnit: b}).handleConnectionError(err)
}

// GetConnectionAndRunForTest invokes the unexported retry helper, for tests in
// the workceptor_test package.
func GetConnectionAndRunForTest(b BaseWorkUnitForWorkUnit, ctx context.Context, firstTimeSync bool, action func(context.Context, net.Conn, *bufio.Reader) error, failure func()) error {
	return (&remoteUnit{BaseWorkUnitForWorkUnit: b}).getConnectionAndRun(ctx, firstTimeSync, action, failure)
}
