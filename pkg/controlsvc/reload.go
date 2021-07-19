package controlsvc

import (
	"github.com/project-receptor/receptor/pkg/logger"
	"github.com/project-receptor/receptor/pkg/netceptor"
	"runtime"
)

type reloadCommandType struct{}
type reloadCommand struct{}

// ReloadCL is ParseAndRun closure set with the initial receptor arguments
var ReloadCL func() error

func (t *reloadCommandType) InitFromString(params string) (ControlCommand, error) {
	c := &reloadCommand{}
	return c, nil
}

func (t *reloadCommandType) InitFromJSON(config map[string]interface{}) (ControlCommand, error) {
	c := &reloadCommand{}
	return c, nil
}

func (c *reloadCommand) ControlFunc(nc *netceptor.Netceptor, cfo ControlFuncOperations) (map[string]interface{}, error) {
	// Reload command stops all backends, and re-runs the ParseAndRun() on the
	// initial config file
	logger.Debug("Reloading")
	logger.Debug("goroutines before cancel: %d", runtime.NumGoroutine())
	nc.CancelBackends()
	logger.Debug("goroutines after cancel: %d", runtime.NumGoroutine())
	// ReloadCL is a ParseAndRun closure, set in receptor.go/main()
	ReloadCL()
	logger.Debug("goroutines after reload: %d", runtime.NumGoroutine())
	cfr := make(map[string]interface{})
	cfr["Success"] = true
	return cfr, nil
}
