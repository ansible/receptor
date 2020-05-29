package backends

//TODO: configurable reconnect

import (
	"fmt"
	"github.org/ghjm/sockceptor/pkg/debug"
	"github.org/ghjm/sockceptor/pkg/netceptor"
	"net"
	"os"
	"sync"
	"syscall"
	"time"
)

// UDPDialer implements Backend for outbound UDP
type UDPDialer struct {
	address *net.UDPAddr
}

// NewUDPDialer instantiates a new UDPDialer backend
func NewUDPDialer(address string) (*UDPDialer, error) {
	ua, err := net.ResolveUDPAddr("udp", address); if err != nil {
		return nil, err
	}
	nd := UDPDialer{
		address: ua,
	}
	return &nd, nil
}

// Start runs the given session function over this backend service
func (b *UDPDialer) Start(bsf netceptor.BackendSessFunc, errf netceptor.ErrorFunc) {
	go func() {
		for {
			conn, err := net.DialUDP("udp", nil, b.address)
			if err == nil {
				ns := UDPDialerSession{
					conn: conn,
				}
				err = bsf(&ns)
			}
			operr, ok := err.(*net.OpError)
			if ok {
				syserr, ok := operr.Err.(*os.SyscallError)
				if ok {
					if syserr.Err == syscall.ECONNREFUSED {
						// If the other end isn't listening, just keep trying
						time.Sleep(5 * time.Second)
						continue
					}
				}
			}
			errf(err)
			return
		}
	}()
}

// UDPDialerSession implements BackendSession for UDPDialer
type UDPDialerSession struct {
	conn *net.UDPConn
}

// Send sends data over the session
func (ns *UDPDialerSession) Send(data []byte) error {
	n, err := ns.conn.Write(data)
	debug.Tracef("UDP sent data %s len %d sent %d err %s\n", data, len(data), n, err)
	if err != nil {
		return err
	}
	if n != len(data) {
		return fmt.Errorf("partial data sent")
	}
	return nil
}

// Recv receives data via the session
func (ns *UDPDialerSession) Recv() ([]byte, error) {
	buf := make([]byte, netceptor.MTU)
	n, err := ns.conn.Read(buf)
	debug.Tracef("UDP sending data %s len %d sent %d err %s\n", buf, len(buf), n, err)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

// Close closes the session
func (ns *UDPDialerSession) Close() error {
	return ns.conn.Close()
}

// UDPListener implements Backend for inbound UDP
type UDPListener struct {
	laddr           *net.UDPAddr
	conn            *net.UDPConn
	sessChan        chan *UDPListenerSession
	sessRegLock		sync.Mutex
	sessionRegistry map[string]*UDPListenerSession
}

// NewUDPListener instantiates a new UDPListener backend
func NewUDPListener(address string) (*UDPListener, error) {
	addr, err := net.ResolveUDPAddr("udp", address); if err != nil {
		return nil, err
	}
	uc, err := net.ListenUDP("udp", addr); if err != nil {
		return nil, err
	}
	ul := UDPListener{
		laddr:           addr,
		conn:            uc,
		sessChan:        make(chan *UDPListenerSession),
		sessRegLock:	 sync.Mutex{},
		sessionRegistry: make(map[string]*UDPListenerSession),
	}
	return &ul, nil
}

// Start runs the given session function over the UDPListener backend
func (b *UDPListener) Start(bsf netceptor.BackendSessFunc, errf netceptor.ErrorFunc) {
	go func() {
		buf := make([]byte, netceptor.MTU)
		for {
			n, addr, err := b.conn.ReadFromUDP(buf)
			data := make([]byte, n)
			copy(data, buf)
			debug.Tracef("UDP received data %s len %d err %s\n", data, n, err)
			if err != nil {
				errf(err)
				return
			}
			addrStr := addr.String()
			b.sessRegLock.Lock()
			sess, ok := b.sessionRegistry[addrStr]
			if !ok {
				debug.Printf("Creating new UDP listener session for %s\n", addrStr)
				sess = &UDPListenerSession{
					li:       b,
					raddr:    addr,
					recvChan: make(chan []byte),
				}
				b.sessionRegistry[addrStr] = sess
				b.sessRegLock.Unlock()
				go func () {
					err := bsf(sess); if err != nil {
						errf(err)
					}
				}()
			} else {
				b.sessRegLock.Unlock()
			}
			sess.recvChan <- data
		}
	}()
}

// UDPListenerSession implements BackendSession for UDPListener
type UDPListenerSession struct {
	li *UDPListener
	raddr *net.UDPAddr
	recvChan chan []byte
}

// Send sends data over the session
func (ns *UDPListenerSession) Send(data []byte) error{
	n, err := ns.li.conn.WriteToUDP(data, ns.raddr)
	debug.Tracef("UDP sent data %s len %d sent %d err %s\n", data, len(data), n, err)
	if err != nil {
		return err
	} else if n != len(data) {
		return fmt.Errorf("partial data sent")
	}
	return nil
}

// Recv receives data from the session
func (ns *UDPListenerSession) Recv() ([]byte, error) {
	data := <- ns.recvChan
	return data, nil
}

// Close closes the session
func (ns *UDPListenerSession) Close() error {
	ns.li.sessRegLock.Lock()
	defer ns.li.sessRegLock.Unlock()
	delete(ns.li.sessionRegistry, ns.raddr.String())
	return nil
}
