package netceptor_test

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/netceptor/mock_netceptor"
	"github.com/quic-go/quic-go"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

type TestConn struct {
	pc netceptor.PacketConner
	qc netceptor.QuicConnectionForConn
	qs netceptor.QuicStreamForConn
}

func makeConn(t testing.TB, tc TestConn) *netceptor.Conn {
	t.Helper()
	conn := netceptor.NewConn(
		netceptor.New(context.TODO(), "test-node"), // netceptor
		tc.pc,                  // PacketConner
		tc.qc,                  // Connection
		tc.qs,                  // Stream
		make(chan struct{}, 1), // doneChan
		&sync.Once{},           // doneOnce
		context.TODO(),         // context
	)

	return conn
}

// These tests operate on the quic Stream.
func TestRead(t *testing.T) {
	ctrl := gomock.NewController(t)
	buf := make([]byte, 1)
	// Create a mock QuicStream
	mockQs := mock_netceptor.NewMockQuicStreamForConn(ctrl)
	// both success and error
	t.Run("Returns number of bytes from successful Read", func(t *testing.T) {
		want := 1
		mockQs.EXPECT().Read(gomock.Eq(buf)).Return(want, nil).Times(1)
		conn := makeConn(t, TestConn{qs: mockQs})
		got, err := conn.Read(buf)
		if err != nil {
			t.Fatalf("Read returned unexpected error %v", err)
		}
		if got != want {
			t.Errorf("Wanted %v, got %v", want, got)
		}
	})

	t.Run("Returns error from unsuccessful Read", func(t *testing.T) {
		wantErr := errors.New("Read error")
		mockQs.EXPECT().Read(gomock.Eq(buf)).Return(0, wantErr).Times(1)
		conn := makeConn(t, TestConn{qs: mockQs})
		_, gotErr := conn.Read(buf)
		if gotErr == nil {
			t.Errorf("Read did not return expected error")
		}
		if gotErr != wantErr {
			t.Errorf("Wanted %v, got %v", wantErr, gotErr)
		}
	})
}

func TestCancelRead(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockQs := mock_netceptor.NewMockQuicStreamForConn(ctrl)
	mockQs.EXPECT().CancelRead(gomock.Eq(quic.StreamErrorCode(499))).Times(1)
	conn := makeConn(t, TestConn{qs: mockQs})
	conn.CancelRead()
}

func TestWrite(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockQs := mock_netceptor.NewMockQuicStreamForConn(ctrl)
	bytes := []byte{4, 8, 15, 16, 23, 42}
	t.Run("Returns number of bytes written in successful Write", func(t *testing.T) {
		want := 6
		mockQs.EXPECT().Write(gomock.Eq(bytes)).Return(want, nil).Times(1)
		conn := makeConn(t, TestConn{qs: mockQs})
		got, err := conn.Write(bytes)
		if err != nil {
			t.Fatalf("Write returned unexpected error %v", err)
		}
		if got != want {
			t.Errorf("Wanted %v, got %v", want, got)
		}
	})
	t.Run("Returns error from unsuccessful Write", func(t *testing.T) {
		wantErr := errors.New("Write error")
		mockQs.EXPECT().Write(gomock.Eq(bytes)).Return(0, wantErr).Times(1)
		conn := makeConn(t, TestConn{qs: mockQs})
		_, gotErr := conn.Write(bytes)
		if gotErr == nil {
			t.Errorf("Write did not return expected error")
		}
		if gotErr != wantErr {
			t.Errorf("Wanted %v, got %v", wantErr, gotErr)
		}
	})
}

func TestClose(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockQs := mock_netceptor.NewMockQuicStreamForConn(ctrl)
	mockQs.EXPECT().Close().Return(nil)
	conn := makeConn(t, TestConn{qs: mockQs})
	err := conn.Close() // This calls the doneOnce and closes the doneChan
	// would be nice to test that the doneChan is closed
	if err != nil {
		t.Fatalf("conn.Close returned error %v", err)
	}
}

