package controlsvc

import (
	"fmt"
	"github.com/project-receptor/receptor/pkg/netceptor"
)

type statusCommandType struct{}
type statusCommand struct{}

func (t *statusCommandType) InitFromString(params string) (ControlCommand, error) {
	if params != "" {
		return nil, fmt.Errorf("status command does not take parameters")
	}
	c := &statusCommand{}
	return c, nil
}

func (t *statusCommandType) InitFromJSON(config map[string]interface{}) (ControlCommand, error) {
	c := &statusCommand{}
	return c, nil
}

func (c *statusCommand) ControlFunc(nc *netceptor.Netceptor, cfo ControlFuncOperations) (map[string]interface{}, error) {
	status := nc.Status()
	cfr := make(map[string]interface{})
	cfr["NodeID"] = status.NodeID
	cfr["Connections"] = status.Connections
	cfr["RoutingTable"] = status.RoutingTable
	cfr["Advertisements"] = status.Advertisements
	cfr["KnownConnectionCosts"] = status.KnownConnectionCosts
	return cfr, nil
}
