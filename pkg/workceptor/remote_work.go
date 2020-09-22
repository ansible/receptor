// +build !no_workceptor

package workceptor

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/project-receptor/receptor/pkg/logger"
	"github.com/project-receptor/receptor/pkg/utils"
	"io"
	"net"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"
)

// remoteUnit implements the WorkUnit interface for the Receptor remote worker plugin
type remoteUnit struct {
	BaseWorkUnit
	topJC *utils.JobContext
}

// remoteExtraData is the content of the ExtraData JSON field for a remote work unit
type remoteExtraData struct {
	RemoteNode     string
	RemoteWorkType string
	RemoteParams   map[string]string
	RemoteUnitID   string
	RemoteStarted  bool
	LocalCancelled bool
	LocalReleased  bool
	TLSClient      string
}

type actionFunc func(context.Context, net.Conn, *bufio.Reader) error

// connectToRemote establishes a control socket connection to a remote node
func (rw *remoteUnit) connectToRemote(ctx context.Context) (net.Conn, *bufio.Reader, error) {
	rw.statusLock.RLock()
	red, ok := rw.status.ExtraData.(*remoteExtraData)
	if !ok {
		rw.statusLock.RUnlock()
		return nil, nil, fmt.Errorf("remote ExtraData missing")
	}
	node := red.RemoteNode
	rw.statusLock.RUnlock()
	tlsClient := rw.Status().ExtraData.(*remoteExtraData).TLSClient
	expectedHostName := rw.Status().ExtraData.(*remoteExtraData).RemoteNode
	tlsConfig, err := rw.w.nc.GetClientTLSConfig(tlsClient, expectedHostName)
	if err != nil {
		return nil, nil, err
	}
	conn, err := rw.w.nc.DialContext(ctx, node, "control", tlsConfig)
	if err != nil {
		return nil, nil, err
	}
	reader := bufio.NewReader(conn)
	ctxChild, _ := context.WithTimeout(ctx, 5*time.Second)
	hello, err := utils.ReadStringContext(ctxChild, reader, '\n')
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	if !strings.Contains(hello, node) {
		conn.Close()
		return nil, nil, fmt.Errorf("while expecting node ID %s, got message: %s", node,
			strings.TrimRight(hello, "\n"))
	}
	return conn, reader, nil
}

// getConnection retries connectToRemote until connected or the context expires
func (rw *remoteUnit) getConnection(mw *utils.JobContext) (net.Conn, *bufio.Reader) {
	connectDelay := utils.NewIncrementalDuration(SuccessWorkSleep, MaxWorkSleep, 1.5)
	for {
		conn, reader, err := rw.connectToRemote(mw)
		if err == nil {
			return conn, reader
		}
		logger.Warning("Connection to %s failed with error: %s",
			rw.Status().ExtraData.(*remoteExtraData).RemoteNode, err)
		errStr := err.Error()
		if strings.Contains(errStr, "CRYPTO_ERROR") {
			var shouldExit = false
			rw.UpdateFullStatus(func(status *StatusFileData) {
				status.Detail = fmt.Sprintf("TLS error connecting to remote service: %s", errStr)
				if !status.ExtraData.(*remoteExtraData).RemoteStarted {
					shouldExit = true
					status.State = WorkStateFailed
				}
			})
			if shouldExit {
				mw.Cancel()
				return nil, nil
			}
		}
		select {
		case <-mw.Done():
			return nil, nil
		case <-connectDelay.NextTimeout():
		}
	}
}

// connectAndRun makes a single attempt to connect to a remote node and runs an action function
func (rw *remoteUnit) connectAndRun(ctx context.Context, action actionFunc) error {
	conn, reader, err := rw.connectToRemote(ctx)
	if err != nil {
		return utils.WrapErrorWithKind(err, "connection")
	}
	return action(ctx, conn, reader)
}

// getConnectionAndRun retries connecting to a host and, once the connection succeeds, runs an action function.
// If firstTimeSync is true, a single attempt is made on the calling goroutine. If the initial attempt fails to
// connect or firstTimeSync is false, we run return ErrPending to the caller.
func (rw *remoteUnit) getConnectionAndRun(mw *utils.JobContext, firstTimeSync bool, action actionFunc) error {
	if firstTimeSync {
		err := rw.connectAndRun(mw, action)
		if err == nil {
			return nil
		} else if !utils.ErrorIsKind(err, "connection") {
			return err
		}
	}
	go func() {
		conn, reader := rw.getConnection(mw)
		if conn != nil {
			_ = action(mw, conn, reader)
		}
		mw.WorkerDone()
	}()
	return ErrPending
}

