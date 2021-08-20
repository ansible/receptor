package controlsvc

import (
	"fmt"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
)

type (
	reloadCommandType struct{}
	reloadCommand     struct{}
)

// ReloadCL is ParseAndRun closure set with the initial receptor arguments.
var ReloadCL func(bool) error

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
	cfr := make(map[string]interface{})
	logger.Debug("Reloading")

	// Do a quick check to catch any yaml errors before canceling backends
	err := ReloadCL(true)
	if err != nil {
		cfr["Success"] = false
		cfr["Error"] = err.Error()

		return cfr, err
	}

	nc.CancelBackends()
	// ReloadCL is a ParseAndRun closure, set in receptor.go/main()
	err = ReloadCL(false)
	if err != nil {
		cfr["Success"] = false
		cfr["Error"] = err.Error()

		return cfr, err
	}
	cfr["Success"] = true

	return cfr, nil
}

func init() {
	ReloadCL = func(dryRun bool) error {
		return fmt.Errorf("reload function not set")
	}
}
