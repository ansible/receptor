//go:build !no_udp_backend && !no_backends
// +build !no_udp_backend,!no_backends

package backends

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/utils"
	"github.com/ghjm/cmdline"
	"github.com/spf13/viper"
)

// UDPMaxPacketLen is the maximum size of a message that can be sent over UDP.
const UDPMaxPacketLen = 65507

// UDPDialer implements Backend for outbound UDP.
type UDPDialer struct {
	address string
	redial  bool
	logger  *logger.ReceptorLogger
}

func (b *UDPDialer) GetAddr() string {
	return b.address
}

func (b *UDPDialer) GetTLS() *tls.Config {
	return nil
}

// NewUDPDialer instantiates a new UDPDialer backend.
func NewUDPDialer(address string, redial bool, logger *logger.ReceptorLogger) (*UDPDialer, error) {
	_, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}
	nd := UDPDialer{
		address: address,
		redial:  redial,
		logger:  logger,
	}

	return &nd, nil
}

// Start runs the given session function over this backend service.
func (b *UDPDialer) Start(ctx context.Context, wg *sync.WaitGroup) (chan netceptor.BackendSession, error) {
	return dialerSession(ctx, wg, b.redial, 5*time.Second, b.logger,
		func(closeChan chan struct{}) (netceptor.BackendSession, error) {
			dialer := net.Dialer{}
			conn, err := dialer.DialContext(ctx, "udp", b.address)
			if err != nil {
				return nil, err
			}
			udpconn, ok := conn.(*net.UDPConn)
			if !ok {
				return nil, fmt.Errorf("DialContext returned a non-UDP connection")
			}
			ns := &UDPDialerSession{
				conn:            udpconn,
				closeChan:       closeChan,
				closeChanCloser: sync.Once{},
			}

			return ns, nil
		})
}

// UDPDialerSession implements BackendSession for UDPDialer.
type UDPDialerSession struct {
	conn            *net.UDPConn
	closeChan       chan struct{}
	closeChanCloser sync.Once
}

// Send sends data over the session.
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

// Recv receives data via the session.
func (ns *UDPDialerSession) Recv(timeout time.Duration) ([]byte, error) {
	err := ns.conn.SetReadDeadline(time.Now().Add(timeout))
	if err != nil {
		return nil, err
	}
	buf := make([]byte, utils.NormalBufferSize)
	n, err := ns.conn.Read(buf)
	if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
		return nil, netceptor.ErrTimeout
	}
	if err != nil {
		return nil, err
	}

	return buf[:n], nil
}

// Close closes the session.
func (ns *UDPDialerSession) Close() error {
	if ns.closeChan != nil {
		ns.closeChanCloser.Do(func() {
			close(ns.closeChan)
			ns.closeChan = nil
		})
	}

	return ns.conn.Close()
}

// UDPListener implements Backend for inbound UDP.
type UDPListener struct {
	laddr           *net.UDPAddr
	conn            *net.UDPConn
	sessChan        chan *UDPListenerSession
	sessRegLock     sync.RWMutex
	sessionRegistry map[string]*UDPListenerSession
	logger          *logger.ReceptorLogger
}

func (b *UDPListener) GetAddr() string {
	return b.LocalAddr().String()
}

func (b *UDPListener) GetTLS() *tls.Config {
	return &tls.Config{}
}

// NewUDPListener instantiates a new UDPListener backend.
func NewUDPListener(address string, logger *logger.ReceptorLogger) (*UDPListener, error) {
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
		logger:          logger,
	}

	return &ul, nil
}

// LocalAddr returns the local address the listener is listening on.
func (b *UDPListener) LocalAddr() net.Addr {
	if b.conn == nil {
		return nil
	}

	return b.conn.LocalAddr()
}

// Start runs the given session function over the UDPListener backend.
func (b *UDPListener) Start(ctx context.Context, wg *sync.WaitGroup) (chan netceptor.BackendSession, error) {
	sessChan := make(chan netceptor.BackendSession)
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, utils.NormalBufferSize)
		for {
			select {
			case <-ctx.Done():
				_ = b.conn.Close()

				return
			default:
			}
			err := b.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			if err != nil {
				b.logger.Error("Error setting UDP timeout: %s\n", err)

				return
			}
			n, addr, err := b.conn.ReadFromUDP(buf)
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			if err != nil {
				b.logger.Error("UDP read error: %s\n", err)

				return
			}
			data := make([]byte, n)
			copy(data, buf)
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
				select {
				case <-ctx.Done():
					_ = b.conn.Close()

					return
				case sessChan <- sess:
				}
			}
			select {
			case <-ctx.Done():
				_ = b.conn.Close()

				return
			case sess.recvChan <- data:
			}
		}
	}()
	if b.conn != nil {
		b.logger.Debug("Listening on UDP %s\n", b.LocalAddr().String())
	}

	return sessChan, nil
}

// UDPListenerSession implements BackendSession for UDPListener.
type UDPListenerSession struct {
	li       *UDPListener
	raddr    *net.UDPAddr
	recvChan chan []byte
}

