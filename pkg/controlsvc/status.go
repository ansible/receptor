package controlsvc

import (
	"fmt"
	"github.com/project-receptor/receptor/pkg/netceptor"
	"github.com/project-receptor/receptor/pkg/utils"
	"github.com/project-receptor/receptor/pkg/version"
)

type statusCommandType struct{}
type statusCommand struct {
	requestedFields []string
}

func (t *statusCommandType) InitFromString(params string) (ControlCommand, error) {
	if params != "" {
		return nil, fmt.Errorf("status command does not take parameters")
	}
	c := &statusCommand{}
	return c, nil
}

func (t *statusCommandType) InitFromJSON(config map[string]interface{}) (ControlCommand, error) {
	requestedFields, ok := config["requested_fields"]
	var requestedFieldsStr []string
	if ok {
		requestedFieldsStr = make([]string, 0)
		for _, v := range requestedFields.([]interface{}) {
			vStr, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("each element of requested_fields must be a string")
			}
			requestedFieldsStr = append(requestedFieldsStr, vStr)
		}
	} else {
		requestedFieldsStr = nil
	}
	c := &statusCommand{
		requestedFields: requestedFieldsStr,
	}
	return c, nil
}

func (c *statusCommand) ControlFunc(nc *netceptor.Netceptor, cfo ControlFuncOperations) (map[string]interface{}, error) {
	status := nc.Status()
	statusGetters := make(map[string]func() interface{})
	statusGetters["Version"] = func() interface{} { return version.Version }
	statusGetters["SystemCPUCores"] = func() interface{} { return utils.GetSysCPUCores() }
	statusGetters["SystemMemoryMB"] = func() interface{} { return utils.GetSysMemoryMB() }
	statusGetters["NodeID"] = func() interface{} { return status.NodeID }
	statusGetters["Connections"] = func() interface{} { return status.Connections }
	statusGetters["RoutingTable"] = func() interface{} { return status.RoutingTable }
	statusGetters["Advertisements"] = func() interface{} { return status.Advertisements }
	statusGetters["KnownConnectionCosts"] = func() interface{} { return status.KnownConnectionCosts }
	cfr := make(map[string]interface{})
	if c.requestedFields == nil { // if nil, fill it with the keys in statusGetters
		for field := range statusGetters {
			c.requestedFields = append(c.requestedFields, field)
		}
	}
	for _, field := range c.requestedFields {
		getter, ok := statusGetters[field]
		if ok {
			cfr[field] = getter()
		}
	}
	return cfr, nil
}