// These tests operate on the quic Connection.
func TestCloseConnection(t *testing.T) {
	ctrl := gomock.NewController(t)
	// quic Connection should be closed
	mockQc := mock_netceptor.NewMockQuicConnectionForConn(ctrl)
	mockQc.EXPECT().CloseWithError(quic.ApplicationErrorCode(0), gomock.Eq("normal close")).Return(nil).Times(1)

	// PacketConner should be cancelled
	mockPc := mock_netceptor.NewMockPacketConner(ctrl)
	mockPc.EXPECT().Cancel().Times(1)

	// The CloseConnection method logs some information to the netceptor's Logger, so mock them
	mockPc.EXPECT().LocalService().Return("test-local-service").Times(1)
	mockQc.EXPECT().RemoteAddr().Return(netceptor.Addr{})

	conn := makeConn(t, TestConn{pc: mockPc, qc: mockQc})
	err := conn.CloseConnection()
	if err != nil {
		t.Fatalf("conn.CloseConnection returned error %v", err)
	}
}

func TestLocalAddr(t *testing.T) {
	want := netceptor.Addr{} // Could mock the net interacted here rather than an empty Addr{}
	ctrl := gomock.NewController(t)
	mockQc := mock_netceptor.NewMockQuicConnectionForConn(ctrl)
	mockQc.EXPECT().LocalAddr().Return(want).Times(1)
	conn := makeConn(t, TestConn{qc: mockQc})
	got := conn.LocalAddr()
	if got != want {
		t.Errorf("Wanted %v, got %v", want, got)
	}
}

func TestRemoteAddr(t *testing.T) {
	want := netceptor.Addr{} // Could mock the net interacted here rather than an empty Addr{}
	ctrl := gomock.NewController(t)
	mockQc := mock_netceptor.NewMockQuicConnectionForConn(ctrl)
	mockQc.EXPECT().RemoteAddr().Return(want).Times(1)
	conn := makeConn(t, TestConn{qc: mockQc})
	got := conn.RemoteAddr()
	if got != want {
		t.Errorf("Wanted %v, got %v", want, got)
	}
}

func TestSetDeadline(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockQs := mock_netceptor.NewMockQuicStreamForConn(ctrl)
	want := time.Now().Add(10 * time.Second)
	t.Run("Returns no error after successful SetDeadline", func(t *testing.T) {
		mockQs.EXPECT().SetDeadline(gomock.Eq(want)).Return(nil)
		conn := makeConn(t, TestConn{qs: mockQs})
		err := conn.SetDeadline(want)
		if err != nil {
			t.Fatalf("conn.TestSetDeadline returned error %v", err)
		}
	})
	t.Run("Returns error from unsuccessful SetDeadline", func(t *testing.T) {
		wantErr := errors.New("SetDeadline error")
		mockQs.EXPECT().SetDeadline(gomock.Eq(want)).Return(wantErr)
		conn := makeConn(t, TestConn{qs: mockQs})
		gotErr := conn.SetDeadline(want)
		if gotErr != wantErr {
			t.Errorf("Wanted %v, got %v", wantErr, gotErr)
		}
	})
}

func TestSetReadDeadline(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockQs := mock_netceptor.NewMockQuicStreamForConn(ctrl)
	want := time.Now().Add(10 * time.Second)
	t.Run("Returns no error after successful SetReadDeadline", func(t *testing.T) {
		mockQs.EXPECT().SetReadDeadline(gomock.Eq(want)).Return(nil)
		conn := makeConn(t, TestConn{qs: mockQs})
		err := conn.SetReadDeadline(want)
		if err != nil {
			t.Fatalf("conn.SetReadDeadline returned error %v", err)
		}
	})
	t.Run("Returns error from unsuccessful SetReadDeadline", func(t *testing.T) {
		wantErr := errors.New("SetReadDeadline error")
		mockQs.EXPECT().SetReadDeadline(gomock.Eq(want)).Return(wantErr)
		conn := makeConn(t, TestConn{qs: mockQs})
		gotErr := conn.SetReadDeadline(want)
		if gotErr != wantErr {
			t.Errorf("Wanted %v, got %v", wantErr, gotErr)
		}
	})
}

