//go:build !no_workceptor
// +build !no_workceptor

package workceptor

import (
	"context"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/ansible/receptor/pkg/controlsvc"
)

type workceptorCommandType struct {
	w *Workceptor
}

type workceptorCommand struct {
	w          *Workceptor
	subcommand string
	params     map[string]interface{}
}

func (t *workceptorCommandType) InitFromString(params string) (controlsvc.ControlCommand, error) {
	tokens := strings.Split(params, " ")
	if len(tokens) == 0 {
		return nil, fmt.Errorf("no work subcommand")
	}
	c := &workceptorCommand{
		w:          t.w,
		subcommand: strings.ToLower(tokens[0]),
		params:     make(map[string]interface{}),
	}
	switch c.subcommand {
	case "submit":
		if len(tokens) < 3 {
			return nil, fmt.Errorf("work submit requires a target node and work type")
		}
		c.params["node"] = tokens[1]
		c.params["worktype"] = tokens[2]
		if len(tokens) > 3 {
			c.params["params"] = strings.Join(tokens[3:], " ")
		}
	case "list":
		if len(tokens) > 1 {
			c.params["unitid"] = tokens[1]
		}
	case "status", "cancel", "release", "force-release":
		if len(tokens) < 2 {
			return nil, fmt.Errorf("work %s requires a unit ID", c.subcommand)
		}
		if len(tokens) > 2 {
			return nil, fmt.Errorf("work %s does not take parameters after the unit ID", c.subcommand)
		}
		c.params["unitid"] = tokens[1]
	case "results":
		if len(tokens) < 2 {
			return nil, fmt.Errorf("work results requires a unit ID")
		}
		if len(tokens) > 3 {
			return nil, fmt.Errorf("work results only takes a unit ID and optional start position")
		}
		c.params["unitid"] = tokens[1]
		if len(tokens) > 2 {
			var err error
			c.params["startpos"], err = strconv.ParseInt(tokens[2], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("error converting start position to integer: %s", err)
			}
		} else {
			c.params["startpos"] = int64(0)
		}
	}

	return c, nil
}

// strFromMap extracts a string from a map[string]interface{}, handling errors.
func strFromMap(config map[string]interface{}, name string) (string, error) {
	value, ok := config[name]
	if !ok {
		return "", fmt.Errorf("field %s missing", name)
	}
	valueStr, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("field %s must be a string", name)
	}

	return valueStr, nil
}

// intFromMap extracts an int64 from a map[string]interface{}, handling errors.
func intFromMap(config map[string]interface{}, name string) (int64, error) {
	value, ok := config[name]
	if !ok {
		return 0, fmt.Errorf("field %s missing", name)
	}
	valueInt, ok := value.(int64)
	if ok {
		return valueInt, nil
	}
	valueFloat, ok := value.(float64)
	if ok {
		return int64(valueFloat), nil
	}
	valueStr, ok := value.(string)
	if ok {
		return strconv.ParseInt(valueStr, 10, 64)
	}

	return 0, fmt.Errorf("field %s value %s is not convertible to an int", name, value)
}

func boolFromMap(config map[string]interface{}, name string) (bool, error) {
	value, ok := config[name]
	if !ok {
		return false, fmt.Errorf("field %s missing", name)
	}
	valueBoolStr, ok := value.(string)
	if !ok {
		return false, fmt.Errorf("field %s must be a string", name)
	}
	if valueBoolStr == "true" {
		return true, nil
	}
	if valueBoolStr == "false" {
		return false, nil
	}

	return false, fmt.Errorf("field %s value %s is not convertible to a bool", name, value)
}

func (t *workceptorCommandType) InitFromJSON(config map[string]interface{}) (controlsvc.ControlCommand, error) {
	subCmd, err := strFromMap(config, "subcommand")
	if err != nil {
		return nil, err
	}
	c := &workceptorCommand{
		w:          t.w,
		subcommand: strings.ToLower(subCmd),
		params:     make(map[string]interface{}),
	}
	switch c.subcommand {
	case "submit":
		for k, v := range config {
			_, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("submit parameters must all be strings and %s is not", k)
			}
			c.params[k] = v
		}
		_, err := strFromMap(c.params, "node")
		if err != nil {
			return nil, err
		}
		_, err = strFromMap(c.params, "worktype")
		if err != nil {
			return nil, err
		}
	case "status", "cancel", "release", "force-release":
		c.params["unitid"], err = strFromMap(config, "unitid")
		if err != nil {
			return nil, err
		}
		signature, err := strFromMap(config, "signature")
		if err == nil {
			c.params["signature"] = signature
		}
	case "list":
		unitID, err := strFromMap(config, "unitid")
		if err == nil {
			c.params["unitid"] = unitID
		}
	case "results":
		c.params["unitid"], err = strFromMap(config, "unitid")
		if err != nil {
			return nil, err
		}
		c.params["startpos"], err = intFromMap(config, "startpos")
		if err != nil {
			return nil, err
		}
		signature, err := strFromMap(config, "signature")
		if err == nil {
			c.params["signature"] = signature
		}
	}

	return c, nil
}

