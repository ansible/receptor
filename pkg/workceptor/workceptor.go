package workceptor

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/ghjm/sockceptor/pkg/controlsvc"
	"github.com/ghjm/sockceptor/pkg/debug"
	"github.com/ghjm/sockceptor/pkg/netceptor"
	"github.com/ghjm/sockceptor/pkg/randstr"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	//    "strings"
)

// NewWorkerFunc is called to initialize a new, empty WorkType object
type NewWorkerFunc func() WorkType

// Work sleep constants
const (
	SuccessWorkSleep = 1 * time.Second // Normal time to wait between checks
	MaxWorkSleep     = 1 * time.Minute // Max time to ever wait between checks
)

// Work state constants
const (
	WorkStatePending   = 0
	WorkStateRunning   = 1
	WorkStateSucceeded = 2
	WorkStateFailed    = 3
)

// IsComplete returns true if a given WorkState indicates the job is finished
func IsComplete(WorkState int) bool {
	return WorkState == WorkStateSucceeded || WorkState == WorkStateFailed
}

// StatusInfo describes the status of a unit of work
type StatusInfo struct {
	State        int
	Detail       string
	StdoutSize   int64
	Node         string
	WorkType     string
	RemoteUnitID string
	Params       string
}

// WorkType represents a unique type of worker
type WorkType interface {
	Start(params string, unitdir string) (err error)
	Cancel() error
}

// Internal data for a registered worker type
type workType struct {
	newWorker NewWorkerFunc
}

// Internal data for a single unit of work
type workUnit struct {
	started  bool
	released bool
	worker   WorkType
	status   *StatusInfo
}

// Workceptor is the main object that handles unit-of-work management
type Workceptor struct {
	nc              *netceptor.Netceptor
	dataDir         string
	workTypes       map[string]*workType
	activeUnitsLock *sync.RWMutex
	activeUnits     map[string]*workUnit
}

// Save saves a StatusInfo to a file
func (si *StatusInfo) Save(filename string) error {
	jsonBytes, err := json.Marshal(si)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	jsonBytes = append(jsonBytes, '\n')
	_, err = file.Write(jsonBytes)
	if err != nil {
		return err
	}
	err = file.Close()
	if err != nil {
		return err
	}
	return nil
}

// Load loads a StatusInfo from a file
func (si *StatusInfo) Load(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	jsonBytes, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}
	err = file.Close()
	if err != nil {
		return err
	}
	err = json.Unmarshal(jsonBytes, si)
	if err != nil {
		return err
	}
	return nil
}

