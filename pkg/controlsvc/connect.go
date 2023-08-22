package controlsvc

import (
	"context"
	"fmt"
	"strings"

	"github.com/ansible/receptor/pkg/netceptor"
)

type (
	connectCommandType struct{}
	connectCommand     struct {
		targetNode    string
		targetService string
		tlsConfigName string
	}
)

func (t *connectCommandType) InitFromString(params string) (ControlCommand, error) {
	tokens := strings.Split(params, " ")
	if len(tokens) < 2 {
		return nil, fmt.Errorf("no connect target")
	}
	if len(tokens) > 3 {
		return nil, fmt.Errorf("too many parameters")
	}
	var tlsConfigName string
	if len(tokens) == 3 {
		tlsConfigName = tokens[2]
	}
	c := &connectCommand{
		targetNode:    tokens[0],
		targetService: tokens[1],
		tlsConfigName: tlsConfigName,
	}

	return c, nil
}

func (t *connectCommandType) InitFromJSON(config map[string]interface{}) (ControlCommand, error) {
	targetNode, ok := config["node"]
	if !ok {
		return nil, fmt.Errorf("no connect target node")
	}
	targetNodeStr, ok := targetNode.(string)
	if !ok {
		return nil, fmt.Errorf("connect target node must be string")
	}
	targetService, ok := config["service"]
	if !ok {
		return nil, fmt.Errorf("no connect target service")
	}
	targetServiceStr, ok := targetService.(string)
	if !ok {
		return nil, fmt.Errorf("connect target service must be string")
	}
	var tlsConfigStr string
	tlsConfig, ok := config["tls"]
	if ok {
		tlsConfigStr, ok = tlsConfig.(string)
		if !ok {
			return nil, fmt.Errorf("connect tls name must be string")
		}
	} else {
		tlsConfigStr = ""
	}
	c := &connectCommand{
		targetNode:    targetNodeStr,
		targetService: targetServiceStr,
		tlsConfigName: tlsConfigStr,
	}

	return c, nil
}

func (c *connectCommand) ControlFunc(_ context.Context, nc NetceptorForControlCommand, cfo ControlFuncOperations) (map[string]interface{}, error) {
	tlscfg, err := nc.GetClientTLSConfig(c.tlsConfigName, c.targetNode, netceptor.ExpectedHostnameTypeReceptor)
	if err != nil {
		return nil, err
	}
	rc, err := nc.Dial(c.targetNode, c.targetService, tlscfg)
	if err != nil {
		return nil, err
	}
	err = cfo.BridgeConn("Connecting\n", rc, "connected service", nc.GetLogger())
	if err != nil {
		return nil, err
	}

	return nil, nil
}
