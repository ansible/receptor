package controlsvc

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/project-receptor/receptor/pkg/cmdline"
	"github.com/project-receptor/receptor/pkg/logger"
	"github.com/project-receptor/receptor/pkg/netceptor"
	"github.com/project-receptor/receptor/pkg/utils"
	"io"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ControlCommandType is a type of command that can be run from the control service
type ControlCommandType interface {
	InitFromString(string) (ControlCommand, error)
	InitFromJSON(map[string]interface{}) (ControlCommand, error)
}

// ControlCommand is an instance of a command that is being run from the control service
type ControlCommand interface {
	ControlFunc(*netceptor.Netceptor, ControlFuncOperations) (map[string]interface{}, error)
}

// ControlFuncOperations provides callbacks for control services to take actions
type ControlFuncOperations interface {
	BridgeConn(message string, bc io.ReadWriteCloser, bcName string) error
	ReadFromConn(message string, out io.Writer) error
	WriteToConn(message string, in chan []byte) error
	Close() error
}

type pingCommandType struct{}
type pingCommand struct {
	target string
}

func (t *pingCommandType) InitFromString(params string) (ControlCommand, error) {
	if params == "" {
		return nil, fmt.Errorf("no ping target")
	}
	c := &pingCommand{
		target: params,
	}
	return c, nil
}

func (t *pingCommandType) InitFromJSON(config map[string]interface{}) (ControlCommand, error) {
	target, ok := config["target"]
	if !ok {
		return nil, fmt.Errorf("no ping target")
	}
	targetStr, ok := target.(string)
	if !ok {
		return nil, fmt.Errorf("ping target must be string")
	}
	c := &pingCommand{
		target: targetStr,
	}
	return c, nil
}

func (c *pingCommand) ControlFunc(nc *netceptor.Netceptor, cfo ControlFuncOperations) (map[string]interface{}, error) {
	pc, err := nc.ListenPacket("")
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = pc.Close()
	}()
	startTime := time.Now()
	replyChan := make(chan net.Addr)
	go func() {
		buf := make([]byte, 8)
		_, addr, err := pc.ReadFrom(buf)
		if err == nil {
			replyChan <- addr
		}
	}()
	_, err = pc.WriteTo([]byte{}, nc.NewAddr(c.target, "ping"))
	if err != nil {
		return nil, err
	}
	cfr := make(map[string]interface{})
	select {
	case addr := <-replyChan:
		cfr["Result"] = fmt.Sprintf("Reply from %s in %s", addr.String(), time.Since(startTime))
	case <-time.After(10 * time.Second):
		cfr["Result"] = "Timeout waiting for ping response"
	}
	return cfr, nil
}

type statusCommandType struct{}
type statusCommand struct{}

func (t *statusCommandType) InitFromString(params string) (ControlCommand, error) {
	if params != "" {
		return nil, fmt.Errorf("status command does not take parameters")
	}
	c := &statusCommand{}
	return c, nil
}

func (t *statusCommandType) InitFromJSON(config map[string]interface{}) (ControlCommand, error) {
	c := &statusCommand{}
	return c, nil
}

func (c *statusCommand) ControlFunc(nc *netceptor.Netceptor, cfo ControlFuncOperations) (map[string]interface{}, error) {
	status := nc.Status()
	cfr := make(map[string]interface{})
	cfr["NodeID"] = status.NodeID
	cfr["Connections"] = status.Connections
	cfr["RoutingTable"] = status.RoutingTable
	cfr["Advertisements"] = status.Advertisements
	cfr["KnownConnectionCosts"] = status.KnownConnectionCosts
	return cfr, nil
}

type connectCommandType struct{}
type connectCommand struct {
	targetNode    string
	targetService string
	tlsConfigName string
}

func (t *connectCommandType) InitFromString(params string) (ControlCommand, error) {
	tokens := strings.Split(params, " ")
	if len(tokens) < 2 {
		return nil, fmt.Errorf("no connect target")
	}
	if len(tokens) > 3 {
		return nil, fmt.Errorf("too many parameters")
	}
	var tlsConfigName string
	if len(tokens) == 3 {
		tlsConfigName = tokens[2]
	}
	c := &connectCommand{
		targetNode:    tokens[0],
		targetService: tokens[1],
		tlsConfigName: tlsConfigName,
	}
	return c, nil
}

func (t *connectCommandType) InitFromJSON(config map[string]interface{}) (ControlCommand, error) {
	targetNode, ok := config["node"]
	if !ok {
		return nil, fmt.Errorf("no connect target node")
	}
	targetNodeStr, ok := targetNode.(string)
	if !ok {
		return nil, fmt.Errorf("connect target node must be string")
	}
	targetService, ok := config["service"]
	if !ok {
		return nil, fmt.Errorf("no connect target service")
	}
	targetServiceStr, ok := targetService.(string)
	if !ok {
		return nil, fmt.Errorf("connect target service must be string")
	}
	var tlsConfigStr string
	tlsConfig, ok := config["tls"]
	if ok {
		tlsConfigStr, ok = tlsConfig.(string)
		if !ok {
			return nil, fmt.Errorf("connect tls name must be string")
		}
	} else {
		tlsConfigStr = ""
	}
	c := &connectCommand{
		targetNode:    targetNodeStr,
		targetService: targetServiceStr,
		tlsConfigName: tlsConfigStr,
	}
	return c, nil
}

