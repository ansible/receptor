//go:build !no_workceptor
// +build !no_workceptor

package workceptor

import (
	"bufio"
	"context"
	"net"

	"github.com/ansible/receptor/pkg/controlsvc"
)

// NewWorkceptorCommandTypeForTest exposes the unexported work command type to
// tests in the workceptor_test package.
func NewWorkceptorCommandTypeForTest(w *Workceptor) controlsvc.ControlCommandType {
	return &workceptorCommandType{w: w}
}

// TestRemoteUnit wraps remoteUnit to expose unexported methods for tests in the
// workceptor_test package without requiring internal package access.
type TestRemoteUnit struct{ u *remoteUnit }

// NewTestRemoteUnit creates a TestRemoteUnit backed by the given BaseWorkUnitForWorkUnit.
func NewTestRemoteUnit(base BaseWorkUnitForWorkUnit) *TestRemoteUnit {
	return &TestRemoteUnit{u: &remoteUnit{BaseWorkUnitForWorkUnit: base}}
}

func (t *TestRemoteUnit) RunActionAsync(ctx context.Context, action func(context.Context, net.Conn, *bufio.Reader) error, failure func()) {
	t.u.runActionAsync(ctx, action, failure)
}
