//go:build !no_controlsvc
// +build !no_controlsvc

package controlsvc

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/utils"
	"github.com/ghjm/cmdline"
)

const (
	normalCloseError         = "normal close"
	writeControlServiceError = "Write error in control service"
)

type Copier interface {
	Copy(dst io.Writer, src io.Reader) (written int64, err error)
}

type SocketConnIO struct{}

func (s *SocketConnIO) Copy(dst io.Writer, src io.Reader) (written int64, err error) {
	return io.Copy(dst, src)
}

type NetceptorForControlsvc interface {
	ListenAndAdvertise(service string, tlscfg *tls.Config, tags map[string]string) (*netceptor.Listener, error)
	NetceptorForControlCommand
}

type Utiler interface {
	BridgeConns(c1 io.ReadWriteCloser, c1Name string, c2 io.ReadWriteCloser, c2Name string, logger *logger.ReceptorLogger)
	UnixSocketListen(filename string, permissions fs.FileMode) (net.Listener, *utils.FLock, error)
}

type Util struct{}

func (u *Util) BridgeConns(c1 io.ReadWriteCloser, c1Name string, c2 io.ReadWriteCloser, c2Name string, logger *logger.ReceptorLogger) {
	utils.BridgeConns(c1, c1Name, c2, c2Name, logger)
}

func (u *Util) UnixSocketListen(filename string, permissions fs.FileMode) (net.Listener, *utils.FLock, error) {
	return utils.UnixSocketListen(filename, permissions)
}

type Neter interface {
	Listen(network string, address string) (net.Listener, error)
}

type Net struct{}

func (n *Net) Listen(network string, address string) (net.Listener, error) {
	return net.Listen(network, address)
}

type Tlser interface {
	NewListener(inner net.Listener, config *tls.Config) net.Listener
}

type TLS struct{}

func (t *TLS) NewListener(inner net.Listener, config *tls.Config) net.Listener {
	return tls.NewListener(inner, config)
}

// SockControl implements the ControlFuncOperations interface that is passed back to control functions.
type SockControl struct {
	conn net.Conn
}

func NewSockControl(conn net.Conn) *SockControl {
	return &SockControl{
		conn: conn,
	}
}

func (s *SockControl) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

// WriteMessage attempts to write a message to a connection.
func (s *SockControl) WriteMessage(message string) error {
	if message != "" {
		_, err := s.conn.Write([]byte(message))
		if err != nil {
			return err
		}
	}

	return nil
}

// BridgeConn bridges the socket to another socket.
func (s *SockControl) BridgeConn(message string, bc io.ReadWriteCloser, bcName string, logger *logger.ReceptorLogger, utils Utiler) error {
	if err := s.WriteMessage(message); err != nil {
		return err
	}
	utils.BridgeConns(s.conn, "control service", bc, bcName, logger)

	return nil
}

// ReadFromConn copies from the socket to an io.Writer, until EOF.
func (s *SockControl) ReadFromConn(message string, out io.Writer, io Copier) error {
	if err := s.WriteMessage(message); err != nil {
		return err
	}
	if _, err := io.Copy(out, s.conn); err != nil {
		return err
	}

	return nil
}

