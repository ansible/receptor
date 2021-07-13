package controlsvc

import (
	"github.com/project-receptor/receptor/pkg/netceptor"
  "github.com/project-receptor/receptor/pkg/logger"
)

type reloadCommandType struct{}
type reloadCommand struct {}

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
  logger.Debug("Reloading")
  // mark the backends for cancel
	nc.MarkAllForCancel()
  // add each tcp-peer, but
  ReloadCL()
  nc.CancelMarked()
	cfr := make(map[string]interface{})
	cfr["Success"] = true
	return cfr, nil
}