func (c *connectCommand) ControlFunc(nc *netceptor.Netceptor, cfo ControlFuncOperations) (map[string]interface{}, error) {
	tlscfg, err := nc.GetClientTLSConfig(c.tlsConfigName, c.targetNode)
	if err != nil {
		return nil, err
	}
	rc, err := nc.Dial(c.targetNode, c.targetService, tlscfg)
	if err != nil {
		return nil, err
	}
	err = cfo.BridgeConn("Connecting\n", rc, "connected service")
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// sockControl implements the ControlFuncOperations interface that is passed back to control functions
type sockControl struct {
	conn net.Conn
}

// BridgeConn bridges the socket to another socket
func (s *sockControl) BridgeConn(message string, bc io.ReadWriteCloser, bcName string) error {
	if message != "" {
		_, err := s.conn.Write([]byte(message))
		if err != nil {
			return err
		}
	}
	utils.BridgeConns(s.conn, "control service", bc, bcName)
	return nil
}

// ReadFromConn copies from the socket to an io.Writer, until EOF
func (s *sockControl) ReadFromConn(message string, out io.Writer) error {
	if message != "" {
		_, err := s.conn.Write([]byte(message))
		if err != nil {
			return err
		}
	}
	_, err := io.Copy(out, s.conn)
	if err != nil {
		return err
	}
	return nil
}

// WriteToConn writes an initial string, and then messages to a channel, to the connection
func (s *sockControl) WriteToConn(message string, in chan []byte) error {
	if message != "" {
		_, err := s.conn.Write([]byte(message))
		if err != nil {
			return err
		}
	}
	for bytes := range in {
		_, err := s.conn.Write(bytes)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *sockControl) Close() error {
	return s.conn.Close()
}

// Server is an instance of a control service
type Server struct {
	nc              *netceptor.Netceptor
	controlFuncLock sync.RWMutex
	controlTypes    map[string]ControlCommandType
}

// New returns a new instance of a control service.
func New(stdServices bool, nc *netceptor.Netceptor) *Server {
	s := &Server{
		nc:              nc,
		controlFuncLock: sync.RWMutex{},
		controlTypes:    make(map[string]ControlCommandType),
	}
	if stdServices {
		s.controlTypes["ping"] = &pingCommandType{}
		s.controlTypes["status"] = &statusCommandType{}
		s.controlTypes["connect"] = &connectCommandType{}
	}
	return s
}

// MainInstance is the global instance of the control service instantiated by the command-line main() function
var MainInstance *Server

// AddControlFunc registers a function that can be used from a control socket.
func (s *Server) AddControlFunc(name string, cType ControlCommandType) error {
	s.controlFuncLock.Lock()
	defer s.controlFuncLock.Unlock()
	_, ok := s.controlTypes[name]
	if ok {
		return fmt.Errorf("control function named %s already exists", name)
	}
	s.controlTypes[name] = cType
	return nil
}

// RunControlSession runs the server protocol on the given connection
func (s *Server) RunControlSession(conn net.Conn) {
	logger.Info("Client connected to control service\n")
	defer func() {
		logger.Info("Client disconnected from control service\n")
		err := conn.Close()
		if err != nil {
			logger.Error("Error closing connection: %s\n", err)
		}
	}()
	_, err := conn.Write([]byte(fmt.Sprintf("Receptor Control, node %s\n", s.nc.NodeID())))
	if err != nil {
		logger.Error("Write error in control service: %s\n", err)
		return
	}
	done := false
	for !done {
		// Inefficiently read one line from the socket - we can't use bufio
		// because we cannot read ahead beyond the newline character
		cmdBytes := make([]byte, 0)
		buf := make([]byte, 1)
		for {
			n, err := conn.Read(buf)
			if err == io.EOF {
				logger.Info("Control service closed\n")
				done = true
				break
			} else if err != nil {
				logger.Error("Read error in control service: %s\n", err)
				return
			}
			if n == 1 {
				if buf[0] == '\n' {
					break
				}
				cmdBytes = append(cmdBytes, buf[0])
			}
		}
		if len(cmdBytes) == 0 {
			continue
		}
		var cmd string
		var params string
		var jsonData map[string]interface{}
		if cmdBytes[0] == '{' {
			err = json.Unmarshal(cmdBytes, &jsonData)
			if err == nil {
				cmdIf, ok := jsonData["command"]
				if ok {
					cmd, ok = cmdIf.(string)
					if !ok {
						err = fmt.Errorf("command must be a string")
					}
				} else {
					err = fmt.Errorf("JSON did not contain a command")
				}
			}
			if err != nil {
				_, err = conn.Write([]byte(fmt.Sprintf("ERROR: %s\n", err)))
				if err != nil {
					logger.Error("Write error in control service: %s\n", err)
					return
				}
			}
		} else {
			tokens := strings.SplitN(string(cmdBytes), " ", 2)
			if len(tokens) > 0 {
				cmd = strings.ToLower(tokens[0])
				if len(tokens) > 1 {
					params = tokens[1]
				}
			}
		}
		s.controlFuncLock.RLock()
		var ct ControlCommandType
		for f := range s.controlTypes {
			if f == cmd {
				ct = s.controlTypes[f]
				break
			}
		}
		s.controlFuncLock.RUnlock()
		if ct != nil {
			cfo := &sockControl{
				conn: conn,
			}
			var cfr map[string]interface{}
			var cc ControlCommand
			if jsonData == nil {
				cc, err = ct.InitFromString(params)
			} else {
				cc, err = ct.InitFromJSON(jsonData)
			}
			if err == nil {
				cfr, err = cc.ControlFunc(s.nc, cfo)
			}
			if err != nil {
				_, err = conn.Write([]byte(fmt.Sprintf("ERROR: %s\n", err)))
				if err != nil {
					logger.Error("Write error in control service: %s\n", err)
					return
				}
			} else {
				if cfr != nil {
					rbytes, err := json.Marshal(cfr)
					if err != nil {
						_, err = conn.Write([]byte(fmt.Sprintf("ERROR: could not convert response to JSON: %s\n", err)))
						if err != nil {
							logger.Error("Write error in control service: %s\n", err)
							return
						}
					}
					rbytes = append(rbytes, '\n')
					_, err = conn.Write(rbytes)
					if err != nil {
						logger.Error("Write error in control service: %s\n", err)
						return
					}
				}
			}
		} else {
			_, err = conn.Write([]byte(fmt.Sprintf("ERROR: Unknown command\n")))
			if err != nil {
				logger.Error("Write error in control service: %s\n", err)
				return
			}
		}
	}
}

// RunControlSvc runs the main accept loop of the control service
func (s *Server) RunControlSvc(ctx context.Context, service string, tlscfg *tls.Config,
	unixSocket string, unixSocketPermissions os.FileMode) error {
	var uli net.Listener
	var lock *utils.FLock
	var err error
	if unixSocket != "" {
		uli, lock, err = utils.UnixSocketListen(unixSocket, unixSocketPermissions)
		if err != nil {
			return fmt.Errorf("error opening Unix socket: %s", err)
		}
	} else {
		uli = nil
	}
	var li *netceptor.Listener
	if service != "" {
		li, err = s.nc.ListenAndAdvertise(service, tlscfg, nil)
		if err != nil {
			return fmt.Errorf("error opening Unix socket: %s", err)
		}
	} else {
		li = nil
	}
	if uli == nil && li == nil {
		return fmt.Errorf("no listeners specified")
	}
	logger.Info("Running control service %s\n", service)
	go func() {
		select {
		case <-ctx.Done():
			if uli != nil {
				_ = uli.Close()
				_ = lock.Unlock()
			}
			if li != nil {
				_ = li.Close()
			}
			return
		}
	}()
	if uli != nil {
		go func() {
			for {
				conn, err := uli.Accept()
				if err != nil {
					logger.Error("Error accepting Unix socket connection: %s. Closing socket.\n", err)
					return
				}
				go s.RunControlSession(conn)
			}
		}()
	}
	if li != nil {
		go func() {
			for {
				conn, err := li.Accept()
				if err != nil {
					logger.Error("Error accepting connection: %s. Closing socket.\n", err)
					return
				}
				go s.RunControlSession(conn)
			}
		}()
	}
	return nil
}

// **************************************************************************
// Command line
// **************************************************************************

// CmdlineConfigWindows is the cmdline configuration object for a control service on Windows
type CmdlineConfigWindows struct {
	Service string `description:"Receptor service name to listen on" default:"control"`
	TLS     string `description:"Name of TLS server config for the Receptor listener"`
}

// CmdlineConfigUnix is the cmdline configuration object for a control service on Unix
type CmdlineConfigUnix struct {
	Service     string `description:"Receptor service name to listen on" default:"control"`
	Filename    string `description:"Filename of local Unix socket to bind to the service"`
	Permissions int    `description:"Socket file permissions" default:"0600"`
	TLS         string `description:"Name of TLS server config for the Receptor listener"`
}

// Run runs the action
func (cfg CmdlineConfigUnix) Run() error {
	tlscfg, err := netceptor.MainInstance.GetServerTLSConfig(cfg.TLS)
	if err != nil {
		return err
	}
	err = MainInstance.RunControlSvc(context.Background(), cfg.Service, tlscfg, cfg.Filename, os.FileMode(cfg.Permissions))
	if err != nil {
		return err
	}
	return nil
}

// Run runs the action
func (cfg CmdlineConfigWindows) Run() error {
	return CmdlineConfigUnix{
		Service: cfg.Service,
		TLS:     cfg.TLS,
	}.Run()
}

func init() {
	if runtime.GOOS == "windows" {
		cmdline.AddConfigType("control-service", "Run a control service",
			CmdlineConfigWindows{}, false, false, false, false, nil)
	} else {
		cmdline.AddConfigType("control-service", "Run a control service",
			CmdlineConfigUnix{}, false, false, false, false, nil)
	}
}
