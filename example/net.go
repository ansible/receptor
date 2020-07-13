package main

import (
	"context"
	"fmt"
	"github.com/project-receptor/receptor/pkg/backends"
	"github.com/project-receptor/receptor/pkg/netceptor"
	"io"
	"net"
	"os"
	"time"
)

/*
   This is an example of the use of Receptor as a Go library.
*/

func main() {
	// Create two nodes of the Receptor network-layer protocol (Netceptors).
	n1 := netceptor.New(context.Background(), "node1", nil)
	n2 := netceptor.New(context.Background(), "node2", nil)

	// Start a TCP listener on the first node
	b1, err := backends.NewTCPListener("localhost:3333", nil)
	if err != nil {
		fmt.Printf("Error listening on TCP: %s\n", err)
		os.Exit(1)
	}
	err = n1.AddBackend(b1, 1.0)
	if err != nil {
		fmt.Printf("Error starting backend: %s\n", err)
		os.Exit(1)
	}

	// Start a TCP dialer on the second node - this will connect to the listener we just started
	b2, err := backends.NewTCPDialer("localhost:3333", false, nil)
	if err != nil {
		fmt.Printf("Error dialing on TCP: %s\n", err)
		os.Exit(1)
	}
	err = n2.AddBackend(b2, 1.0)

	// Start an echo server on node 1
	l1, err := n1.Listen("echo", nil)
	if err != nil {
		fmt.Printf("Error listening on Receptor network: %s\n", err)
		os.Exit(1)
	}
	go func() {
		// Accept an incoming connection - note that conn is just a regular net.Conn
		conn, err := l1.Accept()
		if err != nil {
			fmt.Printf("Error accepting connection: %s\n", err)
			return
		}
		fmt.Printf("Accepted a connection\n")
		go func() {
			defer conn.Close()
			buf := make([]byte, 1024)
			done := false
			for !done {
				n, err := conn.Read(buf)
				if err == io.EOF {
					done = true
				} else if err != nil {
					fmt.Printf("Read error in Receptor listener: %s\n", err)
					return
				}
				fmt.Printf("Echo server got %d bytes\n", n)
				if n > 0 {
					_, err := conn.Write(buf[:n])
					if err != nil {
						fmt.Printf("Write error in Receptor listener: %s\n", err)
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
		fmt.Printf("Dialing node1\n")
		c2, err = n2.Dial("node1", "echo", nil)
		if err != nil {
			fmt.Printf("Error dialing on Receptor network: %s\n", err)
			time.Sleep(1 * time.Second)
			continue
		}
		break
	}

	// Start a listener function that prints received data to the screen
	// Note that because net.Conn is a stream connection, it is not guaranteed
	// that received messages will be the same size as the messages that are sent.
	// For datagram use, Receptor also provides a net.PacketConn.
	go func() {
		rbuf := make([]byte, 1024)
		for {
			n, err := c2.Read(rbuf)
			if n > 0 {
				fmt.Printf("Received data: %s\n", rbuf[:n])
			}
			if err == io.EOF {
				// Shut down the whole Netceptor when any connection closes, because this is just a demo
				n2.Shutdown()
				return
			}
			if err != nil {
				fmt.Printf("Read error in Receptor dialer: %s\n", err)
				return
			}
		}
	}()

	// Send some data, which should be processed through the echo server back to our
	// receive function and printed to the screen.
	_, err = c2.Write([]byte("Hello, world!"))
	if err != nil && err != io.EOF {
		fmt.Printf("Write error in Receptor dialer: %s\n", err)
	}

	// Close our end of the connection
	_ = c2.Close()

	// Wait for n2 to shut down
	n2.BackendWait()

	// Gracefully shut down n1
	n1.Shutdown()
	n1.BackendWait()

}