func TestSetWriteDeadline(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockQs := mock_netceptor.NewMockQuicStreamForConn(ctrl)
	want := time.Now().Add(10 * time.Second)
	t.Run("Returns no error after successful SetWriteDeadline", func(t *testing.T) {
		mockQs.EXPECT().SetWriteDeadline(gomock.Eq(want)).Return(nil)
		conn := makeConn(t, TestConn{qs: mockQs})
		err := conn.SetWriteDeadline(want)
		if err != nil {
			t.Fatalf("conn.SetWriteDeadline returned error %v", err)
		}
	})
	t.Run("Returns error from unsuccessful SetWriteDeadline", func(t *testing.T) {
		wantErr := errors.New("SetWriteDeadline error")
		mockQs.EXPECT().SetWriteDeadline(gomock.Eq(want)).Return(wantErr)
		conn := makeConn(t, TestConn{qs: mockQs})
		gotErr := conn.SetWriteDeadline(want)
		if gotErr != wantErr {
			t.Errorf("Wanted %v, got %v", wantErr, gotErr)
		}
	})
}

func TestNewListener(t *testing.T) {
	tests := []struct {
		name             string
		netceptor        *netceptor.Netceptor
		validateChannels bool
		shouldNotBeNil   bool
	}{
		{
			name:             "creates listener with all fields set",
			netceptor:        &netceptor.Netceptor{},
			validateChannels: true,
			shouldNotBeNil:   true,
		},
		{
			name:           "creates listener with nil netceptor",
			netceptor:      nil,
			shouldNotBeNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPacketConner := mock_netceptor.NewMockPacketConner(ctrl)
			mockQL := mock_netceptor.NewMockQuicListenerForListener(ctrl)
			doneChan := make(chan struct{})
			acceptChan := make(chan *netceptor.AcceptResult)
			syncOnce := &sync.Once{}

			listener := netceptor.NewListener(tt.netceptor, mockPacketConner, mockQL, acceptChan, doneChan, syncOnce)

			if tt.shouldNotBeNil {
				if listener == nil {
					t.Error("NewListener should not return nil")
				}
			}

			if tt.validateChannels && listener != nil {
				if listener.AcceptChan != acceptChan {
					t.Error("AcceptChan not properly assigned")
				}
				if listener.DoneChan != doneChan {
					t.Error("DoneChan not properly assigned")
				}
			}
		})
	}
}

func TestListenerAddr(t *testing.T) {
	tests := []struct {
		name         string
		setupMock    func(*mock_netceptor.MockPacketConner)
		expectedAddr net.Addr
	}{
		{
			name: "returns non-nil addr",
			setupMock: func(mockPC *mock_netceptor.MockPacketConner) {
				testAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}
				mockPC.EXPECT().LocalAddr().Return(testAddr)
			},
			expectedAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080},
		},
		{
			name: "handles nil addr from PacketConner",
			setupMock: func(mockPC *mock_netceptor.MockPacketConner) {
				mockPC.EXPECT().LocalAddr().Return(nil)
			},
			expectedAddr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPacketConner := mock_netceptor.NewMockPacketConner(ctrl)
			mockNetC := &netceptor.Netceptor{}
			ql := &quic.Listener{}
			doneChan := make(chan struct{})
			acceptChan := make(chan *netceptor.AcceptResult)
			syncOnce := &sync.Once{}

			listener := netceptor.NewListener(mockNetC, mockPacketConner, ql, acceptChan, doneChan, syncOnce)

			tt.setupMock(mockPacketConner)

			got := listener.Addr()
			if !reflect.DeepEqual(got, tt.expectedAddr) {
				t.Errorf("Expected %v, got %v", tt.expectedAddr, got)
			}
		})
	}
}

