package backends

import (
	"fmt"
	"github.com/ghjm/sockceptor/pkg/cmdline"
	"github.com/ghjm/sockceptor/pkg/debug"
	"github.com/ghjm/sockceptor/pkg/netceptor"
	"net"
	"os"
	"sync"
	"syscall"
	"time"
)

//TODO: DTLS?

// UDPMaxPacketLen is the maximum size of a message that can be sent over UDP
const UDPMaxPacketLen = 65507

// UDPDialer implements Backend for outbound UDP
type UDPDialer struct {
	address *net.UDPAddr
	redial  bool
}

// NewUDPDialer instantiates a new UDPDialer backend
func NewUDPDialer(address string, redial bool) (*UDPDialer, error) {
	ua, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}
	nd := UDPDialer{
		address: ua,
		redial:  redial,
	}
	return &nd, nil
}

// Start runs the given session function over this backend service
func (b *UDPDialer) Start(bsf netceptor.BackendSessFunc, errf netceptor.ErrorFunc) {
	go func() {
		for {
			conn, err := net.DialUDP("udp", nil, b.address)
			if b.redial {
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
			}
			if err != nil {
				errf(err, true)
				return
			}
			ns := UDPDialerSession{
				conn: conn,
			}
			err = bsf(&ns)
			if err != nil {
				errf(err, false)
			}
		}
	}()
}

// UDPDialerSession implements BackendSession for UDPDialer
type UDPDialerSession struct {
	conn *net.UDPConn
}

// Send sends data over the session
func (ns *UDPDialerSession) Send(data []byte) error {
	if len(data) > UDPMaxPacketLen {
		return fmt.Errorf("data too large")
	}
	n, err := ns.conn.Write(data)
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
	sessRegLock     sync.RWMutex
	sessionRegistry map[string]*UDPListenerSession
}

// NewUDPListener instantiates a new UDPListener backend
func NewUDPListener(address string) (*UDPListener, error) {
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}
	uc, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	ul := UDPListener{
		laddr:           addr,
		conn:            uc,
		sessChan:        make(chan *UDPListenerSession),
		sessRegLock:     sync.RWMutex{},
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
			if err != nil {
				errf(err, true)
				return
			}
			addrStr := addr.String()
			b.sessRegLock.RLock()
			sess, ok := b.sessionRegistry[addrStr]
			b.sessRegLock.RUnlock()
			if !ok {
				b.sessRegLock.Lock()
				sess = &UDPListenerSession{
					li:       b,
					raddr:    addr,
					recvChan: make(chan []byte),
				}
				b.sessionRegistry[addrStr] = sess
				b.sessRegLock.Unlock()
				go func() {
					err := bsf(sess)
					if err != nil {
						errf(err, false)
					}
				}()
			}
			sess.recvChan <- data
		}
	}()
}

// UDPListenerSession implements BackendSession for UDPListener
type UDPListenerSession struct {
	li       *UDPListener
	raddr    *net.UDPAddr
	recvChan chan []byte
}

// Send sends data over the session
func (ns *UDPListenerSession) Send(data []byte) error {
	n, err := ns.li.conn.WriteToUDP(data, ns.raddr)
	if err != nil {
		return err
	} else if n != len(data) {
		return fmt.Errorf("partial data sent")
	}
	return nil
}

// Recv receives data from the session
func (ns *UDPListenerSession) Recv() ([]byte, error) {
	data := <-ns.recvChan
	return data, nil
}

// Close closes the session
func (ns *UDPListenerSession) Close() error {
	ns.li.sessRegLock.Lock()
	defer ns.li.sessRegLock.Unlock()
	delete(ns.li.sessionRegistry, ns.raddr.String())
	return nil
}

// **************************************************************************
// Command line
// **************************************************************************

// UDPListenerCfg is the cmdline configuration object for a UDP listener
type UDPListenerCfg struct {
	BindAddr string `description:"Local address to bind to" default:"0.0.0.0"`
	Port     int    `description:"Local UDP port to listen on" barevalue:"yes" required:"yes"`
}

// Run runs the action
func (cfg UDPListenerCfg) Run() error {
	address := fmt.Sprintf("%s:%d", cfg.BindAddr, cfg.Port)
	debug.Printf("Running listener %s\n", address)
	li, err := NewUDPListener(address)
	if err != nil {
		debug.Printf("Error creating listener %s: %s\n", address, err)
		return err
	}
	netceptor.AddBackend()
	netceptor.MainInstance.RunBackend(li, func(err error, fatal bool) {
		fmt.Printf("Error in listener backend: %s\n", err)
		if fatal {
			netceptor.DoneBackend()
		}
	})
	return nil
}

// UDPDialerCfg is the cmdline configuration object for a UDP listener
type UDPDialerCfg struct {
	Address string `description:"Host:Port to connect to" barevalue:"yes" required:"yes"`
	Redial  bool   `description:"Keep redialing on lost connection" default:"true"`
}

// Run runs the action
func (cfg UDPDialerCfg) Run() error {
	debug.Printf("Running UDP peer connection %s\n", cfg.Address)
	li, err := NewUDPDialer(cfg.Address, cfg.Redial)
	if err != nil {
		debug.Printf("Error creating peer %s: %s\n", cfg.Address, err)
		return err
	}
	netceptor.AddBackend()
	netceptor.MainInstance.RunBackend(li, func(err error, fatal bool) {
		fmt.Printf("Error in peer connection backend: %s\n", err)
		if fatal {
			netceptor.DoneBackend()
		}
	})
	return nil
}

func init() {
	cmdline.AddConfigType("UDP-listener", "Run a backend listener on a UDP port", UDPListenerCfg{}, false, backendSection)
	cmdline.AddConfigType("UDP-peer", "Make an outbound backend connection to a UDP peer", UDPDialerCfg{}, false, backendSection)
}
