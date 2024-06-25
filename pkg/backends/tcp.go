//go:build !no_tcp_backend && !no_backends
// +build !no_tcp_backend,!no_backends

package backends

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/ansible/receptor/pkg/framer"
	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/utils"
	"github.com/ghjm/cmdline"
	"github.com/spf13/viper"
)

// TCPDialer implements Backend for outbound TCP.
type TCPDialer struct {
	address string
	redial  bool
	tls     *tls.Config
	logger  *logger.ReceptorLogger
}

// NewTCPDialer instantiates a new TCP backend.
func NewTCPDialer(address string, redial bool, tls *tls.Config, logger *logger.ReceptorLogger) (*TCPDialer, error) {
	td := TCPDialer{
		address: address,
		redial:  redial,
		tls:     tls,
		logger:  logger,
	}

	return &td, nil
}

func (b *TCPDialer) GetAddr() string {
	return b.address
}

func (b *TCPDialer) GetTLS() *tls.Config {
	return b.tls
}

// Start runs the given session function over this backend service.
func (b *TCPDialer) Start(ctx context.Context, wg *sync.WaitGroup) (chan netceptor.BackendSession, error) {
	return dialerSession(ctx, wg, b.redial, 5*time.Second, b.logger,
		func(closeChan chan struct{}) (netceptor.BackendSession, error) {
			var conn net.Conn
			var err error
			dialer := &net.Dialer{}
			if b.tls == nil {
				conn, err = dialer.DialContext(ctx, "tcp", b.address)
			} else {
				dialer.Timeout = 15 * time.Second // tls library does not have a DialContext equivalent
				conn, err = tls.DialWithDialer(dialer, "tcp", b.address, b.tls)
			}
			if err != nil {
				return nil, err
			}

			return newTCPSession(conn, closeChan), nil
		})
}

// TCPListener implements Backend for inbound TCP.
type TCPListener struct {
	address string
	TLS     *tls.Config
	li      net.Listener
	innerLi *net.TCPListener
	logger  *logger.ReceptorLogger
}

// NewTCPListener instantiates a new TCPListener backend.
func NewTCPListener(address string, tls *tls.Config, logger *logger.ReceptorLogger) (*TCPListener, error) {
	tl := TCPListener{
		address: address,
		TLS:     tls,
		li:      nil,
		logger:  logger,
	}

	return &tl, nil
}

// Addr returns the network address the listener is listening on.
func (b *TCPListener) GetAddr() string {
	return b.li.Addr().String()
}

func (b *TCPListener) GetCost() string {
	return b.li.Addr().String()
}

func (b *TCPListener) GetTLS() *tls.Config {
	return b.TLS
}

// Start runs the given session function over the TCPListener backend.
func (b *TCPListener) Start(ctx context.Context, wg *sync.WaitGroup) (chan netceptor.BackendSession, error) {
	sessChan, err := listenerSession(ctx, wg, b.logger,
		func() error {
			var err error
			lc := net.ListenConfig{}
			li, err := lc.Listen(ctx, "tcp", b.address)
			if err != nil {
				return err
			}
			var ok bool
			tli, ok := li.(*net.TCPListener)
			if !ok {
				return fmt.Errorf("listen returned a non-TCP listener")
			}
			if b.TLS == nil {
				b.li = li
				b.innerLi = tli
			} else {
				tlsLi := tls.NewListener(tli, b.TLS)
				b.li = tlsLi
				b.innerLi = tli
			}

			return nil
		}, func() (netceptor.BackendSession, error) {
			var c net.Conn
			for {
				err := b.innerLi.SetDeadline(time.Now().Add(1 * time.Second))
				if err != nil {
					return nil, err
				}
				c, err = b.li.Accept()
				select {
				case <-ctx.Done():
					return nil, io.EOF
				default:
				}
				if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
					continue
				}
				if err != nil {
					return nil, err
				}

				break
			}

			return newTCPSession(c, nil), nil
		}, func() {
			_ = b.li.Close()
		})
	if err == nil {
		b.logger.Debug("Listening on TCP %s\n", b.GetAddr())
	}

	return sessChan, err
}

// TCPSession implements BackendSession for TCP backend.
type TCPSession struct {
	conn            net.Conn
	framer          framer.Framer
	closeChan       chan struct{}
	closeChanCloser sync.Once
}

// newTCPSession allocates a new TCPSession.
func newTCPSession(conn net.Conn, closeChan chan struct{}) *TCPSession {
	ts := &TCPSession{
		conn:            conn,
		framer:          framer.New(),
		closeChan:       closeChan,
		closeChanCloser: sync.Once{},
	}

	return ts
}

// Send sends data over the session.
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

