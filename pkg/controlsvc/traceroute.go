package controlsvc

import (
	"context"
	"fmt"
	"strconv"

	"github.com/ansible/receptor/pkg/netceptor"
)

type (
	tracerouteCommandType struct{}
	tracerouteCommand     struct {
		target string
	}
)

func (t *tracerouteCommandType) InitFromString(params string) (ControlCommand, error) {
	if params == "" {
		return nil, fmt.Errorf("no traceroute target")
	}
	c := &tracerouteCommand{
		target: params,
	}

	return c, nil
}

func (t *tracerouteCommandType) InitFromJSON(config map[string]interface{}) (ControlCommand, error) {
	target, ok := config["target"]
	if !ok {
		return nil, fmt.Errorf("no traceroute target")
	}
	targetStr, ok := target.(string)
	if !ok {
		return nil, fmt.Errorf("traceroute target must be string")
	}
	c := &tracerouteCommand{
		target: targetStr,
	}

	return c, nil
}

func (c *tracerouteCommand) ControlFunc(ctx context.Context, nc *netceptor.Netceptor, _ ControlFuncOperations) (map[string]interface{}, error) {
	cfr := make(map[string]interface{})
	results := nc.Traceroute(ctx, c.target)
	i := 0
	for res := range results {
		thisResult := make(map[string]interface{})
		thisResult["From"] = res.From
		thisResult["Time"] = res.Time
		thisResult["TimeStr"] = fmt.Sprint(res.Time)
		if res.Err != nil {
			thisResult["Error"] = res.Err.Error()
		}
		cfr[strconv.Itoa(i)] = thisResult
		i++
	}

	return cfr, nil
}
