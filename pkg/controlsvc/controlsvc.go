package controlsvc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/ghjm/sockceptor/pkg/cmdline"
	"github.com/ghjm/sockceptor/pkg/debug"
	"github.com/ghjm/sockceptor/pkg/netceptor"
	"github.com/ghjm/sockceptor/pkg/services"
	"github.com/ghjm/sockceptor/pkg/sockutils"
	"io"
	"net"
	"runtime"
	"strings"
	"sync"
	"time"
)

type sock struct {
	conn   net.Conn
	reader *bufio.Reader
}

// Printf prints formatted text to a socket
func Printf(sock net.Conn, format string, a ...interface{}) error {
	_, err := sock.Write([]byte(fmt.Sprintf(format, a...)))
	return err
}

// PrintError deals with an error, optionally printing it to the socket
func PrintError(sock net.Conn, printToSock bool, format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	debug.Printf("Error in control socket session: %s\n", msg)
	if printToSock {
		err := Printf(sock, msg)
		if err != nil {
			debug.Printf("Write error in control socket: %s\n", err)
		}
	}
}

// ControlFunc is a function called when a control service command is called
type ControlFunc func(net.Conn, string) error

// Server is an instance of a control service
type Server struct {
	nc              *netceptor.Netceptor
	controlFuncLock sync.RWMutex
	controlFuncs    map[string]ControlFunc
}

// NewServer returns a new instance of a control service.
func NewServer(stdServices bool, nc *netceptor.Netceptor) *Server {
	s := &Server{
		nc:              nc,
		controlFuncLock: sync.RWMutex{},
		controlFuncs:    make(map[string]ControlFunc),
	}
	if stdServices {
		s.controlFuncs["ping"] = s.controlPing
		s.controlFuncs["status"] = s.controlStatus
		s.controlFuncs["connect"] = s.controlConnect
	}
	return s
}

var mainInstance *Server

// MainInstance returns a global singleton instance of Server
func MainInstance() *Server {
	if mainInstance == nil {
		mainInstance = NewServer(true, netceptor.MainInstance)
	}
	return mainInstance
}

// AddControlFunc registers a function that can be used from a control socket.
func (s *Server) AddControlFunc(name string, cFunc ControlFunc) error {
	s.controlFuncLock.Lock()
	defer s.controlFuncLock.Unlock()
	_, ok := s.controlFuncs[name]
	if ok {
		return fmt.Errorf("control function named %s already exists", name)
	}
	s.controlFuncs[name] = cFunc
	return nil
}

func (s *Server) pingReplyHandler(conn net.Conn, pc *netceptor.PacketConn, startTime time.Time) {
	defer func() {
		err := pc.Close()
		if err != nil {
			debug.Printf("Error closing Netceptor connection\n")
		}
	}()
	buf := make([]byte, 8)
	_ = pc.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, addr, err := pc.ReadFrom(buf)
	if err != nil {
		nerr, ok := err.(net.Error)
		if ok && nerr.Timeout() {
			err = Printf(conn, "Timeout waiting for ping reply\n")
			if err != nil {
				PrintError(conn, false, "Write error in control socket: %s\n", err)
			}
		}
		return
	}
	err = Printf(conn, "Reply from %s in %s\n", addr.String(), time.Since(startTime))
	if err != nil {
		PrintError(conn, false, "Write error in control socket: %s\n", err)
	}
}

func (s *Server) controlPing(conn net.Conn, params string) error {
	pc, err := s.nc.ListenPacket("")
	if err != nil {
		return err
	}
	go s.pingReplyHandler(conn, pc, time.Now())
	_, err = pc.WriteTo([]byte{}, netceptor.NewAddr(params, "ping"))
	if err != nil {
		PrintError(conn, true, "Error sending ping: %s\n", err)
	}
	return nil
}

func (s *Server) controlStatus(conn net.Conn, params string) error {
	status := netceptor.MainInstance.Status()
	bytes, err := json.Marshal(status)
	if err != nil {
		PrintError(conn, true, "JSON error marshaling status: %s\n", err)
		return nil
	}
	err = Printf(conn, "%s\n", bytes)
	if err != nil {
		return err
	}
	return nil
}

