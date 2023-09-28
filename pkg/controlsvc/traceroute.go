package controlsvc

import (
	"context"
	"fmt"
	"strconv"
)

type (
	TracerouteCommandType struct{}
	TracerouteCommand     struct {
		target string
	}
)

func (t *TracerouteCommandType) InitFromString(params string) (ControlCommand, error) {
	if params == "" {
		return nil, fmt.Errorf("no traceroute target")
	}
	c := &TracerouteCommand{
		target: params,
	}

	return c, nil
}

func (t *TracerouteCommandType) InitFromJSON(config map[string]interface{}) (ControlCommand, error) {
	target, ok := config["target"]
	if !ok {
		return nil, fmt.Errorf("no traceroute target")
	}
	targetStr, ok := target.(string)
	if !ok {
		return nil, fmt.Errorf("traceroute target must be string")
	}
	c := &TracerouteCommand{
		target: targetStr,
	}

	return c, nil
}

func (c *TracerouteCommand) ControlFunc(ctx context.Context, nc NetceptorForControlCommand, _ ControlFuncOperations) (map[string]interface{}, error) {
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
