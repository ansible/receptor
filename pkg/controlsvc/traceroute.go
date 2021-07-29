package controlsvc

import (
	"fmt"
	"strconv"

	"github.com/project-receptor/receptor/pkg/netceptor"
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

func (c *tracerouteCommand) ControlFunc(nc *netceptor.Netceptor, cfo ControlFuncOperations) (map[string]interface{}, error) {
	cfr := make(map[string]interface{})
	for i := 0; i <= int(nc.MaxForwardingHops()); i++ {
		thisResult := make(map[string]interface{})
		pingTime, pingRemote, err := ping(nc, c.target, byte(i))
		thisResult["From"] = pingRemote
		thisResult["Time"] = pingTime
		thisResult["TimeStr"] = fmt.Sprint(pingTime)
		if err != nil && err.Error() != netceptor.ProblemExpiredInTransit {
			thisResult["Error"] = err.Error()
		}
		cfr[strconv.Itoa(i)] = thisResult
		if err == nil || err.Error() != netceptor.ProblemExpiredInTransit {
			break
		}
	}
	return cfr, nil
}
