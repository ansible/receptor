package controlsvc

import (
	"context"
	"fmt"
	"strings"
	"time"

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

// ping is the internal implementation of sending a single ping packet and waiting for a reply or error.
func ping(nc *netceptor.Netceptor, target string, hopsToLive byte) (time.Duration, string, error) {
	pc, err := nc.ListenPacket("")
	if err != nil {
		return 0, "", err
	}
	ctx, ctxCancel := context.WithCancel(nc.Context())
	defer func() {
		ctxCancel()
		_ = pc.Close()
	}()
	pc.SetHopsToLive(hopsToLive)
	unrCh := pc.SubscribeUnreachable()
	type errorResult struct {
		err      error
		fromNode string
	}
	errorChan := make(chan errorResult)
	go func() {
		select {
		case <-ctx.Done():
			return
		case msg := <-unrCh:
			errorChan <- errorResult{
				err:      fmt.Errorf(msg.Problem),
				fromNode: msg.ReceivedFromNode,
			}
		}
	}()
	startTime := time.Now()
	replyChan := make(chan string)
	go func() {
		buf := make([]byte, 8)
		_, addr, err := pc.ReadFrom(buf)
		fromNode := ""
		if addr != nil {
			fromNode = addr.String()
			fromNode = strings.TrimSuffix(fromNode, ":ping")
		}
		if err == nil {
			select {
			case replyChan <- fromNode:
			case <-ctx.Done():
			}
		} else {
			select {
			case errorChan <- errorResult{
				err:      err,
				fromNode: fromNode,
			}:
			case <-ctx.Done():
			}
		}
	}()
	_, err = pc.WriteTo([]byte{}, nc.NewAddr(target, "ping"))
	if err != nil {
		return time.Since(startTime), nc.NodeID(), err
	}
	select {
	case errRes := <-errorChan:
		return time.Since(startTime), errRes.fromNode, errRes.err
	case remote := <-replyChan:
		return time.Since(startTime), remote, nil
	case <-time.After(10 * time.Second):
		return time.Since(startTime), "", fmt.Errorf("timeout")
	}
}

func (c *pingCommand) ControlFunc(nc *netceptor.Netceptor, cfo ControlFuncOperations) (map[string]interface{}, error) {
	pingTime, pingRemote, err := ping(nc, c.target, nc.MaxForwardingHops())
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
