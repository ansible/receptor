//go:build !no_workceptor
// +build !no_workceptor

package workceptor

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/utils"
)

const errMsgRemoteExtraDataMissing = "remote ExtraData missing"

// remoteUnit implements the WorkUnit interface for the Receptor remote worker plugin.
type remoteUnit struct {
	BaseWorkUnitForWorkUnit
	topJC  *utils.JobContext
	logger *logger.ReceptorLogger
}

// RemoteExtraData is the content of the ExtraData JSON field for a remote work unit.
type RemoteExtraData struct {
	RemoteNode     string
	RemoteWorkType string
	RemoteParams   map[string]string
	RemoteUnitID   string
	RemoteStarted  bool
	LocalCancelled bool
	LocalReleased  bool
	SignWork       bool
	TLSClient      string
	Expiration     time.Time
}

type actionFunc func(context.Context, net.Conn, *bufio.Reader) error

// ConnectToRemote establishes a control socket connection to a remote node.
func (rw *remoteUnit) ConnectToRemote(ctx context.Context) (net.Conn, *bufio.Reader, error) {
	status := rw.Status()
	red, ok := status.ExtraData.(*RemoteExtraData)
	if !ok {
		return nil, nil, errors.New(errMsgRemoteExtraDataMissing)
	}
	tlsConfig, err := rw.GetWorkceptor().nc.GetClientTLSConfig(red.TLSClient, red.RemoteNode, netceptor.ExpectedHostnameTypeReceptor)
	if err != nil {
		return nil, nil, err
	}
	conn, err := rw.GetWorkceptor().nc.DialContext(ctx, red.RemoteNode, "control", tlsConfig)
	if err != nil {
		return nil, nil, err
	}
	reader := bufio.NewReader(conn)
	ctxChild, ctxCancel := context.WithTimeout(ctx, 5*time.Second)
	defer ctxCancel()

	hello, err := utils.ReadStringContext(ctxChild, reader, '\n')
	if err != nil {
		conn.CloseConnection()

		return nil, nil, err
	}
	if !strings.Contains(hello, red.RemoteNode) {
		conn.CloseConnection()

		return nil, nil, fmt.Errorf("while expecting node ID %s, got message: %s", red.RemoteNode,
			strings.TrimRight(hello, "\n"))
	}

	return conn, reader, nil
}

// getCryptoErrorDetail checks if an error is a crypto-related error and returns
// a detailed error message if it is, or an empty string if it's not.
func getCryptoErrorDetail(errStr string) string {
	if strings.Contains(errStr, "CRYPTO_BUFFER_EXCEEDED") {
		return fmt.Sprintf("QUIC crypto buffer exceeded. CA bundle may be too large (limit: 16KB). See KCS 7129200: %s", errStr)
	}
	if strings.Contains(errStr, "CRYPTO_ERROR") {
		return fmt.Sprintf("TLS error connecting to remote service: %s", errStr)
	}

	return ""
}

// GetConnection retries connectToRemote until connected or the context expires.
func (rw *remoteUnit) GetConnection(ctx context.Context) (net.Conn, *bufio.Reader) {
	connectDelay := utils.NewIncrementalDuration(SuccessWorkSleep, MaxWorkSleep, 1.5)
	for {
		conn, reader, err := rw.ConnectToRemote(ctx)
		if err == nil {
			return conn, reader
		}
		rw.GetWorkceptor().nc.GetLogger().Info("Connection to %s failed with error: %s",
			rw.Status().ExtraData.(*RemoteExtraData).RemoteNode, err)
		errStr := err.Error()

		// Only return on CRYPTO errors, others are retryable.
		detail := getCryptoErrorDetail(errStr)
		if detail != "" {
			shouldExit := false
			rw.UpdateFullStatus(func(status *StatusFileData) {
				status.Detail = detail
				if red, ok := status.ExtraData.(*RemoteExtraData); ok && !red.RemoteStarted {
					shouldExit = true
					status.State = WorkStateFailed
				}
			})

			if shouldExit {
				// Log the helpful error message so it appears in logs, not just status file
				rw.GetWorkceptor().nc.GetLogger().Error("%s", detail)

				return nil, nil
			}
		}
		select {
		case <-ctx.Done():
			return nil, nil
		case <-connectDelay.NextTimeout():
		}
	}
}

