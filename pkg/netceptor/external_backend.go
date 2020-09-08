package netceptor

import (
	"context"
	"fmt"
	"github.com/project-receptor/receptor/pkg/framer"
	"net"
	"time"
)

// ExternalBackend is a backend implementation for the situation when non-Receptor code
// is initiating connections, outside the control of a Receptor-managed accept loop.
type ExternalBackend struct {
	ctx      context.Context
	cancel   context.CancelFunc
	sessChan chan BackendSession
}

// NewExternalBackend initializes a new ExternalBackend object
func NewExternalBackend() (*ExternalBackend, error) {
	return &ExternalBackend{}, nil
}

// Start launches the backend from Receptor's point of view, and waits for connections to happen.
func (b *ExternalBackend) Start(ctx context.Context) (chan BackendSession, error) {
	b.ctx, b.cancel = context.WithCancel(ctx)
	b.ctx = ctx
	b.sessChan = make(chan BackendSession)
	return b.sessChan, nil
}

// ExternalSession implements BackendSession for external backends.
type ExternalSession struct {
	eb          *ExternalBackend
	conn        net.Conn
	framer      framer.Framer
	shouldClose bool
}

// NewConnection is called by the external code when a new connection is available.  The
// connection will be closed when the session ends if closeConnWithSession is true.
func (b *ExternalBackend) NewConnection(conn net.Conn, closeConnWithSession bool) {
	ebs := &ExternalSession{
		eb:          b,
		conn:        conn,
		framer:      framer.New(),
		shouldClose: closeConnWithSession,
	}
	b.sessChan <- ebs
}

// Send sends data over the session
func (es *ExternalSession) Send(data []byte) error {
	if es.eb.ctx.Err() != nil {
		return fmt.Errorf("session closed: %s", es.eb.ctx.Err())
	}
	buf := es.framer.SendData(data)
	n, err := es.conn.Write(buf)
	if err != nil {
		return err
	}
	if n != len(buf) {
		return fmt.Errorf("partial data sent")
	}
	return nil
}

// Recv receives data via the session
func (es *ExternalSession) Recv(timeout time.Duration) ([]byte, error) {
	// Recv receives data via the session
	buf := make([]byte, MTU)
	for {
		if es.eb.ctx.Err() != nil {
			return nil, fmt.Errorf("session closed: %s", es.eb.ctx.Err())
		}
		if es.framer.MessageReady() {
			break
		}
		err := es.conn.SetReadDeadline(time.Now().Add(timeout))
		if err != nil {
			return nil, err
		}
		n, err := es.conn.Read(buf)
		if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
			return nil, ErrTimeout
		}
		if err != nil {
			return nil, err
		}
		es.framer.RecvData(buf[:n])
	}
	buf, err := es.framer.GetMessage()
	if err != nil {
		return nil, err
	}
	return buf, nil
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
