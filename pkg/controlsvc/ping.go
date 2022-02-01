package controlsvc

import (
	"context"
	"fmt"

	"github.com/ansible/receptor/pkg/netceptor"
)

type (
	pingCommandType struct{}
	pingCommand     struct {
		target string
	}
)

func (t *pingCommandType) InitFromString(params string) (ControlCommand, error) {
	if params == "" {
		return nil, fmt.Errorf("no ping target")
	}
	c := &pingCommand{
		target: params,
	}

	return c, nil
}

func (t *pingCommandType) InitFromJSON(config map[string]interface{}) (ControlCommand, error) {
	target, ok := config["target"]
	if !ok {
		return nil, fmt.Errorf("no ping target")
	}
	targetStr, ok := target.(string)
	if !ok {
		return nil, fmt.Errorf("ping target must be string")
	}
	c := &pingCommand{
		target: targetStr,
	}

	return c, nil
}

func (c *pingCommand) ControlFunc(ctx context.Context, nc *netceptor.Netceptor, cfo ControlFuncOperations) (map[string]interface{}, error) {
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
