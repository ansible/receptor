package backends

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/project-receptor/receptor/pkg/cmdline"
	"github.com/project-receptor/receptor/pkg/framer"
	"github.com/project-receptor/receptor/pkg/logger"
	"github.com/project-receptor/receptor/pkg/netceptor"
	"net"
	"time"
)

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
func (b *TCPDialer) Start(ctx context.Context) (chan netceptor.BackendSession, error) {
	return dialerSession(ctx, b.redial, 5*time.Second,
		func(closeChan chan struct{}) (netceptor.BackendSession, error) {
			var conn net.Conn
			var err error
			if b.tls == nil {
				conn, err = net.Dial("tcp", b.address)
			} else {
				conn, err = tls.Dial("tcp", b.address, b.tls)
			}
			if err != nil {
				return nil, err
			}
			return newTCPSession(conn, closeChan), nil
		})
}

// TCPListener implements Backend for inbound TCP
type TCPListener struct {
	address string
	tls     *tls.Config
	li      net.Listener
}

// NewTCPListener instantiates a new TCPListener backend
func NewTCPListener(address string, tls *tls.Config) (*TCPListener, error) {
	tl := TCPListener{
		address: address,
		tls:     tls,
		li:      nil,
	}
	return &tl, nil
}

// Addr returns the network address the listener is listening on
func (b *TCPListener) Addr() net.Addr {
	if b.li == nil {
		return nil
	}
	return b.li.Addr()
}

// Start runs the given session function over the WebsocketListener backend
func (b *TCPListener) Start(ctx context.Context) (chan netceptor.BackendSession, error) {
	sessChan, err := listenerSession(ctx,
		func() error {
			var err error
			b.li, err = net.Listen("tcp", b.address)
			if err != nil {
				return err
			}
			if b.tls != nil {
				b.li = tls.NewListener(b.li, b.tls)
			}
			return nil
		}, func() (netceptor.BackendSession, error) {
			c, err := b.li.Accept()
			if err != nil {
				return nil, err
			}
			return newTCPSession(c, nil), nil
		}, func() {
			_ = b.li.Close()
		})
	if err == nil {
		logger.Debug("Listening on %s\n", b.address)
	}
	return sessChan, err
}

// TCPSession implements BackendSession for TCP backend
type TCPSession struct {
	conn      net.Conn
	framer    framer.Framer
	closeChan chan struct{}
}

// newTCPSession allocates a new TCPSession
func newTCPSession(conn net.Conn, closeChan chan struct{}) *TCPSession {
	ts := &TCPSession{
		conn:      conn,
		framer:    framer.New(),
		closeChan: closeChan,
	}
	return ts
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
	if ns.closeChan != nil {
		close(ns.closeChan)
		ns.closeChan = nil
	}
	return ns.conn.Close()
}

// **************************************************************************
// Command line
// **************************************************************************

// TCPListenerCfg is the cmdline configuration object for a TCP listener
type TCPListenerCfg struct {
	BindAddr string  `description:"Local address to bind to" default:"0.0.0.0"`
	Port     int     `description:"Local TCP port to listen on" barevalue:"yes" required:"yes"`
	TLS      string  `description:"Name of TLS server config"`
	Cost     float64 `description:"Connection cost (weight)" default:"1.0"`
}

// Prepare verifies the parameters are correct
func (cfg TCPListenerCfg) Prepare() error {
	if cfg.Cost <= 0.0 {
		return fmt.Errorf("connection cost must be positive")
	}
	return nil
}

// Run runs the action
func (cfg TCPListenerCfg) Run() error {
	address := fmt.Sprintf("%s:%d", cfg.BindAddr, cfg.Port)
	logger.Debug("Running TCP listener on %s\n", address)
	tlscfg, err := netceptor.GetServerTLSConfig(cfg.TLS)
	if err != nil {
		return err
	}
	b, err := NewTCPListener(address, tlscfg)
	if err != nil {
		logger.Error("Error creating listener %s: %s\n", address, err)
		return err
	}
	err = netceptor.MainInstance.AddBackend(b, cfg.Cost)
	if err != nil {
		return err
	}
	return nil
}

// TCPDialerCfg is the cmdline configuration object for a TCP dialer
type TCPDialerCfg struct {
	Address string  `description:"Remote address (Host:Port) to connect to" barevalue:"yes" required:"yes"`
	Redial  bool    `description:"Keep redialing on lost connection" default:"true"`
	TLS     string  `description:"Name of TLS client config"`
	Cost    float64 `description:"Connection cost (weight)" default:"1.0"`
}

// Prepare verifies the parameters are correct
func (cfg TCPDialerCfg) Prepare() error {
	if cfg.Cost <= 0.0 {
		return fmt.Errorf("connection cost must be positive")
	}
	return nil
}

// Run runs the action
func (cfg TCPDialerCfg) Run() error {
	logger.Debug("Running TCP peer connection %s\n", cfg.Address)
	host, _, err := net.SplitHostPort(cfg.Address)
	if err != nil {
		return err
	}
	tlscfg, err := netceptor.GetClientTLSConfig(cfg.TLS, host)
	if err != nil {
		return err
	}
	b, err := NewTCPDialer(cfg.Address, cfg.Redial, tlscfg)
	if err != nil {
		logger.Error("Error creating peer %s: %s\n", cfg.Address, err)
		return err
	}
	err = netceptor.MainInstance.AddBackend(b, cfg.Cost)
	if err != nil {
		return err
	}
	return nil
}

func init() {
	cmdline.AddConfigType("tcp-listener", "Run a backend listener on a TCP port", TCPListenerCfg{}, false, false, false, backendSection)
	cmdline.AddConfigType("tcp-peer", "Make an outbound backend connection to a TCP peer", TCPDialerCfg{}, false, false, false, backendSection)
}
