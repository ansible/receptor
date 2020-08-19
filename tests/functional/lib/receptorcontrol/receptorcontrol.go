package receptorcontrol

import (
	"errors"
	"fmt"
	"gopkg.in/yaml.v2"
	_ "io/ioutil"
	"net"
	_ "os"
	_ "path/filepath"
	_ "reflect"
	"regexp"
	"strings"
	_ "time"
)

type JsonResponse struct {
	data interface{}
}

type ReceptorControl struct {
	socketConn *net.UnixConn
}

func New() *ReceptorControl {
	return &ReceptorControl{
		socketConn: nil,
	}
}

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

func (r *ReceptorControl) Write(data []byte) (int, error) {
	return r.socketConn.Write(data)
}

func (r *ReceptorControl) ReadStr() (string, error) {
	data, err := r.Read()
	if err != nil {
		return "", err
	}
	return string(data), nil
}

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

func (r *ReceptorControl) Close() error {
	err := r.socketConn.Close()
	r.socketConn = nil
	return err
}

func (r *ReceptorControl) ReadAndParseJson() (map[string]interface{}, error) {
	data, err := r.Read()
	if err != nil {
		return nil, err
	}
	yamlData := make(map[string]interface{})

	err = yaml.Unmarshal(data, &yamlData)
	if err != nil {
		return nil, err
	}
	return yamlData, nil
}

func (r *ReceptorControl) Ping(node string) (map[string]string, error) {
	_, err := r.WriteStr(fmt.Sprintf("ping %s\n", node))
	if err != nil {
		return nil, err
	}
	yamlData, err := r.ReadAndParseJson()
	if err != nil {
		return nil, err
	}
	pingData := make(map[string]string)
	// Convert to map[string]string
	for k, v := range yamlData {
		pingData[k] = v.(string)
	}
	if strings.HasPrefix(pingData["Result"], "Reply") != true {
		return nil, errors.New(pingData["Result"])
	}
	return pingData, nil
}