// connectAndRun makes a single attempt to connect to a remote node and runs an action function.
func (rw *remoteUnit) connectAndRun(ctx context.Context, action actionFunc) error {
	conn, reader, err := rw.ConnectToRemote(ctx)
	if err != nil {
		return utils.WrapErrorWithKind(err, "connection")
	}

	return action(ctx, conn, reader)
}

// getConnectionAndRun retries connecting to a host and, once the connection succeeds, runs an action function.
// If firstTimeSync is true, a single attempt is made on the calling goroutine. If the initial attempt fails to
// connect or firstTimeSync is false, we run return ErrPending to the caller.
func (rw *remoteUnit) getConnectionAndRun(ctx context.Context, firstTimeSync bool, action actionFunc, failure func()) error {
	if firstTimeSync {
		err := rw.connectAndRun(ctx, action)
		if err == nil {
			return nil
		} else if !utils.ErrorIsKind(err, "connection") {
			return err
		}
	}
	go func() {
		conn, reader := rw.GetConnection(ctx)
		if conn != nil {
			err := action(ctx, conn, reader)
			if err != nil {
				rw.GetWorkceptor().nc.GetLogger().Error("Error running action function: %s", err)
			}
		} else {
			failure()
		}
	}()

	return ErrPending
}

// StartRemoteUnit makes a single attempt to start a remote unit.
func (rw *remoteUnit) StartRemoteUnit(ctx context.Context, conn net.Conn, reader *bufio.Reader) error {
	defer conn.(interface{ CloseConnection() error }).CloseConnection()
	red := rw.UnredactedStatus().ExtraData.(*RemoteExtraData)
	workSubmitCmd := make(map[string]interface{})
	for k, v := range red.RemoteParams {
		workSubmitCmd[k] = v
	}
	workSubmitCmd["workUnitID"] = rw.ID()
	workSubmitCmd["command"] = "work"
	workSubmitCmd["subcommand"] = "submit"
	workSubmitCmd["node"] = red.RemoteNode
	workSubmitCmd["worktype"] = red.RemoteWorkType
	workSubmitCmd["tlsclient"] = red.TLSClient
	if red.SignWork {
		signature, err := rw.GetWorkceptor().createSignature(red.RemoteNode)
		if err != nil {
			return err
		}
		workSubmitCmd["signature"] = signature
	}
	wscBytes, err := json.Marshal(workSubmitCmd)
	if err != nil {
		return fmt.Errorf("error constructing work submit command: %s", err)
	}
	_, err = conn.Write(wscBytes)
	if err == nil {
		_, err = conn.Write([]byte("\n"))
	}
	if err != nil {
		return fmt.Errorf("write error sending to %s: %s", red.RemoteNode, err)
	}
	response, err := utils.ReadStringContext(ctx, reader, '\n')
	if err != nil {
		return fmt.Errorf("read error reading from %s: %s", red.RemoteNode, err)
	}
	submitIDRegex := regexp.MustCompile(`with ID ([a-zA-Z0-9]+)\.`)
	match := submitIDRegex.FindSubmatch([]byte(response))
	if match == nil || len(match) != 2 {
		return fmt.Errorf("could not parse response: %s", strings.TrimRight(response, "\n"))
	}
	red.RemoteUnitID = string(match[1])
	rw.UpdateFullStatus(func(status *StatusFileData) {
		ed := status.ExtraData.(*RemoteExtraData)
		ed.RemoteUnitID = red.RemoteUnitID
	})
	stdin, err := os.Open(path.Join(rw.UnitDir(), "stdin"))
	if err != nil {
		return fmt.Errorf("error opening stdin file: %s", err)
	}
	defer func() {
		err := stdin.Close()
		if err != nil {
			MainInstance.nc.GetLogger().Error("Error closing %s: %s", path.Join(rw.UnitDir(), "stdin"), err)
		}
	}()
	_, err = io.Copy(conn, stdin)
	if err != nil {
		return fmt.Errorf("error sending stdin file: %s", err)
	}
	err = conn.Close()
	if err != nil {
		return fmt.Errorf("error closing stdin file: %s", err)
	}
	response, err = utils.ReadStringContext(ctx, reader, '\n')
	if err != nil {
		return fmt.Errorf("read error reading from %s: %s", red.RemoteNode, err)
	}
	resultErrorRegex := regexp.MustCompile("ERROR: (.*)")
	match = resultErrorRegex.FindSubmatch([]byte(response))
	if match != nil {
		return fmt.Errorf("error from remote: %s", match[1])
	}
	rw.UpdateFullStatus(func(status *StatusFileData) {
		ed := status.ExtraData.(*RemoteExtraData)
		ed.RemoteStarted = true
	})

	return nil
}

