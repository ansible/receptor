package backends

import (
	"fmt"
	"github.com/ghjm/sockceptor/pkg/debug"
	"github.com/ghjm/sockceptor/pkg/framer"
	"github.com/ghjm/sockceptor/pkg/netceptor"
	"net"
	"os"
	"syscall"
	"time"
)

//TODO: TLS
//TODO: configurable reconnect

// TCPDialer implements Backend for outbound TCP
type TCPDialer struct {
	address string
}

// NewTCPDialer instantiates a new TCP backend
func NewTCPDialer(address string) (*TCPDialer, error) {
	td := TCPDialer{
		address: address,
	}
	return &td, nil
}

// Start runs the given session function over this backend service
func (b *TCPDialer) Start(bsf netceptor.BackendSessFunc, errf netceptor.ErrorFunc) {
	go func() {
		for {
			conn, err := net.Dial("tcp", b.address)
			operr, ok := err.(*net.OpError)
			if ok {
				syserr, ok := operr.Err.(*os.SyscallError)
				if ok {
					if syserr.Err == syscall.ECONNREFUSED {
						errf(err, false)
						time.Sleep(5 * time.Second)
						continue
					}
				}
			}
			if err != nil {
				errf(err, true)
				return
			}
			ns := newTCPSession(conn)
			err = bsf(ns)
			if err != nil {
				errf(err, false)
			}
		}
	}()
}

// TCPListener implements Backend for inbound TCP
type TCPListener struct {
	address string
}

// NewTCPListener instantiates a new TCPListener backend
func NewTCPListener(address string) (*TCPListener, error) {
	tl := TCPListener{
		address: address,
	}
	return &tl, nil
}

// Start runs the given session function over the WebsocketListener backend
func (b *TCPListener) Start(bsf netceptor.BackendSessFunc, errf netceptor.ErrorFunc) {
	debug.Printf("listening\n")
	li, err := net.Listen("tcp", b.address); if err != nil {
		errf(err, true)
		return
	}
	go func() {
		for {
			debug.Printf("accepting\n")
			c, err := li.Accept();
			if err != nil {
				errf(err, true)
				return
			}
			go func() {
				debug.Printf("running a session\n")
				sess := newTCPSession(c)
				err = bsf(sess)
				if err != nil {
					errf(err, false)
				}
			}()
		}
	}()
	debug.Printf("Listening on %s\n", b.address)
}

// TCPSession implements BackendSession for TCP backent
type TCPSession struct {
	conn net.Conn
	framer framer.Framer
}

func newTCPSession(conn net.Conn) *TCPSession {
	ws := &TCPSession{
		conn:   conn,
		framer: framer.New(),
	}
	return ws
}

// Send sends data over the session
func (ns *TCPSession) Send(data []byte) error {
	buf := ns.framer.SendData(data)
	n, err := ns.conn.Write(buf)
	debug.Tracef("Websocket sent data %s len %d sent %d err %s\n", data, len(data), n, err)
	if err != nil {
		return err
	}
	if n != len(buf) {
		return fmt.Errorf("partial data sent")
	}
	return nil
}

// Recv receives data via the session
func (ns *TCPSession) Recv() ([]byte, error) {
	buf := make([]byte, netceptor.MTU)
	for {
		if ns.framer.MessageReady() {
			break
		}
		n, err := ns.conn.Read(buf); if err != nil {
			return nil, err
		}
		ns.framer.RecvData(buf[:n])
	}
	buf, err := ns.framer.GetMessage(); if err != nil {
		return nil, err
	}
	debug.Tracef("Websocket received data %s len %d\n", buf, len(buf))
	return buf, nil
}

// Close closes the session
func (ns *TCPSession) Close() error {
	return ns.conn.Close()
}
