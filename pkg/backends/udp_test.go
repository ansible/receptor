package backends

import (
	"context"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
)

func TestNewUDPListener(t *testing.T) {
	type args struct {
		address string
		logger  *logger.ReceptorLogger
	}

	tests := []struct {
		name    string
		args    args
		want    *UDPListener
		wantErr bool
	}{
		{
			name: "Positive",
			args: args{
				address: "127.0.0.1:9997",
				logger:  logger.NewReceptorLogger("UDPtest"),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewUDPListener(tt.args.address, tt.args.logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewUDPListener() error = %+v, wantErr %+v", err, tt.wantErr)

				return
			}
			if got == nil {
				t.Errorf("NewUDPListener(): want UDP Listener, got nil")
			}
		})
	}
}

func TestUDPListenerStart(t *testing.T) {
	type fields struct {
		laddr           *net.UDPAddr
		conn            *net.UDPConn
		sessChan        chan *UDPListenerSession
		sessionRegistry map[string]*UDPListenerSession
		logger          *logger.ReceptorLogger
	}

	type args struct {
		ctx context.Context
		wg  *sync.WaitGroup
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    chan netceptor.BackendSession
		wantErr bool
	}{
		{
			name: "Positive",
			fields: fields{
				laddr: &net.UDPAddr{
					IP:   net.IPv4(127, 0, 0, 1),
					Port: 9999,
					Zone: "",
				},
				conn:            &net.UDPConn{},
				sessChan:        make(chan *UDPListenerSession),
				sessionRegistry: make(map[string]*UDPListenerSession),
				logger:          logger.NewReceptorLogger("UDPtest"),
			},
			args: args{
				ctx: context.Background(),
				wg:  &sync.WaitGroup{},
			},
			want:    make(chan netceptor.BackendSession),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, err := net.ListenUDP("udp", tt.fields.laddr)
			if err != nil {
				t.Errorf("ListenUDP error = %+v", err)
			}

			b := &UDPListener{
				laddr:           tt.fields.laddr,
				conn:            uc,
				sessChan:        tt.fields.sessChan,
				sessRegLock:     sync.RWMutex{},
				sessionRegistry: tt.fields.sessionRegistry,
				logger:          tt.fields.logger,
			}
			got, err := b.Start(tt.args.ctx, tt.args.wg)
			if (err != nil) != tt.wantErr {
				t.Errorf("UDPListener.Start() error = %+v, wantErr %+v", err, tt.wantErr)

				return
			}
			if got == nil {
				t.Errorf("UDPListener.Start() returned nil")
			}
		})
	}
}

func TestUDPDialerStart(t *testing.T) {
	type fields struct {
		address string
		redial  bool
		logger  *logger.ReceptorLogger
	}
	type args struct {
		ctx context.Context
		wg  *sync.WaitGroup
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    chan netceptor.BackendSession
		wantErr bool
	}{
		{
			name: "Positive",
			fields: fields{
				address: "127.0.0.1:9998",
				redial:  true,
				logger:  logger.NewReceptorLogger("UDPtest"),
			},
			args: args{
				ctx: context.Background(),
				wg:  &sync.WaitGroup{},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &UDPDialer{
				address: tt.fields.address,
				redial:  tt.fields.redial,
				logger:  tt.fields.logger,
			}
			got, err := b.Start(tt.args.ctx, tt.args.wg)
			if (err != nil) != tt.wantErr {
				t.Errorf("UDPDialer.Start() error = %+v, wantErr %+v", err, tt.wantErr)

				return
			}
			if got == nil {
				t.Errorf("UDPDialer.Start() returned nil")
			}
		})
	}
}

// setupUDPServer creates a UDP listener for testing.
func setupUDPServer(t *testing.T) *net.UDPConn {
	t.Helper()

	serverAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to resolve server address: %v", err)
	}
	serverConn, err := net.ListenUDP("udp", serverAddr)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	return serverConn
}

// setupUDPClient creates a UDP client connected to the given server address.
func setupUDPClient(t *testing.T, serverAddr net.Addr) *net.UDPConn {
	t.Helper()

	clientConn, err := net.Dial("udp", serverAddr.String())
	if err != nil {
		t.Fatalf("Failed to dial server: %v", err)
	}

	udpClientConn, ok := clientConn.(*net.UDPConn)
	if !ok {
		t.Fatal("Failed to cast to UDPConn")
	}

	return udpClientConn
}

