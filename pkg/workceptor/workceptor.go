// +build !no_workceptor

package workceptor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/project-receptor/receptor/pkg/controlsvc"
	"github.com/project-receptor/receptor/pkg/logger"
	"github.com/project-receptor/receptor/pkg/netceptor"
	"github.com/project-receptor/receptor/pkg/randstr"
	"github.com/project-receptor/receptor/pkg/utils"
)

// Workceptor is the main object that handles unit-of-work management
type Workceptor struct {
	ctx             context.Context
	nc              *netceptor.Netceptor
	dataDir         string
	workTypesLock   *sync.RWMutex
	workTypes       map[string]*workType
	activeUnitsLock *sync.RWMutex
	activeUnits     map[string]WorkUnit
}

// workType is the record for a registered type of work
type workType struct {
	newWorkerFunc NewWorkerFunc
}

// New constructs a new Workceptor instance
func New(ctx context.Context, nc *netceptor.Netceptor, dataDir string) (*Workceptor, error) {
	if dataDir == "" {
		dataDir = path.Join(os.TempDir(), "receptor")
	}
	dataDir = path.Join(dataDir, nc.NodeID())
	w := &Workceptor{
		ctx:             ctx,
		nc:              nc,
		dataDir:         dataDir,
		workTypesLock:   &sync.RWMutex{},
		workTypes:       make(map[string]*workType),
		activeUnitsLock: &sync.RWMutex{},
		activeUnits:     make(map[string]WorkUnit),
	}
	err := w.RegisterWorker("remote", newRemoteWorker)
	if err != nil {
		return nil, fmt.Errorf("could not register remote worker function: %w", err)
	}
	return w, nil
}

// MainInstance is the global instance of Workceptor instantiated by the command-line main() function
var MainInstance *Workceptor

// stdoutSize returns size of stdout, if it exists, or 0 otherwise
func stdoutSize(unitdir string) int64 {
	stat, err := os.Stat(path.Join(unitdir, "stdout"))
	if err != nil {
		return 0
	}
	return stat.Size()
}

// RegisterWithControlService registers this workceptor instance with a control service instance
func (w *Workceptor) RegisterWithControlService(cs *controlsvc.Server) error {
	err := cs.AddControlFunc("work", &workceptorCommandType{
		w: w,
	})
	if err != nil {
		return fmt.Errorf("could not add work control function: %w", err)
	}
	return nil
}

// RegisterWorker notifies the Workceptor of a new kind of work that can be done
func (w *Workceptor) RegisterWorker(typeName string, newWorkerFunc NewWorkerFunc) error {
	w.workTypesLock.Lock()
	_, ok := w.workTypes[typeName]
	if ok {
		w.workTypesLock.Unlock()
		return fmt.Errorf("work type %s already registered", typeName)
	}
	w.workTypes[typeName] = &workType{
		newWorkerFunc: newWorkerFunc,
	}
	if typeName != "remote" { // all workceptors get a remote command by default
		if err := w.nc.AddWorkCommand(typeName); err != nil {
			panic(err)
		}
	}
	w.workTypesLock.Unlock()

	// Check if any unknown units have now become known
	w.activeUnitsLock.Lock()
	for id, worker := range w.activeUnits {
		_, ok := worker.(*unknownUnit)
		if ok && worker.Status().WorkType == typeName {
			delete(w.activeUnits, id)
		}
	}
	w.activeUnitsLock.Unlock()
	w.scanForUnits()
	return nil
}

func (w *Workceptor) generateUnitID(lock bool) (string, error) {
	if lock {
		w.activeUnitsLock.RLock()
		defer w.activeUnitsLock.RUnlock()
	}
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
			return ident, os.MkdirAll(unitdir, 0o700)
		}
	}
}

// AllocateUnit creates a new local work unit and generates an identifier for it
func (w *Workceptor) AllocateUnit(workTypeName string, params map[string]string) (WorkUnit, error) {
	w.workTypesLock.RLock()
	wt, ok := w.workTypes[workTypeName]
	w.workTypesLock.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown work type %s", workTypeName)
	}
	w.activeUnitsLock.Lock()
	defer w.activeUnitsLock.Unlock()
	ident, err := w.generateUnitID(false)
	if err != nil {
		return nil, err
	}
	worker := wt.newWorkerFunc(w, ident, workTypeName)
	err = worker.SetFromParams(params)
	if err == nil {
		err = worker.Save()
	}
	if err != nil {
		return nil, err
	}
	w.activeUnits[ident] = worker
	return worker, nil
}

// AllocateRemoteUnit creates a new remote work unit and generates a local identifier for it
func (w *Workceptor) AllocateRemoteUnit(remoteNode, remoteWorkType, tlsclient, ttl string, params map[string]string) (WorkUnit, error) {
	if tlsclient != "" {
		_, err := w.nc.GetClientTLSConfig(tlsclient, "testhost", "receptor")
		if err != nil {
			return nil, err
		}
	}
	hasSecrets := false
	for k := range params {
		if strings.HasPrefix(strings.ToLower(k), "secret_") {
			hasSecrets = true
			break
		}
	}
	if hasSecrets && tlsclient == "" {
		return nil, fmt.Errorf("cannot send secrets over a non-TLS connection")
	}
	rw, err := w.AllocateUnit("remote", params)
	if err != nil {
		return nil, err
	}
	var expiration time.Time
	if ttl != "" {
		duration, err := time.ParseDuration(ttl)
		if err != nil {
			logger.Error("Failed to parse provided ttl -- valid examples include '1.5h', '30m', '30m10s'")
			return nil, err
		}
		expiration = time.Now().Add(duration)
	} else {
		expiration = time.Time{}
	}
	rw.UpdateFullStatus(func(status *StatusFileData) {
		ed, ok := status.ExtraData.(*remoteExtraData)
		if !ok {
			panic("extra data is not the correct type")
		}
		ed.RemoteNode = remoteNode
		ed.RemoteWorkType = remoteWorkType
		ed.TLSClient = tlsclient
		ed.Expiration = expiration
	})
	if rw.LastUpdateError() != nil {
		return nil, rw.LastUpdateError()
	}
	return rw, nil
}

