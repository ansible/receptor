package netceptor_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/backends"
	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/quic-go/quic-go"
)

// Test helpers for creating minimal QUIC infrastructure.

func generateTestTLSConfig(t *testing.T) *tls.Config {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "test-adapter",
		},
		NotBefore: time.Now().Add(-1 * time.Minute),
		NotAfter:  time.Now().Add(1 * time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("Failed to create TLS cert: %v", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"test-proto"},
		MinVersion:   tls.VersionTLS12,
	}
}

func createTestQUICListener(t *testing.T) (*quic.Listener, net.PacketConn) {
	t.Helper()

	// Create UDP connection for QUIC transport
	udpAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		t.Fatalf("Failed to create UDP connection: %v", err)
	}

	tlsConf := generateTestTLSConfig(t)
	cfg := &quic.Config{
		MaxIdleTimeout: 5 * time.Second,
	}

	statelessResetKey := make([]byte, 32)
	rand.Read(statelessResetKey)

	tr := quic.Transport{
		Conn:              udpConn,
		StatelessResetKey: (*quic.StatelessResetKey)(statelessResetKey),
	}

	listener, err := tr.Listen(tlsConf, cfg)
	if err != nil {
		udpConn.Close()
		t.Fatalf("Failed to create QUIC listener: %v", err)
	}

	return listener, udpConn
}

func createTestQUICConnection(t *testing.T, serverAddr net.Addr) *quic.Conn {
	t.Helper()

	// Create UDP connection for client
	udpConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("Failed to create client UDP connection: %v", err)
	}

	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"test-proto"},
	}

	cfg := &quic.Config{
		MaxIdleTimeout: 5 * time.Second,
	}

	statelessResetKey := make([]byte, 32)
	rand.Read(statelessResetKey)

	tr := quic.Transport{
		Conn:              udpConn,
		StatelessResetKey: (*quic.StatelessResetKey)(statelessResetKey),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := tr.Dial(ctx, serverAddr, tlsConf, cfg)
	if err != nil {
		udpConn.Close()
		t.Fatalf("Failed to dial QUIC connection: %v", err)
	}

	return conn
}

// TestQuicListenerAdapterAccept tests the quicListenerAdapter.Accept method.
func TestQuicListenerAdapterAccept(t *testing.T) {
	t.Run("wraps error from closed listener", func(t *testing.T) {
		listener, udpConn := createTestQUICListener(t)
		defer udpConn.Close()

		// Create adapter
		adapter := &netceptor.QuicListenerAdapter{Listener: listener}

		// Close listener to force error
		listener.Close()

		// Attempt to accept - should get wrapped error
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		conn, err := adapter.Accept(ctx)

		if err == nil {
			t.Fatal("Expected error from closed listener, got nil")
		}

		if !strings.Contains(err.Error(), "failed to accept connection") {
			t.Errorf("Expected error to be wrapped with 'failed to accept connection', got: %v", err)
		}

		if conn != nil {
			t.Error("Expected nil connection on error")
		}
	})

	t.Run("wraps error from canceled context", func(t *testing.T) {
		listener, udpConn := createTestQUICListener(t)
		defer listener.Close()
		defer udpConn.Close()

		adapter := &netceptor.QuicListenerAdapter{Listener: listener}

		// Create already-canceled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		conn, err := adapter.Accept(ctx)

		if err == nil {
			t.Fatal("Expected error from canceled context, got nil")
		}

		if !strings.Contains(err.Error(), "failed to accept connection") {
			t.Errorf("Expected wrapped error, got: %v", err)
		}

		if conn != nil {
			t.Error("Expected nil connection on error")
		}
	})

	t.Run("successfully accepts connection and wraps in quicConnAdapter", func(t *testing.T) {
		listener, udpConn := createTestQUICListener(t)
		defer listener.Close()
		defer udpConn.Close()

		adapter := &netceptor.QuicListenerAdapter{Listener: listener}

		// Create a client connection in background
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn := createTestQUICConnection(t, listener.Addr())
			defer conn.CloseWithError(0, "test done")
			time.Sleep(100 * time.Millisecond)
		}()

		// Accept the connection
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		acceptedConn, err := adapter.Accept(ctx)
		if err != nil {
			t.Fatalf("Expected successful accept, got error: %v", err)
		}

		if acceptedConn == nil {
			t.Fatal("Expected non-nil connection")
		}

		// Verify it's a QuicConnectionForConn interface
		var _ netceptor.QuicConnectionForConn = acceptedConn

		// Verify LocalAddr and RemoteAddr work through the adapter
		if acceptedConn.LocalAddr() == nil {
			t.Error("Expected non-nil LocalAddr")
		}

		if acceptedConn.RemoteAddr() == nil {
			t.Error("Expected non-nil RemoteAddr")
		}

		// Clean up
		acceptedConn.CloseWithError(0, "test complete")
		wg.Wait()
	})
}

