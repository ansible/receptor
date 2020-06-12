package workceptor

import (
	"fmt"
	"github.com/ghjm/sockceptor/pkg/controlsvc"
	"github.com/ghjm/sockceptor/pkg/randstr"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	//    "strings"
)

// NewWorkerFunc is called to initialize a new, empty WorkType object
type NewWorkerFunc func() WorkType

// UnmarshalWorkerFunc is called to unmarshal a WorkType object from disk
type UnmarshalWorkerFunc func([]byte) (WorkType, error)

// Work state constants
const (
	WorkStatePending   = 0
	WorkStateRunning   = 1
	WorkStateSucceeded = 2
	WorkStateFailed    = 3
)

// WorkType represents a unique type of worker
type WorkType interface {
	Start(params string, stdinFilename string) (err error)
	Status() (state int, detail string, err error)
	Results() (results chan []byte, err error)
	Release() error
	Marshal() ([]byte, error)
}

// Internal data for a registered worker type
type workType struct {
	newWorker       NewWorkerFunc
	unmarshalWorker UnmarshalWorkerFunc
}

// Internal data for a single unit of work
type workUnit struct {
	started bool
	worker  WorkType
}

// Workceptor is the main object that handles unit-of-work management
type Workceptor struct {
	workTypes   map[string]workType
	activeUnits map[string]workUnit
}

