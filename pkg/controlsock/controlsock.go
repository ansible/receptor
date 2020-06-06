package controlsock

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/ghjm/sockceptor/pkg/cmdline"
	"github.com/ghjm/sockceptor/pkg/debug"
	"github.com/ghjm/sockceptor/pkg/netceptor"
	"github.com/juju/fslock"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// Sock is a limited view into the control socket
type Sock interface {
	Printf(format string, a ...interface{}) error
	Writer() (io.Writer, error)
	ReadLine() (string, error)
	PrintError(printToSock bool, format string, a ...interface{})
}

type sock struct {
	conn   net.Conn
	reader *bufio.Reader
}

// Printf prints formatted text to the control socket
func (s *sock) Printf(format string, a ...interface{}) error {
	_, err := s.conn.Write([]byte(fmt.Sprintf(format, a...)))
	return err
}

// Writer gets an io.writer for sending output to the socket
func (s *sock) Writer() (io.Writer, error) {
	return io.Writer(s.conn), nil
}

// ReadLine reads a line of text from the control socket
func (s *sock) ReadLine() (string, error) {
	str, err := s.reader.ReadString('\n')
	return strings.TrimRight(str, "\n"), err
}

// Error deals with an error, optionally printing it to the socket
func (s *sock) PrintError(printToSock bool, format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	debug.Printf("Error in control socket session: %s\n", msg)
	if printToSock {
		err := s.Printf(msg)
		if err != nil {
			debug.Printf("Write error in control socket: %s\n", err)
		}
	}
}

// ControlFunc is a function called when a control socket command is executed
type ControlFunc func(Sock, string) error

// Server is an instance of a controlsock server
type Server struct {
	nc              *netceptor.Netceptor
	controlFuncLock sync.RWMutex
	controlFuncs    map[string]ControlFunc
}

// NewServer returns a new instance of a controlsock server.  The Netceptor
// reference is only needed if stdServices is true (it is used for ping).
func NewServer(stdServices bool, nc *netceptor.Netceptor) *Server {
	s := &Server{
		nc:              nc,
		controlFuncLock: sync.RWMutex{},
		controlFuncs:    make(map[string]ControlFunc),
	}
	if stdServices {
		s.controlFuncs["ping"] = s.controlPing
		s.controlFuncs["status"] = s.controlStatus
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

func (s *Server) pingReplyHandler(cs Sock, pc *netceptor.PacketConn, startTime time.Time) {
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
			err = cs.Printf("Timeout waiting for ping reply\n")
			if err != nil {
				cs.PrintError(false, "Write error in control socket: %s\n", err)
			}
		}
		return
	}
	err = cs.Printf("Reply from %s in %s\n", addr.String(), time.Since(startTime))
	if err != nil {
		cs.PrintError(false, "Write error in control socket: %s\n", err)
	}
}

func (s *Server) controlPing(cs Sock, params string) error {
	pc, err := s.nc.ListenPacket("")
	if err != nil {
		return err
	}
	go s.pingReplyHandler(cs, pc, time.Now())
	_, err = pc.WriteTo([]byte{}, netceptor.NewAddr(params, "ping"))
	if err != nil {
		cs.PrintError(true, "Error sending ping: %s\n", err)
	}
	return nil
}

func (s *Server) controlStatus(cs Sock, params string) error {
	status := netceptor.MainInstance.Status()
	bytes, err := json.Marshal(status)
	if err != nil {
		cs.PrintError(true, "JSON error marshaling status: %s\n", err)
		return nil
	}
	err = cs.Printf("%s\n", bytes)
	if err != nil {
		return err
	}
	return nil
}

// RunSockServer runs the server protocol on the given connection
func (s *Server) RunSockServer(conn net.Conn) {
	debug.Printf("Client connected to control socket\n")
	cs := &sock{
		conn:   conn,
		reader: bufio.NewReader(conn),
	}
	done := false
	defer func() {
		err := conn.Close()
		if err != nil {
			debug.Printf("Error closing connection: %s\n", err)
		}
	}()
	for !done {
		cmd, err := cs.ReadLine()
		if err == io.EOF {
			debug.Printf("Control socket closed\n")
			done = true
		} else if err != nil {
			debug.Printf("Read error in control socket: %s\n", err)
			return
		}
		if len(cmd) == 0 {
			continue
		}
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
				err := cf(cs, params)
				if err != nil {
					debug.Printf("Error in control socket %s command: %s\n", cmd, err)
					return
				}
			} else {
				err = cs.Printf("Unknown command\n")
				if err != nil {
					debug.Printf("Write error in control socket: %s\n", err)
					return
				}
			}
		}
	}
}

// CmdlineConfig is the cmdline configuration object for a control socket
type CmdlineConfig struct {
	Path string `required:"yes" barevalue:"yes" description:"Path to the socket file"`
}

// Run runs the action
func (cfg CmdlineConfig) Run() error {
	lock := fslock.New(cfg.Path + ".lock")
	err := lock.TryLock()
	if err != nil {
		return fmt.Errorf("could not acquire lock on control socket: %s", err)
	}
	err = os.RemoveAll(cfg.Path)
	if err != nil {
		return err
	}
	li, err := net.Listen("unix", cfg.Path)
	if err != nil {
		return err
	}
	debug.Printf("Running control socket on %s\n", cfg.Path)
	go func() {
		for {
			conn, err := li.Accept()
			if err != nil {
				debug.Printf("Error accepting socket connection: %s. Closing socket.\n", err)
				return
			}
			go MainInstance().RunSockServer(conn)
		}
	}()
	return nil
}

func init() {
	cmdline.AddConfigType("controlsock", "Accept control commands through a Unix socket", CmdlineConfig{}, false, nil)
}
