package workceptor

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/project-receptor/receptor/pkg/controlsvc"
	"github.com/project-receptor/receptor/pkg/logger"
	"github.com/project-receptor/receptor/pkg/netceptor"
	"github.com/project-receptor/receptor/pkg/randstr"
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
	LocalCancel  bool
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
	lock         *sync.RWMutex
	started      bool
	released     bool
	waitRemote   *sync.WaitGroup
	worker       WorkType
	status       *StatusInfo
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

// saveStatus updates the status metadata file in the unitdir
func saveStatus(unitdir string, state int, detail string, stdoutSize int64) error {
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
		unit.lock.Lock()
		unit.status = si
		unit.lock.Unlock()
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
		unit.lock.RLock()
		complete := IsComplete(unit.status.State)
		unit.lock.RUnlock()
		if complete {
			break
		}
	}
}

func (w *Workceptor) connectToRemote(node string) (net.Conn, *bufio.Reader, error) {
	conn, err := w.nc.Dial(node, "control", nil)
	if err != nil {
		return nil, nil, err
	}
	reader := bufio.NewReader(conn)
	hello, err := reader.ReadString('\n')
	if err != nil {
		return nil, nil, err
	}
	if !strings.Contains(hello, node) {
		return nil, nil, fmt.Errorf("while expecting node ID %s, got message: %s", node,
			strings.TrimRight(hello, "\n"))
	}
	return conn, reader, nil
}

// monitorRemoteStatus monitors the remote status file and copies results to the local one
func (w *Workceptor) monitorRemoteStatus(ctx context.Context, cancel context.CancelFunc, wg *sync.WaitGroup,
	unit *workUnit, unitdir string, conn net.Conn, reader *bufio.Reader) {
	defer func() {
		cancel()
		wg.Done()
	}()
	unit.lock.RLock()
	remoteNodeID := unit.status.Node
	remoteUnitID := unit.status.RemoteUnitID
	unit.lock.RUnlock()
	for {
		_, err := conn.Write([]byte(fmt.Sprintf("work status %s\n", remoteUnitID)))
		if err != nil {
			logger.Error("Write error sending to %s: %s\n", remoteNodeID, err)
			return
		}
		status, err := reader.ReadString('\n')
		if err != nil {
			logger.Error("Read error reading from %s: %s\n", remoteNodeID, err)
			return
		}
		if status[:5] == "ERROR" {
			if strings.Contains(status, "unknown work unit") {
				logger.Debug("Work unit %s on node %s appears to be gone.\n", remoteUnitID, remoteNodeID)
				unit.lock.Lock()
				unit.status.State = WorkStateFailed
				unit.status.Detail = "Remote work unit is gone"
				_ = unit.status.Save(path.Join(unitdir, "status"))
				unit.lock.Unlock()
				return
			}
			logger.Error("Remote error%s\n", strings.TrimRight(status[5:], "\n"))
			return
		}
		si := StatusInfo{}
		err = json.Unmarshal([]byte(status), &si)
		if err != nil {
			logger.Error("Error unmarshalling JSON: %s\n", status)
			return
		}
		unit.lock.Lock()
		unit.status.State = si.State
		unit.status.Detail = si.Detail
		unit.status.StdoutSize = si.StdoutSize
		err = unit.status.Save(path.Join(unitdir, "status"))
		state := unit.status.State
		released := unit.released
		unit.lock.Unlock()
		if err != nil {
			logger.Error("Error saving local status file: %s\n", err)
			return
		}
		if IsComplete(state) {
			return
		}
		if released {
			return
		}
		if sleepOrDone(ctx.Done(), 1*time.Second) {
			return
		}
	}
}