// newTestUDPDialerSession creates a UDPDialerSession for testing.
func newTestUDPDialerSession(clientConn *net.UDPConn) *UDPDialerSession {
	return &UDPDialerSession{
		conn:            clientConn,
		closeChan:       make(chan struct{}),
		closeChanCloser: sync.Once{},
	}
}

// setupUDPDialerSessionTest sets up a complete test environment with server, client, and session.
func setupUDPDialerSessionTest(t *testing.T) (func(), *net.UDPConn, *net.UDPConn, *UDPDialerSession) {
	t.Helper()

	serverConn := setupUDPServer(t)
	clientConn := setupUDPClient(t, serverConn.LocalAddr())
	session := newTestUDPDialerSession(clientConn)

	cleanup := func() {
		clientConn.Close()
		serverConn.Close()
	}

	return cleanup, serverConn, clientConn, session
}

// checkTestError is a helper function that verifies error expectations in tests.
func checkTestError(t *testing.T, err error, wantErr bool, errMsg, funcName string) {
	t.Helper()

	if (err != nil) != wantErr {
		t.Errorf("%s error = %v, wantErr %v", funcName, err, wantErr)

		return
	}

	if wantErr && errMsg != "" && err != nil {
		if !strings.Contains(err.Error(), errMsg) {
			t.Errorf("%s error = %v, want error containing %q", funcName, err, errMsg)
		}
	}
}

func TestUDPDialer_GetAddr(t *testing.T) {
	tests := []struct {
		name    string
		address string
	}{
		{
			name:    "IPv4 address",
			address: "127.0.0.1:8080",
		},
		{
			name:    "IPv6 address",
			address: "[::1]:8080",
		},
		{
			name:    "Hostname",
			address: "example.com:443",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := NewUDPDialer(tt.address, true, nil)
			if err != nil {
				t.Errorf("NewUDPDialer() error = %v", err)

				return
			}
			if got := b.GetAddr(); got != tt.address {
				t.Errorf("UDPDialer.GetAddr() = %v, want %v", got, tt.address)
			}
		})
	}
}

func TestUDPDialer_GetTLS(t *testing.T) {
	b := &UDPDialer{}

	if got := b.GetTLS(); got != nil {
		t.Errorf("UDPDialer.GetTLS() = %v, want nil", got)
	}
}

func TestNewUDPDialer(t *testing.T) {
	type args struct {
		address string
		redial  bool
		logger  *logger.ReceptorLogger
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Valid address",
			args: args{
				address: "127.0.0.1:8080",
				redial:  true,
				logger:  nil,
			},
			wantErr: false,
		},
		{
			name: "Invalid format",
			args: args{
				address: "not:a:valid:address:format",
				redial:  true,
				logger:  nil,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewUDPDialer(tt.args.address, tt.args.redial, tt.args.logger)
			checkTestError(t, err, tt.wantErr, "", "NewUDPDialer()")

			if !tt.wantErr && got == nil {
				t.Errorf("NewUDPDialer() returned nil, want UDPDialer")
			}
			if tt.wantErr && got != nil {
				t.Errorf("NewUDPDialer() returned %v, want nil", got)
			}
		})
	}
}

type udpDialerSessionTest struct {
	name                string
	data                []byte
	timeout             time.Duration
	closeConn           bool
	closeConnDuringRead bool
	shouldSend          bool
	wantErr             bool
	wantTimeout         bool
	errMsg              string
	multipleCalls       bool
}