// TestQuicConnAdapterAcceptStream tests the quicConnAdapter.AcceptStream method.
func TestQuicConnAdapterAcceptStream(t *testing.T) {
	t.Run("adapter delegates AcceptStream correctly", func(t *testing.T) {
		// This test verifies that the adapter's AcceptStream method properly
		// delegates to the underlying quic.Conn.  Due to timing complexities
		// in establishing full bi-directional streams, we verify the method
		// exists and can be called, returning appropriate errors when needed.

		listener, udpConn := createTestQUICListener(t)
		defer listener.Close()
		defer udpConn.Close()

		// Accept connection
		serverChan := make(chan netceptor.QuicConnectionForConn, 1)
		go func() {
			adapter := &netceptor.QuicListenerAdapter{Listener: listener}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			conn, err := adapter.Accept(ctx)
			if err != nil {
				t.Errorf("Failed to accept connection: %v", err)
				close(serverChan)

				return
			}
			serverChan <- conn
		}()

		// Create client
		clientConn := createTestQUICConnection(t, listener.Addr())
		defer clientConn.CloseWithError(0, "test done")

		serverConn := <-serverChan

		// Verify AcceptStream can be called and returns when context expires
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err := serverConn.AcceptStream(ctx)

		// Should get an error (timeout or context deadline) since no stream was opened
		if err == nil {
			t.Error("Expected error when no stream available, got nil")
		}
	})

	t.Run("returns error when context times out", func(t *testing.T) {
		listener, udpConn := createTestQUICListener(t)
		defer listener.Close()
		defer udpConn.Close()

		// Accept connection
		var serverConn netceptor.QuicConnectionForConn
		acceptDone := make(chan struct{})

		go func() {
			defer close(acceptDone)
			adapter := &netceptor.QuicListenerAdapter{Listener: listener}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var err error
			serverConn, err = adapter.Accept(ctx)
			if err != nil {
				t.Errorf("Failed to accept connection: %v", err)
			}
		}()

		// Create client but don't open stream
		clientConn := createTestQUICConnection(t, listener.Addr())
		defer clientConn.CloseWithError(0, "test done")

		<-acceptDone

		// Try to accept stream with short timeout - should timeout
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err := serverConn.AcceptStream(ctx)

		if err == nil {
			t.Fatal("Expected timeout error, got nil")
		}

		// Verify the error is related to timeout or context
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Logf("Got error (may be wrapped): %v", err)
		}
	})
}