func TestListenerAccept(t *testing.T) {
	tests := []struct {
		name          string
		setupAction   func(*netceptor.Listener)
		expectedError string
		expectedConn  bool
	}{
		{
			name: "accept channel error",
			setupAction: func(listener *netceptor.Listener) {
				go func() {
					listener.AcceptChan <- &netceptor.AcceptResult{
						Conn: nil,
						Err:  errors.New("accept channel error"),
					}
				}()
			},
			expectedError: "accept channel error",
		},
		{
			name: "accept channel closed",
			setupAction: func(listener *netceptor.Listener) {
				close(listener.AcceptChan)
			},
			expectedError: "listener accept channel closed",
		},
		{
			name: "done channel closed",
			setupAction: func(listener *netceptor.Listener) {
				close(listener.DoneChan)
			},
			expectedError: "listener done channel closed",
		},
		{
			name: "successful accept",
			setupAction: func(listener *netceptor.Listener) {
				go func() {
					listener.AcceptChan <- &netceptor.AcceptResult{
						Conn: &netceptor.Conn{},
						Err:  nil,
					}
				}()
			},
			expectedConn: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Common listener setup moved outside the table
			mockNetC := &netceptor.Netceptor{}
			ql := &quic.Listener{}
			doneChan := make(chan struct{})
			acceptChan := make(chan *netceptor.AcceptResult)
			syncOnce := &sync.Once{}
			listener := netceptor.NewListener(mockNetC, nil, ql, acceptChan, doneChan, syncOnce)

			tt.setupAction(listener)

			conn, err := listener.Accept()

			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("Expected error %q, got nil", tt.expectedError)
				} else if err.Error() != tt.expectedError {
					t.Errorf("Expected error %q, got %q", tt.expectedError, err.Error())
				}
			} else if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}

			if tt.expectedConn {
				if conn == nil {
					t.Error("Expected connection, got nil")
				}
			} else if conn != nil {
				t.Errorf("Expected no connection, got %v", conn)
			}
		})
	}
}

func TestListenerAcceptEdgeCases(t *testing.T) {
	tests := []struct {
		name              string
		setupAction       func(*netceptor.Listener)
		expectedError     string
		expectedConnCount int
		concurrent        bool
	}{
		{
			name: "accept returns nil AcceptResult",
			setupAction: func(listener *netceptor.Listener) {
				go func() {
					listener.AcceptChan <- nil
				}()
			},
			expectedError: "listener accept channel closed",
		},
		{
			name: "accept with successful connection",
			setupAction: func(listener *netceptor.Listener) {
				conn := &netceptor.Conn{}
				go func() {
					listener.AcceptChan <- &netceptor.AcceptResult{
						Conn: conn,
						Err:  nil,
					}
				}()
			},
			expectedConnCount: 1,
		},
		{
			name: "concurrent accepts",
			setupAction: func(listener *netceptor.Listener) {
				conn1 := &netceptor.Conn{}
				conn2 := &netceptor.Conn{}
				// Send two connections
				go func() {
					listener.AcceptChan <- &netceptor.AcceptResult{Conn: conn1, Err: nil}
					listener.AcceptChan <- &netceptor.AcceptResult{Conn: conn2, Err: nil}
				}()
			},
			expectedConnCount: 2,
			concurrent:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockNetC := &netceptor.Netceptor{}
			ql := &quic.Listener{}
			doneChan := make(chan struct{})
			acceptChan := make(chan *netceptor.AcceptResult, 2)
			syncOnce := &sync.Once{}

			listener := netceptor.NewListener(mockNetC, nil, ql, acceptChan, doneChan, syncOnce)

			tt.setupAction(listener)

			if tt.concurrent {
				// Handle concurrent accepts
				results := make(chan net.Conn, 2)
				errors := make(chan error, 2)

				for i := 0; i < tt.expectedConnCount; i++ {
					go func() {
						conn, err := listener.Accept()
						results <- conn
						errors <- err
					}()
				}

				// Collect results
				var conns []net.Conn
				for i := 0; i < tt.expectedConnCount; i++ {
					conn := <-results
					err := <-errors
					if err != nil {
						t.Errorf("Accept %d failed: %v", i, err)
					}
					conns = append(conns, conn)
				}

				if len(conns) != tt.expectedConnCount {
					t.Errorf("Expected %d connections, got %d", tt.expectedConnCount, len(conns))
				}
			} else {
				// Handle single accept
				conn, err := listener.Accept()

				if tt.expectedError != "" {
					if err == nil {
						t.Errorf("Expected error %q, got nil", tt.expectedError)
					} else if err.Error() != tt.expectedError {
						t.Errorf("Expected error %q, got %q", tt.expectedError, err.Error())
					}
				} else if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}

				if tt.expectedConnCount > 0 {
					if conn == nil {
						t.Error("Expected connection, got nil")
					}
				} else if conn != nil {
					t.Errorf("Expected no connection, got %v", conn)
				}
			}
		})
	}
}