// cancelOrReleaseRemoteUnit makes a single attempt to cancel or release a remote unit.
func (rw *remoteUnit) cancelOrReleaseRemoteUnit(ctx context.Context, conn net.Conn, reader *bufio.Reader,
	release bool,
) error {
	defer conn.(interface{ CloseConnection() error }).CloseConnection()
	red := rw.Status().ExtraData.(*RemoteExtraData)
	var workCmd string
	if release {
		workCmd = "release"
	} else {
		workCmd = "cancel"
	}
	workSubmitCmd := make(map[string]interface{})
	workSubmitCmd["command"] = "work"
	workSubmitCmd["subcommand"] = workCmd
	workSubmitCmd["unitid"] = red.RemoteUnitID
	if red.SignWork {
		signature, err := rw.GetWorkceptor().createSignature(red.RemoteNode)
		if err != nil {
			return err
		}
		workSubmitCmd["signature"] = signature
	}
	wscBytes, err := json.Marshal(workSubmitCmd)
	if err != nil {
		return fmt.Errorf("error constructing work %s command: %s", workCmd, err)
	}
	wscBytes = append(wscBytes, '\n')
	_, err = conn.Write(wscBytes)
	if err != nil {
		return fmt.Errorf("write error sending to %s: %s", red.RemoteNode, err)
	}
	response, err := utils.ReadStringContext(ctx, reader, '\n')
	if err != nil {
		return fmt.Errorf("read error reading from %s: %s", red.RemoteNode, err)
	}
	if response[:5] == "ERROR" {
		return fmt.Errorf("error cancelling remote unit: %s", response[6:])
	}

	return nil
}

// closeStatusConn closes a connection used by monitorRemoteStatus and logs any error.
func (rw *remoteUnit) closeStatusConn(conn net.Conn, label, id string) {
	cerr := conn.(interface{ CloseConnection() error }).CloseConnection()
	if cerr != nil {
		rw.GetWorkceptor().nc.GetLogger().Error("Error closing connection %s %s: %s", label, id, cerr)
	}
}

// handleRemoteStatusResponse parses a status response string and applies it locally.
// Returns true when the caller should stop the monitoring loop.
func (rw *remoteUnit) handleRemoteStatusResponse(status, remoteNode, remoteUnitID string) bool {
	if status[:5] == "ERROR" {
		if strings.Contains(status, "unknown work unit") {
			rw.GetWorkceptor().nc.GetLogger().Debug("Work unit %s on node %s is gone.\n", remoteUnitID, remoteNode)
			rw.UpdateFullStatus(func(s *StatusFileData) {
				s.State = WorkStateFailed
				s.Detail = "Remote work unit is gone"
			})
		} else {
			rw.GetWorkceptor().nc.GetLogger().Error("Remote error: %s\n", strings.TrimRight(status[6:], "\n"))
		}

		return true
	}
	si := StatusFileData{}
	if err := json.Unmarshal([]byte(status), &si); err != nil {
		rw.GetWorkceptor().nc.GetLogger().Error("Error unmarshalling JSON: %s\n", status)

		return true
	}
	rw.UpdateBasicStatus(si.State, si.Detail, si.StdoutSize)

	return false
}