// startRemoteUnit makes a single attempt to start a remote unit.
func (rw *remoteUnit) startRemoteUnit(ctx context.Context, conn net.Conn, reader *bufio.Reader) error {
	closeOnce := sync.Once{}
	doClose := func() error {
		var err error
		closeOnce.Do(func() {
			err = conn.Close()
		})
		return err
	}
	defer doClose()
	red := rw.Status().ExtraData.(*remoteExtraData)
	_, err := conn.Write([]byte(fmt.Sprintf("work submit localhost %s\n", red.RemoteWorkType)))
	if err != nil {
		return fmt.Errorf("write error sending to %s: %s", red.RemoteNode, err)
	}
	response, err := utils.ReadStringContext(ctx, reader, '\n')
	if err != nil {
		conn.Close()
		return fmt.Errorf("read error reading from %s: %s", red.RemoteNode, err)
	}
	submitIDRegex := regexp.MustCompile("with ID ([a-zA-Z0-9]+)\\.")
	match := submitIDRegex.FindSubmatch([]byte(response))
	if match == nil || len(match) != 2 {
		return fmt.Errorf("could not parse response: %s", strings.TrimRight(response, "\n"))
	}
	red.RemoteUnitID = string(match[1])
	rw.UpdateFullStatus(func(status *StatusFileData) {
		ed := status.ExtraData.(*remoteExtraData)
		ed.RemoteUnitID = red.RemoteUnitID
	})
	stdin, err := os.Open(path.Join(rw.UnitDir(), "stdin"))
	if err != nil {
		return fmt.Errorf("error opening stdin file: %s", err)
	}
	_, err = io.Copy(conn, stdin)
	if err != nil {
		return fmt.Errorf("error sending stdin file: %s", err)
	}
	err = doClose()
	if err != nil {
		return fmt.Errorf("error closing stdin file: %s", err)
	}
	response, err = utils.ReadStringContext(ctx, reader, '\n')
	if err != nil {
		conn.Close()
		return fmt.Errorf("read error reading from %s: %s", red.RemoteNode, err)
	}
	resultErrorRegex := regexp.MustCompile("ERROR: (.*)")
	match = resultErrorRegex.FindSubmatch([]byte(response))
	if match != nil {
		return fmt.Errorf("error from remote: %s", match[1])
	}
	rw.UpdateFullStatus(func(status *StatusFileData) {
		ed := status.ExtraData.(*remoteExtraData)
		ed.RemoteStarted = true
	})
	return nil
}

// cancelOrReleaseRemoteUnit makes a single attempt to cancel or release a remote unit.
func (rw *remoteUnit) cancelOrReleaseRemoteUnit(ctx context.Context, conn net.Conn, reader *bufio.Reader,
	release bool, force bool) error {
	defer conn.Close()
	red := rw.Status().ExtraData.(*remoteExtraData)
	var workCmd string
	if release {
		workCmd = "release"
	} else {
		workCmd = "cancel"
	}
	_, err := conn.Write([]byte(fmt.Sprintf("work %s %s\n", workCmd, red.RemoteUnitID)))
	if err != nil {
		return fmt.Errorf("write error sending to %s: %s", red.RemoteNode, err)
	}
	response, err := utils.ReadStringContext(ctx, reader, '\n')
	if err != nil {
		conn.Close()
		return fmt.Errorf("read error reading from %s: %s", red.RemoteNode, err)
	}
	if response[:5] == "ERROR" {
		return fmt.Errorf("error cancelling remote unit: %s", response[6:])
	}
	return nil
}