// WriteToConn writes an initial string, and then messages to a channel, to the connection.
func (s *SockControl) WriteToConn(message string, in chan []byte) error {
	if err := s.WriteMessage(message); err != nil {
		return err
	}
	for bytes := range in {
		_, err := s.conn.Write(bytes)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *SockControl) Close() error {
	return s.conn.Close()
}

// Server is an instance of a control service.
type Server struct {
	nc              NetceptorForControlsvc
	controlFuncLock sync.RWMutex
	controlTypes    map[string]ControlCommandType
	serverUtils     Utiler
	serverNet       Neter
	serverTLS       Tlser
}

// New returns a new instance of a control service.
func New(stdServices bool, nc NetceptorForControlsvc) *Server {
	s := &Server{
		nc:              nc,
		controlFuncLock: sync.RWMutex{},
		controlTypes:    make(map[string]ControlCommandType),
		serverUtils:     &Util{},
		serverNet:       &Net{},
		serverTLS:       &TLS{},
	}
	if stdServices {
		s.controlTypes["ping"] = &PingCommandType{}
		s.controlTypes["status"] = &StatusCommandType{}
		s.controlTypes["connect"] = &ConnectCommandType{}
		s.controlTypes["traceroute"] = &TracerouteCommandType{}
		s.controlTypes["reload"] = &ReloadCommandType{}
	}

	return s
}

func (s *Server) SetServerUtils(u Utiler) {
	s.serverUtils = u
}

func (s *Server) SetServerNet(n Neter) {
	s.serverNet = n
}

func (s *Server) SetServerTLS(t Tlser) {
	s.serverTLS = t
}

// MainInstance is the global instance of the control service instantiated by the command-line main() function.
var MainInstance *Server

// AddControlFunc registers a function that can be used from a control socket.
func (s *Server) AddControlFunc(name string, cType ControlCommandType) error {
	s.controlFuncLock.Lock()
	defer s.controlFuncLock.Unlock()

	if _, ok := s.controlTypes[name]; ok {
		return fmt.Errorf("control function named %s already exists", name)
	}
	s.controlTypes[name] = cType

	return nil
}

func errorNormal(nc NetceptorForControlsvc, logMessage string, err error) bool {
	if err == nil {
		return false
	}
	if !strings.HasSuffix(err.Error(), normalCloseError) {
		nc.GetLogger().Error("%s: %s\n", logMessage, err)
	}

	return true
}

func writeToConnWithLog(conn net.Conn, nc NetceptorForControlsvc, writeMessage string, logMessage string) bool {
	_, err := conn.Write([]byte(writeMessage))

	return errorNormal(nc, logMessage, err)
}

// RunControlSession runs the server protocol on the given connection.
func (s *Server) RunControlSession(conn net.Conn) {
	s.nc.GetLogger().Debug("Client connected to control service %s\n", conn.RemoteAddr().String())
	defer func() {
		s.nc.GetLogger().Debug("Client disconnected from control service %s\n", conn.RemoteAddr().String())
		if conn != nil {
			err := conn.Close()
			if err != nil {
				s.nc.GetLogger().Warning("Could not close connection: %s\n", err)
			}
		}
	}()

	writeMsg := fmt.Sprintf("Receptor Control, node %s\n", s.nc.NodeID())
	logMsg := "Could not write in control service"
	if writeToConnWithLog(conn, s.nc, writeMsg, logMsg) {
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
				s.nc.GetLogger().Debug("Control service closed\n")
				done = true

				break
			} else if err != nil {
				if !strings.HasSuffix(err.Error(), normalCloseError) {
					s.nc.GetLogger().Warning("Could not read in control service: %s\n", err)
				}

				return
			}
			if n == 1 {
				if buf[0] == '\r' {
					continue
				} else if buf[0] == '\n' {
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
			err := json.Unmarshal(cmdBytes, &jsonData)
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
				writeMsg := fmt.Sprintf("ERROR: %s\n", err)
				if writeToConnWithLog(conn, s.nc, writeMsg, writeControlServiceError) {
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
			cfo := NewSockControl(conn)

			var cfr map[string]interface{}
			var cc ControlCommand
			var err error
			if jsonData == nil {
				cc, err = ct.InitFromString(params)
			} else {
				cc, err = ct.InitFromJSON(jsonData)
			}
			if err == nil {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				cfr, err = cc.ControlFunc(ctx, s.nc, cfo)
			}
			if err != nil {
				errorNormal(s.nc, "", err)

				writeMsg := fmt.Sprintf("ERROR: %s\n", err)
				if writeToConnWithLog(conn, s.nc, writeMsg, writeControlServiceError) {
					return
				}
			} else if cfr != nil {
				rbytes, err := json.Marshal(cfr)
				if err != nil {
					writeMsg := fmt.Sprintf("ERROR: could not convert response to JSON: %s\n", err)
					if writeToConnWithLog(conn, s.nc, writeMsg, writeControlServiceError) {
						return
					}
				}
				rbytes = append(rbytes, '\n')
				writeMsg := string(rbytes)
				if writeToConnWithLog(conn, s.nc, writeMsg, writeControlServiceError) {
					return
				}
			}
		} else {
			writeMsg := "ERROR: Unknown command\n"
			if writeToConnWithLog(conn, s.nc, writeMsg, writeControlServiceError) {
				return
			}
		}
	}
}

func (s *Server) ConnectionListener(ctx context.Context, listener net.Listener) {
	for {
		if ctx.Err() != nil {
			return
		}
		conn, err := listener.Accept()
		if err != nil {
			if !strings.HasSuffix(err.Error(), normalCloseError) {
				s.nc.GetLogger().Error("Error accepting connection: %s\n", err)
			}

			continue
		}
		go s.SetupConnection(conn)
	}
}

func (s *Server) SetupConnection(conn net.Conn) {
	defer conn.Close()
	tlsConn, ok := conn.(*tls.Conn)
	if ok {
		// Explicitly run server TLS handshake so we can deal with timeout and errors here
		err := conn.SetDeadline(time.Now().Add(10 * time.Second))
		if err != nil {
			s.nc.GetLogger().Error("Error setting timeout: %s. Closing socket.\n", err)

			return
		}
		err = tlsConn.Handshake()
		if err != nil {
			s.nc.GetLogger().Error("TLS handshake error: %s. Closing socket.\n", err)

			return
		}
		err = conn.SetDeadline(time.Time{})
		if err != nil {
			s.nc.GetLogger().Error("Error clearing timeout: %s. Closing socket.\n", err)

			return
		}
	}
	s.RunControlSession(conn)
}

// RunControlSvc runs the main accept loop of the control service.
func (s *Server) RunControlSvc(ctx context.Context, service string, tlscfg *tls.Config,
	unixSocket string, unixSocketPermissions os.FileMode, tcpListen string, tcptls *tls.Config,
) error {
	var uli net.Listener
	var lock *utils.FLock
	var err error
	if unixSocket != "" {
		uli, lock, err = s.serverUtils.UnixSocketListen(unixSocket, unixSocketPermissions)
		if err != nil {
			return fmt.Errorf("error opening Unix socket: %s", err)
		}
	} else {
		uli = nil
	}
	var tli net.Listener
	if tcpListen != "" {
		var listenAddr string
		if strings.Contains(tcpListen, ":") {
			listenAddr = tcpListen
		} else {
			listenAddr = fmt.Sprintf("0.0.0.0:%s", tcpListen)
		}
		tli, err = s.serverNet.Listen("tcp", listenAddr)
		if err != nil {
			return fmt.Errorf("error listening on TCP socket: %s", err)
		}
		if tcptls != nil {
			tli = s.serverTLS.NewListener(tli, tcptls)
		}
	} else {
		tli = nil
	}
	var li *netceptor.Listener
	if service != "" {
		li, err = s.nc.ListenAndAdvertise(service, tlscfg, map[string]string{
			"type": "Control Service",
		})
		if err != nil {
			return fmt.Errorf("error opening Unix socket: %s", err)
		}
	} else {
		li = nil
	}
	if uli == nil && tli == nil && li == nil {
		return fmt.Errorf("no listeners specified")
	}
	s.nc.GetLogger().Info("Running control service %s\n", service)
	go func() {
		<-ctx.Done()
		if uli != nil {
			_ = uli.Close()
			_ = lock.Unlock()
		}
		if li != nil {
			_ = li.Close()
		}
		if tli != nil {
			_ = tli.Close()
		}
	}()
	for _, listener := range []net.Listener{uli, tli, li} {
		if listener == nil || reflect.ValueOf(listener).IsNil() {
			continue
		}
		go s.ConnectionListener(ctx, listener)
	}

	return nil
}

// **************************************************************************
// Command line
// **************************************************************************

// cmdlineConfigWindows is the cmdline configuration object for a control service on Windows.
type cmdlineConfigWindows struct {
	Service   string `description:"Receptor service name to listen on" default:"control"`
	TLS       string `description:"Name of TLS server config for the Receptor listener"`
	TCPListen string `description:"Local TCP port or host:port to bind to the control service"`
	TCPTLS    string `description:"Name of TLS server config for the TCP listener"`
}

// cmdlineConfigUnix is the cmdline configuration object for a control service on Unix.
type CmdlineConfigUnix struct {
	Service     string `description:"Receptor service name to listen on" default:"control"`
	Filename    string `description:"Specifies the filename of a local Unix socket to bind to the service."`
	Permissions int    `description:"Socket file permissions" default:"0600"`
	TLS         string `description:"Name of TLS server config for the Receptor listener"`
	TCPListen   string `description:"Local TCP port or host:port to bind to the control service"`
	TCPTLS      string `description:"Name of TLS server config for the TCP listener"`
}

// Run runs the action.
func (cfg CmdlineConfigUnix) Run() error {
	if cfg.TLS != "" && cfg.TCPListen != "" && cfg.TCPTLS == "" {
		netceptor.MainInstance.Logger.Warning("Control service %s has TLS configured on the Receptor listener but not the TCP listener.", cfg.Service)
	}
	tlscfg, err := netceptor.MainInstance.GetServerTLSConfig(cfg.TLS)
	if err != nil {
		return err
	}
	var tcptls *tls.Config
	if cfg.TCPListen != "" {
		tcptls, err = netceptor.MainInstance.GetServerTLSConfig(cfg.TCPTLS)
		if err != nil {
			return err
		}
	}
	err = MainInstance.RunControlSvc(context.Background(), cfg.Service, tlscfg, cfg.Filename,
		os.FileMode(cfg.Permissions), cfg.TCPListen, tcptls)
	if err != nil {
		return err
	}

	return nil
}

// Run runs the action.
func (cfg cmdlineConfigWindows) Run() error {
	return CmdlineConfigUnix{
		Service:   cfg.Service,
		TLS:       cfg.TLS,
		TCPListen: cfg.TCPListen,
		TCPTLS:    cfg.TCPTLS,
	}.Run()
}

func init() {
	if runtime.GOOS == "windows" {
		cmdline.RegisterConfigTypeForApp("receptor-control-service",
			"control-service", "Runs a control service", cmdlineConfigWindows{})
	} else {
		cmdline.RegisterConfigTypeForApp("receptor-control-service",
			"control-service", "Runs a control service", CmdlineConfigUnix{})
	}
}
