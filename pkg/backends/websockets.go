package backends

import (
	"fmt"
	"github.org/ghjm/sockceptor/pkg/debug"
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
				ns := WebsocketDialerSession{
					conn: conn,
				}
				err = bsf(&ns)
			}
			errf(err)
			return
		}
	}()
}

// WebsocketDialerSession implements BackendSession for WebsocketDialer
type WebsocketDialerSession struct {
	conn *websocket.Conn
}

// Send sends data over the session
func (ns *WebsocketDialerSession) Send(data []byte) error {
	n, err := ns.conn.Write(data)
	debug.Tracef("Websocket sent data %s len %d sent %d err %s\n", data, len(data), n, err)
	if err != nil {
		return err
	}
	if n != len(data) {
		return fmt.Errorf("partial data sent")
	}
	return nil
}

// Recv receives data via the session
func (ns *WebsocketDialerSession) Recv() ([]byte, error) {
	buf := make([]byte, netceptor.MTU)
	n, err := ns.conn.Read(buf)
	debug.Tracef("Websocket sending data %s len %d sent %d err %s\n", buf, len(buf), n, err)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

// Close closes the session
func (ns *WebsocketDialerSession) Close() error {
	return ns.conn.Close()
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
		wls := &WebsocketListenerSession{
			conn: conn,
		}
		bsf(wls)
	}))
	err := http.ListenAndServe(b.address, mux); if err != nil {
		errf(err)
		return
	}
	debug.Printf("Listening on %s\n", b.address)
}

// WebsocketListenerSession implements BackendSession for WebsocketListener
type WebsocketListenerSession struct {
	conn *websocket.Conn
}

// Send sends data over the session
func (ns *WebsocketListenerSession) Send(data []byte) error{
	n, err := ns.conn.Write(data)
	debug.Tracef("Websocket sent data %s len %d sent %d err %s\n", data, len(data), n, err)
	if err != nil {
		return err
	} else if n != len(data) {
		return fmt.Errorf("partial data sent")
	}
	return nil
}

// Recv receives data from the session
func (ns *WebsocketListenerSession) Recv() ([]byte, error) {
	buf := make([]byte, netceptor.MTU)
	n, err := ns.conn.Read(buf)
	debug.Tracef("Websocket received data %s len %d err %s\n", buf[:n], n, err)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

// Close closes the session
func (ns *WebsocketListenerSession) Close() error {
	return ns.conn.Close()
}
