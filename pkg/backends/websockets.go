// +build !no_websocket_backend
// +build !no_backends

package backends

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/ghjm/cmdline"
	"github.com/gorilla/websocket"
	"github.com/project-receptor/receptor/pkg/logger"
	"github.com/project-receptor/receptor/pkg/netceptor"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// WebsocketDialer implements Backend for outbound Websocket
type WebsocketDialer struct {
	address     string
	origin      string
	redial      bool
	tlscfg      *tls.Config
	extraHeader string
}

// NewWebsocketDialer instantiates a new WebsocketDialer backend
func NewWebsocketDialer(address string, tlscfg *tls.Config, extraHeader string, redial bool) (*WebsocketDialer, error) {
	addrURL, err := url.Parse(address)
	if err != nil {
		return nil, err
	}
	httpScheme := "http"
	if addrURL.Scheme == "wss" {
		httpScheme = "https"
	}
	wd := WebsocketDialer{
		address:     address,
		origin:      fmt.Sprintf("%s://%s", httpScheme, addrURL.Host),
		redial:      redial,
		tlscfg:      tlscfg,
		extraHeader: extraHeader,
	}
	return &wd, nil
}

// Start runs the given session function over this backend service
func (b *WebsocketDialer) Start(ctx context.Context) (chan netceptor.BackendSession, error) {
	return dialerSession(ctx, b.redial, 5*time.Second,
		func(closeChan chan struct{}) (netceptor.BackendSession, error) {
			dialer := websocket.Dialer{
				TLSClientConfig: b.tlscfg,
				Proxy:           http.ProxyFromEnvironment,
			}
			header := make(http.Header, 0)
			if b.extraHeader != "" {
				extraHeaderParts := strings.SplitN(b.extraHeader, ":", 2)
				header.Add(http.CanonicalHeaderKey(extraHeaderParts[0]), extraHeaderParts[1])
			}
			header.Add(http.CanonicalHeaderKey("origin"), b.origin)
			conn, _, err := dialer.DialContext(ctx, b.address, header)
			if err != nil {
				return nil, err
			}
			ns := newWebsocketSession(conn, closeChan)
			return ns, nil
		})
}

// WebsocketListener implements Backend for inbound Websocket
type WebsocketListener struct {
	address string
	path    string
	tlscfg  *tls.Config
	li      net.Listener
	server  *http.Server
}

// NewWebsocketListener instantiates a new WebsocketListener backend
func NewWebsocketListener(address string, tlscfg *tls.Config) (*WebsocketListener, error) {
	ul := WebsocketListener{
		address: address,
		path:    "/",
		tlscfg:  tlscfg,
		li:      nil,
	}
	return &ul, nil
}

// SetPath sets the URI path that the listener will be hosted on.
// It is only effective if used prior to calling Start.
func (b *WebsocketListener) SetPath(path string) {
	b.path = path
}

// Addr returns the network address the listener is listening on
func (b *WebsocketListener) Addr() net.Addr {
	if b.li == nil {
		return nil
	}
	return b.li.Addr()
}

// Path returns the URI path the websocket is configured on
func (b *WebsocketListener) Path() string {
	return b.path
}

// Start runs the given session function over the WebsocketListener backend
func (b *WebsocketListener) Start(ctx context.Context) (chan netceptor.BackendSession, error) {
	var err error
	sessChan := make(chan netceptor.BackendSession)
	mux := http.NewServeMux()
	mux.HandleFunc(b.path, func(w http.ResponseWriter, r *http.Request) {
		var upgrader = websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error("Error upgrading websocket connection: %s\n", err)
			return
		}
		ws := newWebsocketSession(conn, nil)
		sessChan <- ws
	})
	b.li, err = net.Listen("tcp", b.address)
	if err != nil {
		return nil, err
	}
	go func() {
		var err error
		b.server = &http.Server{
			Addr:    b.address,
			Handler: mux,
		}
		if b.tlscfg == nil {
			err = b.server.Serve(b.li)
		} else {
			b.server.TLSConfig = b.tlscfg
			err = b.server.ServeTLS(b.li, "", "")
		}
		if err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error: %s\n", err)
		}
	}()
	go func() {
		<-ctx.Done()
		_ = b.server.Close()
	}()
	logger.Debug("Listening on Websocket %s path %s\n", b.Addr().String(), b.Path())
	return sessChan, nil
}

// WebsocketSession implements BackendSession for WebsocketDialer and WebsocketListener
type WebsocketSession struct {
	conn            *websocket.Conn
	recvChan        chan *recvResult
	closeChan       chan struct{}
	closeChanCloser sync.Once
}

type recvResult struct {
	data []byte
	err  error
}

func newWebsocketSession(conn *websocket.Conn, closeChan chan struct{}) *WebsocketSession {
	ws := &WebsocketSession{
		conn:            conn,
		recvChan:        make(chan *recvResult),
		closeChan:       closeChan,
		closeChanCloser: sync.Once{},
	}
	go ws.recvChannelizer()
	return ws
}