func TestListenerAcceptWithContextCancellation(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockPacketConner := mock_netceptor.NewMockPacketConner(ctrl)
	mockNetC := &netceptor.Netceptor{}
	ql := &quic.Listener{}

	t.Run("accept blocks and then receives done signal", func(t *testing.T) {
		doneChan := make(chan struct{})
		acceptChan := make(chan *netceptor.AcceptResult)
		syncOnce := &sync.Once{}
		listener := netceptor.NewListener(mockNetC, mockPacketConner, ql, acceptChan, doneChan, syncOnce)
		resultChan := make(chan error, 1)
		expectErrMsg := "listener done channel closed"

		// Start accept in goroutine
		go func() {
			_, err := listener.Accept()
			resultChan <- err
		}()

		// Give Accept time to start blocking
		time.Sleep(10 * time.Millisecond)

		// Close done channel to signal shutdown
		close(doneChan)

		// Should receive error quickly
		select {
		case err := <-resultChan:
			if err == nil {
				t.Error("Expected error when done channel closed")
			}
			if err.Error() != expectErrMsg {
				t.Errorf("Expected '%s', got %v", expectErrMsg, err)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("Accept should have returned quickly after done channel closed")
		}
	})
}

func TestListenerClose(t *testing.T) {
	tests := []struct {
		name             string
		setupMocks       func(mockPC *mock_netceptor.MockPacketConner, mockQL *mock_netceptor.MockQuicListenerForListener)
		expectedError    string
		multipleClose    bool
		validateDoneChan bool
	}{
		{
			name: "packetconner error",
			setupMocks: func(mockPC *mock_netceptor.MockPacketConner, mockQL *mock_netceptor.MockQuicListenerForListener) {
				mockQL.EXPECT().Close().Return(nil)
				mockPC.EXPECT().Close().Return(errors.New("packetconner error"))
			},
			expectedError: "packetconner error",
		},
		{
			name: "quiclistener error",
			setupMocks: func(mockPC *mock_netceptor.MockPacketConner, mockQL *mock_netceptor.MockQuicListenerForListener) {
				mockPC.EXPECT().Close().Return(nil)
				mockQL.EXPECT().Close().Return(errors.New("quiclistener error"))
			},
			expectedError: "quiclistener error",
		},
		{
			name: "successful close",
			setupMocks: func(mockPC *mock_netceptor.MockPacketConner, mockQL *mock_netceptor.MockQuicListenerForListener) {
				mockPC.EXPECT().Close().Return(nil)
				mockQL.EXPECT().Close().Return(nil)
			},
			validateDoneChan: true,
		},
		{
			name: "multiple close calls are safe",
			setupMocks: func(mockPC *mock_netceptor.MockPacketConner, mockQL *mock_netceptor.MockQuicListenerForListener) {
				mockPC.EXPECT().Close().Return(nil).Times(2)
				mockQL.EXPECT().Close().Return(nil).Times(2)
			},
			multipleClose:    true,
			validateDoneChan: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPacketConner := mock_netceptor.NewMockPacketConner(ctrl)
			mockListener := mock_netceptor.NewMockQuicListenerForListener(ctrl)
			mockNetC := &netceptor.Netceptor{}
			doneChan := make(chan struct{})
			acceptChan := make(chan *netceptor.AcceptResult)
			syncOnce := &sync.Once{}

			listener := netceptor.NewListener(mockNetC, mockPacketConner, mockListener, acceptChan, doneChan, syncOnce)

			tt.setupMocks(mockPacketConner, mockListener)

			// First close
			err1 := listener.Close()
			if tt.expectedError != "" {
				if err1 == nil {
					t.Errorf("Expected error %q, got nil", tt.expectedError)
				} else if err1.Error() != tt.expectedError {
					t.Errorf("Expected error %q, got %q", tt.expectedError, err1.Error())
				}
			} else if err1 != nil {
				t.Errorf("Expected no error, got %v", err1)
			}

			// Test multiple close if requested
			if tt.multipleClose {
				err2 := listener.Close()
				if err2 != nil {
					t.Errorf("Second close should not error, got %v", err2)
				}
			}

			// Validate DoneChan is closed if requested
			if tt.validateDoneChan {
				select {
				case <-listener.DoneChan:
					// Expected - channel should be closed
				default:
					t.Error("DoneChan should be closed after Close()")
				}
			}
		})
	}
}