// saveState updates the status metadata file in the unitdir
func saveState(unitdir string, state int, detail string, stdoutSize int64) error {
	statusFilename := path.Join(unitdir, "status")
	si := &StatusInfo{}
	_ = si.Load(statusFilename)
	si.State = state
	si.Detail = detail
	si.StdoutSize = stdoutSize
	return si.Save(statusFilename)
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
func New(cs *controlsvc.Server, nc *netceptor.Netceptor, dataDir string) (*Workceptor, error) {
	if dataDir == "" {
		dataDir = path.Join(os.TempDir(), "receptor")
	}
	dataDir = path.Join(dataDir, nc.NodeID())
	w := &Workceptor{
		nc:              nc,
		dataDir:         dataDir,
		workTypes:       make(map[string]*workType),
		activeUnitsLock: &sync.RWMutex{},
		activeUnits:     make(map[string]*workUnit),
	}
	err := cs.AddControlFunc("work", w.workFunc)
	if err != nil {
		return nil, fmt.Errorf("could not add work control function: %s", err)
	}
	w.scanForUnits()
	return w, nil
}

// MainInstance is the global instance of Workceptor instantiated by the command-line main() function
var MainInstance *Workceptor

// RegisterWorker notifies the Workceptor of a new kind of work that can be done
func (w *Workceptor) RegisterWorker(typeName string, newWorker NewWorkerFunc) error {
	_, ok := w.workTypes[typeName]
	if ok {
		return fmt.Errorf("work type %s already registered", typeName)
	}
	w.workTypes[typeName] = &workType{
		newWorker: newWorker,
	}
	return nil
}

// updateLocalStatus updates the status information in a workUnit from disk
func (w *Workceptor) updateLocalStatus(filename string, unit *workUnit) {
	si := &StatusInfo{}
	err := si.Load(filename)
	if err == nil {
		unit.status = si
	}
}

// monitorLocalStatus watches a unit dir and keeps the workUnit up to date with status changes
func (w *Workceptor) monitorLocalStatus(unitdir string, unit *workUnit) {
	statusFile := path.Join(unitdir, "status")
	watcher, err := fsnotify.NewWatcher()
	if err == nil {
		err = watcher.Add(statusFile)
		if err == nil {
			defer func() {
				_ = watcher.Close()
			}()
		} else {
			_ = watcher.Close()
			watcher = nil
		}
	} else {
		watcher = nil
	}
	fi, err := os.Stat(statusFile)
	if err != nil {
		fi = nil
	}
	var watcherEvents chan fsnotify.Event
	if watcher == nil {
		watcherEvents = nil
	} else {
		watcherEvents = watcher.Events
	}
	for {
		select {
		case event := <-watcherEvents:
			if event.Op&fsnotify.Write == fsnotify.Write {
				w.updateLocalStatus(statusFile, unit)
			}
		case <-time.After(time.Second):
			newFi, err := os.Stat(statusFile)
			if err == nil {
				if fi == nil || fi.ModTime() != newFi.ModTime() {
					fi = newFi
					w.updateLocalStatus(statusFile, unit)
				}
			}
		}
		if IsComplete(unit.status.State) {
			break
		}
	}
}

// monitorRemoteStatus watches a remote unit on another node and maintains local status
func (w *Workceptor) monitorRemoteStatus(unit *workUnit, unitID string) {
	var conn net.Conn
	var err error
	var nextDelay = SuccessWorkSleep
	unitdir := path.Join(w.dataDir, unitID)
	submitIDRegex := regexp.MustCompile("with ID ([a-zA-Z0-9]+)\\.")
	for {
		if conn != nil && !reflect.ValueOf(conn).IsNil() {
			_ = conn.Close()
		}
		time.Sleep(nextDelay)
		nextDelay = time.Duration(1.5 * float64(nextDelay))
		if nextDelay > MaxWorkSleep {
			nextDelay = MaxWorkSleep
		}
		if unit.released {
			return
		}
		//TODO: this assumes every work-processing node will have a control service named
		//     "control." Perhaps we need to hardcode this as something we always start.
		//     We also need to deal with authenticating this connection, supplying a TLS
		//     client certificate and CA bundle, etc.
		conn, err = w.nc.Dial(unit.status.Node, "control", nil)
		if err != nil {
			debug.Printf("Connection failed to %s: %s\n", unit.status.Node, err)
			continue
		}
		reader := bufio.NewReader(conn)
		hello, err := reader.ReadString('\n')
		if !strings.Contains(hello, unit.status.Node) {
			debug.Printf("While expecting node ID %s, got message: %s. Exiting.\n", unit.status.Node,
				strings.TrimRight(hello, "\n"))
		}
		if unit.status.RemoteUnitID == "" {
			_, err = conn.Write([]byte(fmt.Sprintf("work start %s\n", unit.status.WorkType)))
			if err != nil {
				debug.Printf("Write error sending to %s: %s\n", unit.status.Node, err)
				continue
			}
			response, err := reader.ReadString('\n')
			if err != nil {
				debug.Printf("Read error reading from %s: %s\n", unit.status.Node, err)
				continue
			}
			match := submitIDRegex.FindSubmatch([]byte(response))
			if match == nil || len(match) != 2 {
				debug.Printf("Could not parse response: %s\n", strings.TrimRight(response, "\n"))
				continue
			}
			unit.status.RemoteUnitID = string(match[1])
			err = unit.status.Save(path.Join(unitdir, "status"))
			if err != nil {
				debug.Printf("Error saving local status file: %s\n", err)
				continue
			}
			stdin, err := os.Open(path.Join(unitdir, "stdin"))
			if err != nil {
				debug.Printf("Error opening stdin file: %s\n", err)
				continue
			}
			_, err = io.Copy(conn, stdin)
			if err != nil {
				debug.Printf("Error sending stdin file: %s\n", err)
				continue
			}
			cw, ok := conn.(interface{ CloseWrite() error })
			if ok {
				err := cw.CloseWrite()
				if err != nil {
					debug.Printf("Error closing write: %s\n", err)
					continue
				}
				response, err = reader.ReadString('\n')
				if err != nil {
					debug.Printf("Read error reading from %s: %s\n", unit.status.Node, err)
					continue
				}
				debug.Printf(response)
			}
			continue
		}
		if !IsComplete(unit.status.State) {
			// Check if remote job has completed yet
			_, err = conn.Write([]byte(fmt.Sprintf("work status %s\n", unit.status.RemoteUnitID)))
			if err != nil {
				debug.Printf("Write error sending to %s: %s\n", unit.status.Node, err)
				continue
			}
			status, err := reader.ReadString('\n')
			if err != nil {
				debug.Printf("Read error reading from %s: %s\n", unit.status.Node, err)
				continue
			}
			if status[:5] == "ERROR" {
				if strings.Contains(status, "unknown work unit") {
					debug.Printf("Work unit %s on node %s appears to be gone.\n", unit.status.RemoteUnitID, unit.status.Node)
					unit.status.State = WorkStateFailed
					unit.status.Detail = "Remote work unit is gone"
					_ = unit.status.Save(path.Join(unitdir, "status"))
					return
				}
				debug.Printf("Remote error%s\n", strings.TrimRight(status[5:], "\n"))
				continue
			}
			debug.Printf("%s\n", status)
			si := StatusInfo{}
			err = json.Unmarshal([]byte(status), &si)
			if err != nil {
				debug.Printf("Error unmarshalling JSON: %s\n", status)
				continue
			}
			unit.status.State = si.State
			unit.status.Detail = si.Detail
			unit.status.StdoutSize = si.StdoutSize
			err = unit.status.Save(path.Join(unitdir, "status"))
			if err != nil {
				debug.Printf("Error saving local status file: %s\n", err)
				continue
			}
		}
		stdoutFilename := path.Join(unitdir, "stdout")
		stdoutStat, err := os.Stat(stdoutFilename)
		var stdoutCurSize int64
		if err != nil {
			stdoutCurSize = 0
		} else {
			stdoutCurSize = stdoutStat.Size()
		}
		if IsComplete(unit.status.State) && err == nil && stdoutCurSize >= unit.status.StdoutSize {
			debug.Printf("Transfer complete, monitor exiting\n")
			return
		}
		if !os.IsExist(err) || stdoutCurSize < unit.status.StdoutSize {
			_, err = conn.Write([]byte(fmt.Sprintf("work results %s %d\n", unit.status.RemoteUnitID, stdoutCurSize)))
			if err != nil {
				debug.Printf("Write error sending to %s: %s\n", unit.status.Node, err)
				continue
			}
			status, err := reader.ReadString('\n')
			if err != nil {
				debug.Printf("Read error reading from %s: %s\n", unit.status.Node, err)
				continue
			}
			if !strings.Contains(status, "Streaming results") {
				debug.Printf("Remote node %s did not stream results\n", unit.status.Node)
				continue
			}
			stdout, err := os.OpenFile(stdoutFilename, os.O_CREATE+os.O_APPEND+os.O_WRONLY, 0600)
			if err != nil {
				debug.Printf("Could not open stdout file %s: %s\n", stdoutFilename, err)
				continue
			}
			_, err = io.Copy(stdout, conn)
			if err != nil {
				debug.Printf("Error copying to stdout file %s: %s\n", stdoutFilename, err)
				continue
			}
		}
	}
}

func (w *Workceptor) generateUnitID() (string, error) {
	w.activeUnitsLock.Lock()
	defer w.activeUnitsLock.Unlock()
	var ident string
	for {
		ident = randstr.RandomString(8)
		_, ok := w.activeUnits[ident]
		if !ok {
			unitdir := path.Join(w.dataDir, ident)
			_, err := os.Stat(unitdir)
			if err == nil {
				continue
			}
			return ident, os.MkdirAll(unitdir, 0700)
		}
	}
}

// PreStartUnit creates a new work unit and generates an identifier for it
func (w *Workceptor) PreStartUnit(nodeID string, workTypeName string, params string) (string, error) {
	var wT *workType
	if nodeID == w.nc.NodeID() {
		var ok bool
		wT, ok = w.workTypes[workTypeName]
		if !ok {
			return "", fmt.Errorf("unknown work type %s", workTypeName)
		}
	}
	ident, err := w.generateUnitID()
	if err != nil {
		return "", err
	}
	var worker WorkType
	if wT != nil {
		worker = wT.newWorker()
	}
	status := &StatusInfo{
		State:      WorkStatePending,
		Detail:     "Waiting for Input Data",
		StdoutSize: 0,
		Node:       nodeID,
		WorkType:   workTypeName,
		Params:     params,
	}
	err = status.Save(path.Join(w.dataDir, ident, "status"))
	if err != nil {
		return "", err
	}
	w.activeUnitsLock.Lock()
	defer w.activeUnitsLock.Unlock()
	w.activeUnits[ident] = &workUnit{
		started:  false,
		released: false,
		worker:   worker,
		status:   status,
	}
	return ident, nil
}

// startLocalUnit starts running a local unit of work
func (w *Workceptor) startLocalUnit(unit *workUnit, unitdir string) error {
	if unit.worker != nil {
		err := unit.worker.Start(unit.status.Params, unitdir)
		if err != nil {
			unit.status.State = WorkStateFailed
			unit.status.Detail = err.Error()
			_ = unit.status.Save(path.Join(unitdir, "status"))
			return fmt.Errorf("error starting work: %s", err)
		}
		unit.started = true
		go w.monitorLocalStatus(unitdir, unit)
	} else {
		return fmt.Errorf("tried to start work without worker")
	}
	return nil
}

// StartUnit starts a unit of work
func (w *Workceptor) StartUnit(unitID string) error {
	unitdir := path.Join(w.dataDir, unitID)
	w.activeUnitsLock.Lock()
	defer w.activeUnitsLock.Unlock()
	unit, ok := w.activeUnits[unitID]
	if !ok {
		return fmt.Errorf("unknown work unit %s", unitID)
	}
	if unit.started {
		return fmt.Errorf("work unit %s was already started", unitID)
	}
	if unit.status.Node == w.nc.NodeID() {
		return w.startLocalUnit(unit, unitdir)
	}
	go w.monitorRemoteStatus(unit, unitID)
	return nil
}

func (w *Workceptor) scanForUnits() {
	files, err := ioutil.ReadDir(w.dataDir)
	if err != nil {
		return
	}
	w.activeUnitsLock.Lock()
	defer w.activeUnitsLock.Unlock()
	for i := range files {
		fi := files[i]
		if fi.IsDir() {
			_, ok := w.activeUnits[fi.Name()]
			if !ok {
				si := &StatusInfo{}
				statusFilename := path.Join(w.dataDir, fi.Name(), "status")
				_ = si.Load(statusFilename)
				unit := &workUnit{
					started:  true, // If we're finding it now, we don't want to start it again
					released: false,
					worker:   nil,
					status:   si,
				}
				if unit.status.State == WorkStatePending {
					unit.status.State = WorkStateFailed
					unit.status.Detail = "Failed to start"
					_ = unit.status.Save(statusFilename)
				}
				w.activeUnits[fi.Name()] = unit
				if si.Node != "" && si.Node != w.nc.NodeID() {
					go w.monitorRemoteStatus(unit, fi.Name())
				}
			}
		}
	}
}

// ListKnownUnitIDs returns a slice containing the known unit IDs
func (w *Workceptor) ListKnownUnitIDs() []string {
	w.scanForUnits()
	w.activeUnitsLock.RLock()
	defer w.activeUnitsLock.RUnlock()
	result := make([]string, 0, len(w.activeUnits))
	for id := range w.activeUnits {
		result = append(result, id)
	}
	return result
}

// UnitStatus returns the state of a unit
func (w *Workceptor) UnitStatus(unitID string) (status *StatusInfo, err error) {
	w.scanForUnits()
	w.activeUnitsLock.RLock()
	unit, ok := w.activeUnits[unitID]
	w.activeUnitsLock.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown work unit %s", unitID)
	}
	statusCopy := *unit.status
	return &statusCopy, nil
}

