//go:build !no_workceptor
// +build !no_workceptor

package workceptor

import "github.com/ansible/receptor/pkg/controlsvc"

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
