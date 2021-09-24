package controlsvc

import (
	"context"
	"fmt"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
)

type (
	purgeCommandType struct{}
	purgeCommand     struct {
		node string
	}
)

func (t *purgeCommandType) InitFromString(params string) (ControlCommand, error) {
	c := &purgeCommand{
		node: params,
	}

	return c, nil
}

func (t *purgeCommandType) InitFromJSON(config map[string]interface{}) (ControlCommand, error) {
	c := &purgeCommand{
		node: "",
	}
	node, ok := config["node"]
	if !ok {
		return c, nil
	}
	nodeStr, ok := node.(string)
	if !ok {
		return nil, fmt.Errorf("node must be string")
	}
	c.node = nodeStr

	return c, nil
}

func (c *purgeCommand) ControlFunc(ctx context.Context, nc *netceptor.Netceptor, cfo ControlFuncOperations) (map[string]interface{}, error) {
	logger.Debug("Purging all unreachable nodes")

	cfr := make(map[string]interface{})
	cfr["Success"] = true
	purgedNodes, err := nc.PurgeUnreachableNodes(c.node)
	if err != nil {
		cfr["Success"] = false
		cfr["Error"] = err.Error()
	}
	cfr["PurgedNodes"] = purgedNodes

	return cfr, nil
}
