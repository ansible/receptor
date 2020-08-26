package receptorcontrol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/project-receptor/receptor/pkg/netceptor"
	"net"
	"regexp"
	"strings"
	"time"
)

// ReceptorControl Connects to a control socket and provides basic commands
type ReceptorControl struct {
	socketConn *net.UnixConn
}

// New Returns an empty ReceptorControl
func New() *ReceptorControl {
	return &ReceptorControl{
		socketConn: nil,
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
	jsonData := make(map[string]interface{})

	err = json.Unmarshal(data, &jsonData)
	if err != nil {
		return nil, err
	}
	return jsonData, nil
}

// Ping pings the specified node
func (r *ReceptorControl) Ping(node string) (map[string]string, error) {
	_, err := r.WriteStr(fmt.Sprintf("ping %s\n", node))
	if err != nil {
		return nil, err
	}
	jsonData, err := r.ReadAndParseJSON()
	if err != nil {
		return nil, err
	}
	pingData := make(map[string]string)
	// Convert to map[string]string
	for k, v := range jsonData {
		pingData[k] = v.(string)
	}
	if strings.HasPrefix(pingData["Result"], "Reply") != true {
		return nil, errors.New(pingData["Result"])
	}
	return pingData, nil
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

// WorkSubmit begins work on remote node
func (r *ReceptorControl) WorkSubmit(node, serviceName string) (string, error) {
	_, err := r.WriteStr(fmt.Sprintf("work submit %s %s\n", node, serviceName))
	if err != nil {
		return "", err
	}
	_, err = r.ReadStr() // flush response
	if err != nil {
		return "", err
	}
	r.CloseWrite()
	response, err := r.ReadAndParseJSON()
	if err != nil {
		return "", err
	}
	unitID := fmt.Sprintf("%v", response["unitid"])
	return unitID, nil
}

// WorkStart begins work on local node
func (r *ReceptorControl) WorkStart(workID string) (string, error) {
	_, err := r.WriteStr(fmt.Sprintf("work start %s\n", workID))
	if err != nil {
		return "", err
	}
	_, err = r.ReadStr() // flush response
	if err != nil {
		return "", err
	}
	r.CloseWrite() // clost write half to signal EOF
	response, err := r.ReadAndParseJSON()
	if err != nil {
		return "", err
	}
	r.Close()
	unitID := fmt.Sprintf("%v", response["unitid"])
	return unitID, nil
}

// WorkCancel cancels work
func (r *ReceptorControl) WorkCancel(workID string) error {
	_, err := r.WriteStr(fmt.Sprintf("work cancel %s\n", workID))
	if err != nil {
		return err
	}
	_, err = r.ReadAndParseJSON() // flush response
	if err != nil {
		return err
	}
	return nil
}

// WorkRelease cancels and deletes work
func (r *ReceptorControl) WorkRelease(workID string) error {
	_, err := r.WriteStr(fmt.Sprintf("work release %s\n", workID))
	if err != nil {
		return err
	}
	_, err = r.ReadAndParseJSON() // flush response
	if err != nil {
		return err
	}
	return nil
}

func (r *ReceptorControl) getWorkStatus(workID string) (map[string]interface{}, error) {
	_, err := r.WriteStr(fmt.Sprintf("work status %s\n", workID))
	if err != nil {
		return nil, err
	}
	status, err := r.ReadAndParseJSON()
	if err != nil {
		return nil, err
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

//AssertWorkRunning waits until work status is running
func (r *ReceptorControl) AssertWorkRunning(ctx context.Context, workID string) error {
	for {
		workStatus, err := r.getWorkStatus(workID)
		if err != nil {
			return err
		}
		if workStatus["StateName"] == "Running" {
			break
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("assert %s is running has timed out", workID)
		case <-time.After(250 * time.Millisecond):
		}
	}
	return nil
}

//AssertWorkCancelled waits until work status is cancelled
func (r *ReceptorControl) AssertWorkCancelled(ctx context.Context, workID string) error {
	for {
		workStatus, err := r.getWorkStatus(workID)
		if err != nil {
			return err
		}
		if workStatus["StateName"] == "Failed" && workStatus["Detail"] == "Cancelled" {
			break
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("assert %s is cancelled has timed out", workID)
		case <-time.After(250 * time.Millisecond):
		}
	}
	return nil
}

// AssertWorkReleased asserts that work is not in work list
func (r *ReceptorControl) AssertWorkReleased(workID string) error {
	workList, err := r.getWorkList()
	if err != nil {
		return err
	}
	_, ok := workList[workID] // workID should not be in list
	if ok {
		return fmt.Errorf("assert %s released has failed", workID)
	}
	return nil
}
