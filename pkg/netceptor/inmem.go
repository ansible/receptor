package netceptor

import (
	"context"
	"time"
)

// InMemoryBackend is a simple in-memory channels-based backend, intended for testing.
type InMemoryBackend struct {
	peer         *InMemoryBackend
	sessChan     chan BackendSession
	lastActivity *time.Time
}

// NewInMemoryBackendPair instantiates two connected in-memory backends
func NewInMemoryBackendPair() (*InMemoryBackend, *InMemoryBackend, error) {
	var lastActivity time.Time
	b1 := &InMemoryBackend{
		lastActivity: &lastActivity,
	}
	b2 := &InMemoryBackend{
		lastActivity: &lastActivity,
	}
	b1.peer = b2
	b2.peer = b1
	return b1, b2, nil
}

// Start generates a single session for both "sides" when the second call to Start occurs
func (b *InMemoryBackend) Start(ctx context.Context) (chan BackendSession, error) {
	b.sessChan = make(chan BackendSession)
	if b.peer.sessChan != nil {
		sessionCtx, sessionCancel := context.WithCancel(ctx)
		chan1 := make(chan []byte)
		chan2 := make(chan []byte)
		myIms := &InMemorySession{
			ctx:          sessionCtx,
			cancel:       sessionCancel,
			sendChan:     chan1,
			recvChan:     chan2,
			lastActivity: b.lastActivity,
		}
		peerIms := &InMemorySession{
			ctx:          sessionCtx,
			cancel:       sessionCancel,
			sendChan:     chan2,
			recvChan:     chan1,
			lastActivity: b.lastActivity,
		}
		go func() {
			b.sessChan <- myIms
		}()
		go func() {
			b.peer.sessChan <- peerIms
		}()
	}
	return b.sessChan, nil
}

// InMemorySession implements BackendSession for in-memory pairs
type InMemorySession struct {
	ctx          context.Context
	cancel       context.CancelFunc
	sendChan     chan []byte
	recvChan     chan []byte
	lastActivity *time.Time
}

// Send sends data over the session
func (ims *InMemorySession) Send(data []byte) error {
	*ims.lastActivity = time.Now()
	ims.sendChan <- data
	return nil
}

// Recv receives data via the session
func (ims *InMemorySession) Recv(timeout time.Duration) ([]byte, error) {
	select {
	case data := <-ims.recvChan:
		*ims.lastActivity = time.Now()
		return data, nil
	case <-time.After(timeout):
		return nil, &TimeoutError{}
	case <-ims.ctx.Done():
		return nil, ims.ctx.Err()
	}
}

// Close closes the session
func (ims *InMemorySession) Close() error {
	ims.cancel()
	return nil
}