// monitorRemoteStatus monitors the remote status file and copies results to the local one.
func (rw *remoteUnit) monitorRemoteStatus(mw *utils.JobContext, forRelease bool) {
	defer func() {
		mw.Cancel()
		mw.WorkerDone()
	}()
	status := rw.Status()
	red, ok := status.ExtraData.(*RemoteExtraData)
	if !ok {
		rw.GetWorkceptor().nc.GetLogger().Error(errMsgRemoteExtraDataMissing)

		return
	}
	remoteNode := red.RemoteNode
	remoteUnitID := red.RemoteUnitID
	conn, reader := rw.GetConnection(mw)
	defer func() {
		if conn != nil {
			rw.closeStatusConn(conn, "to remote work unit", remoteUnitID)
		}
	}()
	if conn == nil {
		return
	}
	writeStatusFailures := 0
	for {
		if conn == nil {
			conn, reader = rw.GetConnection(mw)
			if conn == nil {
				return
			}
		}
		if _, err := fmt.Fprintf(conn, "work status %s\n", remoteUnitID); err != nil {
			rw.GetWorkceptor().nc.GetLogger().Debug("Write error sending to %s: %s\n", remoteUnitID, err)
			rw.closeStatusConn(conn, "to remote work unit", remoteUnitID)
			conn = nil

			continue
		}
		resp, err := utils.ReadStringContext(mw, reader, '\n')
		if err != nil {
			rw.GetWorkceptor().nc.GetLogger().Debug("Read error reading from %s: %s\n", remoteNode, err)
			rw.closeStatusConn(conn, "from node", remoteNode)
			conn = nil

			continue
		}
		if rw.handleRemoteStatusResponse(resp, remoteNode, remoteUnitID) {
			return
		}
		if rw.LastUpdateError() != nil {
			writeStatusFailures++
			if writeStatusFailures > 3 {
				rw.GetWorkceptor().nc.GetLogger().Error("Exceeded retries for updating status file for work unit %s", rw.ID())

				return
			}
		} else {
			writeStatusFailures = 0
		}
		if sleepOrDone(mw.Done(), 1*time.Second) {
			return
		}
	}
}

// buildResultsCmd constructs the JSON "work results" command, adding a signature when required.
func (rw *remoteUnit) buildResultsCmd(remoteUnitID string, diskStdoutSize int64, red *RemoteExtraData) ([]byte, error) {
	cmd := map[string]interface{}{
		"command":    "work",
		"subcommand": "results",
		"unitid":     remoteUnitID,
		"startpos":   diskStdoutSize,
	}
	if red.SignWork {
		sig, err := rw.GetWorkceptor().createSignature(red.RemoteNode)
		if err != nil {
			return nil, fmt.Errorf("could not create signature to get results")
		}
		cmd["signature"] = sig
	}
	b, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("error constructing work results command: %s", err)
	}

	return append(b, '\n'), nil
}

// copyStdoutWithCancel copies reader → stdout and cancels the connection if mw is done.
func (rw *remoteUnit) copyStdoutWithCancel(mw *utils.JobContext, conn net.Conn, reader io.Reader, stdout io.Writer, remoteUnitID string) error {
	doneChan := make(chan struct{})
	go func() {
		select {
		case <-doneChan:
		case <-mw.Done():
			if cr, ok := conn.(interface{ CancelRead() }); ok {
				cr.CancelRead()
			}
			if cerr := conn.(interface{ CloseConnection() error }).CloseConnection(); cerr != nil {
				rw.GetWorkceptor().nc.GetLogger().Error("Error closing connection to %s: %s", remoteUnitID, cerr)
			}
		}
	}()
	_, err := io.Copy(stdout, reader)
	close(doneChan)

	return err
}

