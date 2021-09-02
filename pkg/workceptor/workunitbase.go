//go:build !no_workceptor
// +build !no_workceptor

package workceptor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"sync"
	"time"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/fsnotify/fsnotify"
	"github.com/rogpeppe/go-internal/lockedfile"
)

// Work sleep constants.
const (
	SuccessWorkSleep = 1 * time.Second // Normal time to wait between checks
	MaxWorkSleep     = 1 * time.Minute // Max time to ever wait between checks
)

// Work state constants.
const (
	WorkStatePending   = 0
	WorkStateRunning   = 1
	WorkStateSucceeded = 2
	WorkStateFailed    = 3
)

// IsComplete returns true if a given WorkState indicates the job is finished.
func IsComplete(workState int) bool {
	return workState == WorkStateSucceeded || workState == WorkStateFailed
}

// WorkStateToString returns a string representation of a WorkState.
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

// ErrPending is returned when an operation hasn't succeeded or failed yet.
var ErrPending = fmt.Errorf("operation pending")

// IsPending returns true if the error is an ErrPending.
func IsPending(err error) bool {
	return err == ErrPending
}

// BaseWorkUnit includes data common to all work units, and partially implements the WorkUnit interface.
type BaseWorkUnit struct {
	w               *Workceptor
	status          StatusFileData
	unitID          string
	unitDir         string
	statusFileName  string
	stdoutFileName  string
	statusLock      *sync.RWMutex
	lastUpdateError error
	ctx             context.Context
	cancel          context.CancelFunc
	signWork        bool
}

// Init initializes the basic work unit data, in memory only.
func (bwu *BaseWorkUnit) Init(w *Workceptor, unitID string, workType string) {
	bwu.w = w
	bwu.status.State = WorkStatePending
	bwu.status.Detail = "Unit Created"
	bwu.status.StdoutSize = 0
	bwu.status.WorkType = workType
	bwu.unitID = unitID
	bwu.unitDir = path.Join(w.dataDir, unitID)
	bwu.statusFileName = path.Join(bwu.unitDir, "status")
	bwu.stdoutFileName = path.Join(bwu.unitDir, "stdout")
	bwu.statusLock = &sync.RWMutex{}
	bwu.ctx, bwu.cancel = context.WithCancel(w.ctx)
	bwu.signWork = false
}

func (bwu *BaseWorkUnit) SetSignWork(signWork bool) {
	bwu.signWork = signWork
}

// SetFromParams sets the in-memory state from parameters.
func (bwu *BaseWorkUnit) SetFromParams(params map[string]string) error {
	return nil
}

// UnitDir returns the unit directory of this work unit.
func (bwu *BaseWorkUnit) UnitDir() string {
	return bwu.unitDir
}

// ID returns the unique identifier of this work unit.
func (bwu *BaseWorkUnit) ID() string {
	return bwu.unitID
}

// StatusFileName returns the full path to the status file in the unit dir.
func (bwu *BaseWorkUnit) StatusFileName() string {
	return bwu.statusFileName
}

// StdoutFileName returns the full path to the stdout file in the unit dir.
func (bwu *BaseWorkUnit) StdoutFileName() string {
	return bwu.stdoutFileName
}

// lockStatusFile gains a lock on the status file.
func (sfd *StatusFileData) lockStatusFile(filename string) (*lockedfile.File, error) {
	lockFileName := filename + ".lock"
	lockFile, err := lockedfile.OpenFile(lockFileName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, err
	}

	return lockFile, nil
}

// unlockStatusFile releases the lock on the status file.
func (sfd *StatusFileData) unlockStatusFile(filename string, lockFile *lockedfile.File) {
	if err := lockFile.Close(); err != nil {
		logger.Error("Error closing %s.lock: %s", filename, err)
	}
}

// saveToFile saves status to an already-open file.
func (sfd *StatusFileData) saveToFile(file io.Writer) error {
	jsonBytes, err := json.Marshal(sfd)
	if err != nil {
		return err
	}
	jsonBytes = append(jsonBytes, '\n')
	_, err = file.Write(jsonBytes)

	return err
}

