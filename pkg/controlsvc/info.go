package controlsvc

import (
	"fmt"
	"github.com/project-receptor/receptor/pkg/netceptor"
)

type infoCommandType struct{}
type infoCommand struct{}

// Version is receptor app version
var Version string

func (t *infoCommandType) InitFromString(params string) (ControlCommand, error) {
	if params != "" {
		return nil, fmt.Errorf("info command does not take parameters")
	}
	c := &infoCommand{}
	return c, nil
}

func (t *infoCommandType) InitFromJSON(config map[string]interface{}) (ControlCommand, error) {
	c := &infoCommand{}
	return c, nil
}

func (c *infoCommand) ControlFunc(nc *netceptor.Netceptor, cfo ControlFuncOperations) (map[string]interface{}, error) {
	cfr := make(map[string]interface{})
	cfr["Version"] = Version
	return cfr, nil
}