// fetchStdoutChunk requests and streams a chunk of remote stdout into the local file.
// Returns true when the outer loop should continue (recoverable issue), false to return.
func (rw *remoteUnit) fetchStdoutChunk(mw *utils.JobContext, remoteNode, remoteUnitID string, diskStdoutSize int64, red *RemoteExtraData) (shouldContinue bool) {
	conn, reader := rw.GetConnection(mw)
	if conn == nil {
		return false
	}
	defer func() {
		if conn != nil {
			if cerr := conn.(interface{ CloseConnection() error }).CloseConnection(); cerr != nil {
				rw.GetWorkceptor().nc.GetLogger().Error("Error closing connection to %s: %s", remoteUnitID, cerr)
			}
		}
	}()

	cmdBytes, err := rw.buildResultsCmd(remoteUnitID, diskStdoutSize, red)
	if err != nil {
		rw.GetWorkceptor().nc.GetLogger().Error("%s", err)

		return false
	}
	if _, err = conn.Write(cmdBytes); err != nil {
		rw.GetWorkceptor().nc.GetLogger().Warning("Write error sending to %s: %s\n", remoteNode, err)

		return true
	}
	resp, err := utils.ReadStringContext(mw, reader, '\n')
	if err != nil {
		rw.GetWorkceptor().nc.GetLogger().Warning("Read error reading from %s: %s\n", remoteNode, err)

		return true
	}
	if !strings.Contains(resp, "Streaming results") {
		rw.GetWorkceptor().nc.GetLogger().Warning("Remote node %s did not stream results\n", remoteNode)

		return true
	}
	stdout, err := os.OpenFile(rw.StdoutFileName(), os.O_CREATE+os.O_APPEND+os.O_WRONLY, 0o600)
	if err != nil {
		rw.GetWorkceptor().nc.GetLogger().Error("Could not open stdout file %s: %s\n", rw.StdoutFileName(), err)

		return false
	}
	defer func() {
		if cerr := stdout.Close(); cerr != nil {
			MainInstance.nc.GetLogger().Error("Error closing %s: %s", rw.StdoutFileName(), cerr)
		}
	}()
	if err = rw.copyStdoutWithCancel(mw, conn, reader, stdout, remoteUnitID); err != nil {
		errmsg := err.Error()
		if strings.HasSuffix(errmsg, "error code 499") {
			errmsg = "read operation cancelled"
		}
		rw.GetWorkceptor().nc.GetLogger().Warning("Could not copy to stdout file %s: %s\n", rw.StdoutFileName(), errmsg)

		return true
	}

	return true
}

// monitorRemoteStdout copies the remote stdout stream to the local buffer.
func (rw *remoteUnit) monitorRemoteStdout(mw *utils.JobContext) {
	defer func() {
		mw.Cancel()
		mw.WorkerDone()
	}()
	status := rw.Status()
	red, ok := status.ExtraData.(*RemoteExtraData)
	if !ok {
		rw.GetWorkceptor().nc.GetLogger().Error(errMsgRemoteExtraDataMissing)

		return
	}
	remoteNode := red.RemoteNode
	remoteUnitID := red.RemoteUnitID

	// Touch the stdout file to ensure it exists.
	if f, err := os.OpenFile(rw.StdoutFileName(), os.O_CREATE+os.O_APPEND+os.O_WRONLY, 0o600); err != nil {
		rw.GetWorkceptor().nc.GetLogger().Error("Could not open stdout file %s: %s\n", rw.StdoutFileName(), err)

		return
	} else {
		_ = f.Close()
	}

	firstTime := true
	for {
		if firstTime {
			firstTime = false
			if mw.Err() != nil {
				return
			}
		} else if sleepOrDone(mw.Done(), 1*time.Second) {
			return
		}
		if err := rw.Load(); err != nil {
			rw.GetWorkceptor().nc.GetLogger().Error("Could not read status file %s: %s\n", rw.StatusFileName(), err)

			return
		}
		status := rw.Status()
		diskStdoutSize := stdoutSize(rw.UnitDir())
		if IsComplete(status.State) && diskStdoutSize >= status.StdoutSize {
			return
		}
		if diskStdoutSize < status.StdoutSize {
			if !rw.fetchStdoutChunk(mw, remoteNode, remoteUnitID, diskStdoutSize, red) {
				return
			}
		}
	}
}