// WorkStateToString returns a string representation of a WorkState
func WorkStateToString(workState int) string {
	switch workState {
	case WorkStatePending:
		return "Pending"
	case WorkStateRunning:
		return "Running"
	case WorkStateSucceeded:
		return "Succeeded"
	case WorkStateFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}

// New constructs a new Workceptor instance
func New(cs *controlsvc.Server) (*Workceptor, error) {
	w := &Workceptor{
		workTypes:   make(map[string]workType),
		activeUnits: make(map[string]workUnit),
	}
	err := cs.AddControlFunc("work", w.workFunc)
	if err != nil {
		return nil, fmt.Errorf("could not add work control function: %s", err)
	}
	return w, nil
}

var mainInstance *Workceptor

// MainInstance returns a global singleton instance of Workceptor
func MainInstance() *Workceptor {
	if mainInstance == nil {
		var err error
		mainInstance, err = New(controlsvc.MainInstance())
		if err != nil {
			panic(err)
		}
	}
	return mainInstance
}

// RegisterWorker notifies the Workceptor of a new kind of work that can be done
func (w *Workceptor) RegisterWorker(typeName string, newWorker NewWorkerFunc, unmarshalWorker UnmarshalWorkerFunc) error {
	_, ok := w.workTypes[typeName]
	if ok {
		return fmt.Errorf("work type %s already registered", typeName)
	}
	w.workTypes[typeName] = workType{
		newWorker:       newWorker,
		unmarshalWorker: unmarshalWorker,
	}
	return nil
}

// PreStartUnit creates a new work unit and generates an identifier for it
func (w *Workceptor) PreStartUnit(workType string) (string, error) {
	wT, ok := w.workTypes[workType]
	if !ok {
		return "", fmt.Errorf("unknown work type %s", workType)
	}
	var ident string
	for {
		ident = randstr.RandomString(8)
		_, ok := w.activeUnits[ident]
		if !ok {
			break
		}
	}
	w.activeUnits[ident] = workUnit{
		started: false,
		worker:  wT.newWorker(),
	}
	return ident, nil
}

// StartUnit starts a unit of work
func (w *Workceptor) StartUnit(unitID string, params string, stdinFilename string) error {
	unit, ok := w.activeUnits[unitID]
	if !ok {
		return fmt.Errorf("unknown work unit %s", unitID)
	}
	if unit.started {
		return fmt.Errorf("work unit %s was already started", unitID)
	}
	err := unit.worker.Start(params, stdinFilename)
	if err != nil {
		return fmt.Errorf("error starting work: %s", err)
	}
	unit.started = true
	return nil
}

// ListActiveUnitIDs returns a slice containing the active unit IDs
func (w *Workceptor) ListActiveUnitIDs() []string {
	result := make([]string, 0, len(w.activeUnits))
	for id := range w.activeUnits {
		result = append(result, id)
	}
	return result
}

// UnitStatus returns the status of a unit
func (w *Workceptor) UnitStatus(unitID string) (state int, detail string, err error) {
	unit, ok := w.activeUnits[unitID]
	if !ok {
		return -1, "", fmt.Errorf("unknown work unit %s", unitID)
	}
	return unit.worker.Status()
}

// ReleaseUnit releases a unit, canceling it if it is still running
func (w *Workceptor) ReleaseUnit(unitID string) error {
	unit, ok := w.activeUnits[unitID]
	if !ok {
		return fmt.Errorf("unknown work unit %s", unitID)
	}
	delete(w.activeUnits, unitID)
	return unit.worker.Release()
}

// GetResults returns a live stream of the results of a unit
func (w *Workceptor) GetResults(unitID string) (chan []byte, error) {
	unit, ok := w.activeUnits[unitID]
	if !ok {
		return nil, fmt.Errorf("unknown work unit %s", unitID)
	}
	resultChan, err := unit.worker.Results()
	if err != nil {
		return nil, err
	}
	return resultChan, nil
}

// Worker function called by the control service to process a "work" command
func (w *Workceptor) workFunc(params string, cfo controlsvc.ControlFuncOperations) (map[string]interface{}, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("bad command")
	}
	tokens := strings.Split(params, " ")
	switch tokens[0] {
	case "start":
		if len(tokens) < 2 {
			return nil, fmt.Errorf("bad command")
		}
		workType := tokens[1]
		params := ""
		if len(tokens) > 2 {
			params = strings.Join(tokens[2:], " ")
		}
		ident, err := w.PreStartUnit(workType)
		if err != nil {
			return nil, err
		}
		stdin, err := ioutil.TempFile(os.TempDir(), "receptor-stdin*.tmp")
		if err != nil {
			return nil, err
		}
		stdinFilename, err := filepath.Abs(stdin.Name())
		if err != nil {
			return nil, err
		}
		err = cfo.ReadFromConn(fmt.Sprintf("Work unit created with ID %s. Send stdin data and EOF.\n", ident), stdin)
		if err != nil {
			return nil, err
		}
		err = stdin.Close()
		if err != nil {
			return nil, err
		}
		err = w.StartUnit(ident, params, stdinFilename)
		if err != nil {
			return nil, err
		}
		cfr := make(map[string]interface{})
		cfr["unitid"] = ident
		cfr["result"] = "Job Started"
		return cfr, nil
	case "list":
		unitList := w.ListActiveUnitIDs()
		cfr := make(map[string]interface{})
		for i := range unitList {
			unitID := unitList[i]
			state, detail, err := w.UnitStatus(unitID)
			if err != nil {
				return nil, err
			}
			sub := make(map[string]interface{})
			sub["state"] = WorkStateToString(state)
			sub["detail"] = detail
			cfr[unitID] = sub
		}
		return cfr, nil
	case "status":
		if len(tokens) != 2 {
			return nil, fmt.Errorf("bad command")
		}
		state, detail, err := w.UnitStatus(tokens[1])
		if err != nil {
			return nil, err
		}
		cfr := make(map[string]interface{})
		cfr["state"] = WorkStateToString(state)
		cfr["detail"] = detail
		return cfr, nil
	case "release":
		if len(tokens) != 2 {
			return nil, fmt.Errorf("bad command")
		}
		err := w.ReleaseUnit(tokens[1])
		if err != nil {
			return nil, err
		}
		cfr := make(map[string]interface{})
		cfr["released"] = tokens[1]
		return cfr, nil
	case "results":
		// TODO: Take a parameter here to begin streaming results from a byte position
		if len(tokens) != 2 {
			return nil, fmt.Errorf("bad command")
		}
		resultChan, err := w.GetResults(tokens[1])
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
