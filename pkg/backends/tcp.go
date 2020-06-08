package backends

import (
	"crypto/tls"
	"fmt"
	"github.com/ghjm/sockceptor/pkg/cmdline"
	"github.com/ghjm/sockceptor/pkg/debug"
	"github.com/ghjm/sockceptor/pkg/framer"
	"github.com/ghjm/sockceptor/pkg/netceptor"
	"net"
	"strconv"
	"time"
)

//TODO: TLS

// TCPDialer implements Backend for outbound TCP
type TCPDialer struct {
	address string
	redial  bool
	tls     *tls.Config
}

// NewTCPDialer instantiates a new TCP backend
func NewTCPDialer(address string, redial bool, tls *tls.Config) (*TCPDialer, error) {
	td := TCPDialer{
		address: address,
		redial:  redial,
		tls:     tls,
	}
	return &td, nil
}

// Start runs the given session function over this backend service
func (b *TCPDialer) Start(bsf netceptor.BackendSessFunc, errf netceptor.ErrorFunc) {
	go func() {
		for {
			var conn net.Conn
			var err error
			if b.tls == nil {
				conn, err = net.Dial("tcp", b.address)
			} else {
				conn, err = tls.Dial("tcp", b.address, b.tls)
			}
			if err != nil {
				if b.redial {
					errf(err, false)
					time.Sleep(5 * time.Second)
					continue
				}
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
	tls     *tls.Config
}

// NewTCPListener instantiates a new TCPListener backend
func NewTCPListener(address string, tls *tls.Config) (*TCPListener, error) {
	tl := TCPListener{
		address: address,
		tls:     tls,
	}
	return &tl, nil
}

// Start runs the given session function over the WebsocketListener backend
func (b *TCPListener) Start(bsf netceptor.BackendSessFunc, errf netceptor.ErrorFunc) {
	li, err := net.Listen("tcp", b.address)
	if err != nil {
		errf(err, true)
		return
	}
	if b.tls != nil {
		li = tls.NewListener(li, b.tls)
	}
	go func() {
		for {
			c, err := li.Accept()
			if err != nil {
				errf(err, true)
				return
			}
			go func() {
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

// TCPSession implements BackendSession for TCP backend
type TCPSession struct {
	conn   net.Conn
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
		n, err := ns.conn.Read(buf)
		if err != nil {
			return nil, err
		}
		ns.framer.RecvData(buf[:n])
	}
	buf, err := ns.framer.GetMessage()
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// Close closes the session
func (ns *TCPSession) Close() error {
	return ns.conn.Close()
}

// **************************************************************************
// Command line
// **************************************************************************

// TCPListenerCfg is the cmdline configuration object for a TCP listener
type TCPListenerCfg struct {
	BindAddr string `description:"Local address to bind to" default:"0.0.0.0"`
	Port     int    `description:"Local TCP port to listen on" barevalue:"yes" required:"yes"`
	TLS      string `description:"Name of TLS server config"`
}

// Run runs the action
func (cfg TCPListenerCfg) Run() error {
	address := fmt.Sprintf("%s:%d", cfg.BindAddr, cfg.Port)
	debug.Printf("Running TCP listener on %s\n", address)
	tlscfg, err := netceptor.GetServerTLSConfig(cfg.TLS)
	if err != nil {
		return err
	}
	li, err := NewTCPListener(address, tlscfg)
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

// TCPDialerCfg is the cmdline configuration object for a TCP dialer
type TCPDialerCfg struct {
	Address string `description:"Remote address (Host:Port) to connect to" barevalue:"yes" required:"yes"`
	Redial  string `description:"Keep redialing on lost connection" default:"true"`
	TLS     string `description:"Name of TLS client config"`
}

// Run runs the action
func (cfg TCPDialerCfg) Run() error {
	debug.Printf("Running TCP peer connection %s\n", cfg.Address)
	redial, _ := strconv.ParseBool(cfg.Redial)
	host, _, err := net.SplitHostPort(cfg.Address)
	if err != nil {
		return err
	}
	tlscfg, err := netceptor.GetClientTLSConfig(cfg.TLS, host)
	if err != nil {
		return err
	}
	li, err := NewTCPDialer(cfg.Address, redial, tlscfg)
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
	cmdline.AddConfigType("tcp-listener", "Run a backend listener on a TCP port", TCPListenerCfg{}, false, backendSection)
	cmdline.AddConfigType("tcp-peer", "Make an outbound backend connection to a TCP peer", TCPDialerCfg{}, false, backendSection)
}