func (c *workceptorCommand) processSignature(workType, signature string, connIsUnix, signWork bool) error {
	shouldVerifySignature := c.w.ShouldVerifySignature(workType, signWork)
	if !shouldVerifySignature && signature != "" {
		return fmt.Errorf("work type did not expect a signature")
	}
	if shouldVerifySignature && !connIsUnix {
		err := c.w.VerifySignature(signature)
		if err != nil {
			return err
		}
	}

	return nil
}

func getSignWorkFromStatus(status *StatusFileData) bool {
	red, ok := status.ExtraData.(*RemoteExtraData)
	if ok {
		return red.SignWork
	}

	return false
}

// buildWorkParams extracts user-supplied key/value pairs from c.params, skipping
// the known protocol keys that are not work parameters.
func (c *workceptorCommand) buildWorkParams() (map[string]string, error) {
	nonParams := map[string]struct{}{
		"command": {}, "subcommand": {}, "node": {}, "worktype": {},
		"tlsclient": {}, "ttl": {}, "signwork": {}, "signature": {},
	}
	workParams := make(map[string]string)
	for k, v := range c.params {
		if _, skip := nonParams[k]; skip {
			continue
		}
		vStr, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("%s must be a string", k)
		}
		workParams[k] = vStr
	}

	return workParams, nil
}

// allocateWorker creates either a local or a remote work unit depending on workNode.
func (c *workceptorCommand) allocateWorker(nc controlsvc.NetceptorForControlCommand, workNode, workType, workUnitID, tlsClient, ttl string, signWork bool, workParams map[string]string) (WorkUnit, error) {
	isLocal := workNode == nc.NodeID() || strings.EqualFold(workNode, "localhost")
	if isLocal {
		if ttl != "" {
			return nil, fmt.Errorf("ttl option is intended for remote work only")
		}

		return c.w.AllocateUnit(workType, workUnitID, workParams)
	}

	return c.w.AllocateRemoteUnit(workNode, workType, workUnitID, tlsClient, ttl, signWork, workParams)
}