// monitorRemoteStatus monitors the remote status file and copies results to the local one
func (rw *remoteUnit) monitorRemoteStatus(mw *utils.JobContext, forRelease bool) {
	defer func() {
		mw.Cancel()
		mw.WorkerDone()
	}()
	status := rw.Status()
	red, ok := status.ExtraData.(*remoteExtraData)
	if !ok {
		logger.Error("remote ExtraData missing")
		return
	}
	remoteNode := red.RemoteNode
	remoteUnitID := red.RemoteUnitID
	conn, reader := rw.getConnection(mw)
	if conn == nil {
		return
	}
	for {
		if conn == nil {
			conn, reader = rw.getConnection(mw)
			if conn == nil {
				return
			}
		}
		_, err := conn.Write([]byte(fmt.Sprintf("work status %s\n", remoteUnitID)))
		if err != nil {
			logger.Debug("Write error sending to %s: %s\n", remoteUnitID, err)
			_ = conn.Close()
			conn = nil
			continue
		}
		status, err := utils.ReadStringContext(mw, reader, '\n')
		if err != nil {
			logger.Debug("Read error reading from %s: %s\n", remoteNode, err)
			_ = conn.Close()
			conn = nil
			continue
		}
		if status[:5] == "ERROR" {
			if strings.Contains(status, "unknown work unit") {
				if !forRelease {
					logger.Debug("Work unit %s on node %s is gone.\n", remoteUnitID, remoteNode)
					rw.UpdateFullStatus(func(status *StatusFileData) {
						status.State = WorkStateFailed
						status.Detail = "Remote work unit is gone"
					})
				}
				return
			}
			logger.Error("Remote error: %s\n", strings.TrimRight(status[6:], "\n"))
			return
		}
		si := StatusFileData{}
		err = json.Unmarshal([]byte(status), &si)
		if err != nil {
			logger.Error("Error unmarshalling JSON: %s\n", status)
			return
		}
		rw.UpdateBasicStatus(si.State, si.Detail, si.StdoutSize)
		if err != nil {
			logger.Error("Error saving local status file: %s\n", err)
			return
		}
		if sleepOrDone(mw.Done(), 1*time.Second) {
			return
		}
	}
}