// recvChannelizer receives messages and pushes them to a channel.
func (ns *WebsocketSession) recvChannelizer() {
	for {
		_, data, err := ns.conn.ReadMessage()
		ns.recvChan <- &recvResult{
			data: data,
			err:  err,
		}
		if err != nil {
			return
		}
	}
}

// Send sends data over the session
func (ns *WebsocketSession) Send(data []byte) error {
	err := ns.conn.WriteMessage(websocket.BinaryMessage, data)
	if err != nil {
		return err
	}
	return nil
}

// Recv receives data via the session
func (ns *WebsocketSession) Recv(timeout time.Duration) ([]byte, error) {
	select {
	case rr := <-ns.recvChan:
		return rr.data, rr.err
	case <-time.After(timeout):
		return nil, netceptor.ErrTimeout
	}
}

// Close closes the session
func (ns *WebsocketSession) Close() error {
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

// websocketListenerCfg is the cmdline configuration object for a websocket listener
type websocketListenerCfg struct {
	BindAddr     string             `description:"Local address to bind to" default:"0.0.0.0"`
	Port         int                `description:"Local TCP port to run http server on" barevalue:"yes" required:"yes"`
	Path         string             `description:"URI path to the websocket server" default:"/"`
	TLS          string             `description:"Name of TLS server config"`
	Cost         float64            `description:"Connection cost (weight)" default:"1.0"`
	NodeCost     map[string]float64 `description:"Per-node costs"`
	AllowedPeers []string           `description:"Peer node IDs to allow via this connection"`
}

// Prepare verifies the parameters are correct
func (cfg websocketListenerCfg) Prepare() error {
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

// Run runs the action
func (cfg websocketListenerCfg) Run() error {
	address := fmt.Sprintf("%s:%d", cfg.BindAddr, cfg.Port)
	tlscfg, err := netceptor.MainInstance.GetServerTLSConfig(cfg.TLS)
	if err != nil {
		return err
	}
	b, err := NewWebsocketListener(address, tlscfg)
	if err != nil {
		logger.Error("Error creating listener %s: %s\n", address, err)
		return err
	}
	b.SetPath(cfg.Path)
	err = netceptor.MainInstance.AddBackend(b,
		netceptor.BackendConnectionCost(cfg.Cost),
		netceptor.BackendNodeCost(cfg.NodeCost),
		netceptor.BackendAllowedPeers(cfg.AllowedPeers))
	if err != nil {
		return err
	}
	return nil
}

// websocketDialerCfg is the cmdline configuration object for a Websocket listener
type websocketDialerCfg struct {
	Address      string   `description:"URL to connect to" barevalue:"yes" required:"yes"`
	Redial       bool     `description:"Keep redialing on lost connection" default:"true"`
	ExtraHeader  string   `description:"Sends extra HTTP header on initial connection"`
	TLS          string   `description:"Name of TLS client config"`
	Cost         float64  `description:"Connection cost (weight)" default:"1.0"`
	AllowedPeers []string `description:"Peer node IDs to allow via this connection"`
}

// Prepare verifies that we are reasonably ready to go
func (cfg websocketDialerCfg) Prepare() error {
	if cfg.Cost <= 0.0 {
		return fmt.Errorf("connection cost must be positive")
	}
	_, err := url.Parse(cfg.Address)
	if err != nil {
		return fmt.Errorf("address %s is not a valid URL: %s", cfg.Address, err)
	}
	if cfg.ExtraHeader != "" && !strings.Contains(cfg.ExtraHeader, ":") {
		return fmt.Errorf("extra header must be in the form key:value")
	}
	return nil
}

// Run runs the action
func (cfg websocketDialerCfg) Run() error {
	logger.Debug("Running Websocket peer connection %s\n", cfg.Address)
	u, err := url.Parse(cfg.Address)
	if err != nil {
		return err
	}
	tlsCfgName := cfg.TLS
	if u.Scheme == "wss" && tlsCfgName == "" {
		tlsCfgName = "default"
	}
	tlscfg, err := netceptor.MainInstance.GetClientTLSConfig(tlsCfgName, u.Hostname(), "dns")
	if err != nil {
		return err
	}
	b, err := NewWebsocketDialer(cfg.Address, tlscfg, cfg.ExtraHeader, cfg.Redial)
	if err != nil {
		logger.Error("Error creating peer %s: %s\n", cfg.Address, err)
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

func init() {
	cmdline.RegisterConfigTypeForApp("receptor-backends",
		"ws-listener", "Run an http server that accepts websocket connections", websocketListenerCfg{}, cmdline.Section(backendSection))
	cmdline.RegisterConfigTypeForApp("receptor-backends",
		"ws-peer", "Connect outbound to a websocket peer", websocketDialerCfg{}, cmdline.Section(backendSection))
}
