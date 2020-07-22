package receptorcontrol

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/project-receptor/receptor/pkg/netceptor"
	"net"
	"regexp"
	"strings"
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