// Save saves status to a file.
func (sfd *StatusFileData) Save(filename string) error {
	lockFile, err := sfd.lockStatusFile(filename)
	if err != nil {
		return err
	}
	defer sfd.unlockStatusFile(filename, lockFile)
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	err = sfd.saveToFile(file)
	if err != nil {
		_ = file.Close()

		return err
	}

	return file.Close()
}

// Save saves status to a file.
func (bwu *BaseWorkUnit) Save() error {
	bwu.statusLock.RLock()
	defer bwu.statusLock.RUnlock()

	return bwu.status.Save(bwu.statusFileName)
}

// loadFromFile loads status from an already open file.
func (sfd *StatusFileData) loadFromFile(file io.Reader) error {
	jsonBytes, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}

	return json.Unmarshal(jsonBytes, sfd)
}

// Load loads status from a file.
func (sfd *StatusFileData) Load(filename string) error {
	lockFile, err := sfd.lockStatusFile(filename)
	if err != nil {
		return err
	}
	defer sfd.unlockStatusFile(filename, lockFile)
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	err = sfd.loadFromFile(file)
	if err != nil {
		_ = file.Close()

		return err
	}

	return file.Close()
}

// Load loads status from a file.
func (bwu *BaseWorkUnit) Load() error {
	bwu.statusLock.Lock()
	defer bwu.statusLock.Unlock()

	return bwu.status.Load(bwu.statusFileName)
}

// UpdateFullStatus atomically updates the status metadata file.  Changes should be made in the callback function.
// Errors are logged rather than returned.
func (sfd *StatusFileData) UpdateFullStatus(filename string, statusFunc func(*StatusFileData)) error {
	lockFile, err := sfd.lockStatusFile(filename)
	if err != nil {
		return err
	}
	defer sfd.unlockStatusFile(filename, lockFile)
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		err := file.Close()
		if err != nil {
			logger.Error("Error closing %s: %s", filename, err)
		}
	}()
	size, err := file.Seek(0, 2)
	if err != nil {
		return err
	}
	_, err = file.Seek(0, 0)
	if err != nil {
		return err
	}
	if size > 0 {
		err = sfd.loadFromFile(file)
		if err != nil {
			return err
		}
	}
	statusFunc(sfd)
	_, err = file.Seek(0, 0)
	if err != nil {
		return err
	}
	err = file.Truncate(0)
	if err != nil {
		return err
	}
	err = sfd.saveToFile(file)
	if err != nil {
		return err
	}

	return nil
}

// UpdateFullStatus atomically updates the whole status record.  Changes should be made in the callback function.
// Errors are logged rather than returned.
func (bwu *BaseWorkUnit) UpdateFullStatus(statusFunc func(*StatusFileData)) {
	bwu.statusLock.Lock()
	defer bwu.statusLock.Unlock()
	err := bwu.status.UpdateFullStatus(bwu.statusFileName, statusFunc)
	bwu.lastUpdateError = err
	if err != nil {
		logger.Error("Error updating status file %s: %s.", bwu.statusFileName, err)
	}
}

// UpdateBasicStatus atomically updates key fields in the status metadata file.  Errors are logged rather than returned.
// Passing -1 as stdoutSize leaves it unchanged.
func (sfd *StatusFileData) UpdateBasicStatus(filename string, state int, detail string, stdoutSize int64) error {
	return sfd.UpdateFullStatus(filename, func(status *StatusFileData) {
		status.State = state
		status.Detail = detail
		if stdoutSize >= 0 {
			status.StdoutSize = stdoutSize
		}
	})
}

// UpdateBasicStatus atomically updates key fields in the status metadata file.  Errors are logged rather than returned.
// Passing -1 as stdoutSize leaves it unchanged.
func (bwu *BaseWorkUnit) UpdateBasicStatus(state int, detail string, stdoutSize int64) {
	bwu.statusLock.Lock()
	defer bwu.statusLock.Unlock()
	err := bwu.status.UpdateBasicStatus(bwu.statusFileName, state, detail, stdoutSize)
	bwu.lastUpdateError = err
	if err != nil {
		logger.Error("Error updating status file %s: %s.", bwu.statusFileName, err)
	}
}