// submitWork implements the "submit" subcommand.
func (c *workceptorCommand) submitWork(nc controlsvc.NetceptorForControlCommand, cfo controlsvc.ControlFuncOperations, connIsUnix bool) (map[string]interface{}, error) {
	workNode, err := strFromMap(c.params, "node")
	if err != nil {
		return nil, err
	}
	workType, err := strFromMap(c.params, "worktype")
	if err != nil {
		return nil, err
	}
	tlsClient, _ := strFromMap(c.params, "tlsclient")
	ttl, _ := strFromMap(c.params, "ttl")
	signWork, _ := boolFromMap(c.params, "signwork")
	signature, _ := strFromMap(c.params, "signature")
	workUnitID, _ := strFromMap(c.params, "workUnitID")

	workParams, err := c.buildWorkParams()
	if err != nil {
		return nil, err
	}
	if err = c.processSignature(workType, signature, connIsUnix, signWork); err != nil {
		return nil, err
	}
	worker, err := c.allocateWorker(nc, workNode, workType, workUnitID, tlsClient, ttl, signWork, workParams)
	if err != nil {
		return nil, err
	}

	cfr := map[string]interface{}{"unitid": worker.ID()}
	stdin, err := os.OpenFile(path.Join(worker.UnitDir(), "stdin"), os.O_CREATE+os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	worker.UpdateBasicStatus(WorkStatePending, "Waiting for Input Data", 0)
	if err = cfo.ReadFromConn(fmt.Sprintf("Work unit created with ID %s. Send stdin data and EOF.\n", worker.ID()), stdin, &controlsvc.SocketConnIO{}); err != nil {
		worker.UpdateBasicStatus(WorkStateFailed, fmt.Sprintf("Error reading input data: %s", err), 0)

		return nil, err
	}
	if err = stdin.Close(); err != nil {
		worker.UpdateBasicStatus(WorkStateFailed, fmt.Sprintf("Error reading input data: %s", err), 0)

		return nil, err
	}
	worker.UpdateBasicStatus(WorkStatePending, "Starting Worker", 0)
	if err = worker.Start(); err != nil && !IsPending(err) {
		worker.UpdateBasicStatus(WorkStateFailed, fmt.Sprintf("Error starting worker: %s", err), 0)

		return cfr, err
	}
	if IsPending(err) {
		cfr["result"] = "Job Submitted"
	} else {
		cfr["result"] = "Job Started"
	}

	return cfr, nil
}

// listWork implements the "list" subcommand.
func (c *workceptorCommand) listWork() (map[string]interface{}, error) {
	var unitList []string
	if targetUnitID, ok := c.params["unitid"].(string); ok {
		unitList = append(unitList, targetUnitID)
	} else {
		unitList = c.w.ListKnownUnitIDs()
	}
	cfr := make(map[string]interface{})
	for _, unitID := range unitList {
		status, err := c.w.unitStatusForCFR(unitID)
		if err != nil {
			return nil, err
		}
		cfr[unitID] = status
	}

	return cfr, nil
}

// cancelOrRelease implements the "cancel", "release", and "force-release" subcommands.
func (c *workceptorCommand) cancelOrRelease(connIsUnix bool) (map[string]interface{}, error) {
	unitid, err := strFromMap(c.params, "unitid")
	if err != nil {
		return nil, err
	}
	signature, _ := strFromMap(c.params, "signature")

	pendingMsg, completeMsg := "release pending", "released"
	if c.subcommand == "cancel" {
		pendingMsg, completeMsg = "cancel pending", "cancelled"
	}

	unit, err := c.w.findUnit(unitid)
	if err != nil {
		return map[string]interface{}{"unit not found": unitid}, err
	}
	status := unit.Status()
	if err = c.processSignature(status.WorkType, signature, connIsUnix, getSignWorkFromStatus(status)); err != nil {
		return nil, err
	}
	if c.subcommand == "cancel" {
		err = unit.Cancel()
	} else {
		err = unit.Release(c.subcommand == "force-release")
	}
	if err != nil && !IsPending(err) {
		return nil, err
	}
	cfr := make(map[string]interface{})
	if IsPending(err) {
		cfr[pendingMsg] = unitid
	} else {
		cfr[completeMsg] = unitid
	}

	return cfr, nil
}

// streamResults implements the "results" subcommand.
func (c *workceptorCommand) streamResults(ctx context.Context, cfo controlsvc.ControlFuncOperations, connIsUnix bool) (map[string]interface{}, error) {
	unitid, err := strFromMap(c.params, "unitid")
	if err != nil {
		return nil, err
	}
	startPos, err := intFromMap(c.params, "startpos")
	if err != nil {
		return nil, err
	}
	signature, _ := strFromMap(c.params, "signature")

	unit, err := c.w.findUnit(unitid)
	if err != nil {
		return nil, err
	}
	status := unit.Status()
	if err = c.processSignature(status.WorkType, signature, connIsUnix, getSignWorkFromStatus(status)); err != nil {
		return nil, err
	}
	resultChan, err := c.w.GetResults(ctx, unitid, startPos)
	if err != nil {
		return nil, err
	}
	if err = cfo.WriteToConn(fmt.Sprintf("Streaming results for work unit %s\n", unitid), resultChan); err != nil {
		return nil, err
	}

	return nil, nil
}

// Worker function called by the control service to process a "work" command.
func (c *workceptorCommand) ControlFunc(ctx context.Context, nc controlsvc.NetceptorForControlCommand, cfo controlsvc.ControlFuncOperations) (map[string]interface{}, error) {
	connIsUnix := cfo.RemoteAddr().Network() == "unix"
	switch c.subcommand {
	case "submit":
		return c.submitWork(nc, cfo, connIsUnix)
	case "list":
		return c.listWork()
	case "status":
		unitid, err := strFromMap(c.params, "unitid")
		if err != nil {
			return nil, err
		}

		return c.w.unitStatusForCFR(unitid)
	case "cancel", "release", "force-release":
		return c.cancelOrRelease(connIsUnix)
	case "results":
		return c.streamResults(ctx, cfo, connIsUnix)
	}

	return nil, fmt.Errorf("bad command")
}
