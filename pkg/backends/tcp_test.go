package backends

import (
	"context"
	"crypto/tls"
	"net"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/logger"
)

func TestTCPDialer_GetAddr(t *testing.T) {
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
			b, err := NewTCPDialer(tt.address, true, nil, nil)
			if err != nil {
				t.Errorf("NewTCPDialer() error = %v", err)

				return
			}
			if got := b.GetAddr(); got != tt.address {
				t.Errorf("TCPDialer.GetAddr() = %v, want %v", got, tt.address)
			}
		})
	}
}

func TestTCPDialer_GetTLS(t *testing.T) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	tests := []struct {
		name string
		tls  *tls.Config
	}{
		{
			name: "No TLS",
			tls:  nil,
		},
		{
			name: "With TLS config",
			tls:  tlsConfig,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := NewTCPDialer("127.0.0.1:8080", true, tt.tls, nil)
			if err != nil {
				t.Errorf("NewTCPDialer() error = %v", err)

				return
			}
			if got := b.GetTLS(); !reflect.DeepEqual(got, tt.tls) {
				t.Errorf("TCPDialer.GetTLS() = %v, want %v", got, tt.tls)
			}
		})
	}
}

func TestTCPDialerStart(t *testing.T) {
	tests := []struct {
		name    string
		address string
		redial  bool
		tls     *tls.Config
		wantErr bool
	}{
		{
			name:    "Successful start",
			address: "127.0.0.1:0",
			redial:  true,
			tls:     nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := NewTCPDialer(tt.address, tt.redial, tt.tls, logger.NewReceptorLogger("TCPtest"))
			if err != nil {
				t.Errorf("NewTCPDialer() error = %v", err)

				return
			}

			ctx, cancel := context.WithCancel(context.Background())
			wg := &sync.WaitGroup{}
			defer func() {
				cancel()
				wg.Wait()
			}()

			got, err := b.Start(ctx, wg)
			if (err != nil) != tt.wantErr {
				t.Errorf("TCPDialer.Start() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if got == nil {
				t.Errorf("TCPDialer.Start() returned nil channel")
			}
		})
	}
}

