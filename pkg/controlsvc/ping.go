package controlsvc

import (
	"context"
	"fmt"
)

type (
	PingCommandType struct{}
	PingCommand     struct {
		target string
	}
)

func (t *PingCommandType) InitFromString(params string) (ControlCommand, error) {
	if params == "" {
		return nil, fmt.Errorf("no ping target")
	}
	c := &PingCommand{
		target: params,
	}

	return c, nil
}

func (t *PingCommandType) InitFromJSON(config map[string]interface{}) (ControlCommand, error) {
	target, ok := config["target"]
	if !ok {
		return nil, fmt.Errorf("no ping target")
	}
	targetStr, ok := target.(string)
	if !ok {
		return nil, fmt.Errorf("ping target must be string")
	}
	c := &PingCommand{
		target: targetStr,
	}

	return c, nil
}

func (c *PingCommand) ControlFunc(ctx context.Context, nc NetceptorForControlCommand, _ ControlFuncOperations) (map[string]interface{}, error) {
	pingTime, pingRemote, err := nc.Ping(ctx, c.target, nc.MaxForwardingHops())
	cfr := make(map[string]interface{})
	if err == nil {
		cfr["Success"] = true
		cfr["From"] = pingRemote
		cfr["Time"] = pingTime
		cfr["TimeStr"] = fmt.Sprint(pingTime)
	} else {
		cfr["Success"] = false
		cfr["Error"] = err.Error()
	}

	return cfr, nil
}
