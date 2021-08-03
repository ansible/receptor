// +build !no_controlsvc

package controlsvc

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/ghjm/cmdline"
	"github.com/project-receptor/receptor/pkg/logger"
	"github.com/project-receptor/receptor/pkg/netceptor"
	"github.com/project-receptor/receptor/pkg/utils"
)

// sockControl implements the ControlFuncOperations interface that is passed back to control functions.
type sockControl struct {
	conn net.Conn
}

// BridgeConn bridges the socket to another socket.
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

// ReadFromConn copies from the socket to an io.Writer, until EOF.
func (s *sockControl) ReadFromConn(message string, out io.Writer) error {
	if message != "" {
		_, err := s.conn.Write([]byte(message))
		if err != nil {
			return err
		}
	}

	if _, err := io.Copy(out, s.conn); err != nil {
		return err
	}

	return nil
}

// WriteToConn writes an initial string, and then messages to a channel, to the connection.
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

// Server is an instance of a control service.
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
		s.controlTypes["traceroute"] = &tracerouteCommandType{}
	}

	return s
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

// RunControlSession runs the server protocol on the given connection.
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
			} else if cfr != nil {
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
		} else {
			_, err = conn.Write([]byte("ERROR: Unknown command\n"))
			if err != nil {
				logger.Error("Write error in control service: %s\n", err)

				return
			}
		}
	}
}

// RunControlSvc runs the main accept loop of the control service.
func (s *Server) RunControlSvc(ctx context.Context, service string, tlscfg *tls.Config,
	unixSocket string, unixSocketPermissions os.FileMode, tcpListen string, tcptls *tls.Config) error {
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
	var tli net.Listener
	if tcpListen != "" {
		var listenAddr string
		if strings.Contains(tcpListen, ":") {
			listenAddr = tcpListen
		} else {
			listenAddr = fmt.Sprintf("0.0.0.0:%s", tcpListen)
		}
		tli, err = net.Listen("tcp", listenAddr)
		if err != nil {
			return fmt.Errorf("error listening on TCP socket: %s", err)
		}
		if tcptls != nil {
			tli = tls.NewListener(tli, tcptls)
		}
	} else {
		tli = nil
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
		if listener != nil {
			go func(listener net.Listener) {
				for {
					conn, err := listener.Accept()
					if ctx.Err() != nil {
						return
					}
					if err != nil {
						logger.Error("Error accepting connection: %s. Closing listener.\n", err)
						_ = listener.Close()

						return
					}
					go func() {
						tlsConn, ok := conn.(*tls.Conn)
						if ok {
							// Explicitly run server TLS handshake so we can deal with timeout and errors here
							err = conn.SetDeadline(time.Now().Add(10 * time.Second))
							if err != nil {
								logger.Error("Error setting timeout: %s. Closing socket.\n", err)
								_ = conn.Close()

								return
							}
							err = tlsConn.Handshake()
							if err != nil {
								logger.Error("TLS handshake error: %s. Closing socket.\n", err)
								_ = conn.Close()

								return
							}
							err = conn.SetDeadline(time.Time{})
							if err != nil {
								logger.Error("Error clearing timeout: %s. Closing socket.\n", err)
								_ = conn.Close()

								return
							}
						}
						s.RunControlSession(conn)
					}()
				}
			}(listener)
		}
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
type cmdlineConfigUnix struct {
	Service     string `description:"Receptor service name to listen on" default:"control"`
	Filename    string `description:"Filename of local Unix socket to bind to the service"`
	Permissions int    `description:"Socket file permissions" default:"0600"`
	TLS         string `description:"Name of TLS server config for the Receptor listener"`
	TCPListen   string `description:"Local TCP port or host:port to bind to the control service"`
	TCPTLS      string `description:"Name of TLS server config for the TCP listener"`
}

// Run runs the action.
func (cfg cmdlineConfigUnix) Run() error {
	if cfg.TLS != "" && cfg.TCPListen != "" && cfg.TCPTLS == "" {
		logger.Warning("Control service %s has TLS configured on the Receptor listener but not the TCP listener.", cfg.Service)
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
	return cmdlineConfigUnix{
		Service:   cfg.Service,
		TLS:       cfg.TLS,
		TCPListen: cfg.TCPListen,
		TCPTLS:    cfg.TCPTLS,
	}.Run()
}

func init() {
	if runtime.GOOS == "windows" {
		cmdline.RegisterConfigTypeForApp("receptor-control-service",
			"control-service", "Run a control service", cmdlineConfigWindows{})
	} else {
		cmdline.RegisterConfigTypeForApp("receptor-control-service",
			"control-service", "Run a control service", cmdlineConfigUnix{})
	}
}