// Send sends data over the session.
func (ns *UDPListenerSession) Send(data []byte) error {
	n, err := ns.li.conn.WriteToUDP(data, ns.raddr)
	if err != nil {
		return err
	} else if n != len(data) {
		return fmt.Errorf("partial data sent")
	}

	return nil
}

// Recv receives data from the session.
func (ns *UDPListenerSession) Recv(timeout time.Duration) ([]byte, error) {
	select {
	case data := <-ns.recvChan:
		return data, nil
	case <-time.After(timeout):
		return nil, netceptor.ErrTimeout
	}
}

// Close closes the session.
func (ns *UDPListenerSession) Close() error {
	ns.li.sessRegLock.Lock()
	defer ns.li.sessRegLock.Unlock()
	delete(ns.li.sessionRegistry, ns.raddr.String())

	return nil
}

// **************************************************************************
// Command line
// **************************************************************************

// TODO make these fields private
// UDPListenerCfg is the cmdline configuration object for a UDP listener.
type UDPListenerCfg struct {
	BindAddr     string             `description:"Local address to bind to" default:"0.0.0.0"`
	Port         int                `description:"Local UDP port to listen on" barevalue:"yes" required:"yes"`
	Cost         float64            `description:"Connection cost (weight)" default:"1.0"`
	NodeCost     map[string]float64 `description:"Per-node costs"`
	AllowedPeers []string           `description:"Peer node IDs to allow via this connection"`
}

func (cfg UDPListenerCfg) GetCost() float64 {
	return cfg.Cost
}

func (cfg UDPListenerCfg) GetNodeCost() map[string]float64 {
	return cfg.NodeCost
}

func (cfg UDPListenerCfg) GetAddr() string {
	return cfg.BindAddr
}

func (cfg UDPListenerCfg) GetPort() int {
	return cfg.Port
}

func (cfg UDPListenerCfg) GetTLS() string {
	return ""
}

// Prepare verifies the parameters are correct.
func (cfg UDPListenerCfg) Prepare() error {
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
func (cfg UDPListenerCfg) Run() error {
	address := fmt.Sprintf("%s:%d", cfg.BindAddr, cfg.Port)
	b, err := NewUDPListener(address, netceptor.MainInstance.Logger)
	if err != nil {
		netceptor.MainInstance.Logger.Error("Error creating listener %s: %s\n", address, err)

		return err
	}
	err = netceptor.MainInstance.AddBackend(b,
		netceptor.BackendConnectionCost(cfg.Cost),
		netceptor.BackendNodeCost(cfg.NodeCost),
		netceptor.BackendAllowedPeers(cfg.AllowedPeers))
	if err != nil {
		netceptor.MainInstance.Logger.Error("Error creating backend for %s: %s\n", address, err)

		return err
	}

	return nil
}

// udpDialerCfg is the cmdline configuration object for a UDP listener.
type UDPDialerCfg struct {
	Address      string   `description:"Host:Port to connect to" barevalue:"yes" required:"yes"`
	Redial       bool     `description:"Keep redialing on lost connection" default:"true"`
	Cost         float64  `description:"Connection cost (weight)" default:"1.0"`
	AllowedPeers []string `description:"Peer node IDs to allow via this connection"`
}

// Prepare verifies the parameters are correct.
func (cfg UDPDialerCfg) Prepare() error {
	if cfg.Cost <= 0.0 {
		return fmt.Errorf("connection cost must be positive")
	}

	return nil
}

// Run runs the action.
func (cfg UDPDialerCfg) Run() error {
	netceptor.MainInstance.Logger.Debug("Running UDP peer connection %s\n", cfg.Address)
	b, err := NewUDPDialer(cfg.Address, cfg.Redial, netceptor.MainInstance.Logger)
	if err != nil {
		netceptor.MainInstance.Logger.Error("Error creating peer %s: %s\n", cfg.Address, err)

		return err
	}
	err = netceptor.MainInstance.AddBackend(b,
		netceptor.BackendConnectionCost(cfg.Cost),
		netceptor.BackendAllowedPeers(cfg.AllowedPeers))
	if err != nil {
		netceptor.MainInstance.Logger.Error("Error creating backend for %s: %s\n", cfg.Address, err)

		return err
	}

	return nil
}

func (cfg UDPDialerCfg) PreReload() error {
	return cfg.Prepare()
}

func (cfg UDPListenerCfg) PreReload() error {
	return cfg.Prepare()
}

func (cfg UDPDialerCfg) Reload() error {
	return cfg.Run()
}

func (cfg UDPListenerCfg) Reload() error {
	return cfg.Run()
}

func init() {
	version := viper.GetInt("version")
	if version > 1 {
		return
	}
	cmdline.RegisterConfigTypeForApp("receptor-backends",
		"UDP-listener", "Run a backend listener on a UDP port", UDPListenerCfg{}, cmdline.Section(backendSection))
	cmdline.RegisterConfigTypeForApp("receptor-backends",
		"UDP-peer", "Make an outbound backend connection to a UDP peer", UDPDialerCfg{}, cmdline.Section(backendSection))
}