func TestNewTCPListener(t *testing.T) {
	type args struct {
		address string
		tls     *tls.Config
		logger  *logger.ReceptorLogger
	}
	tests := []struct {
		name    string
		args    args
		want    *TCPListener
		wantErr bool
	}{
		{
			name: "Positive",
			args: args{
				address: "127.0.0.1:9999",
				tls:     nil,
				logger:  nil,
			},
			want: &TCPListener{
				address: "127.0.0.1:9999",
				TLS:     nil,
				li:      nil,
				innerLi: nil,
				logger:  nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewTCPListener(tt.args.address, tt.args.tls, tt.args.logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewTCPListener() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewTCPListener() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestTCPListener_GetAddr(t *testing.T) {
	tests := []struct {
		name         string
		address      string
		expectedHost string
	}{
		{
			name:         "IPv4 address with port 0",
			address:      "127.0.0.1:0",
			expectedHost: "127.0.0.1",
		},
		{
			name:         "IPv6 address with port 0",
			address:      "[::1]:0",
			expectedHost: "::1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := NewTCPListener(tt.address, nil, logger.NewReceptorLogger("TCPtest"))
			if err != nil {
				t.Errorf("NewTCPListener() error = %v", err)

				return
			}

			// Address is bound after listener start
			ctx, cancel := context.WithCancel(context.Background())
			wg := &sync.WaitGroup{}
			defer func() {
				cancel()
				wg.Wait()
			}()
			_, err = b.Start(ctx, wg)
			if err != nil {
				t.Errorf("TCPListener.Start() error = %v", err)

				return
			}

			got := b.GetAddr()
			if got == "" {
				t.Errorf("TCPListener.GetAddr() returned empty string")

				return
			}

			host, _, err := net.SplitHostPort(got)
			if err != nil {
				t.Errorf("TCPListener.GetAddr() returned invalid address format %q: %v", got, err)

				return
			}

			if host != tt.expectedHost {
				t.Errorf("TCPListener.GetAddr() host = %v, want %v", host, tt.expectedHost)
			}
		})
	}
}

func TestTCPListener_GetTLS(t *testing.T) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	tests := []struct {
		name string
		tls  *tls.Config
		want *tls.Config
	}{
		{
			name: "No TLS",
			tls:  nil,
			want: nil,
		},
		{
			name: "With TLS config",
			tls:  tlsConfig,
			want: tlsConfig,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := NewTCPListener("127.0.0.1:0", tt.tls, nil)
			if err != nil {
				t.Errorf("NewTCPListener() error = %v", err)

				return
			}
			if got := b.GetTLS(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TCPListener.GetTLS() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTCPListenerStart(t *testing.T) {
	tests := []struct {
		name    string
		address string
		tls     *tls.Config
		wantErr bool
	}{
		{
			name:    "Successful start",
			address: "127.0.0.1:0",
			tls:     nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := NewTCPListener(tt.address, tt.tls, logger.NewReceptorLogger("TCPtest"))
			if err != nil {
				t.Errorf("NewTCPListener() error = %v", err)

				return
			}

			ctx, cancel := context.WithCancel(context.Background())
			wg := &sync.WaitGroup{}
			defer func() {
				cancel()
				wg.Wait()
			}()

			got, err := b.Start(ctx, wg)
			if (err != nil) != tt.wantErr {
				t.Errorf("TCPListener.Start() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if got == nil {
				t.Errorf("TCPListener.Start() returned nil channel")
			}
		})
	}
}

func TestNewTCPSession(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	closeChan := make(chan struct{})

	session := newTCPSession(client, closeChan)

	if session == nil {
		t.Fatal("NewTCPSession() returned nil")
	}

	if session.conn != client {
		t.Errorf("NewTCPSession() conn = %v, want %v", session.conn, client)
	}

	if session.framer == nil {
		t.Error("NewTCPSession() framer is nil, expected non-nil")
	}

	if session.closeChan == nil {
		t.Error("NewTCPSession() closeChan is nil, expected non-nil")
	}
}

func TestTCPSession_Send(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		closeConn bool
		wantErr   bool
	}{
		{
			name:      "Send message",
			data:      []byte("hello"),
			closeConn: false,
			wantErr:   false,
		},
		{
			name:      "Send to closed connection",
			data:      []byte("test"),
			closeConn: true,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := net.Pipe()
			defer server.Close()
			defer client.Close()

			senderSession := newTCPSession(client, nil)
			receiverSession := newTCPSession(server, nil)

			if tt.closeConn {
				client.Close()
			}

			receiverDone := make(chan struct{})
			// Start receiver goroutine
			go func() {
				defer close(receiverDone)
				_, recvErr := receiverSession.Recv(500 * time.Millisecond)
				if recvErr != nil && !tt.wantErr {
					t.Errorf("%s: TCPSession.Recv() error = %v", tt.name, recvErr)
				}
			}()

			// Ensure goroutine cleanup
			defer func() {
				<-receiverDone
			}()
			sendErr := senderSession.Send(tt.data)

			if (sendErr != nil) != tt.wantErr {
				t.Errorf("%s: TCPSession.Send() error = %v, wantErr %v", tt.name, sendErr, tt.wantErr)
			}
		})
	}
}

func TestTCPSession_Recv(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		closeConn bool
		wantErr   bool
	}{
		{
			name:      "Receive message",
			data:      []byte("hello"),
			closeConn: false,
			wantErr:   false,
		},
		{
			name:      "Receive from closed connection",
			data:      []byte("test"),
			closeConn: true,
			wantErr:   true,
		},
		{
			name:      "Receive timeout",
			data:      nil,
			closeConn: false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := net.Pipe()
			defer server.Close()
			defer client.Close()

			senderSession := newTCPSession(client, nil)
			receiverSession := newTCPSession(server, nil)

			if tt.closeConn {
				server.Close()
			}

			// Channel to signal completion of Recv operation
			done := make(chan struct{})
			var received []byte
			var recvErr error

			// Start receiver goroutine
			go func() {
				received, recvErr = receiverSession.Recv(500 * time.Millisecond)
				close(done)
			}()

			// Send data only if we have data to send (skip for timeout test)
			// For timeout test: data is nil, so we skip sending and let Recv timeout
			if tt.data != nil && !tt.closeConn {
				sendErr := senderSession.Send(tt.data)
				if sendErr != nil {
					t.Errorf("%s: unexpected send error: %v", tt.name, sendErr)

					return
				}
			}

			// Wait for receiver goroutine to complete
			<-done

			if (recvErr != nil) != tt.wantErr {
				t.Errorf("%s: TCPSession.Recv() error = %v, wantErr %v", tt.name, recvErr, tt.wantErr)

				return
			}

			if !tt.wantErr {
				if !reflect.DeepEqual(received, tt.data) {
					t.Errorf("%s: Received data = %q, want %q", tt.name, received, tt.data)
				}
			}
		})
	}
}

func TestTCPSession_Close(t *testing.T) {
	tests := []struct {
		name          string
		withCloseChan bool
		multipleCalls bool
	}{
		{
			name:          "Close with closeChan",
			withCloseChan: true,
			multipleCalls: false,
		},
		{
			name:          "Close without closeChan",
			withCloseChan: false,
			multipleCalls: false,
		},
		{
			name:          "Multiple Close calls",
			withCloseChan: true,
			multipleCalls: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := net.Pipe()
			defer client.Close()
			defer server.Close()

			var closeChan chan struct{}
			if tt.withCloseChan {
				closeChan = make(chan struct{})
			}

			session := newTCPSession(client, closeChan)

			err := session.Close()
			if err != nil {
				t.Errorf("First TCPSession.Close() error = %v", err)
			}

			if tt.withCloseChan {
				select {
				// Verify channel is closed
				case <-closeChan:
				default:
					t.Error("closeChan was not closed")
				}
			}

			// Check if connection is actually closed by trying to read from it
			buf := make([]byte, 1)
			_, readErr := client.Read(buf)
			if readErr == nil {
				t.Error("connection was not closed")
			}

			if tt.multipleCalls {
				// Calling Close() multiple times should be idempotent
				session.Close()

				if tt.withCloseChan {
					select {
					case <-closeChan:
					default:
						t.Error("closeChan should remain closed")
					}
				}
			}
		})
	}
}

func TestTCPListenerCfg_GetCost(t *testing.T) {
	tests := []struct {
		name string
		cfg  TCPListenerCfg
	}{
		{
			name: "Positive cost",
			cfg: TCPListenerCfg{
				Cost: 1.0,
			},
		},
		{
			name: "Zero cost",
			cfg: TCPListenerCfg{
				Cost: 0.0,
			},
		},
		{
			name: "Negative cost",
			cfg: TCPListenerCfg{
				Cost: -1.0,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.GetCost(); got != tt.cfg.Cost {
				t.Errorf("TCPListenerCfg.GetCost() = %v, want %v", got, tt.cfg.Cost)
			}
		})
	}
}

func TestTCPListenerCfg_GetNodeCost(t *testing.T) {
	tests := []struct {
		name string
		cfg  TCPListenerCfg
	}{
		{
			name: "No node costs",
			cfg: TCPListenerCfg{
				NodeCost: nil,
			},
		},
		{
			name: "Empty node costs",
			cfg: TCPListenerCfg{
				NodeCost: map[string]float64{},
			},
		},
		{
			name: "Single node cost",
			cfg: TCPListenerCfg{
				NodeCost: map[string]float64{
					"node1": 1.5,
				},
			},
		},
		{
			name: "Multiple node costs",
			cfg: TCPListenerCfg{
				NodeCost: map[string]float64{
					"node1": 1.5,
					"node2": 2.0,
					"node3": 3.5,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.GetNodeCost(); !reflect.DeepEqual(got, tt.cfg.NodeCost) {
				t.Errorf("TCPListenerCfg.GetNodeCost() = %v, want %v", got, tt.cfg.NodeCost)
			}
		})
	}
}

func TestTCPListenerCfg_GetAddr(t *testing.T) {
	tests := []struct {
		name string
		cfg  TCPListenerCfg
	}{
		{
			name: "Empty address",
			cfg: TCPListenerCfg{
				BindAddr: "",
			},
		},
		{
			name: "IPv4 address",
			cfg: TCPListenerCfg{
				BindAddr: "127.0.0.1",
			},
		},
		{
			name: "IPv6 address",
			cfg: TCPListenerCfg{
				BindAddr: "::1",
			},
		},
		{
			name: "Wildcard address",
			cfg: TCPListenerCfg{
				BindAddr: "0.0.0.0",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.GetAddr(); got != tt.cfg.BindAddr {
				t.Errorf("TCPListenerCfg.GetAddr() = %v, want %v", got, tt.cfg.BindAddr)
			}
		})
	}
}

func TestTCPListenerCfg_GetTLS(t *testing.T) {
	tests := []struct {
		name string
		cfg  TCPListenerCfg
	}{
		{
			name: "Empty TLS",
			cfg: TCPListenerCfg{
				TLS: "",
			},
		},
		{
			name: "TLS config name",
			cfg: TCPListenerCfg{
				TLS: "tls-server-config",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.GetTLS(); got != tt.cfg.TLS {
				t.Errorf("TCPListenerCfg.GetTLS() = %v, want %v", got, tt.cfg.TLS)
			}
		})
	}
}

func TestTCPListenerCfg_Prepare(t *testing.T) {
	tests := []struct {
		name    string
		cfg     TCPListenerCfg
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid config with positive cost",
			cfg: TCPListenerCfg{
				Cost: 1.0,
			},
			wantErr: false,
		},
		{
			name: "Valid config with positive cost and node costs",
			cfg: TCPListenerCfg{
				Cost: 1.0,
				NodeCost: map[string]float64{
					"node1": 1.5,
					"node2": 2.0,
				},
			},
			wantErr: false,
		},
		{
			name: "Invalid: zero cost",
			cfg: TCPListenerCfg{
				Cost: 0.0,
			},
			wantErr: true,
			errMsg:  "connection cost must be positive",
		},
		{
			name: "Invalid: negative cost",
			cfg: TCPListenerCfg{
				Cost: -1.0,
			},
			wantErr: true,
			errMsg:  "connection cost must be positive",
		},
		{
			name: "Invalid: zero node cost",
			cfg: TCPListenerCfg{
				Cost: 1.0,
				NodeCost: map[string]float64{
					"node1": 0.0,
				},
			},
			wantErr: true,
			errMsg:  "connection cost must be positive for node1",
		},
		{
			name: "Invalid: negative node cost",
			cfg: TCPListenerCfg{
				Cost: 1.0,
				NodeCost: map[string]float64{
					"node1": -1.0,
				},
			},
			wantErr: true,
			errMsg:  "connection cost must be positive for node1",
		},
		{
			name: "Invalid: one valid and one invalid node cost",
			cfg: TCPListenerCfg{
				Cost: 1.0,
				NodeCost: map[string]float64{
					"node1": 1.5,
					"node2": -0.5,
				},
			},
			wantErr: true,
			errMsg:  "connection cost must be positive for node2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Prepare()
			if (err != nil) != tt.wantErr {
				t.Errorf("TCPListenerCfg.Prepare() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if tt.wantErr && err.Error() != tt.errMsg {
				t.Errorf("TCPListenerCfg.Prepare() error message = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestTCPDialerCfg_Prepare(t *testing.T) {
	tests := []struct {
		name    string
		cfg     TCPDialerCfg
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid config with positive cost",
			cfg: TCPDialerCfg{
				Cost: 1.0,
			},
			wantErr: false,
		},
		{
			name: "Invalid: zero cost",
			cfg: TCPDialerCfg{
				Cost: 0.0,
			},
			wantErr: true,
			errMsg:  "connection cost must be positive",
		},
		{
			name: "Invalid: negative cost",
			cfg: TCPDialerCfg{
				Cost: -1.0,
			},
			wantErr: true,
			errMsg:  "connection cost must be positive",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Prepare()
			if (err != nil) != tt.wantErr {
				t.Errorf("TCPDialerCfg.Prepare() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if tt.wantErr && err.Error() != tt.errMsg {
				t.Errorf("TCPDialerCfg.Prepare() error message = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}
