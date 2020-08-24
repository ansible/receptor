package workceptor

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/project-receptor/receptor/pkg/logger"
	"io"
	"net"
	"os"
	"path"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"
)

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
			select {
			case <-ctx.Done():
				return
			default:
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

			if err != nil {
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

func (w *Workceptor) cancelRemote(remoteNodeID, remoteUnitID string, unit *workUnit, firstAttemptSuccess chan bool) {
	// to prevent blocking on sending to channel firstAttemptSuccess
	firstAttemptOnce := sync.Once{}
	reportFirstResult := func(success bool) {
		firstAttemptOnce.Do(func() {
			if firstAttemptSuccess != nil {
				firstAttemptSuccess <- success
			}
		})
	}
	retryInterval := SuccessWorkSleep
	firstTime := true
	for {
		if firstTime {
			firstTime = false
		} else {
			reportFirstResult(false)
			time.Sleep(retryInterval)
			retryInterval = time.Duration(1.5 * float64(retryInterval))
			if retryInterval > MaxWorkSleep {
				retryInterval = MaxWorkSleep
			}
		}
		// check that unit still exists, could have been released
		unit.lock.RLock()
		released := unit.released
		unit.lock.RUnlock()
		if released {
			reportFirstResult(true)
			return
		}
		conn, reader, err := w.connectToRemote(remoteNodeID)
		if err != nil {
			logger.Error("Connection failed to %s: %s\n", remoteNodeID, err)
			continue
		}
		_, err = conn.Write([]byte(fmt.Sprintf("work cancel %s\n", remoteUnitID)))
		if err != nil {
			logger.Error("Write error sending to %s: %s\n", remoteNodeID, err)
			conn.Close()
			continue
		}
		response, err := reader.ReadString('\n')
		if err != nil {
			logger.Error("Read error reading from %s: %s\n", remoteNodeID, err)
			conn.Close()
			continue
		}
		if response[:5] == "ERROR" {
			logger.Warning("Error cancelling remote unit: %s\n", response[6:])
			conn.Close()
			continue
		} else {
			reportFirstResult(true)
			conn.Close()
			return
		}
	}
}