func (w *Workceptor) unitStatusForCFR(unitID string) (map[string]interface{}, error) {
	status, err := w.UnitStatus(unitID)
	if err != nil {
		return nil, err
	}
	retMap := make(map[string]interface{})
	v := reflect.ValueOf(*status)
	t := reflect.TypeOf(*status)
	for i := 0; i < v.NumField(); i++ {
		retMap[t.Field(i).Name] = v.Field(i).Interface()
	}
	retMap["StateName"] = WorkStateToString(status.State)
	return retMap, nil
}

// CancelUnit cancels a unit, killing it if it is still running
func (w *Workceptor) CancelUnit(unitID string) error {
	//TODO: Handle remote cancellation
	w.scanForUnits()
	w.activeUnitsLock.RLock()
	unit, ok := w.activeUnits[unitID]
	if !ok {
		w.activeUnitsLock.RUnlock()
		return fmt.Errorf("unknown work unit %s", unitID)
	}
	w.activeUnitsLock.RUnlock()
	if unit.worker != nil {
		err := unit.worker.Cancel()
		if err != nil {
			return err
		}
	}
	unit.status.State = WorkStateFailed
	unit.status.Detail = "Cancelled"
	return nil
}

// ReleaseUnit releases a unit, canceling it if it is still running
func (w *Workceptor) ReleaseUnit(unitID string) error {
	//TODO: Handle remote release
	err := w.CancelUnit(unitID)
	if err != nil {
		return err
	}
	w.activeUnitsLock.Lock()
	unit, ok := w.activeUnits[unitID]
	if !ok {
		w.activeUnitsLock.Unlock()
		return fmt.Errorf("unknown work unit %s", unitID)
	}
	delete(w.activeUnits, unitID)
	w.activeUnitsLock.Unlock()
	unit.released = true
	err = os.RemoveAll(path.Join(w.dataDir, unitID))
	if err != nil {
		return err
	}
	return nil
}