// monitorRemoteStdout copies the remote stdout stream to the local buffer
func (w *Workceptor) monitorRemoteStdout(ctx context.Context, cancel context.CancelFunc, wg *sync.WaitGroup,
	unit *workUnit, unitdir string) {
	defer func() {
		cancel()
		wg.Done()
	}()
	stdoutFilename := path.Join(unitdir, "stdout")
	unit.lock.RLock()
	remoteNodeID := unit.status.Node
	remoteUnitID := unit.status.RemoteUnitID
	unit.lock.RUnlock()
	firstTime := true
	for {
		if firstTime {
			firstTime = false
			if checkDone(ctx.Done()) {
				return
			}
		} else {
			if sleepOrDone(ctx.Done(), 1*time.Second) {
				return
			}
		}
		diskStdoutSize := fileSizeOrZero(stdoutFilename)
		unit.lock.RLock()
		remoteStdoutSize := unit.status.StdoutSize
		unit.lock.RUnlock()
		var conn net.Conn
		if diskStdoutSize < remoteStdoutSize {
			if conn != nil && !reflect.ValueOf(conn).IsNil() {
				_ = conn.Close()
			}
			conn, reader, err := w.connectToRemote(remoteNodeID)
			if err != nil {
				logger.Error("Connection failed to %s: %s\n", remoteNodeID, err)
				continue
			}
			_, err = conn.Write([]byte(fmt.Sprintf("work results %s %d\n", remoteUnitID, diskStdoutSize)))
			if err != nil {
				logger.Error("Write error sending to %s: %s\n", remoteNodeID, err)
				continue
			}
			status, err := reader.ReadString('\n')
			if err != nil {
				logger.Error("Read error reading from %s: %s\n", remoteNodeID, err)
				continue
			}
			if !strings.Contains(status, "Streaming results") {
				logger.Debug("Remote node %s did not stream results\n", remoteNodeID)
				continue
			}
			stdout, err := os.OpenFile(stdoutFilename, os.O_CREATE+os.O_APPEND+os.O_WRONLY, 0600)
			if err != nil {
				logger.Debug("Could not open stdout file %s: %s\n", stdoutFilename, err)
				continue
			}
			doneChan := make(chan struct{})
			go func() {
				select {
				case <-doneChan:
					return
				case <-ctx.Done():
					cr, ok := conn.(interface{ CancelRead() })
					if ok {
						cr.CancelRead()
					}
					_ = conn.Close()
					return
				}
			}()
			_, err = io.Copy(stdout, conn)
			close(doneChan)

			unit.lock.RLock()
			released := unit.released
			unit.lock.RUnlock()

			if err != nil && !released {
				logger.Error("Error copying to stdout file %s: %s\n", stdoutFilename, err)
				continue
			}
		}
	}
}

// monitorRemoteUnit watches a remote unit on another node and maintains local status
func (w *Workceptor) monitorRemoteUnit(unit *workUnit, unitID string) {
	// TODO: handle cancellation of the overall monitor
	defer func() {
		unit.waitRemote.Done()
	}()
	var conn net.Conn
	var nextDelay = SuccessWorkSleep
	unitdir := path.Join(w.dataDir, unitID)
	submitIDRegex := regexp.MustCompile("with ID ([a-zA-Z0-9]+)\\.")
	unit.lock.Lock()
	remoteNodeID := unit.status.Node
	remoteUnitID := unit.status.RemoteUnitID
	workType := unit.status.WorkType
	unit.lock.Unlock()
	unit.waitRemote.Add(1)
	for {
		if conn != nil && !reflect.ValueOf(conn).IsNil() {
			_ = conn.Close()
		}
		time.Sleep(nextDelay)
		nextDelay = time.Duration(1.5 * float64(nextDelay))
		if nextDelay > MaxWorkSleep {
			nextDelay = MaxWorkSleep
		}
		unit.lock.RLock()
		released := unit.released
		unit.lock.RUnlock()
		if released {
			return
		}
		conn, reader, err := w.connectToRemote(remoteNodeID)
		if err != nil {
			logger.Error("Connection failed to %s: %s\n", remoteNodeID, err)
			continue
		}
		if remoteUnitID == "" {
			_, err = conn.Write([]byte(fmt.Sprintf("work start %s\n", workType)))
			if err != nil {
				logger.Error("Write error sending to %s: %s\n", remoteNodeID, err)
				continue
			}
			response, err := reader.ReadString('\n')
			if err != nil {
				logger.Error("Read error reading from %s: %s\n", remoteNodeID, err)
				continue
			}
			match := submitIDRegex.FindSubmatch([]byte(response))
			if match == nil || len(match) != 2 {
				logger.Warning("Could not parse response: %s\n", strings.TrimRight(response, "\n"))
				continue
			}
			remoteUnitID = string(match[1])
			unit.lock.Lock()
			unit.status.RemoteUnitID = remoteUnitID
			err = unit.status.Save(path.Join(unitdir, "status"))
			unit.lock.Unlock()
			if err != nil {
				logger.Error("Error saving local status file: %s\n", err)
				continue
			}
			stdin, err := os.Open(path.Join(unitdir, "stdin"))
			if err != nil {
				logger.Error("Error opening stdin file: %s\n", err)
				continue
			}
			_, err = io.Copy(conn, stdin)
			if err != nil {
				logger.Error("Error sending stdin file: %s\n", err)
				continue
			}
			err = conn.Close()
			conn = nil
			if err != nil {
				logger.Error("Error closing connection: %s\n", err)
				continue
			}
			continue
		}
		ctx, cancel := context.WithCancel(context.Background())
		wg := &sync.WaitGroup{}
		wg.Add(2)
		go w.monitorRemoteStatus(ctx, cancel, wg, unit, unitdir, conn, reader)
		go w.monitorRemoteStdout(ctx, cancel, wg, unit, unitdir)
		wg.Wait()
		diskStdoutSize := fileSizeOrZero(path.Join(unitdir, "stdout"))
		unit.lock.RLock()
		stdoutSize := unit.status.StdoutSize
		localCancel := unit.status.LocalCancel
		released = unit.released
		state := unit.status.State
		unit.lock.RUnlock()
		complete := IsComplete(state) && diskStdoutSize >= stdoutSize
		if complete {
			return
		}
		if localCancel || released {
			return
		}
	}
}

