package backends

import (
	"fmt"
	"github.com/ghjm/sockceptor/pkg/cmdline"
	"github.com/ghjm/sockceptor/pkg/debug"
	"github.com/ghjm/sockceptor/pkg/framer"
	"github.com/ghjm/sockceptor/pkg/netceptor"
	"golang.org/x/net/websocket"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

//TODO: TLS
//TODO: HTTP/HTTPS proxy support

// WebsocketDialer implements Backend for outbound Websocket
type WebsocketDialer struct {
	address     string
	origin      string
	redial      bool
	extraHeader string
}

// NewWebsocketDialer instantiates a new WebsocketDialer backend
func NewWebsocketDialer(address string, extraHeader string, redial bool) (*WebsocketDialer, error) {
	addrURL, err := url.Parse(address)
	if err != nil {
		return nil, err
	}
	wd := WebsocketDialer{
		address:     address,
		origin:      fmt.Sprintf("http://%s", addrURL.Host),
		redial:      redial,
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
			if b.extraHeader != "" {
				extraHeaderParts := strings.SplitN(b.extraHeader, ":", 2)
				header := make(http.Header, 0)
				header.Add(http.CanonicalHeaderKey(extraHeaderParts[0]), extraHeaderParts[1])
				cfg.Header = header
			}
			conn, err := websocket.DialConfig(cfg)
			if b.redial {
				wsderr, ok := err.(*websocket.DialError)
				if ok {
					operr, ok := wsderr.Err.(*net.OpError)
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
			}
			if err != nil {
				errf(err, true)
				return
			}
			ns := newWebsocketSession(conn)
			err = bsf(ns)
			if err != nil {
				errf(err, false)
			}
		}
	}()
}

// WebsocketListener implements Backend for inbound Websocket
type WebsocketListener struct {
	address string
}

// NewWebsocketListener instantiates a new WebsocketListener backend
func NewWebsocketListener(address string) (*WebsocketListener, error) {
	ul := WebsocketListener{
		address: address,
	}
	return &ul, nil
}

// Start runs the given session function over the WebsocketListener backend
func (b *WebsocketListener) Start(bsf netceptor.BackendSessFunc, errf netceptor.ErrorFunc) {
	mux := http.NewServeMux()
	mux.Handle("/", websocket.Handler(func(conn *websocket.Conn) {
		ws := newWebsocketSession(conn)
		err := bsf(ws)
		if err != nil {
			errf(err, false)
		}
	}))
	go func() {
		err := http.ListenAndServe(b.address, mux)
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
	BindAddr string `description:"Local address to bind to" default:"0.0.0.0"`
	Port     int    `description:"Local TCP port to run http server on" barevalue:"yes" required:"yes"`
}

// Run runs the action
func (cfg WebsocketListenerCfg) Run() error {
	address := fmt.Sprintf("%s:%d", cfg.BindAddr, cfg.Port)
	debug.Printf("Running listener %s\n", address)
	li, err := NewWebsocketListener(address)
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

// WebsocketDialerCfg is the cmdline configuration object for a Websocket listener
type WebsocketDialerCfg struct {
	Address     string `description:"URL to connect to" barevalue:"yes" required:"yes"`
	Redial      string `description:"Keep redialing on lost connection" default:"true"`
	ExtraHeader string `description:"Sends extra HTTP header on initial connection"`
}

// Prepare verifies that we are reasonably ready to go
func (cfg WebsocketDialerCfg) Prepare() error {
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
	redial, _ := strconv.ParseBool(cfg.Redial)
	li, err := NewWebsocketDialer(cfg.Address, cfg.ExtraHeader, redial)
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
	cmdline.AddConfigType("ws-listener", "Run an http server that accepts websocket connections", WebsocketListenerCfg{}, false)
	cmdline.AddConfigType("ws-peer", "Connect outbound to a websocket peer", WebsocketDialerCfg{}, false)
}
