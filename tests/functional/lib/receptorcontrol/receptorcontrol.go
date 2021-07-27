package receptorcontrol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/project-receptor/receptor/pkg/netceptor"
	"github.com/project-receptor/receptor/pkg/workceptor"
	"github.com/project-receptor/receptor/tests/functional/lib/utils"
)

// ReceptorControl Connects to a control socket and provides basic commands
type ReceptorControl struct {
	socketConn     *net.UnixConn
	socketFilename string
}

// New Returns an empty ReceptorControl
func New() *ReceptorControl {
	return &ReceptorControl{
		socketConn:     nil,
		socketFilename: "",
	}
}

// Connect connects to the socket at the specified filename and checks the
// handshake with the control service
func (r *ReceptorControl) Connect(filename string) error {
	if r.socketConn != nil {
		return errors.New("Tried to connect to a socket after already being connected to a socket")
	}
	addr, err := net.ResolveUnixAddr("unix", filename)
	if err != nil {
		return err
	}
	r.socketConn, err = net.DialUnix("unix", nil, addr)
	if err != nil {
		return err
	}
	err = r.handshake()
	if err != nil {
		return err
	}
	r.socketFilename = filename
	return nil
}

// Reconnect to unix socket
func (r *ReceptorControl) Reconnect() error {
	if r.socketFilename != "" {
		err := r.Connect(r.socketFilename)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("Could not reconnect, no socketFilename")
	}
	return nil
}

// Read reads a line from the socket
func (r *ReceptorControl) Read() ([]byte, error) {
	dataBytes := make([]byte, 0)
	buf := make([]byte, 1)
	for {
		n, err := r.socketConn.Read(buf)
		if err != nil {
			return dataBytes, err
		}
		if n == 1 {
			if buf[0] == '\n' {
				break
			}
			dataBytes = append(dataBytes, buf[0])
		}
	}
	return dataBytes, nil
}

// Write writes some data to the socket
func (r *ReceptorControl) Write(data []byte) (int, error) {
	return r.socketConn.Write(data)
}