// Recv receives data via the session.
func (ns *TCPSession) Recv(timeout time.Duration) ([]byte, error) {
	buf := make([]byte, utils.NormalBufferSize)
	for {
		if ns.framer.MessageReady() {
			break
		}
		err := ns.conn.SetReadDeadline(time.Now().Add(timeout))
		if err != nil {
			return nil, err
		}
		n, err := ns.conn.Read(buf)
		if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
			return nil, netceptor.ErrTimeout
		}
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

// Close closes the session.
func (ns *TCPSession) Close() error {
	if ns.closeChan != nil {
		ns.closeChanCloser.Do(func() {
			close(ns.closeChan)
			ns.closeChan = nil
		})
	}

	return ns.conn.Close()
}

// **************************************************************************
// Command line
// **************************************************************************

// TODO make these fields private
// TCPListenerCfg is the cmdline configuration object for a TCP listener.
type TCPListenerCfg struct {
	BindAddr     string             `description:"Local address to bind to" default:"0.0.0.0"`
	Port         int                `description:"Local TCP port to listen on" barevalue:"yes" required:"yes"`
	TLS          string             `description:"Name of TLS server config"`
	Cost         float64            `description:"Connection cost (weight)" default:"1.0"`
	NodeCost     map[string]float64 `description:"Per-node costs"`
	AllowedPeers []string           `description:"Peer node IDs to allow via this connection"`
}

func (cfg TCPListenerCfg) GetCost() float64 {
	return cfg.Cost
}

func (cfg TCPListenerCfg) GetNodeCost() map[string]float64 {
	return cfg.NodeCost
}

func (cfg TCPListenerCfg) GetAddr() string {
	return cfg.BindAddr
}

func (cfg TCPListenerCfg) GetTLS() string {
	return cfg.TLS
}

// Prepare verifies the parameters are correct.
func (cfg TCPListenerCfg) Prepare() error {
	if cfg.Cost <= 0.0 {
		return fmt.Errorf("connection cost must be positive")
	}
	for node, cost := range cfg.NodeCost {
		if cost <= 0.0 {
			return fmt.Errorf("connection cost must be positive for %s", node)
		}
	}

	return nil
}

// Run runs the action.
func (cfg TCPListenerCfg) Run() error {
	if cfg.Cost == 0 {
		cfg.Cost = 1.0
	}

	if cfg.BindAddr == "" {
		cfg.BindAddr = "0.0.0.0"
	}

	address := fmt.Sprintf("%s:%d", cfg.BindAddr, cfg.Port)
	tlscfg, err := netceptor.MainInstance.GetServerTLSConfig(cfg.TLS)
	if err != nil {
		return err
	}
	b, err := NewTCPListener(address, tlscfg, netceptor.MainInstance.Logger)
	if err != nil {
		netceptor.MainInstance.Logger.Error("Error creating listener %s: %s\n", address, err)

		return err
	}
	err = netceptor.MainInstance.AddBackend(b,
		netceptor.BackendConnectionCost(cfg.Cost),
		netceptor.BackendNodeCost(cfg.NodeCost),
		netceptor.BackendAllowedPeers(cfg.AllowedPeers))
	if err != nil {
		return err
	}

	return nil
}

// TODO make these fields private
// TCPDialerCfg is the cmdline configuration object for a TCP dialer.
type TCPDialerCfg struct {
	Address      string   `description:"Remote address (Host:Port) to connect to" barevalue:"yes" required:"yes"`
	Redial       bool     `description:"Keep redialing on lost connection" default:"true"`
	TLS          string   `description:"Name of TLS client config"`
	Cost         float64  `description:"Connection cost (weight)" default:"1.0"`
	AllowedPeers []string `description:"Peer node IDs to allow via this connection"`
}

// Prepare verifies the parameters are correct.
func (cfg TCPDialerCfg) Prepare() error {
	if cfg.Cost <= 0.0 {
		return fmt.Errorf("connection cost must be positive")
	}

	return nil
}

// Run runs the action.
func (cfg TCPDialerCfg) Run() error {
	// FIX ME!
	if cfg.Cost == 0 {
		cfg.Cost = 1.0
	}

	netceptor.MainInstance.Logger.Debug("Running TCP peer connection %s\n", cfg.Address)
	host, _, err := net.SplitHostPort(cfg.Address)
	if err != nil {
		return err
	}
	tlscfg, err := netceptor.MainInstance.GetClientTLSConfig(cfg.TLS, host, netceptor.ExpectedHostnameTypeDNS)
	if err != nil {
		return err
	}
	b, err := NewTCPDialer(cfg.Address, cfg.Redial, tlscfg, netceptor.MainInstance.Logger)
	if err != nil {
		netceptor.MainInstance.Logger.Error("Error creating peer %s: %s\n", cfg.Address, err)

		return err
	}
	err = netceptor.MainInstance.AddBackend(b,
		netceptor.BackendConnectionCost(cfg.Cost),
		netceptor.BackendAllowedPeers(cfg.AllowedPeers))
	if err != nil {
		return err
	}

	return nil
}

func (cfg TCPDialerCfg) PreReload() error {
	return cfg.Prepare()
}

func (cfg TCPListenerCfg) PreReload() error {
	return cfg.Prepare()
}

func (cfg TCPDialerCfg) Reload() error {
	return cfg.Run()
}

func (cfg TCPListenerCfg) Reload() error {
	return cfg.Run()
}

func init() {
	version := viper.GetInt("version")
	if version > 1 {
		return
	}
	cmdline.RegisterConfigTypeForApp("receptor-backends",
		"tcp-listener", "Run a backend listener on a TCP port", TCPListenerCfg{}, cmdline.Section(backendSection))
	cmdline.RegisterConfigTypeForApp("receptor-backends",
		"tcp-peer", "Make an outbound backend connection to a TCP peer", TCPDialerCfg{}, cmdline.Section(backendSection))
}