func TestUDPDialerSession_Send(t *testing.T) {
	tests := []udpDialerSessionTest{
		{
			name:      "Send normal message",
			data:      []byte("hello"),
			closeConn: false,
			wantErr:   false,
		},
		{
			name:      "Send to closed connection",
			data:      []byte("test"),
			closeConn: true,
			wantErr:   true,
			errMsg:    "use of closed network connection",
		},
		{
			name:      "Send data too large",
			data:      make([]byte, UDPMaxPacketLen+1),
			closeConn: false,
			wantErr:   true,
			errMsg:    "data too large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup, serverConn, udpClientConn, session := setupUDPDialerSessionTest(t)
			defer cleanup()

			if tt.closeConn {
				udpClientConn.Close()
			}

			checkTestError(t, session.Send(tt.data), tt.wantErr, tt.errMsg, "UDPDialerSession.Send()")

			if !tt.wantErr && !tt.closeConn && len(tt.data) <= UDPMaxPacketLen {
				buf := make([]byte, UDPMaxPacketLen)
				serverConn.SetReadDeadline(time.Now().Add(1 * time.Second))
				n, _, err := serverConn.ReadFromUDP(buf)
				if err != nil {
					t.Errorf("Server failed to receive data: %v", err)
				}
				if string(buf[:n]) != string(tt.data) {
					t.Errorf("Received data = %v, want %v", string(buf[:n]), string(tt.data))
				}
			}
		})
	}
}

func TestUDPDialerSession_Recv(t *testing.T) {
	tests := []udpDialerSessionTest{
		{
			name:       "Receive data successfully",
			data:       []byte("test message"),
			timeout:    1 * time.Second,
			closeConn:  false,
			shouldSend: true,
			wantErr:    false,
		},
		{
			name:        "Timeout when no data available",
			data:        []byte(""),
			timeout:     100 * time.Millisecond,
			closeConn:   false,
			shouldSend:  false,
			wantErr:     true,
			wantTimeout: true,
		},
		{
			name:       "SetReadDeadline error on closed connection",
			data:       []byte(""),
			timeout:    1 * time.Second,
			closeConn:  true,
			shouldSend: false,
			wantErr:    true,
		},
		{
			name:                "Read error after connection closed during read",
			data:                []byte{},
			timeout:             2 * time.Second,
			closeConn:           false,
			closeConnDuringRead: true,
			shouldSend:          false,
			wantErr:             true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup, serverConn, udpClientConn, session := setupUDPDialerSessionTest(t)
			defer cleanup()

			if tt.shouldSend {
				go func() {
					time.Sleep(50 * time.Millisecond) // Small delay to ensure Recv is called first
					_, err := serverConn.WriteTo(tt.data, udpClientConn.LocalAddr())
					if err != nil {
						t.Logf("Server failed to send data: %v", err)
					}
				}()
			}

			if tt.closeConn {
				udpClientConn.Close()
			}

			if tt.closeConnDuringRead {
				go func() {
					time.Sleep(50 * time.Millisecond)
					udpClientConn.Close()
				}()
			}

			data, err := session.Recv(tt.timeout)
			checkTestError(t, err, tt.wantErr, tt.errMsg, "UDPDialerSession.Recv()")

			if tt.wantTimeout {
				if err != netceptor.ErrTimeout {
					t.Errorf("UDPDialerSession.Recv() error = %v, want netceptor.ErrTimeout", err)
				}
			}

			if !tt.wantErr {
				if string(data) != string(tt.data) {
					t.Errorf("UDPDialerSession.Recv() data = %v, want %v", string(data), string(tt.data))
				}
			}
		})
	}
}

func TestUDPDialerSession_Close(t *testing.T) {
	tests := []udpDialerSessionTest{
		{
			name:    "Close successfully",
			wantErr: false,
		},
		{
			name:          "Close multiple times (idempotent)",
			wantErr:       false,
			multipleCalls: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup, _, _, session := setupUDPDialerSessionTest(t)
			defer cleanup()

			closeChan := session.closeChan
			checkTestError(t, session.Close(), tt.wantErr, tt.errMsg, "UDPDialerSession.Close()")

			if !tt.multipleCalls {
				select {
				case <-closeChan:
				default:
					t.Error("UDPDialerSession.Close() closeChan should be closed")
				}
			}

			if session.closeChan != nil {
				t.Error("UDPDialerSession.Close() closeChan should be nil after close")
			}

			if tt.multipleCalls {
				for callNumber := 2; callNumber <= 3; callNumber++ {
					err := session.Close()
					if err == nil {
						t.Errorf("UDPDialerSession.Close() call #%d expected error for closed connection, got nil", callNumber)
					}
				}
			}
		})
	}
}
