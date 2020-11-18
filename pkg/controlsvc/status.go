package controlsvc

import (
	"fmt"
	"github.com/project-receptor/receptor/pkg/netceptor"
)

type statusCommandType struct{}
type statusCommand struct {
	fields []string
}

// Version is receptor app version
var Version string

func (t *statusCommandType) InitFromString(params string) (ControlCommand, error) {
	if params != "" {
		return nil, fmt.Errorf("status command does not take parameters")
	}
	c := &statusCommand{}
	return c, nil
}

func (t *statusCommandType) InitFromJSON(config map[string]interface{}) (ControlCommand, error) {
	fields, ok := config["fields"]
	var fieldsStr []string
	if ok {
		fieldsStr = make([]string, 0)
		for _, v := range fields.([]interface{}) {
			vStr, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("each element of fields must be a string")
			}
			fieldsStr = append(fieldsStr, vStr)
		}
	} else {
		fieldsStr = nil
	}
	c := &statusCommand{
		fields: fieldsStr,
	}
	return c, nil
}

func (c *statusCommand) ControlFunc(nc *netceptor.Netceptor, cfo ControlFuncOperations) (map[string]interface{}, error) {
	status := nc.Status()
	precfr := make(map[string]interface{})
	precfr["Version"] = Version
	precfr["NodeID"] = status.NodeID
	precfr["Connections"] = status.Connections
	precfr["RoutingTable"] = status.RoutingTable
	precfr["Advertisements"] = status.Advertisements
	precfr["KnownConnectionCosts"] = status.KnownConnectionCosts
	cfr := make(map[string]interface{})
	if c.fields != nil {
		for _, f := range c.fields {
			cfr[f] = precfr[f]
		}
	} else {
		cfr = precfr
	}
	return cfr, nil
}