func (w *Workceptor) generateUnitID() (string, error) {
	w.activeUnitsLock.RLock()
	defer w.activeUnitsLock.RUnlock()
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
		State:       WorkStatePending,
		Detail:      "Waiting for Input Data",
		LocalCancel: false,
		StdoutSize:  0,
		Node:        nodeID,
		WorkType:    workTypeName,
		Params:      params,
	}
	err = status.Save(path.Join(w.dataDir, ident, "status"))
	if err != nil {
		return "", err
	}
	w.activeUnitsLock.Lock()
	defer w.activeUnitsLock.Unlock()
	w.activeUnits[ident] = &workUnit{
		lock:     &sync.RWMutex{},
		started:  false,
		released: false,
		waitRemote: &sync.WaitGroup{},
		worker:   worker,
		status:   status,
	}
	return ident, nil
}

// startLocalUnit starts running a local unit of work
func (w *Workceptor) startLocalUnit(unit *workUnit, unitdir string) error {
	unit.lock.Lock()
	defer unit.lock.Unlock()
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
	unit.lock.RLock()
	if unit.started {
		unit.lock.RUnlock()
		return fmt.Errorf("work unit %s was already started", unitID)
	}
	if unit.status.Node == w.nc.NodeID() {
		unit.lock.RUnlock()
		return w.startLocalUnit(unit, unitdir)
	}
	unit.lock.RUnlock()
	go w.monitorRemoteUnit(unit, unitID)
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
					lock:     &sync.RWMutex{},
					started:  true, // If we're finding it now, we don't want to start it again
					released: false,
					waitRemote: &sync.WaitGroup{},
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
					if si.LocalCancel && !IsComplete(si.State) {
						remoteNodeID := si.Node
						remoteUnitID := si.RemoteUnitID
						go w.cancelRemote(remoteNodeID, remoteUnitID, unit)
					}
					go w.monitorRemoteUnit(unit, fi.Name())
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

func (w *Workceptor) cancelRemote(remoteNodeID, remoteUnitID string, unit *workUnit) error {
	retryInterval := 500 * time.Millisecond
	for {
		// check that unit still exists, could have been released
		unit.lock.RLock()
		released := unit.released
		unit.lock.RUnlock()
		if released {
			return nil
		}
		conn, _, err := w.connectToRemote(remoteNodeID)
		if err != nil {
			logger.Error("Connection failed to %s: %s\n", remoteNodeID, err)
		}
		if conn != nil {
			_, err = conn.Write([]byte(fmt.Sprintf("work cancel %s\n", remoteUnitID)))
			if err != nil {
				logger.Error("Write error sending to %s: %s\n", remoteNodeID, err)
			} else {
				conn.Close()
				break
			}
			conn.Close()
		}
		time.Sleep(retryInterval)
		// backoff scheme, increase retryInterval
		retryInterval = time.Duration(1.5 * float64(retryInterval))
		if retryInterval > MaxWorkSleep {
			retryInterval = MaxWorkSleep
		}
	}
	retryInterval = 500 * time.Millisecond
	var state int
	for {
		// check that unit still exists, could have been released
		unit.lock.RLock()
		released := unit.released
		unit.lock.RUnlock()
		if released {
			return nil
		}
		unit.lock.RLock()
		state = unit.status.State
		unit.lock.RUnlock()
		if IsComplete(state) {
			return nil
		}
		time.Sleep(retryInterval)
	}
}

// CancelUnit cancels a unit, killing it if it is still running
func (w *Workceptor) CancelUnit(unitID string) (bool, error) {
	isPending := false
	w.scanForUnits()
	w.activeUnitsLock.RLock()
	unit, ok := w.activeUnits[unitID]
	w.activeUnitsLock.RUnlock()
	if !ok {
		return isPending, fmt.Errorf("unknown work unit %s", unitID)
	}
	if unit.worker != nil {
		err := unit.worker.Cancel()
		if err != nil {
			return isPending, err
		}
	}
	unit.lock.Lock()
	unit.status.LocalCancel = true
	nodeID := unit.status.Node
	remoteUnitID := unit.status.RemoteUnitID
	released := unit.released
	unit.lock.Unlock()
	if remoteUnitID != "" && !released {
		unit.lock.Lock()
		unit.status.Detail = "Cancel Pending"
		unit.lock.Unlock()
		go w.cancelRemote(nodeID, remoteUnitID, unit)
		isPending = true
	} else {
		unit.lock.Lock()
		unit.status.State = WorkStateFailed
		unit.status.Detail = "Cancelled"
		unit.lock.Unlock()
	}
	unitdir := path.Join(w.dataDir, unitID)
	w.activeUnitsLock.RLock()
	err := unit.status.Save(path.Join(unitdir, "status"))
	w.activeUnitsLock.RUnlock()
	if err != nil {
		logger.Error("Error saving local status file: %s\n", err)
		return isPending, nil
	}
	return isPending, nil
}

// ReleaseUnit releases a unit, canceling it if it is still running
func (w *Workceptor) ReleaseUnit(unitID string) error {
	w.activeUnitsLock.RLock()
	unit, ok := w.activeUnits[unitID]
	w.activeUnitsLock.RUnlock()
	unit.lock.Lock()
	nodeID := unit.status.Node
	remoteUnitID := unit.status.RemoteUnitID
	unit.released = true
	unit.lock.Unlock()
	if !ok {
		return fmt.Errorf("unknown work unit %s", unitID)
	}

	_, err := w.CancelUnit(unitID)
	if err != nil {
		return err
	}

	// wait for monitoringRemoteUnit to return
	unit.waitRemote.Wait()

	if remoteUnitID != "" {
		conn, _, err := w.connectToRemote(nodeID)
		if err != nil {
			logger.Error("Connection failed to %s: %s\n", nodeID, err)
		}
		if conn != nil {
			_, err = conn.Write([]byte(fmt.Sprintf("work release %s\n", remoteUnitID)))
			if err != nil {
				logger.Error("Write error sending to %s: %s\n", nodeID, err)
			}
			conn.Close()
		}
	}
	w.activeUnitsLock.Lock()
	delete(w.activeUnits, unitID)
	w.activeUnitsLock.Unlock()
	err = os.RemoveAll(path.Join(w.dataDir, unitID))
	if err != nil {
		return err
	}
	return nil
}

// checkDone non-blockingly checks if the done channel is signaled
func checkDone(doneChan <-chan struct{}) bool {
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
func sleepOrDone(doneChan <-chan struct{}, interval time.Duration) bool {
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

// Returns file size, or zero if the file does not exist
func fileSizeOrZero(filename string) int64 {
	stat, err := os.Stat(filename)
	if err != nil {
		return 0
	}
	return stat.Size()
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
			_, err := os.Stat(stdoutFilename)
			if err == nil {
				break
			} else if os.IsNotExist(err) {
				if IsComplete(unit.status.State) {
					close(resultChan)
					logger.Warning("Unit completed without producing any stdout\n")
					return
				}
				if sleepOrDone(doneChan, 250*time.Millisecond) {
					return
				}
			} else {
				logger.Error("Error accessing stdout file: %s\n", err)
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
				logger.Warning("Seek error processing stdout\n")
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
					logger.Error("Error closing stdout\n")
					return
				}
				stdout = nil
				stdoutSize := fileSizeOrZero(stdoutFilename)
				if IsComplete(unit.status.State) && stdoutSize >= unit.status.StdoutSize {
					close(resultChan)
					logger.Info("Stdout complete - closing channel\n")
					return
				}
				continue
			} else if err != nil {
				logger.Error("Error reading stdout: %s\n", err)
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
		isPending, err := w.CancelUnit(tokens[1])
		if err != nil {
			return nil, err
		}
		cfr := make(map[string]interface{})
		if isPending {
			cfr["cancel pending"] = tokens[1]
		} else {
			cfr["cancelled"] = tokens[1]
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
