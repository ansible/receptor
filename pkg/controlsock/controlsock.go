package controlsock

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/ghjm/sockceptor/pkg/cmdline"
	"github.com/ghjm/sockceptor/pkg/debug"
	"github.com/ghjm/sockceptor/pkg/netceptor"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

func connPrintf(conn net.Conn, format string, a ...interface{}) error {
	_, err := conn.Write([]byte(fmt.Sprintf(format, a...)))
	return err
}

var controlFuncs = map[string]func(net.Conn, string) error{
	"ping":   controlPing,
	"status": controlStatus,
}

func pingReplyHandler(conn net.Conn, nc *netceptor.PacketConn) {
	defer func() {
		err := nc.Close()
		if err != nil {
			debug.Printf("Error closing Netceptor connection\n")
		}
	}()
	buf := make([]byte, 8)
	_ = nc.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, addr, err := nc.ReadFrom(buf)
	if err != nil {
		nerr, ok := err.(net.Error)
		if ok && nerr.Timeout() {
			err = connPrintf(conn, "Timeout waiting for ping reply\n")
			if err != nil {
				debug.Printf("Write error in control socket: %s\n", err)
			}
		}
		return
	}
	err = connPrintf(conn, "Ping reply from %s\n", addr.String())
	if err != nil {
		debug.Printf("Write error in control socket: %s\n", err)
	}
}

func controlPing(conn net.Conn, params string) error {
	nc, err := netceptor.MainInstance.ListenPacket("")
	if err != nil {
		return err
	}
	go pingReplyHandler(conn, nc)
	_, err = nc.WriteTo([]byte{}, netceptor.NewAddr(params, "ping"))
	if err != nil {
		msg := fmt.Sprintf("Error sending ping: %s\n", err)
		debug.Printf(msg)
		err = connPrintf(conn, msg)
		if err != nil {
			debug.Printf("Write error in control socket: %s\n", err)
		}
	}
	return nil
}

func controlStatus(conn net.Conn, params string) error {
	status := netceptor.MainInstance.Status()
	bytes, err := json.Marshal(status)
	if err != nil {
		msg := fmt.Sprintf("JSON error marshaling status: %s\n", err)
		debug.Printf(msg)
		err = connPrintf(conn, msg)
		if err != nil {
			return err
		}
	}
	bytes = append(bytes, '\n')
	_, err = conn.Write(bytes)
	if err != nil {
		return err
	}
	return nil
}

func controlSockServer(conn net.Conn) {
	debug.Printf("Client connected to control socket\n")
	reader := bufio.NewReader(conn)
	done := false
	defer func() {
		err := conn.Close()
		if err != nil {
			debug.Printf("Error closing connection: %s\n", err)
		}
	}()
	for !done {
		cmd, err := reader.ReadString('\n')
		if err == io.EOF {
			debug.Printf("Control socket closed\n")
			done = true
		} else if err != nil {
			debug.Printf("Read error in control socket: %s\n", err)
			return
		}
		cmd = strings.TrimRight(cmd, "\n")
		if len(cmd) == 0 {
			continue
		}
		tokens := strings.SplitN(cmd, " ", 2)
		if len(tokens) > 0 {
			cmd = strings.ToLower(tokens[0])
			param := ""
			if len(tokens) > 1 {
				param = tokens[1]
			}
			found := false
			for f := range controlFuncs {
				if f == cmd {
					err := controlFuncs[f](conn, param)
					if err != nil {
						debug.Printf("Error in control socket %s command: %s\n", f, err)
						return
					}
					found = true
					break
				}
			}
			if !found {
				err = connPrintf(conn, "Unknown command\n")
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
	err := os.RemoveAll(cfg.Path)
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
			go controlSockServer(conn)
		}
	}()
	return nil
}

func init() {
	cmdline.AddConfigType("controlsock", "Accept control commands through a Unix socket", CmdlineConfig{}, false)
}