// ReadStr reads some data from the socket and converts it to a string
func (r *ReceptorControl) ReadStr() (string, error) {
	data, err := r.Read()
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteStr writes string data to the socket
func (r *ReceptorControl) WriteStr(data string) (int, error) {
	return r.Write([]byte(data))
}

func (r *ReceptorControl) handshake() error {
	data, err := r.Read()
	if err != nil {
		return err
	}
	matched, err := regexp.Match("^Receptor Control, node (.+)$", data)
	if err != nil {
		return err
	}
	if matched != true {
		return fmt.Errorf("Failed control socket handshake, got: %s", data)
	}
	return nil
}

// Close closes the connection to the socket
func (r *ReceptorControl) Close() error {
	err := r.socketConn.Close()
	r.socketConn = nil
	return err
}

// CloseWrite closes the write side of the socket
func (r *ReceptorControl) CloseWrite() error {
	err := r.socketConn.CloseWrite()
	return err
}

// ReadAndParseJSON reads data from the socket and parses it as json
func (r *ReceptorControl) ReadAndParseJSON() (map[string]interface{}, error) {
	data, err := r.Read()
	if err != nil {
		return nil, err
	}
	str := string(data)
	if strings.HasPrefix(str, "ERROR") {
		return nil, fmt.Errorf(str)
	}
	jsonData := make(map[string]interface{})
	err = json.Unmarshal(data, &jsonData)
	if err != nil {
		return nil, err
	}
	return jsonData, nil
}

// Ping pings the specified node
func (r *ReceptorControl) Ping(node string) (string, error) {
	_, err := r.WriteStr(fmt.Sprintf("ping %s\n", node))
	if err != nil {
		return "", err
	}
	jsonData, err := r.ReadAndParseJSON()
	if err != nil {
		return "", err
	}
	success := jsonData["Success"].(bool)
	if !success {
		return "", errors.New(jsonData["Error"].(string))
	}
	return fmt.Sprintf("Reply from %s in %s", jsonData["From"].(string), jsonData["TimeStr"].(string)), nil
}

// Reload reloads the current node
func (r *ReceptorControl) Reload() error {
	_, err := r.WriteStr("reload \n")
	if err != nil {
		return err
	}
	jsonData, err := r.ReadAndParseJSON()
	if err != nil {
		return err
	}
	success := jsonData["Success"].(bool)
	if !success {
		return errors.New("Error")
	}
	return nil
}

// Status retrieves the status of the current node
func (r *ReceptorControl) Status() (*netceptor.Status, error) {
	_, err := r.WriteStr("status\n")
	if err != nil {
		return nil, err
	}
	data, err := r.Read()
	if err != nil {
		return nil, err
	}
	status := netceptor.Status{}
	err = json.Unmarshal(data, &status)

	if err != nil {
		return nil, err
	}

	return &status, nil
}

func (r *ReceptorControl) getWorkSubmitResponse() (string, error) {
	_, err := r.ReadStr() // flush response
	if err != nil {
		return "", err
	}
	err = r.CloseWrite() // close write half to signal EOF
	if err != nil {
		return "", err
	}
	response, err := r.ReadAndParseJSON()
	if err != nil {
		return "", err
	}
	err = r.Close() // since write is closed, we should close the whole socket
	if err != nil {
		return "", err
	}
	err = r.Reconnect()
	if err != nil {
		return "", err
	}
	unitID := fmt.Sprintf("%v", response["unitid"])
	return unitID, nil
}

// WorkSubmitJSON begins work on remote node via JSON command
func (r *ReceptorControl) WorkSubmitJSON(command string) (string, error) {
	_, err := r.WriteStr(fmt.Sprintf("%s\n", command))
	if err != nil {
		return "", err
	}
	unitID, err := r.getWorkSubmitResponse()
	if err != nil {
		return "", err
	}
	return unitID, nil
}

// WorkSubmit begins work on remote node
func (r *ReceptorControl) WorkSubmit(node, workType string) (string, error) {
	_, err := r.WriteStr(fmt.Sprintf("work submit %s %s\n", node, workType))
	if err != nil {
		return "", err
	}
	unitID, err := r.getWorkSubmitResponse()
	if err != nil {
		return "", err
	}
	return unitID, nil
}

// WorkStart begins work on local node
func (r *ReceptorControl) WorkStart(workType string) (string, error) {
	return r.WorkSubmit("localhost", workType)
}

// WorkCancel cancels work
func (r *ReceptorControl) WorkCancel(unitID string) (map[string]interface{}, error) {
	_, err := r.WriteStr(fmt.Sprintf("work cancel %s\n", unitID))
	if err != nil {
		return nil, err
	}
	return r.ReadAndParseJSON()
}

// WorkRelease cancels and deletes work
func (r *ReceptorControl) WorkRelease(unitID string) (map[string]interface{}, error) {
	_, err := r.WriteStr(fmt.Sprintf("work release %s\n", unitID))
	if err != nil {
		return nil, err
	}
	return r.ReadAndParseJSON()
}

// GetWorkStatus returns JSON of status file for a given unitID
func (r *ReceptorControl) GetWorkStatus(unitID string) (*workceptor.StatusFileData, error) {
	status := &workceptor.StatusFileData{}
	_, err := r.WriteStr(fmt.Sprintf("work status %s\n", unitID))
	if err != nil {
		return status, err
	}
	jsonData, err := r.Read()
	if err != nil {
		return status, err
	}
	err = json.Unmarshal(jsonData, status)
	if err != nil {
		return status, err
	}
	return status, nil
}

func (r *ReceptorControl) getWorkList() (map[string]interface{}, error) {
	_, err := r.WriteStr(fmt.Sprintf("work list\n"))
	if err != nil {
		return nil, err
	}
	workList, err := r.ReadAndParseJSON()
	if err != nil {
		return nil, err
	}

	return workList, nil
}

func assertWithTimeout(ctx context.Context, check func() bool) bool {
	return utils.CheckUntilTimeout(ctx, 500*time.Millisecond, check)
}

func (r *ReceptorControl) assertWorkState(ctx context.Context, unitID string, state int) bool {
	check := func() bool {
		workStatus, _ := r.GetWorkStatus(unitID)
		return workStatus.State == state
	}
	return assertWithTimeout(ctx, check)
}

//AssertWorkRunning waits until work status is running
func (r *ReceptorControl) AssertWorkRunning(ctx context.Context, unitID string) error {
	if !r.assertWorkState(ctx, unitID, workceptor.WorkStateRunning) {
		return fmt.Errorf("Failed to assert %s is running or ctx timed out", unitID)
	}
	return nil
}

// AssertWorkPending waits until status is pending
func (r *ReceptorControl) AssertWorkPending(ctx context.Context, unitID string) error {
	if !r.assertWorkState(ctx, unitID, workceptor.WorkStatePending) {
		return fmt.Errorf("Failed to assert %s is pending or ctx timed out", unitID)
	}
	return nil
}

// AssertWorkSucceeded waits until status is successful
func (r *ReceptorControl) AssertWorkSucceeded(ctx context.Context, unitID string) error {
	if !r.assertWorkState(ctx, unitID, workceptor.WorkStateSucceeded) {
		return fmt.Errorf("Failed to assert %s succeeded or ctx timed out", unitID)
	}
	return nil
}

// AssertWorkFailed waits until status is failed
func (r *ReceptorControl) AssertWorkFailed(ctx context.Context, unitID string) error {
	if !r.assertWorkState(ctx, unitID, workceptor.WorkStateFailed) {
		return fmt.Errorf("Failed to assert %s failed or ctx timed out", unitID)
	}
	return nil
}

//AssertWorkCancelled waits until work status is cancelled
func (r *ReceptorControl) AssertWorkCancelled(ctx context.Context, unitID string) error {
	check := func() bool {
		workStatus, err := r.GetWorkStatus(unitID)
		if err != nil {
			return false
		}
		if workStatus.State != workceptor.WorkStateFailed {
			return false
		}
		detail := workStatus.Detail
		detailLc := strings.ToLower(detail)
		keywords := []string{
			"cancel",
			"kill",
			"terminate",
			"stop",
		}
		for kwIdx := range keywords {
			if strings.Contains(detailLc, keywords[kwIdx]) {
				return true
			}
		}
		return false
	}
	if !assertWithTimeout(ctx, check) {
		return fmt.Errorf("Failed to assert %s is cancelled or ctx timed out", unitID)
	}
	return nil
}

// AssertWorkTimedOut asserts that work failed
func (r *ReceptorControl) AssertWorkTimedOut(ctx context.Context, unitID string) error {
	check := func() bool {
		workStatus, err := r.GetWorkStatus(unitID)
		if err != nil {
			return false
		}
		if workStatus.State != workceptor.WorkStateFailed {
			return false
		}
		detail := workStatus.Detail
		if strings.HasPrefix(detail, "Work unit expired on") {
			return true
		}
		return false
	}
	if !assertWithTimeout(ctx, check) {
		fmt.Errorf("Failed to assert work timed out or ctx timed out")
	}
	return nil
}

// AssertWorkReleased asserts that work is not in work list
func (r *ReceptorControl) AssertWorkReleased(ctx context.Context, unitID string) error {
	check := func() bool {
		workList, err := r.getWorkList()
		if err != nil {
			return false
		}
		_, ok := workList[unitID] // unitID should not be in list
		if ok {
			return false
		}
		return true
	}
	if !assertWithTimeout(ctx, check) {
		return fmt.Errorf("Failed to assert %s is released or ctx timed out", unitID)
	}

	return nil
}

func (r *ReceptorControl) getWorkResults(unitID string, readSize int) ([]byte, error) {
	_, err := r.WriteStr(fmt.Sprintf("work results %s\n", unitID))
	if err != nil {
		return nil, err
	}
	str, err := r.ReadStr()
	if err != nil {
		return nil, err
	}
	if str[:5] == "ERROR" {
		return nil, fmt.Errorf("remote error: %s", str)
	}
	buf := make([]byte, readSize)
	n, err := r.socketConn.Read(buf)
	if err != nil {
		return nil, err
	}
	if n != readSize {
		return nil, fmt.Errorf("did not read correct size")
	}
	err = r.Close()
	if err != nil {
		return nil, err
	}
	err = r.Reconnect()
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// AssertWorkResults makes sure results match expected byte array
func (r *ReceptorControl) AssertWorkResults(unitID string, expectedResults []byte) error {
	workResults, err := r.getWorkResults(unitID, len(expectedResults))
	if err != nil {
		return err
	}
	if string(expectedResults) != string(workResults) {
		return fmt.Errorf("work results did not match expected results")
	}
	return nil
}