func TestListenerCloseErrorPrecedence(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(mockPC *mock_netceptor.MockPacketConner, mockQL *mock_netceptor.MockQuicListenerForListener)
		expectedError string
	}{
		{
			name: "quic listener error takes precedence over packet conner error",
			setupMocks: func(mockPC *mock_netceptor.MockPacketConner, mockQL *mock_netceptor.MockQuicListenerForListener) {
				pcErr := errors.New("packet conner error")
				qlErr := errors.New("quic listener error")
				mockPC.EXPECT().Close().Return(pcErr)
				mockQL.EXPECT().Close().Return(qlErr)
			},
			expectedError: "quic listener error",
		},
		{
			name: "packet conner error returned when quic listener succeeds",
			setupMocks: func(mockPC *mock_netceptor.MockPacketConner, mockQL *mock_netceptor.MockQuicListenerForListener) {
				pcErr := errors.New("packet conner error")
				mockPC.EXPECT().Close().Return(pcErr)
				mockQL.EXPECT().Close().Return(nil)
			},
			expectedError: "packet conner error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPacketConner := mock_netceptor.NewMockPacketConner(ctrl)
			mockListener := mock_netceptor.NewMockQuicListenerForListener(ctrl)
			mockNetC := &netceptor.Netceptor{}
			doneChan := make(chan struct{})
			acceptChan := make(chan *netceptor.AcceptResult)
			syncOnce := &sync.Once{}

			listener := netceptor.NewListener(mockNetC, mockPacketConner, mockListener, acceptChan, doneChan, syncOnce)

			tt.setupMocks(mockPacketConner, mockListener)

			gotErr := listener.Close()
			if gotErr == nil {
				t.Errorf("Expected error %q, got nil", tt.expectedError)
			} else if gotErr.Error() != tt.expectedError {
				t.Errorf("Expected error %q, got %q", tt.expectedError, gotErr.Error())
			}
		})
	}
}

func TestNeceptorListen(t *testing.T) {
	t.Parallel()
	t.Run("service is already listening", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		mockNetC := netceptor.New(ctx, "node1")
		wantErr := errors.New("service node1 is already listening")
		_, _ = mockNetC.Listen("node1", &tls.Config{})
		_, gotErr := mockNetC.Listen("node1", &tls.Config{})
		if gotErr.Error() != wantErr.Error() {
			t.Errorf("Wanted %v, got %v", wantErr, gotErr)
		}
	})

	t.Run("context cancelled does not panic", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		mockNetC := netceptor.New(ctx, "nodecc")
		_, _ = mockNetC.Listen("nodecc", &tls.Config{})
		// Assert cancelling netceptor context doesn't create panic
		assert.NotPanics(t, func() { time.AfterFunc(500*time.Millisecond, cancel) })
	})
}
