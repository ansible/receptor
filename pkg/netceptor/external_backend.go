package netceptor

import (
	"context"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/project-receptor/receptor/pkg/framer"
	"github.com/project-receptor/receptor/pkg/utils"
	"net"
	"sync"
	"time"
)

// ExternalBackend is a backend implementation for the situation when non-Receptor code
// is initiating connections, outside the control of a Receptor-managed accept loop.
type ExternalBackend struct {
	ctx      context.Context
	cancel   context.CancelFunc
	sessChan chan BackendSession
}

// MessageConn is an abstract connection that sends and receives whole messages (datagrams)
type MessageConn interface {
	WriteMessage(ctx context.Context, data []byte) error
	ReadMessage(ctx context.Context, timeout time.Duration) ([]byte, error)
	SetReadDeadline(t time.Time) error
	Close() error
}

// netMessageConn implements MessageConn for Go net.Conn
type netMessageConn struct {
	conn   net.Conn
	framer framer.Framer
}

// MessageConnFromNetConn returns a MessageConnection that wraps a net.Conn
func MessageConnFromNetConn(conn net.Conn) MessageConn {
	return &netMessageConn{
		conn:   conn,
		framer: framer.New(),
	}
}

// WriteMessage writes a message to the connection
func (mc *netMessageConn) WriteMessage(ctx context.Context, data []byte) error {
	if ctx.Err() != nil {
		return fmt.Errorf("session closed: %s", ctx.Err())
	}
	buf := mc.framer.SendData(data)
	n, err := mc.conn.Write(buf)
	if err != nil {
		return err
	}
	if n != len(buf) {
		return fmt.Errorf("partial data sent")
	}
	return nil
}

// ReadMessage reads a message from the connection
func (mc *netMessageConn) ReadMessage(ctx context.Context, timeout time.Duration) ([]byte, error) {
	buf := make([]byte, utils.NormalBufferSize)
	err := mc.conn.SetReadDeadline(time.Now().Add(timeout))
	if err != nil {
		return nil, err
	}
	for {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("session closed: %s", ctx.Err())
		}
		if mc.framer.MessageReady() {
			break
		}
		n, err := mc.conn.Read(buf)
		if n > 0 {
			mc.framer.RecvData(buf[:n])
		}
		if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
			return nil, ErrTimeout
		}
		if err != nil {
			return nil, err
		}
	}
	buf, err = mc.framer.GetMessage()
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// SetReadDeadline sets the deadline by which a message must be read from the connection
func (mc *netMessageConn) SetReadDeadline(t time.Time) error {
	panic("implement me")
}

// Close closes the connection
func (mc *netMessageConn) Close() error {
	return mc.conn.Close()
}

// websocketMessageConn implements MessageConn for Gorilla websocket.Conn
type websocketMessageConn struct {
	conn *websocket.Conn
}

// MessageConnFromWebsocketConn returns a MessageConnection that wraps a Gorilla websocket.Conn
func MessageConnFromWebsocketConn(conn *websocket.Conn) MessageConn {
	return &websocketMessageConn{
		conn: conn,
	}
}

// WriteMessage writes a message to the connection
func (mc *websocketMessageConn) WriteMessage(ctx context.Context, data []byte) error {
	if ctx.Err() != nil {
		return fmt.Errorf("session closed: %s", ctx.Err())
	}
	return mc.conn.WriteMessage(websocket.BinaryMessage, data)
}

// ReadMessage reads a message from the connection
func (mc *websocketMessageConn) ReadMessage(ctx context.Context, timeout time.Duration) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, fmt.Errorf("session closed: %s", ctx.Err())
	}
	messageType, data, err := mc.conn.ReadMessage()
	if messageType != websocket.BinaryMessage {
		return nil, fmt.Errorf("received message of wrong type")
	}
	return data, err
}

// SetReadDeadline sets the deadline by which a message must be read from the connection
func (mc *websocketMessageConn) SetReadDeadline(t time.Time) error {
	return mc.conn.SetReadDeadline(t)
}

// Close closes the connection
func (mc *websocketMessageConn) Close() error {
	return mc.conn.Close()
}

// NewExternalBackend initializes a new ExternalBackend object
func NewExternalBackend() (*ExternalBackend, error) {
	return &ExternalBackend{}, nil
}

// Start launches the backend from Receptor's point of view, and waits for connections to happen.
func (b *ExternalBackend) Start(ctx context.Context, wg *sync.WaitGroup) (chan BackendSession, error) {
	b.ctx, b.cancel = context.WithCancel(ctx)
	b.ctx = ctx
	b.sessChan = make(chan BackendSession)
	return b.sessChan, nil
}

// ExternalSession implements BackendSession for external backends.
type ExternalSession struct {
	eb          *ExternalBackend
	conn        MessageConn
	shouldClose bool
}

// NewConnection is called by the external code when a new connection is available.  The
// connection will be closed when the session ends if closeConnWithSession is true.
func (b *ExternalBackend) NewConnection(conn MessageConn, closeConnWithSession bool) {
	ebs := &ExternalSession{
		eb:          b,
		conn:        conn,
		shouldClose: closeConnWithSession,
	}
	b.sessChan <- ebs
}

// Send sends data over the session
func (es *ExternalSession) Send(data []byte) error {
	return es.conn.WriteMessage(es.eb.ctx, data)
}

// Recv receives data via the session
func (es *ExternalSession) Recv(timeout time.Duration) ([]byte, error) {
	return es.conn.ReadMessage(es.eb.ctx, timeout)
}

// Close closes the session
func (es *ExternalSession) Close() error {
	es.eb.cancel()
	var err error
	if es.shouldClose {
		err = es.conn.Close()
	}
	return err
}
