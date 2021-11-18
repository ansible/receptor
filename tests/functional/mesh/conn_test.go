package mesh

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/backends"
	"github.com/ansible/receptor/pkg/netceptor"
)

func TestQuicConnectTimeout(t *testing.T) {
	// Change MaxIdleTimeoutForQuicConnections to 1 seconds (default in lib is 30, our code is 60)
	netceptor.MaxIdleTimeoutForQuicConnections = 1 * time.Second
	// We also have to disable heart beats or the connection will not properly timeout
	netceptor.KeepAliveForQuicConnections = false

	// Create two nodes of the Receptor network-layer protocol (Netceptors).
	n1 := netceptor.New(context.Background(), "node1")
	n2 := netceptor.New(context.Background(), "node2")

	// Start a TCP listener on the first node
	b1, err := backends.NewTCPListener("localhost:3333", nil)
	if err != nil {
		t.Fatal(fmt.Sprintf("Error listening on TCP: %s\n", err))
	}
	err = n1.AddBackend(b1)
	if err != nil {
		t.Fatal(fmt.Sprintf("Error starting backend: %s\n", err))
	}

	// Start a TCP dialer on the second node - this will connect to the listener we just started
	b2, err := backends.NewTCPDialer("localhost:3333", false, nil)
	if err != nil {
		t.Fatal(fmt.Sprintf("Error dialing on TCP: %s\n", err))
	}
	err = n2.AddBackend(b2)
	if err != nil {
		t.Fatal(fmt.Sprintf("Error starting backend: %s\n", err))
	}

	// Start an echo server on node 1
	l1, err := n1.Listen("echo", nil)
	if err != nil {
		t.Fatal(fmt.Sprintf("Error listening on Receptor network: %s\n", err))
	}
	go func() {
		// Accept an incoming connection - note that conn is just a regular net.Conn
		conn, err := l1.Accept()
		if err != nil {
			t.Fatal(fmt.Sprintf("Error accepting connection: %s\n", err))

			return
		}
		go func() {
			defer conn.Close()
			buf := make([]byte, 1024)
			done := false
			for !done {
				n, err := conn.Read(buf)
				if err == io.EOF {
					done = true
				} else if err != nil {
					// Is ok if we got a 'NO_ERROR: No recent network activity' error but anything else is a test failure.
					if strings.Contains(err.Error(), "No recent network activity") {
						t.Log("Successfully got the desired timeout error")
					} else {
						t.Fatal(fmt.Sprintf("Read error in Receptor listener: %s\n", err))
					}

					return
				}
				if n > 0 {
					_, err := conn.Write(buf[:n])
					if err != nil {
						t.Fatal(fmt.Sprintf("Write error in Receptor listener: %s\n", err))

						return
					}
				}
			}
		}()
	}()

	// Connect to the echo server from node 2.  We expect this to error out at first with
	// "no route to node" because it takes a second or two for node1 and node2 to exchange
	// routing information and form a mesh.
	var c2 net.Conn
	for {
		c2, err = n2.Dial("node1", "echo", nil)
		if err != nil {
			time.Sleep(1 * time.Second)

			continue
		}

		break
	}

	// Sleep longer than MaxIdleTimeout (see pkg/netceptor/conn.go for current setting)
	sleepDuration := 6 * time.Second
	time.Sleep(sleepDuration)
	// Start a listener function that prints received data to the screen
	// Note that because net.Conn is a stream connection, it is not guaranteed
	// that received messages will be the same size as the messages that are sent.
	// For datagram use, Receptor also provides a net.PacketConn.
	go func() {
		rbuf := make([]byte, 1024)
		for {
			n, err := c2.Read(rbuf)
			if n > 0 {
				n2.Shutdown()
				t.Fatal("Should not have gotten data back")

				return
			}
			if err == io.EOF {
				// Shut down the whole Netceptor when any connection closes, because this is just a demo
				n2.Shutdown()
				t.Fatal("Should not have gotten an EOF")

				return
			}
			if err != nil {
				n2.Shutdown()

				return
			}
		}
	}()

	// Send some data, which should be processed through the echo server back to our
	// receive function and printed to the screen.
	_, err = c2.Write([]byte("Hello, world!"))
	if !(err != nil && err != io.EOF) {
		t.Fatal("We should have gotten an error here")
	}

	// Close our end of the connection
	_ = c2.Close()

	// Wait for n2 to shut down
	n2.BackendWait()

	// Gracefully shut down n1
	n1.Shutdown()
	n1.BackendWait()
}