// checkDone non-blockingly checks if the done channel is signaled
func checkDone(doneChan chan struct{}) bool {
	if doneChan == nil {
		return false
	}
	select {
	case <-doneChan:
		return true
	default:
		return false
	}
}

// sleepOrDone sleeps until a timeout or the done channel is signaled
func sleepOrDone(doneChan chan struct{}, interval time.Duration) bool {
	if doneChan == nil {
		time.Sleep(interval)
		return false
	}
	select {
	case <-doneChan:
		return true
	case <-time.After(interval):
		return false
	}
}

// GetResults returns a live stream of the results of a unit
func (w *Workceptor) GetResults(unitID string, startPos int64, doneChan chan struct{}) (chan []byte, error) {
	w.scanForUnits()
	w.activeUnitsLock.RLock()
	unit, ok := w.activeUnits[unitID]
	w.activeUnitsLock.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown work unit %s", unitID)
	}
	resultChan := make(chan []byte)
	go func() {
		unitdir := path.Join(w.dataDir, unitID)
		stdoutFilename := path.Join(unitdir, "stdout")
		// Wait for stdout file to exist
		for {
			if checkDone(doneChan) {
				return
			}
			_, err := os.Stat(stdoutFilename)
			if err == nil {
				break
			} else if os.IsNotExist(err) {
				if sleepOrDone(doneChan, 250*time.Millisecond) {
					return
				}
			} else {
				debug.Printf("Error accessing stdout file: %s\n", err)
				return
			}
		}
		var stdout *os.File
		var err error
		filePos := startPos
		buf := make([]byte, 1024)
		for {
			if sleepOrDone(doneChan, 250*time.Millisecond) {
				return
			}
			if stdout == nil {
				stdout, err = os.Open(stdoutFilename)
				if err != nil {
					continue
				}
			}
			newPos, err := stdout.Seek(filePos, 0)
			if newPos != filePos {
				debug.Printf("Seek error processing stdout\n")
				return
			}
			n, err := stdout.Read(buf)
			if n > 0 {
				filePos += int64(n)
				resultChan <- buf[:n]
			}
			if err == io.EOF {
				err = stdout.Close()
				if err != nil {
					debug.Printf("Error closing stdout\n")
					return
				}
				stdout = nil
				stat, err := os.Stat(stdoutFilename)
				var stdoutSize int64
				if err != nil {
					stdoutSize = 0
				} else {
					stdoutSize = stat.Size()
				}
				if IsComplete(unit.status.State) && stdoutSize >= unit.status.StdoutSize {
					close(resultChan)
					debug.Printf("Stdout complete - closing channel\n")
					return
				}
				continue
			} else if err != nil {
				debug.Printf("Error reading stdout: %s\n", err)
				return
			}
		}
	}()
	return resultChan, nil
}

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
		params := ""
		if len(tokens) > paramStart {
			params = strings.Join(tokens[paramStart:], " ")
		}
		ident, err := w.PreStartUnit(workNode, workType, params)
		if err != nil {
			return nil, err
		}
		stdin, err := os.OpenFile(path.Join(w.dataDir, ident, "stdin"), os.O_CREATE+os.O_WRONLY, 0600)
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
		err = w.StartUnit(ident)
		if err != nil {
			return nil, err
		}
		cfr := make(map[string]interface{})
		cfr["unitid"] = ident
		if tokens[0] == "start" {
			cfr["result"] = "Job Started"
		} else {
			cfr["result"] = "Job Submitted"
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
	case "cancel":
		if len(tokens) != 2 {
			return nil, fmt.Errorf("bad command")
		}
		err := w.CancelUnit(tokens[1])
		if err != nil {
			return nil, err
		}
		cfr := make(map[string]interface{})
		cfr["cancelled"] = tokens[1]
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
