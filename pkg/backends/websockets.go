package backends

import (
	"crypto/tls"
	"fmt"
	"github.com/project-receptor/receptor/pkg/cmdline"
	"github.com/project-receptor/receptor/pkg/debug"
	"github.com/project-receptor/receptor/pkg/framer"
	"github.com/project-receptor/receptor/pkg/netceptor"
	"golang.org/x/net/websocket"
	"net/http"
	"net/url"
	"strings"
	"time"
)

//TODO: Switch to better supported websockets library
//TODO: TLS Server
//TODO: HTTP/HTTPS proxy support

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
	wd := WebsocketDialer{
		address:     address,
		origin:      fmt.Sprintf("http://%s", addrURL.Host),
		redial:      redial,
		tlscfg:      tlscfg,
		extraHeader: extraHeader,
	}
	return &wd, nil
}

// Start runs the given session function over this backend service
func (b *WebsocketDialer) Start(bsf netceptor.BackendSessFunc, errf netceptor.ErrorFunc) {
	go func() {
		for {
			cfg, err := websocket.NewConfig(b.address, b.origin)
			if err != nil {
				errf(fmt.Errorf("error creating websocket config"), true)
				return
			}
			if b.tlscfg != nil {
				cfg.TlsConfig = b.tlscfg
			}
			if b.extraHeader != "" {
				extraHeaderParts := strings.SplitN(b.extraHeader, ":", 2)
				header := make(http.Header, 0)
				header.Add(http.CanonicalHeaderKey(extraHeaderParts[0]), extraHeaderParts[1])
				cfg.Header = header
			}
			conn, err := websocket.DialConfig(cfg)
			if err == nil {
				ns := newWebsocketSession(conn)
				err = bsf(ns)
			}
			if err != nil {
				if b.redial {
					errf(err, false)
					time.Sleep(5 * time.Second)
				} else {
					errf(err, true)
					return
				}
			}
		}
	}()
}

// WebsocketListener implements Backend for inbound Websocket
type WebsocketListener struct {
	address string
	tlscfg  *tls.Config
}

// NewWebsocketListener instantiates a new WebsocketListener backend
func NewWebsocketListener(address string, tlscfg *tls.Config) (*WebsocketListener, error) {
	ul := WebsocketListener{
		address: address,
		tlscfg:  tlscfg,
	}
	return &ul, nil
}

// Start runs the given session function over the WebsocketListener backend
func (b *WebsocketListener) Start(bsf netceptor.BackendSessFunc, errf netceptor.ErrorFunc) {
	if b.tlscfg != nil {
		errf(fmt.Errorf("websocket TLS server not implemented yet"), true)
		return
	}
	mux := http.NewServeMux()
	mux.Handle("/", websocket.Handler(func(conn *websocket.Conn) {
		ws := newWebsocketSession(conn)
		err := bsf(ws)
		if err != nil {
			errf(err, false)
		}
	}))
	go func() {
		var err error
		err = http.ListenAndServe(b.address, mux)
		if err != nil {
			errf(err, true)
			return
		}
	}()
	debug.Printf("Listening on %s\n", b.address)
}

// WebsocketSession implements BackendSession for WebsocketDialer and WebsocketListener
type WebsocketSession struct {
	conn   *websocket.Conn
	framer framer.Framer
}

func newWebsocketSession(conn *websocket.Conn) *WebsocketSession {
	ws := &WebsocketSession{
		conn:   conn,
		framer: framer.New(),
	}
	return ws
}

// Send sends data over the session
func (ns *WebsocketSession) Send(data []byte) error {
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
func (ns *WebsocketSession) Recv() ([]byte, error) {
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
func (ns *WebsocketSession) Close() error {
	return ns.conn.Close()
}

// **************************************************************************
// Command line
// **************************************************************************

// WebsocketListenerCfg is the cmdline configuration object for a websocket listener
type WebsocketListenerCfg struct {
	BindAddr string  `description:"Local address to bind to" default:"0.0.0.0"`
	Port     int     `description:"Local TCP port to run http server on" barevalue:"yes" required:"yes"`
	TLS      string  `description:"Name of TLS server config"`
	Cost     float64 `description:"Connection cost (weight)" default:"1.0"`
}

// Prepare verifies the parameters are correct
func (cfg WebsocketListenerCfg) Prepare() error {
	if cfg.Cost <= 0.0 {
		return fmt.Errorf("connection cost must be positive")
	}
	return nil
}

// Run runs the action
func (cfg WebsocketListenerCfg) Run() error {
	address := fmt.Sprintf("%s:%d", cfg.BindAddr, cfg.Port)
	debug.Printf("Running listener %s\n", address)
	tlscfg, err := netceptor.GetServerTLSConfig(cfg.TLS)
	if err != nil {
		return err
	}
	li, err := NewWebsocketListener(address, tlscfg)
	if err != nil {
		debug.Printf("Error creating listener %s: %s\n", address, err)
		return err
	}
	netceptor.AddBackend()
	netceptor.MainInstance.RunBackend(li, cfg.Cost, func(err error, fatal bool) {
		fmt.Printf("Error in listener backend: %s\n", err)
		if fatal {
			netceptor.DoneBackend()
		}
	})
	return nil
}

// WebsocketDialerCfg is the cmdline configuration object for a Websocket listener
type WebsocketDialerCfg struct {
	Address     string  `description:"URL to connect to" barevalue:"yes" required:"yes"`
	Redial      bool    `description:"Keep redialing on lost connection" default:"true"`
	ExtraHeader string  `description:"Sends extra HTTP header on initial connection"`
	TLS         string  `description:"Name of TLS client config"`
	Cost        float64 `description:"Connection cost (weight)" default:"1.0"`
}

// Prepare verifies that we are reasonably ready to go
func (cfg WebsocketDialerCfg) Prepare() error {
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
func (cfg WebsocketDialerCfg) Run() error {
	debug.Printf("Running Websocket peer connection %s\n", cfg.Address)
	u, err := url.Parse(cfg.Address)
	if err != nil {
		return err
	}
	tlscfg, err := netceptor.GetClientTLSConfig(cfg.TLS, u.Hostname())
	if err != nil {
		return err
	}
	li, err := NewWebsocketDialer(cfg.Address, tlscfg, cfg.ExtraHeader, cfg.Redial)
	if err != nil {
		debug.Printf("Error creating peer %s: %s\n", cfg.Address, err)
		return err
	}
	netceptor.AddBackend()
	netceptor.MainInstance.RunBackend(li, cfg.Cost, func(err error, fatal bool) {
		fmt.Printf("Error in peer connection backend: %s\n", err)
		if fatal {
			netceptor.DoneBackend()
		}
	})
	return nil
}

func init() {
	cmdline.AddConfigType("ws-listener", "Run an http server that accepts websocket connections", WebsocketListenerCfg{}, false, false, false, backendSection)
	cmdline.AddConfigType("ws-peer", "Connect outbound to a websocket peer", WebsocketDialerCfg{}, false, false, false, backendSection)
}
