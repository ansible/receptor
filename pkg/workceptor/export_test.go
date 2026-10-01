//go:build !no_workceptor
// +build !no_workceptor

package workceptor

import "github.com/ansible/receptor/pkg/controlsvc"

// NewWorkceptorCommandTypeForTest exposes the unexported work command type to
// tests in the workceptor_test package.
func NewWorkceptorCommandTypeForTest(w *Workceptor) controlsvc.ControlCommandType {
	return &workceptorCommandType{w: w}
}
