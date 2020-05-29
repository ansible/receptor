package backends

import (
	"fmt"
	"github.org/ghjm/sockceptor/pkg/debug"
	"github.org/ghjm/sockceptor/pkg/framer"
	"github.org/ghjm/sockceptor/pkg/netceptor"
	"golang.org/x/net/websocket"
	"net/http"
	"net/url"
)

//TODO: TLS
//TODO: HTTP/HTTPS proxy support
//TODO: ws_extra_headers
//TODO: configurable reconnect

// WebsocketDialer implements Backend for outbound Websocket
type WebsocketDialer struct {
	address string
	origin string
}

// NewWebsocketDialer instantiates a new WebsocketDialer backend
func NewWebsocketDialer(address string) (*WebsocketDialer, error) {
	url, err := url.Parse(address); if err != nil {
		return nil, err
	}
	wd := WebsocketDialer{
		address: address,
		origin: fmt.Sprintf("http://%s", url.Host), 
	}
	return &wd, nil
}

// Start runs the given session function over this backend service
func (b *WebsocketDialer) Start(bsf netceptor.BackendSessFunc, errf netceptor.ErrorFunc) {
	go func() {
		for {
			conn, err := websocket.Dial(b.address, "", b.origin)
			if err == nil {
				ns := newWebsocketSession(conn)
				err = bsf(ns)
			}
			errf(err)
			return
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
			errf(err)
		}
	}))
	go func() {
		err := http.ListenAndServe(b.address, mux);
		if err != nil {
			errf(err)
			return
		}
	}()
	debug.Printf("Listening on %s\n", b.address)
}

// WebsocketSession implements BackendSession for WebsocketDialer and WebsocketListener
type WebsocketSession struct {
	conn *websocket.Conn
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
func (ns *WebsocketSession) Recv() ([]byte, error) {
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
func (ns *WebsocketSession) Close() error {
	return ns.conn.Close()
}

