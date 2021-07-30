// +build !no_workceptor

package workceptor

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/project-receptor/receptor/pkg/controlsvc"
	"github.com/project-receptor/receptor/pkg/netceptor"
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
		valueInt, err := strconv.ParseInt(valueStr, 10, 64)
		if err != nil {
			return valueInt, nil
		}
	}

	return 0, fmt.Errorf("field %s value %s is not convertible to an int", name, value)
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
	}

	return c, nil
}

// Worker function called by the control service to process a "work" command.
func (c *workceptorCommand) ControlFunc(nc *netceptor.Netceptor, cfo controlsvc.ControlFuncOperations) (map[string]interface{}, error) {
	switch c.subcommand {
	case "submit":
		workNode, err := strFromMap(c.params, "node")
		if err != nil {
			return nil, err
		}
		workType, err := strFromMap(c.params, "worktype")
		if err != nil {
			return nil, err
		}
		tlsclient, err := strFromMap(c.params, "tlsclient")
		if err != nil {
			tlsclient = "" // optional so don't return
		}
		ttl, err := strFromMap(c.params, "ttl")
		if err != nil {
			ttl = ""
		}
		workParams := make(map[string]string)
		for k, v := range c.params {
			if k == "command" || k == "subcommand" || k == "node" || k == "worktype" || k == "tlsclient" || k == "ttl" {
				continue
			}
			vStr, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("%s must be a string", k)
			}
			workParams[k] = vStr
		}
		var worker WorkUnit
		if workNode == nc.NodeID() || strings.EqualFold(workNode, "localhost") {
			if ttl != "" {
				return nil, fmt.Errorf("ttl option is intended for remote work only")
			}
			worker, err = c.w.AllocateUnit(workType, workParams)
		} else {
			worker, err = c.w.AllocateRemoteUnit(workNode, workType, tlsclient, ttl, workParams)
		}
		if err != nil {
			return nil, err
		}
		stdin, err := os.OpenFile(path.Join(worker.UnitDir(), "stdin"), os.O_CREATE+os.O_WRONLY, 0o600)
		if err != nil {
			return nil, err
		}
		worker.UpdateBasicStatus(WorkStatePending, "Waiting for Input Data", 0)
		err = cfo.ReadFromConn(fmt.Sprintf("Work unit created with ID %s. Send stdin data and EOF.\n", worker.ID()), stdin)
		if err != nil {
			worker.UpdateBasicStatus(WorkStateFailed, fmt.Sprintf("Error reading input data: %s", err), 0)

			return nil, err
		}
		err = stdin.Close()
		if err != nil {
			worker.UpdateBasicStatus(WorkStateFailed, fmt.Sprintf("Error reading input data: %s", err), 0)

			return nil, err
		}
		worker.UpdateBasicStatus(WorkStatePending, "Starting Worker", 0)
		err = worker.Start()
		if err != nil && !IsPending(err) {
			worker.UpdateBasicStatus(WorkStateFailed, fmt.Sprintf("Error starting worker: %s", err), 0)

			return nil, err
		}
		cfr := make(map[string]interface{})
		cfr["unitid"] = worker.ID()
		if IsPending(err) {
			cfr["result"] = "Job Submitted"
		} else {
			cfr["result"] = "Job Started"
		}

		return cfr, nil
	case "list":
		var unitList []string
		targetUnitID, ok := c.params["unitid"].(string)
		if ok {
			unitList = append(unitList, targetUnitID)
		} else {
			unitList = c.w.ListKnownUnitIDs()
		}
		cfr := make(map[string]interface{})
		for i := range unitList {
			unitID := unitList[i]
			status, err := c.w.unitStatusForCFR(unitID)
			if err != nil {
				return nil, err
			}
			cfr[unitID] = status
		}

		return cfr, nil
	case "status":
		unitid, err := strFromMap(c.params, "unitid")
		if err != nil {
			return nil, err
		}
		cfr, err := c.w.unitStatusForCFR(unitid)
		if err != nil {
			return nil, err
		}

		return cfr, nil
	case "cancel", "release", "force-release":
		unitid, err := strFromMap(c.params, "unitid")
		if err != nil {
			return nil, err
		}
		cfr := make(map[string]interface{})
		var pendingMsg string
		var completeMsg string
		if c.subcommand == "cancel" {
			pendingMsg = "cancel pending"
			completeMsg = "cancelled"
		} else {
			pendingMsg = "release pending"
			completeMsg = "released"
		}
		unit, err := c.w.findUnit(unitid)
		if err != nil {
			cfr["already gone"] = unitid
		} else {
			if c.subcommand == "cancel" {
				err = unit.Cancel()
			} else {
				err = unit.Release(c.subcommand == "force-release")
			}
			if err != nil && !IsPending(err) {
				return nil, err
			}
			if IsPending(err) {
				cfr[pendingMsg] = unitid
			} else {
				cfr[completeMsg] = unitid
			}
		}

		return cfr, nil
	case "results":
		unitid, err := strFromMap(c.params, "unitid")
		if err != nil {
			return nil, err
		}
		startPos, err := intFromMap(c.params, "startpos")
		if err != nil {
			return nil, err
		}
		doneChan := make(chan struct{})
		defer func() {
			close(doneChan)
		}()
		resultChan, err := c.w.GetResults(unitid, startPos, doneChan)
		if err != nil {
			return nil, err
		}
		err = cfo.WriteToConn(fmt.Sprintf("Streaming results for work unit %s\n", unitid), resultChan)
		if err != nil {
			return nil, err
		}
		err = cfo.Close()
		if err != nil {
			return nil, err
		}

		return nil, nil
	}

	return nil, fmt.Errorf("bad command")
}