// monitorRemoteUnit watches a remote unit on another node and maintains local status.
func (rw *remoteUnit) monitorRemoteUnit(ctx context.Context) {
	subJC := &utils.JobContext{}
	subJC.NewJob(ctx, 2, false)
	go rw.monitorRemoteStatus(subJC, false)
	go rw.monitorRemoteStdout(subJC)
	subJC.Wait()
}

// SetFromParams sets the in-memory state from parameters.
func (rw *remoteUnit) SetFromParams(params map[string]string) error {
	for k, v := range params {
		rw.GetStatusCopy().ExtraData.(*RemoteExtraData).RemoteParams[k] = v
	}

	return nil
}

// Status returns a copy of the status currently loaded in memory.
func (rw *remoteUnit) Status() *StatusFileData {
	status := rw.UnredactedStatus()
	ed, ok := status.ExtraData.(*RemoteExtraData)
	if ok {
		keysToDelete := make([]string, 0)
		for k := range ed.RemoteParams {
			if strings.HasPrefix(strings.ToLower(k), "secret_") {
				keysToDelete = append(keysToDelete, k)
			}
		}
		for i := range keysToDelete {
			delete(ed.RemoteParams, keysToDelete[i])
		}
	}

	return status
}

// UnredactedStatus returns a copy of the status currently loaded in memory, including secrets.
func (rw *remoteUnit) UnredactedStatus() *StatusFileData {
	rw.GetStatusLock().RLock()
	status := rw.GetStatusWithoutExtraData()
	ed, ok := rw.GetStatusCopy().ExtraData.(*RemoteExtraData)
	if ok {
		edCopy := *ed
		edCopy.RemoteParams = make(map[string]string)
		for k, v := range ed.RemoteParams {
			edCopy.RemoteParams[k] = v
		}
		status.ExtraData = &edCopy
	}
	rw.GetStatusLock().RUnlock()

	return status
}

// runAndMonitor waits for a connection to be available, then starts the remote unit and monitors it.
func (rw *remoteUnit) runAndMonitor(mw *utils.JobContext, forRelease bool, action actionFunc) error {
	return rw.getConnectionAndRun(mw, true, func(ctx context.Context, conn net.Conn, reader *bufio.Reader) error {
		err := action(ctx, conn, reader)
		if err != nil {
			mw.WorkerDone()

			return err
		}
		go func() {
			if forRelease {
				err := rw.BaseWorkUnitForWorkUnit.Release(false)
				if err != nil {
					rw.GetWorkceptor().nc.GetLogger().Error("Error releasing unit %s: %s", rw.UnitDir(), err)
				}
			} else {
				rw.monitorRemoteUnit(ctx)
			}
			mw.WorkerDone()
		}()

		return nil
	}, func() {
		mw.WorkerDone()
	})
}

func (rw *remoteUnit) setExpiration(mw *utils.JobContext) {
	red := rw.Status().ExtraData.(*RemoteExtraData)
	dur := time.Until(red.Expiration)
	select {
	case <-mw.Done():
	case <-time.After(dur):
		red := rw.Status().ExtraData.(*RemoteExtraData)
		if !red.RemoteStarted {
			rw.UpdateFullStatus(func(status *StatusFileData) {
				status.Detail = fmt.Sprintf("Work unit expired on %s", red.Expiration.Format("Mon Jan 2 15:04:05"))
				status.State = WorkStateFailed
			})
			mw.Cancel()
		}
	}
}