func (s *Server) controlConnect(conn net.Conn, params string) error {
	tokens := strings.Split(params, " ")
	if len(tokens) != 2 {
		PrintError(conn, true, "Syntax: connect <node> <service>\n")
		return nil
	}
	rc, err := s.nc.Dial(tokens[0], tokens[1])
	if err != nil {
		PrintError(conn, true, "Error connecting to node: %s\n", err)
		return nil
	}
	sockutils.BridgeConns(conn, rc)
	return nil
}

// RunControlSession runs the server protocol on the given connection
func (s *Server) RunControlSession(conn net.Conn) {
	debug.Printf("Client connected to control service\n")
	defer func() {
		err := conn.Close()
		if err != nil {
			debug.Printf("Error closing connection: %s\n", err)
		}
	}()
	err := Printf(conn, "Receptor Control, node %s\n", s.nc.NodeID())
	if err != nil {
		debug.Printf("Write error in control service: %s\n", err)
		return
	}
	done := false
	reader := bufio.NewReader(conn)
	for !done {
		cmd, err := reader.ReadString('\n')
		if err == io.EOF {
			debug.Printf("Control service closed\n")
			done = true
		} else if err != nil {
			debug.Printf("Read error in control service: %s\n", err)
			return
		}
		if len(cmd) == 0 {
			continue
		}
		cmd = strings.TrimRight(cmd, "\n")
		tokens := strings.SplitN(cmd, " ", 2)
		if len(tokens) > 0 {
			cmd = strings.ToLower(tokens[0])
			params := ""
			if len(tokens) > 1 {
				params = tokens[1]
			}
			s.controlFuncLock.RLock()
			var cf ControlFunc
			for f := range s.controlFuncs {
				if f == cmd {
					cf = s.controlFuncs[f]
					break
				}
			}
			s.controlFuncLock.RUnlock()
			if cf != nil {
				err := cf(conn, params)
				if err != nil {
					debug.Printf("Error in control service %s command: %s\n", cmd, err)
					return
				}
			} else {
				err = Printf(conn, "Unknown command\n")
				if err != nil {
					debug.Printf("Write error in control service: %s\n", err)
					return
				}
			}
		}
	}
}

// RunControlSvc runs the main accept loop of the control service
func (s *Server) RunControlSvc(service string) error {
	li, err := s.nc.ListenAndAdvertise(service, nil)
	if err != nil {
		return err
	}
	debug.Printf("Running control service %s\n", service)
	go func() {
		for {
			conn, err := li.Accept()
			if err != nil {
				debug.Printf("Error accepting connection: %s. Closing socket.\n", err)
				return
			}
			go MainInstance().RunControlSession(conn)
		}
	}()
	return nil
}

// **************************************************************************
// Command line
// **************************************************************************

// CmdlineConfigWindows is the cmdline configuration object for a control service on Windows
type CmdlineConfigWindows struct {
	Service string `description:"Receptor service name to listen on" default:"control"`
}

// CmdlineConfigUnix is the cmdline configuration object for a control service on Unix
type CmdlineConfigUnix struct {
	Service  string `description:"Receptor service name to listen on" default:"control"`
	Filename string `description:"Filename of local Unix socket to bind to the service"`
}

// Run runs the action
func (cfg CmdlineConfigUnix) Run() error {
	nc := netceptor.MainInstance
	s := NewServer(true, nc)
	err := s.RunControlSvc(cfg.Service)
	if err != nil {
		return err
	}
	if cfg.Filename != "" {
		go services.UnixProxyServiceInbound(nc, cfg.Filename, nc.NodeID(), cfg.Service)
	}
	return nil
}

// Run runs the action
func (cfg CmdlineConfigWindows) Run() error {
	return CmdlineConfigUnix{
		Service:  cfg.Service,
		Filename: "",
	}.Run()
}

func init() {
	if runtime.GOOS == "windows" {
		cmdline.AddConfigType("control-service", "Run a control service",
			CmdlineConfigWindows{}, false, nil)
	} else {
		cmdline.AddConfigType("control-service", "Run a control service",
			CmdlineConfigUnix{}, false, nil)
	}
}