func (w *Workceptor) scanForUnit(unitID string) {
	unitdir := path.Join(w.dataDir, unitID)
	fi, _ := os.Stat(unitdir)
	if fi == nil || !fi.IsDir() {
		logger.Error("Error locating unit: %s", unitID)
		return
	}
	ident := fi.Name()
	w.activeUnitsLock.RLock()
	_, ok := w.activeUnits[ident]
	w.activeUnitsLock.RUnlock()
	if !ok {
		statusFilename := path.Join(unitdir, "status")
		sfd := &StatusFileData{}
		_ = sfd.Load(statusFilename)
		w.workTypesLock.RLock()
		wt, ok := w.workTypes[sfd.WorkType]
		w.workTypesLock.RUnlock()
		var worker WorkUnit
		if ok {
			worker = wt.newWorkerFunc(w, ident, sfd.WorkType)
		} else {
			worker = newUnknownWorker(w, ident, sfd.WorkType)
		}
		if _, err := os.Stat(statusFilename); os.IsNotExist(err) {
			logger.Error("Status file has disappeared for %s.", ident)
			return
		}
		err := worker.Load()
		if err != nil {
			logger.Warning("Failed to restart worker %s due to read error: %s", unitdir, err)
			worker.UpdateBasicStatus(WorkStateFailed, fmt.Sprintf("Failed to restart: %s", err), stdoutSize(unitdir))
		}
		err = worker.Restart()
		if err != nil && !IsPending(err) {
			logger.Warning("Failed to restart worker %s: %s", unitdir, err)
			worker.UpdateBasicStatus(WorkStateFailed, fmt.Sprintf("Failed to restart: %s", err), stdoutSize(unitdir))
		}
		w.activeUnitsLock.Lock()
		defer w.activeUnitsLock.Unlock()
		w.activeUnits[ident] = worker
	}
}

func (w *Workceptor) scanForUnits() {
	files, err := ioutil.ReadDir(w.dataDir)
	if err != nil {
		return
	}
	for i := range files {
		fi := files[i]
		w.scanForUnit(fi.Name())
	}
}

func (w *Workceptor) findUnit(unitID string) (WorkUnit, error) {
	w.scanForUnit(unitID)
	w.activeUnitsLock.RLock()
	defer w.activeUnitsLock.RUnlock()
	unit, ok := w.activeUnits[unitID]
	if !ok {
		return nil, fmt.Errorf("unknown work unit %s", unitID)
	}
	return unit, nil
}

// StartUnit starts a unit of work
func (w *Workceptor) StartUnit(unitID string) error {
	unit, err := w.findUnit(unitID)
	if err != nil {
		return err
	}
	return unit.Start()
}

// ListKnownUnitIDs returns a slice containing the known unit IDs
func (w *Workceptor) ListKnownUnitIDs() []string {
	w.activeUnitsLock.RLock()
	defer w.activeUnitsLock.RUnlock()
	result := make([]string, 0, len(w.activeUnits))
	for id := range w.activeUnits {
		result = append(result, id)
	}
	return result
}

// UnitStatus returns the state of a unit
func (w *Workceptor) UnitStatus(unitID string) (*StatusFileData, error) {
	unit, err := w.findUnit(unitID)
	if err != nil {
		return nil, err
	}
	return unit.Status(), nil
}

// CancelUnit cancels a unit of work, killing any processes
func (w *Workceptor) CancelUnit(unitID string) error {
	unit, err := w.findUnit(unitID)
	if err != nil {
		return err
	}
	return unit.Cancel()
}

// ReleaseUnit releases (deletes) resources from a unit of work, including stdout.  Release implies Cancel.
func (w *Workceptor) ReleaseUnit(unitID string, force bool) error {
	unit, err := w.findUnit(unitID)
	if err != nil {
		return err
	}
	return unit.Release(force)
}

// unitStatusForCFR returns status information as a map, suitable for a control function return value
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

// sleepOrDone sleeps until a timeout or the done channel is signaled
func sleepOrDone(doneChan <-chan struct{}, interval time.Duration) bool {
	select {
	case <-doneChan:
		return true
	case <-time.After(interval):
		return false
	}
}

// GetResults returns a live stream of the results of a unit
func (w *Workceptor) GetResults(unitID string, startPos int64, doneChan chan struct{}) (chan []byte, error) {
	w.scanForUnit(unitID)
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
				if IsComplete(unit.Status().State) {
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
			for err == nil {
				var newPos int64
				newPos, err = stdout.Seek(filePos, 0)
				if err != nil {
					logger.Warning("Seek error processing stdout: %s\n", err)
					return
				}
				if newPos != filePos {
					logger.Warning("Seek error processing stdout\n")
					return
				}
				var n int
				buf := make([]byte, utils.NormalBufferSize)
				n, err = stdout.Read(buf)
				if n > 0 {
					filePos += int64(n)
					resultChan <- buf[:n]
				}
			}
			if errors.Is(err, io.EOF) {
				err = stdout.Close()
				if err != nil {
					logger.Error("Error closing stdout\n")
					return
				}
				stdout = nil
				stdoutSize := stdoutSize(unitdir)
				if IsComplete(unit.Status().State) && stdoutSize >= unit.Status().StdoutSize {
					close(resultChan)
					logger.Info("Stdout complete - closing channel for: %s \n", unitID)
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