// monitorRemoteStdout copies the remote stdout stream to the local buffer
func (rw *remoteUnit) monitorRemoteStdout(mw *utils.JobContext) {
	defer func() {
		mw.Cancel()
		mw.WorkerDone()
	}()
	firstTime := true
	status := rw.Status()
	red, ok := status.ExtraData.(*remoteExtraData)
	if !ok {
		logger.Error("remote ExtraData missing")
		return
	}
	remoteNode := red.RemoteNode
	remoteUnitID := red.RemoteUnitID
	for {
		if firstTime {
			firstTime = false
			if mw.Err() != nil {
				return
			}
		} else {
			if sleepOrDone(mw.Done(), 1*time.Second) {
				return
			}
		}
		err := rw.Load()
		if err != nil {
			logger.Error("Could not read status file %s: %s\n", rw.statusFileName, err)
			return
		}
		status := rw.Status()
		diskStdoutSize := stdoutSize(rw.UnitDir())
		remoteStdoutSize := status.StdoutSize
		if IsComplete(status.State) && diskStdoutSize >= remoteStdoutSize {
			return
		} else if diskStdoutSize < remoteStdoutSize {
			conn, reader := rw.getConnection(mw)
			if conn == nil {
				return
			}
			_, err := conn.Write([]byte(fmt.Sprintf("work results %s %d\n", remoteUnitID, diskStdoutSize)))
			if err != nil {
				logger.Warning("Write error sending to %s: %s\n", remoteNode, err)
				continue
			}
			status, err := utils.ReadStringContext(mw, reader, '\n')
			if err != nil {
				logger.Warning("Read error reading from %s: %s\n", remoteNode, err)
				continue
			}
			if !strings.Contains(status, "Streaming results") {
				logger.Warning("Remote node %s did not stream results\n", remoteNode)
				continue
			}
			stdout, err := os.OpenFile(rw.stdoutFileName, os.O_CREATE+os.O_APPEND+os.O_WRONLY, 0600)
			if err != nil {
				logger.Error("Could not open stdout file %s: %s\n", rw.stdoutFileName, err)
				return
			}
			doneChan := make(chan struct{})
			go func() {
				select {
				case <-doneChan:
					return
				case <-mw.Done():
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
			if err != nil {
				logger.Warning("Error copying to stdout file %s: %s\n", rw.stdoutFileName, err)
				continue
			}
		}
	}
}

// monitorRemoteUnit watches a remote unit on another node and maintains local status
func (rw *remoteUnit) monitorRemoteUnit(mw *utils.JobContext, forRelease bool) {
	subJC := &utils.JobContext{}
	if forRelease {
		subJC.NewJob(mw, 1, false)
		go rw.monitorRemoteStatus(subJC, true)
	} else {
		subJC.NewJob(mw, 2, false)
		go rw.monitorRemoteStatus(subJC, false)
		go rw.monitorRemoteStdout(subJC)
	}
	subJC.Wait()
}

// Init initializes the work unit data
func (rw *remoteUnit) Init(w *Workceptor, ident string, workType string) {
	rw.BaseWorkUnit.Init(w, ident, workType)
	red := &remoteExtraData{}
	red.RemoteParams = make(map[string]string)
	rw.status.ExtraData = red
	rw.topJC = &utils.JobContext{}
}

// SetParamsAndSave sets the unit's parameters and saves it
func (rw *remoteUnit) SetParamsAndSave(params map[string]string) error {
	for k, v := range params {
		rw.status.ExtraData.(*remoteExtraData).RemoteParams[k] = v
	}
	return rw.Save()
}

// Status returns a copy of the status currently loaded in memory
func (rw *remoteUnit) Status() *StatusFileData {
	rw.statusLock.RLock()
	defer rw.statusLock.RUnlock()
	status := rw.getStatus()
	ed, ok := rw.status.ExtraData.(*remoteExtraData)
	if ok {
		edCopy := *ed
		status.ExtraData = &edCopy
	}
	return status
}

// runAndMonitor waits for a connection to be available, then starts the remote unit and monitors it
func (rw *remoteUnit) runAndMonitor(mw *utils.JobContext, forRelease bool, action actionFunc) error {
	return rw.getConnectionAndRun(mw, true, func(ctx context.Context, conn net.Conn, reader *bufio.Reader) error {
		err := action(ctx, conn, reader)
		if err != nil {
			mw.WorkerDone()
			return err
		}
		go func() {
			rw.monitorRemoteUnit(mw, forRelease)
			if forRelease {
				err := rw.BaseWorkUnit.Release(false)
				if err != nil {
					logger.Error("Error releasing unit %s: %s", rw.UnitDir(), err)
				}
			}
			mw.WorkerDone()
		}()
		return nil
	})
}

// startOrRestart is a shared implementation of Start() and Restart()
func (rw *remoteUnit) startOrRestart(start bool) error {
	red := rw.Status().ExtraData.(*remoteExtraData)
	if start && red.RemoteStarted {
		return fmt.Errorf("unit was already started")
	}
	newJobStarted := rw.topJC.NewJob(rw.w.ctx, 1, true)
	if !newJobStarted {
		return fmt.Errorf("start or monitor process already running")
	}
	if start || !red.RemoteStarted {
		return rw.runAndMonitor(rw.topJC, false, rw.startRemoteUnit)
	} else if red.LocalReleased || red.LocalCancelled {
		return rw.runAndMonitor(rw.topJC, true, func(ctx context.Context, conn net.Conn, reader *bufio.Reader) error {
			return rw.cancelOrReleaseRemoteUnit(ctx, conn, reader, red.LocalReleased, false)
		})
	}
	go func() {
		rw.monitorRemoteUnit(rw.topJC, false)
		rw.topJC.WorkerDone()
	}()
	return nil
}

// Start launches a job with given parameters.  If the remote node is unreachable, returns ErrPending
// and continues to retry in the background.
func (rw *remoteUnit) Start() error {
	return rw.startOrRestart(true)
}

// Restart resumes monitoring a job after a Receptor restart
func (rw *remoteUnit) Restart() error {
	return rw.startOrRestart(false)
}

// cancelOrRelease is a shared implementation of Cancel() and Release()
func (rw *remoteUnit) cancelOrRelease(release bool, force bool) error {
	// Update the status file that the unit is locally cancelled/released
	var remoteStarted bool
	rw.UpdateFullStatus(func(status *StatusFileData) {
		status.ExtraData.(*remoteExtraData).LocalCancelled = true
		if release {
			status.ExtraData.(*remoteExtraData).LocalReleased = true
		}
		remoteStarted = status.ExtraData.(*remoteExtraData).RemoteStarted
	})
	// if remote work has not started, don't attempt to connect to remote
	if !remoteStarted {
		rw.topJC.Cancel()
		rw.topJC.Wait()
		if release {
			return rw.BaseWorkUnit.Release(true)
		}
	}
	if release && force {
		_ = rw.connectAndRun(rw.w.ctx, func(ctx context.Context, conn net.Conn, reader *bufio.Reader) error {
			return rw.cancelOrReleaseRemoteUnit(ctx, conn, reader, true, true)
		})
		return rw.BaseWorkUnit.Release(true)
	}
	rw.topJC.NewJob(rw.w.ctx, 1, false)
	return rw.runAndMonitor(rw.topJC, true, func(ctx context.Context, conn net.Conn, reader *bufio.Reader) error {
		return rw.cancelOrReleaseRemoteUnit(ctx, conn, reader, release, false)
	})
}

// Cancel stops a running job.
func (rw *remoteUnit) Cancel() error {
	return rw.cancelOrRelease(false, false)
}

// Release releases resources associated with a job.  Implies Cancel.
func (rw *remoteUnit) Release(force bool) error {
	return rw.cancelOrRelease(true, force)
}

func newRemoteWorker() WorkUnit {
	return &remoteUnit{}
}