// LastUpdateError returns the last error (including nil) resulting from an UpdateBasicStatus or UpdateFullStatus.
func (bwu *BaseWorkUnit) LastUpdateError() error {
	return bwu.lastUpdateError
}

// monitorLocalStatus watches a unit dir and keeps the in-memory workUnit up to date with status changes.
func (bwu *BaseWorkUnit) monitorLocalStatus() {
	statusFile := path.Join(bwu.UnitDir(), "status")
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
		watcherEvents = make(chan fsnotify.Event)
	} else {
		watcherEvents = watcher.Events
	}
loop:
	for {
		select {
		case <-bwu.ctx.Done():
			break loop
		case event := <-watcherEvents:
			if event.Op&fsnotify.Write == fsnotify.Write {
				err = bwu.Load()
				if err != nil {
					logger.Error("Error reading %s: %s", statusFile, err)
				}
			}
		case <-time.After(time.Second):
			newFi, err := os.Stat(statusFile)
			if err == nil {
				if fi == nil || fi.ModTime() != newFi.ModTime() {
					fi = newFi
					err = bwu.Load()
					if err != nil {
						logger.Error("Error reading %s: %s", statusFile, err)
					}
				}
			}
		}
		complete := IsComplete(bwu.Status().State)
		if complete {
			break
		}
	}
}

// getStatus returns a copy of the base status (no ExtraData).  The caller must already hold the statusLock.
func (bwu *BaseWorkUnit) getStatus() *StatusFileData {
	status := bwu.status
	status.ExtraData = nil

	return &status
}

// Status returns a copy of the status currently loaded in memory (use Load to get it from disk).
func (bwu *BaseWorkUnit) Status() *StatusFileData {
	return bwu.UnredactedStatus()
}

// UnredactedStatus returns a copy of the status currently loaded in memory, including secrets.
func (bwu *BaseWorkUnit) UnredactedStatus() *StatusFileData {
	bwu.statusLock.RLock()
	defer bwu.statusLock.RUnlock()

	return bwu.getStatus()
}

// Release releases this unit of work, deleting its files.
func (bwu *BaseWorkUnit) Release(force bool) error {
	bwu.statusLock.Lock()
	defer bwu.statusLock.Unlock()
	attemptsLeft := 3
	for {
		err := os.RemoveAll(bwu.UnitDir())
		if force {
			break
		} else if err != nil {
			attemptsLeft--

			if attemptsLeft > 0 {
				logger.Warning("Error removing directory for %s. Retrying %d more times.", bwu.unitID, attemptsLeft)
				time.Sleep(time.Second)

				continue
			} else {
				logger.Error("Error removing directory for %s. No more retries left.", bwu.unitID)

				return err
			}
		}

		break
	}
	bwu.w.activeUnitsLock.Lock()
	defer bwu.w.activeUnitsLock.Unlock()
	delete(bwu.w.activeUnits, bwu.unitID)

	return nil
}

// =============================================================================================== //

func newUnknownWorker(w *Workceptor, unitID string, workType string) WorkUnit {
	uu := &unknownUnit{}
	uu.BaseWorkUnit.Init(w, unitID, workType)

	return uu
}

// unknownUnit is used to represent units we find on disk, but don't recognize their WorkType.
type unknownUnit struct {
	BaseWorkUnit
}

// Start starts the unit.  Since we don't know what this unit is, we do nothing.
func (uu *unknownUnit) Start() error {
	return nil
}

// Restart restarts the unit.  Since we don't know what this unit is, we do nothing.
func (uu *unknownUnit) Restart() error {
	return nil
}

// Cancel cancels a running unit.  Since we don't know what this unit is, we do nothing.
func (uu *unknownUnit) Cancel() error {
	return nil
}

func (uu *unknownUnit) Status() *StatusFileData {
	status := uu.BaseWorkUnit.Status()
	status.ExtraData = "Unknown WorkType"

	return status
}
