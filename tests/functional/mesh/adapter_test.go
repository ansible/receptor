package mesh

import (
	"context"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/backends"
	"github.com/ansible/receptor/pkg/netceptor"
)

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
