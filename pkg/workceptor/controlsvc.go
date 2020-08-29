package workceptor

import (
	"fmt"
	"github.com/project-receptor/receptor/pkg/controlsvc"
	"os"
	"path"
	"strconv"
	"strings"
)

// Worker function called by the control service to process a "work" command
func (w *Workceptor) workFunc(params string, cfo controlsvc.ControlFuncOperations) (map[string]interface{}, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("bad command")
	}
	tokens := strings.Split(params, " ")
	switch tokens[0] {
	case "start", "submit":
		var workType string
		var workNode string
		var paramStart int
		if tokens[0] == "start" {
			if len(tokens) < 2 {
				return nil, fmt.Errorf("bad command")
			}
			workNode = w.nc.NodeID()
			workType = tokens[1]
			paramStart = 2
		} else {
			if len(tokens) < 3 {
				return nil, fmt.Errorf("bad command")
			}
			workNode = tokens[1]
			workType = tokens[2]
			paramStart = 3

		}
		if workType == "remote" {
			return nil, fmt.Errorf("bad command")
		}
		params := ""
		if len(tokens) > paramStart {
			params = strings.Join(tokens[paramStart:], " ")
		}
		var worker WorkUnit
		var err error
		if tokens[0] == "start" {
			worker, err = w.AllocateUnit(workType, params)
		} else {
			worker, err = w.AllocateRemoteUnit(workNode, workType, params)
		}
		if err != nil {
			return nil, err
		}
		stdin, err := os.OpenFile(path.Join(worker.UnitDir(), "stdin"), os.O_CREATE+os.O_WRONLY, 0600)
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
		unitList := w.ListKnownUnitIDs()
		cfr := make(map[string]interface{})
		for i := range unitList {
			unitID := unitList[i]
			status, err := w.unitStatusForCFR(unitID)
			if err != nil {
				return nil, err
			}
			cfr[unitID] = status
		}
		return cfr, nil
	case "status":
		if len(tokens) != 2 {
			return nil, fmt.Errorf("bad command")
		}
		cfr, err := w.unitStatusForCFR(tokens[1])
		if err != nil {
			return nil, err
		}
		return cfr, nil
	case "cancel", "release", "force-release":
		if len(tokens) != 2 {
			return nil, fmt.Errorf("bad command")
		}
		cfr := make(map[string]interface{})
		var pendingMsg string
		var completeMsg string
		if tokens[0] == "cancel" {
			pendingMsg = "cancel pending"
			completeMsg = "cancelled"
		} else {
			pendingMsg = "release pending"
			completeMsg = "released"
		}
		unit, err := w.findUnit(tokens[1])
		if err != nil {
			cfr["already gone"] = tokens[1]
		} else {
			if tokens[0] == "cancel" {
				err = unit.Cancel()
			} else {
				err = unit.Release(tokens[0] == "force-release")
			}
			if err != nil && !IsPending(err) {
				return nil, err
			}
			if IsPending(err) {
				cfr[pendingMsg] = tokens[1]
			} else {
				cfr[completeMsg] = tokens[1]
			}
		}
		return cfr, nil
	case "results":
		if len(tokens) < 2 || len(tokens) > 3 {
			return nil, fmt.Errorf("bad command")
		}
		var startPos int64
		if len(tokens) == 3 {
			var err error
			startPos, err = strconv.ParseInt(tokens[2], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("bad command")
			}
		}
		doneChan := make(chan struct{})
		defer func() {
			doneChan <- struct{}{}
		}()
		resultChan, err := w.GetResults(tokens[1], startPos, doneChan)
		if err != nil {
			return nil, err
		}
		err = cfo.WriteToConn(fmt.Sprintf("Streaming results for work unit %s\n", tokens[1]), resultChan)
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