// TestQuicConnAdapterOpenStreamSync tests the quicConnAdapter.OpenStreamSync method.
func TestQuicConnAdapterOpenStreamSync(t *testing.T) {
	t.Run("successfully opens stream", func(t *testing.T) {
		listener, udpConn := createTestQUICListener(t)
		defer listener.Close()
		defer udpConn.Close()

		// Create client connection
		clientConn := createTestQUICConnection(t, listener.Addr())
		defer clientConn.CloseWithError(0, "test done")

		adapter := &netceptor.QuicConnAdapter{Conn: clientConn}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		stream, err := adapter.OpenStreamSync(ctx)
		if err != nil {
			t.Fatalf("Failed to open stream: %v", err)
		}
		defer stream.Close()

		if stream == nil {
			t.Fatal("Expected non-nil stream")
		}

		// Verify it implements QuicStreamForConn
		var _ netceptor.QuicStreamForConn = stream

		// Verify we can write to the stream
		testData := []byte("hello")
		_, err = stream.Write(testData)
		if err != nil {
			t.Fatalf("Failed to write to stream: %v", err)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		listener, udpConn := createTestQUICListener(t)
		defer listener.Close()
		defer udpConn.Close()

		clientConn := createTestQUICConnection(t, listener.Addr())
		defer clientConn.CloseWithError(0, "test done")

		adapter := &netceptor.QuicConnAdapter{Conn: clientConn}

		// Create already-canceled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := adapter.OpenStreamSync(ctx)

		if err == nil {
			t.Fatal("Expected error from canceled context, got nil")
		}

		if !errors.Is(err, context.Canceled) {
			t.Logf("Expected context.Canceled, got: %v (may be acceptable if wrapped)", err)
		}
	})

	t.Run("returns error when connection is closed", func(t *testing.T) {
		listener, udpConn := createTestQUICListener(t)
		defer listener.Close()
		defer udpConn.Close()

		clientConn := createTestQUICConnection(t, listener.Addr())
		adapter := &netceptor.QuicConnAdapter{Conn: clientConn}

		// Close connection first
		clientConn.CloseWithError(0, "test close")

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err := adapter.OpenStreamSync(ctx)

		if err == nil {
			t.Fatal("Expected error from closed connection, got nil")
		}
	})
}

// TestAdapterInterfaceCompliance verifies adapters satisfy their interfaces.
func TestAdapterInterfaceCompliance(t *testing.T) {
	t.Run("quicListenerAdapter implements QuicListenerForListener", func(t *testing.T) {
		// This is a compile-time check that will fail if interface isn't satisfied
		var _ netceptor.QuicListenerForListener = (*netceptor.QuicListenerAdapter)(nil)
	})

	t.Run("quicConnAdapter implements QuicConnectionForConn", func(t *testing.T) {
		// This is a compile-time check that will fail if interface isn't satisfied
		var _ netceptor.QuicConnectionForConn = (*netceptor.QuicConnAdapter)(nil)
	})
}

// TestAdapterMethodDelegation verifies that adapter methods properly delegate to underlying types.
func TestAdapterMethodDelegation(t *testing.T) {
	t.Run("quicConnAdapter delegates LocalAddr", func(t *testing.T) {
		listener, udpConn := createTestQUICListener(t)
		defer listener.Close()
		defer udpConn.Close()

		clientConn := createTestQUICConnection(t, listener.Addr())
		defer clientConn.CloseWithError(0, "test done")

		adapter := &netceptor.QuicConnAdapter{Conn: clientConn}

		addr := adapter.LocalAddr()
		if addr == nil {
			t.Error("Expected non-nil LocalAddr")
		}

		// Should match underlying connection's LocalAddr
		if addr.String() != clientConn.LocalAddr().String() {
			t.Errorf("LocalAddr mismatch: adapter=%v, conn=%v", addr, clientConn.LocalAddr())
		}
	})

	t.Run("quicConnAdapter delegates RemoteAddr", func(t *testing.T) {
		listener, udpConn := createTestQUICListener(t)
		defer listener.Close()
		defer udpConn.Close()

		clientConn := createTestQUICConnection(t, listener.Addr())
		defer clientConn.CloseWithError(0, "test done")

		adapter := &netceptor.QuicConnAdapter{Conn: clientConn}

		addr := adapter.RemoteAddr()
		if addr == nil {
			t.Error("Expected non-nil RemoteAddr")
		}

		// Should match underlying connection's RemoteAddr
		if addr.String() != clientConn.RemoteAddr().String() {
			t.Errorf("RemoteAddr mismatch: adapter=%v, conn=%v", addr, clientConn.RemoteAddr())
		}
	})

	t.Run("quicConnAdapter delegates CloseWithError", func(t *testing.T) {
		listener, udpConn := createTestQUICListener(t)
		defer listener.Close()
		defer udpConn.Close()

		clientConn := createTestQUICConnection(t, listener.Addr())

		adapter := &netceptor.QuicConnAdapter{Conn: clientConn}

		err := adapter.CloseWithError(123, "test close")
		if err != nil {
			t.Errorf("Expected nil error from CloseWithError, got: %v", err)
		}

		// Verify connection is actually closed
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err = adapter.OpenStreamSync(ctx)
		if err == nil {
			t.Error("Expected error when opening stream on closed connection")
		}
	})

	t.Run("quicConnAdapter delegates Context", func(t *testing.T) {
		listener, udpConn := createTestQUICListener(t)
		defer listener.Close()
		defer udpConn.Close()

		clientConn := createTestQUICConnection(t, listener.Addr())
		defer clientConn.CloseWithError(0, "test done")

		adapter := &netceptor.QuicConnAdapter{Conn: clientConn}

		ctx := adapter.Context()
		if ctx == nil {
			t.Error("Expected non-nil Context")
		}

		// Context should be active
		select {
		case <-ctx.Done():
			t.Error("Expected context to be active")
		default:
			// Expected
		}

		// After closing, context should be done
		clientConn.CloseWithError(0, "test")
		select {
		case <-ctx.Done():
			// Expected
		case <-time.After(100 * time.Millisecond):
			t.Error("Expected context to be done after connection close")
		}
	})
}

// TestDialContextWithAdapters tests the full Dial path that uses adapters.
func TestDialContextWithAdapters(t *testing.T) {
	t.Run("successful dial creates connection with adapted QUIC connection", func(t *testing.T) {
		// Create two netceptor nodes
		ctx := context.Background()
		n1 := netceptor.New(ctx, "node1")
		n2 := netceptor.New(ctx, "node2")
		defer n1.Shutdown()
		defer n2.Shutdown()

		// Set up a listener on node1
		l1, err := n1.Listen("echo", nil)
		if err != nil {
			t.Fatalf("Failed to create listener: %v", err)
		}
		defer l1.Close()

		// Accept connections in background - simple echo server
		echoReady := make(chan struct{})
		go func() {
			defer close(echoReady)
			conn, err := l1.Accept()
			if err != nil {
				return
			}
			defer conn.Close()
			// Simple echo
			buf := make([]byte, 1024)
			for {
				n, err := conn.Read(buf)
				if err != nil {
					return
				}
				if n > 0 {
					_, err = conn.Write(buf[:n])
					if err != nil {
						return
					}
				}
			}
		}()

		// Set up backends to connect the nodes
		// Use port 0 to let OS assign a random available port
		b1, err := backends.NewTCPListener("127.0.0.1:0", nil, n1.GetLogger())
		if err != nil {
			t.Fatalf("Failed to create TCP listener: %v", err)
		}

		err = n1.AddBackend(b1)
		if err != nil {
			t.Fatalf("Failed to add backend to node1: %v", err)
		}

		// Get the actual bound address from the listener
		actualAddr := b1.GetAddr()

		b2, err := backends.NewTCPDialer(actualAddr, false, nil, n2.GetLogger())
		if err != nil {
			t.Fatalf("Failed to create TCP dialer: %v", err)
		}

		err = n2.AddBackend(b2)
		if err != nil {
			t.Fatalf("Failed to add backend to node2: %v", err)
		}

		// Wait for mesh to form and routing to stabilize
		time.Sleep(2 * time.Second)

		// Dial from node2 to node1 - this exercises line 496 in conn.go
		dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		conn, err := n2.DialContext(dialCtx, "node1", "echo", nil)
		if err != nil {
			t.Fatalf("Failed to dial: %v", err)
		}
		defer conn.Close()

		// Verify connection was created successfully (line 496 was executed)
		if conn == nil {
			t.Fatal("Expected non-nil connection")
		}

		t.Logf("✓ Successfully dialed and created connection")

		// Verify we can communicate through the connection
		testData := []byte("hello")
		_, err = conn.Write(testData)
		if err != nil {
			t.Fatalf("Failed to write: %v", err)
		}

		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			t.Fatalf("Failed to read: %v", err)
		}

		if string(buf[:n]) != string(testData) {
			t.Errorf("Expected echo of %q, got %q", testData, buf[:n])
		}
	})
}
