package netceptor

import (
	"context"
	"github.com/project-receptor/receptor/pkg/logger"
	"log"
	"strings"
	"testing"
	"time"
)

type logWriter struct {
	t          *testing.T
	node1count int
	node2count int
}

func (lw *logWriter) Write(p []byte) (n int, err error) {
	s := strings.Trim(string(p), "\n")
	if strings.HasPrefix(s, "ERROR") {
		if !strings.Contains(s, "maximum number of forwarding hops") {
			lw.t.Fatal(s)
			return
		}
	} else if strings.HasPrefix(s, "TRACE") {
		if strings.Contains(s, "via node1") {
			lw.node1count++
		} else if strings.Contains(s, "via node2") {
			lw.node2count++
		}
	}
	lw.t.Log(s)
	return len(p), nil
}

func TestHopCountLimit(t *testing.T) {
	lw := &logWriter{
		t: t,
	}
	log.SetOutput(lw)
	logger.SetShowTrace(true)

	// Create two Netceptor nodes with an in-memory connection
	b1, b2, err := NewInMemoryBackendPair()
	if err != nil {
		t.Fatal(err)
	}
	n1 := New(context.Background(), "node1", nil)
	err = n1.AddBackend(b1, 1.0, nil)
	if err != nil {
		t.Fatal(err)
	}
	n2 := New(context.Background(), "node2", nil)
	err = n2.AddBackend(b2, 1.0, nil)
	if err != nil {
		t.Fatal(err)
	}
	timeout, _ := context.WithTimeout(context.Background(), 2*time.Second)
	for {
		if timeout.Err() != nil {
			t.Fatal(timeout.Err())
		}
		_, ok := n1.Status().RoutingTable["node2"]
		if ok {
			_, ok := n2.Status().RoutingTable["node1"]
			if ok {
				break
			}
		}
	}

	// Inject a fake node3 that both nodes think the other node has a route to
	n1.addNameHash("node3")
	n1.routingTableLock.Lock()
	n1.routingTable["node3"] = "node2"
	n1.routingTableLock.Unlock()
	n2.addNameHash("node3")
	n2.routingTableLock.Lock()
	n2.routingTable["node3"] = "node1"
	n2.routingTableLock.Unlock()

	// Send a message to node3, which should bounce back and forth until max hops is reached
	pc, err := n1.ListenPacket("test")
	if err != nil {
		t.Fatal(err)
	}
	_, err = pc.WriteTo([]byte("hello"), n1.NewAddr("node3", "test"))
	if err != nil {
		t.Fatal(err)
	}

	// If the hop count limit is not working, the connections will never become inactive
	*b1.lastActivity = time.Now()
	timeout, _ = context.WithTimeout(context.Background(), 2*time.Second)
	for time.Now().Sub(*b1.lastActivity) < 250*time.Millisecond {
		select {
		case <-timeout.Done():
			t.Fatal(timeout.Err())
		case <-time.After(125 * time.Millisecond):
		}
	}

	// Make sure we actually succeeded in creating a routing loop
	if lw.node1count < 10 || lw.node2count < 10 {
		t.Fatal("test did not create a routing loop")
	}

	n1.Shutdown()
	n2.Shutdown()
	n1.BackendWait()
	n2.BackendWait()
}