// startOrRestart is a shared implementation of Start() and Restart().
func (rw *remoteUnit) startOrRestart(start bool) error {
	red := rw.Status().ExtraData.(*RemoteExtraData)
	if start && red.RemoteStarted {
		return fmt.Errorf("unit was already started")
	}
	newJobStarted := rw.topJC.NewJob(rw.GetWorkceptor().ctx, 1, true)
	if !newJobStarted {
		return fmt.Errorf("start or monitor process already running")
	}
	if start || !red.RemoteStarted {
		if !red.Expiration.IsZero() {
			go rw.setExpiration(rw.topJC)
		}

		return rw.runAndMonitor(rw.topJC, false, rw.StartRemoteUnit)
	} else if red.LocalReleased || red.LocalCancelled {
		return rw.runAndMonitor(rw.topJC, red.LocalReleased, func(ctx context.Context, conn net.Conn, reader *bufio.Reader) error {
			return rw.cancelOrReleaseRemoteUnit(ctx, conn, reader, red.LocalReleased)
		})
	}
	go func() {
		rw.monitorRemoteUnit(rw.topJC)
		rw.topJC.WorkerDone()
	}()

	return nil
}

// Start launches a job with given parameters.  If the remote node is unreachable, returns ErrPending
// and continues to retry in the background.
func (rw *remoteUnit) Start() error {
	return rw.startOrRestart(true)
}

// Restart resumes monitoring a job after a Receptor restart.
func (rw *remoteUnit) Restart() error {
	red := rw.Status().ExtraData.(*RemoteExtraData)
	if red.RemoteStarted {
		return rw.startOrRestart(false)
	}

	return fmt.Errorf("remote work had not previously started")
}

// cancelOrRelease is a shared implementation of Cancel() and Release().
func (rw *remoteUnit) cancelOrRelease(release bool, force bool) error {
	// Update the status file that the unit is locally cancelled/released
	var remoteStarted bool
	rw.UpdateFullStatus(func(status *StatusFileData) {
		status.ExtraData.(*RemoteExtraData).LocalCancelled = true
		if release {
			status.ExtraData.(*RemoteExtraData).LocalReleased = true
		}
		remoteStarted = status.ExtraData.(*RemoteExtraData).RemoteStarted
	})
	// if remote work has not started, don't attempt to connect to remote
	if !remoteStarted {
		rw.topJC.Cancel()
		rw.topJC.Wait()
		if release {
			return rw.BaseWorkUnitForWorkUnit.Release(true)
		}
		rw.UpdateBasicStatus(WorkStateFailed, "Locally Cancelled", 0)

		return nil
	}
	if release && force {
		err := rw.connectAndRun(rw.GetWorkceptor().ctx, func(ctx context.Context, conn net.Conn, reader *bufio.Reader) error {
			return rw.cancelOrReleaseRemoteUnit(ctx, conn, reader, true)
		})
		if err != nil {
			rw.GetWorkceptor().nc.GetLogger().Error("Error with connect and run: %s", err)
		}

		return rw.BaseWorkUnitForWorkUnit.Release(true)
	}
	rw.topJC.NewJob(rw.GetWorkceptor().ctx, 1, false)

	return rw.runAndMonitor(rw.topJC, release, func(ctx context.Context, conn net.Conn, reader *bufio.Reader) error {
		return rw.cancelOrReleaseRemoteUnit(ctx, conn, reader, release)
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

func NewRemoteWorker(bwu BaseWorkUnitForWorkUnit, w *Workceptor, unitID, workType string) WorkUnit {
	return newRemoteWorker(bwu, w, unitID, workType)
}

func newRemoteWorker(bwu BaseWorkUnitForWorkUnit, w *Workceptor, unitID, workType string) WorkUnit {
	if bwu == nil {
		bwu = &BaseWorkUnit{}
	}
	rw := &remoteUnit{
		BaseWorkUnitForWorkUnit: bwu,
		logger:                  w.nc.GetLogger(),
	}
	rw.Init(w, unitID, workType, FileSystem{})
	red := &RemoteExtraData{}
	red.RemoteParams = make(map[string]string)
	rw.SetStatusExtraData(red)
	rw.topJC = &utils.JobContext{}

	return rw
}
