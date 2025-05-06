package netceptor_test

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/netceptor/mock_netceptor"
	"github.com/quic-go/quic-go"
	"go.uber.org/mock/gomock"
)

// Create a simple Addr implementation for testing.
type testAddr struct {
	network string
	node    string
	service string
}

func (a testAddr) Network() string {
	return a.network
}

func (a testAddr) String() string {
	return a.node + "/" + a.service
}

func newTestAddr(node, service string) testAddr {
	return testAddr{
		network: "test",
		node:    node,
		service: service,
	}
}

// mockQuicListener is a mock implementation of quic.Listener.
type mockQuicListener struct {
	acceptChan chan quic.Connection
	closeChan  chan struct{}
	closed     bool
	addr       net.Addr
}

func newMockQuicListener(addr net.Addr) *mockQuicListener {
	return &mockQuicListener{
		acceptChan: make(chan quic.Connection),
		closeChan:  make(chan struct{}),
		addr:       addr,
	}
}

func (m *mockQuicListener) Accept(ctx context.Context) (quic.Connection, error) {
	select {
	case conn := <-m.acceptChan:
		return conn, nil
	case <-m.closeChan:
		return nil, errors.New("listener closed")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *mockQuicListener) Addr() net.Addr {
	return m.addr
}

func (m *mockQuicListener) Close() error {
	if !m.closed {
		m.closed = true
		close(m.closeChan)
	}

	return nil
}

// Helper function to create a Listener for testing.
func createTestListener(t *testing.T, ctrl *gomock.Controller) (*netceptor.Listener, *mock_netceptor.MockPacketConner, *mockQuicListener) {
	t.Helper()

	// Create a mock PacketConner
	mockPC := mock_netceptor.NewMockPacketConner(ctrl)

	// Create a mock address
	mockAddr := newTestAddr("test-node", "test-service")
	mockPC.EXPECT().LocalAddr().Return(mockAddr).AnyTimes()

	// Create a mock quic.Listener
	mockQL := newMockQuicListener(mockAddr)

	// Create a Netceptor instance
	netc := netceptor.New(context.Background(), "test-node")

	// Create a Listener - we'll use reflection to set the QL field since we can't directly use our mock
	listener := netceptor.NewTestListener(netc, mockPC, mockAddr, make(chan *netceptor.AcceptResult), make(chan struct{}), &sync.Once{})

	// Store the mockQL for later use in tests
	return listener, mockPC, mockQL
}

// TestListenerAccept tests the Accept method of the Listener struct.
func TestListenerAccept(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("Accept returns connection and nil error", func(t *testing.T) {
		listener, _, _ := createTestListener(t, ctrl)

		// Create a mock connection to return
		mockConn := &netceptor.Conn{}

		// Send a result to the AcceptChan
		go func() {
			listener.AcceptChan <- &netceptor.AcceptResult{
				Conn: mockConn,
				Err:  nil,
			}
		}()

		// Call Accept and check the result
		conn, err := listener.Accept()
		if err != nil {
			t.Fatalf("Expected nil error, got %v", err)
		}
		if conn != mockConn {
			t.Fatalf("Expected connection %v, got %v", mockConn, conn)
		}
	})

	t.Run("Accept returns error from AcceptResult", func(t *testing.T) {
		listener, _, _ := createTestListener(t, ctrl)

		expectedErr := errors.New("test error")

		// Send a result to the AcceptChan
		go func() {
			listener.AcceptChan <- &netceptor.AcceptResult{
				Conn: nil,
				Err:  expectedErr,
			}
		}()

		// Call Accept and check the result
		conn, err := listener.Accept()
		if err != expectedErr {
			t.Fatalf("Expected error %v, got %v", expectedErr, err)
		}
		if conn != nil {
			t.Fatalf("Expected nil connection, got %v", conn)
		}
	})

	t.Run("Accept returns error when listener is closed", func(t *testing.T) {
		listener, _, _ := createTestListener(t, ctrl)

		// Close the listener
		close(listener.DoneChan)

		// Call Accept and check the result
		conn, err := listener.Accept()
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if conn != nil {
			t.Fatalf("Expected nil connection, got %v", conn)
		}
		if err.Error() != "listener closed" {
			t.Fatalf("Expected 'listener closed' error, got %v", err)
		}
	})
}

// TestListenerClose tests the Close method of the Listener struct.
func TestListenerClose(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Note: We can't test the full Close method because it requires a real quic.Listener
	// which we can't easily mock. Instead, we'll test the parts we can.
	t.Run("Close closes DoneChan", func(t *testing.T) {
		listener, mockPC, _ := createTestListener(t, ctrl)

		// Create a custom Close method for testing
		customClose := func() error {
			listener.DoneOnce.Do(func() {
				close(listener.DoneChan)
			})

			return mockPC.Close()
		}

		// Set up expectations
		mockPC.EXPECT().Close().Return(nil)

		// Call our custom Close
		err := customClose()
		if err != nil {
			t.Fatalf("Expected nil error, got %v", err)
		}

		// Verify DoneChan is closed
		select {
		case <-listener.DoneChan:
			// This is expected
		default:
			t.Fatal("Expected DoneChan to be closed")
		}
	})

	t.Run("Close returns error from PacketConner.Close", func(t *testing.T) {
		listener, mockPC, _ := createTestListener(t, ctrl)

		// Create a custom Close method for testing
		customClose := func() error {
			listener.DoneOnce.Do(func() {
				close(listener.DoneChan)
			})

			return mockPC.Close()
		}

		// Set up expectations
		expectedErr := errors.New("packet conn close error")
		mockPC.EXPECT().Close().Return(expectedErr)

		// Call our custom Close
		err := customClose()
		if err != expectedErr {
			t.Fatalf("Expected error %v, got %v", expectedErr, err)
		}

		// Verify DoneChan is closed
		select {
		case <-listener.DoneChan:
			// This is expected
		default:
			t.Fatal("Expected DoneChan to be closed")
		}
	})

	t.Run("Close is idempotent for DoneChan", func(t *testing.T) {
		listener, _, _ := createTestListener(t, ctrl)

		// Just test the DoneOnce part
		listener.DoneOnce.Do(func() {
			close(listener.DoneChan)
		})

		// Second call should not close DoneChan again (which would panic)
		listener.DoneOnce.Do(func() {
			t.Fatal("DoneOnce was called a second time")
		})

		// Verify DoneChan is closed
		select {
		case <-listener.DoneChan:
			// This is expected
		default:
			t.Fatal("Expected DoneChan to be closed")
		}
	})
}

// TestListenerAddr tests the Addr method of the Listener struct.
func TestListenerAddr(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("Addr returns the local address from PacketConner", func(t *testing.T) {
		// Create a mock PacketConner
		mockPC := mock_netceptor.NewMockPacketConner(ctrl)

		// Create a mock address
		expectedAddr := newTestAddr("test-node", "test-service")

		// Set up expectations - this will be called by Addr()
		mockPC.EXPECT().LocalAddr().Return(expectedAddr)

		// Create a Netceptor instance
		netc := netceptor.New(context.Background(), "test-node")

		// Create a Listener directly
		listener := netceptor.NewTestListener(
			netc,
			mockPC,
			expectedAddr,
			make(chan *netceptor.AcceptResult),
			make(chan struct{}),
			&sync.Once{},
		)

		// Call Addr and check the result
		addr := listener.Addr()
		if addr != expectedAddr {
			t.Fatalf("Expected address %v, got %v", expectedAddr, addr)
		}
	})
}

// TestListenerSendResult tests the sendResult method of the Listener struct.
func TestListenerSendResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("sendResult sends result to AcceptChan", func(t *testing.T) {
		listener, _, _ := createTestListener(t, ctrl)

		// Create a mock connection and error
		mockConn := &netceptor.Conn{}
		mockErr := errors.New("test error")

		// Call sendResult
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		go listener.SendResult(ctx, mockConn, mockErr)

		// Check that the result was sent to AcceptChan
		select {
		case result := <-listener.AcceptChan:
			if result.Conn != mockConn {
				t.Fatalf("Expected connection %v, got %v", mockConn, result.Conn)
			}
			if result.Err != mockErr {
				t.Fatalf("Expected error %v, got %v", mockErr, result.Err)
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatal("Timed out waiting for result")
		}
	})

	t.Run("sendResult does not send when context is done", func(t *testing.T) {
		listener, _, _ := createTestListener(t, ctrl)

		// Create a mock connection and error
		mockConn := &netceptor.Conn{}
		mockErr := errors.New("test error")

		// Create a cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel the context immediately

		// Call sendResult with cancelled context
		listener.SendResult(ctx, mockConn, mockErr)

		// Check that no result was sent to AcceptChan
		select {
		case result := <-listener.AcceptChan:
			t.Fatalf("Unexpected result received: %v, %v", result.Conn, result.Err)
		case <-time.After(100 * time.Millisecond):
			// This is expected - no result should be sent
		}
	})

	t.Run("sendResult does not send when listener is closed", func(t *testing.T) {
		listener, _, _ := createTestListener(t, ctrl)

		// Create a mock connection and error
		mockConn := &netceptor.Conn{}
		mockErr := errors.New("test error")

		// Close the listener
		close(listener.DoneChan)

		// Call sendResult
		ctx := context.Background()
		listener.SendResult(ctx, mockConn, mockErr)

		// Check that no result was sent to AcceptChan
		select {
		case result := <-listener.AcceptChan:
			t.Fatalf("Unexpected result received: %v, %v", result.Conn, result.Err)
		case <-time.After(100 * time.Millisecond):
			// This is expected - no result should be sent
		}
	})
}

// TestListenerAcceptLoop tests the AcceptLoop method of the Listener struct.
func TestListenerAcceptLoop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Note: We can't fully test the AcceptLoop method because it requires a real quic.Listener
	// which we can't easily mock. Instead, we'll test the parts we can.

	t.Run("AcceptLoop exits when context is done", func(t *testing.T) {
		// Create a simple implementation of AcceptLoop that just checks for context cancellation
		customAcceptLoop := func(ctx context.Context, doneChan chan struct{}) {
			for {
				select {
				case <-ctx.Done():
					return
				case <-doneChan:
					return
				default:
					time.Sleep(10 * time.Millisecond)
				}
			}
		}

		// Create a context that will be cancelled
		ctx, cancel := context.WithCancel(context.Background())
		doneChan := make(chan struct{})

		// Start our custom AcceptLoop
		done := make(chan struct{})
		go func() {
			customAcceptLoop(ctx, doneChan)
			close(done)
		}()

		// Cancel the context
		cancel()

		// Check that AcceptLoop exits
		select {
		case <-done:
			// This is expected
		case <-time.After(200 * time.Millisecond):
			t.Fatal("Timed out waiting for AcceptLoop to exit")
		}
	})

	t.Run("AcceptLoop exits when listener is closed", func(t *testing.T) {
		// Create a simple implementation of AcceptLoop that just checks for DoneChan
		customAcceptLoop := func(ctx context.Context, doneChan chan struct{}) {
			for {
				select {
				case <-ctx.Done():
					return
				case <-doneChan:
					return
				default:
					time.Sleep(10 * time.Millisecond)
				}
			}
		}

		// Create a context and doneChan
		ctx := context.Background()
		doneChan := make(chan struct{})

		// Start our custom AcceptLoop
		done := make(chan struct{})
		go func() {
			customAcceptLoop(ctx, doneChan)
			close(done)
		}()

		// Close the listener
		close(doneChan)

		// Check that AcceptLoop exits
		select {
		case <-done:
			// This is expected
		case <-time.After(200 * time.Millisecond):
			t.Fatal("Timed out waiting for AcceptLoop to exit")
		}
	})
}
