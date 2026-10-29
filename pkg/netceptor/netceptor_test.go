package netceptor

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ansible/receptor/tests/utils"
	"github.com/prep/socketpair"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/qlog"
)

type logWriter struct {
	t          *testing.T
	node1count int
	node1Lock  sync.RWMutex
	node2count int
	node2Lock  sync.RWMutex
}

func (lw *logWriter) Write(p []byte) (n int, err error) {
	s := strings.Trim(string(p), "\n")
	if strings.HasPrefix(s, "ERROR") {
		if !strings.Contains(s, "maximum number of forwarding hops") {
			fmt.Print(s)
			lw.t.Fatal(s)

			return
		}
	} else if strings.HasPrefix(s, "TRACE") {
		if strings.Contains(s, "via node1") {
			lw.node1Lock.Lock()
			lw.node1count++
			lw.node1Lock.Unlock()
		} else if strings.Contains(s, "via node2") {
			lw.node2Lock.Lock()
			lw.node2count++
			lw.node2Lock.Unlock()
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
	defer func() {
		log.SetOutput(os.Stdout)
	}()

	// Create two Netceptor nodes using external backends
	n1 := New(context.Background(), "node1")
	n1.Logger.SetOutput(lw)
	n1.Logger.SetShowTrace(true)
	b1, err := NewExternalBackend()
	if err != nil {
		t.Fatal(err)
	}
	err = n1.AddBackend(b1)
	if err != nil {
		t.Fatal(err)
	}
	n2 := New(context.Background(), "node2")
	n2.Logger.SetOutput(lw)
	n2.Logger.SetShowTrace(true)
	b2, err := NewExternalBackend()
	if err != nil {
		t.Fatal(err)
	}
	err = n2.AddBackend(b2)
	if err != nil {
		t.Fatal(err)
	}

	// Create a Unix socket pair and use it to connect the backends
	c1, c2, err := socketpair.New("unix")
	if err != nil {
		t.Fatal(err)
	}

	// Subscribe for node list updates
	nCh1 := n1.SubscribeRoutingUpdates()
	nCh2 := n2.SubscribeRoutingUpdates()

	// Connect the two nodes
	b1.NewConnection(MessageConnFromNetConn(c1), true)
	b2.NewConnection(MessageConnFromNetConn(c2), true)

	// Wait for the nodes to establish routing to each other
	var routes1 map[string]string
	var routes2 map[string]string
	timeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		select {
		case <-timeout.Done():
			t.Fatal("timed out waiting for nodes to connect")
		case routes1 = <-nCh1:
		case routes2 = <-nCh2:
		}
		if routes1 != nil && routes2 != nil {
			_, ok := routes1["node2"]
			if ok {
				_, ok := routes2["node1"]
				if ok {
					break
				}
			}
		}
	}

	// Inject a fake node3 that both nodes think the other node has a route to
	n1.AddNameHash("node3")
	n1.routingTableLock.Lock()
	n1.routingTable["node3"] = "node2"
	n1.routingTableLock.Unlock()
	n2.AddNameHash("node3")
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
	timeout, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for {
		c, ok := n1.connections["node2"]
		if !ok {
			t.Fatal("node2 disappeared from node1's connections")
		}
		c.lastReceivedLock.RLock()
		lastReceivedData := c.lastReceivedData
		c.lastReceivedLock.RUnlock()
		if time.Since(lastReceivedData) > 250*time.Millisecond {
			break
		}
		select {
		case <-timeout.Done():
			t.Fatal(timeout.Err())
		case <-time.After(125 * time.Millisecond):
		}
	}

	// Make sure we actually succeeded in creating a routing loop
	lw.node1Lock.RLock()
	node1Count := lw.node1count
	lw.node1Lock.RUnlock()
	lw.node2Lock.RLock()
	node2Count := lw.node2count
	lw.node2Lock.RUnlock()
	if node1Count < 10 || node2Count < 10 {
		t.Fatal("test did not create a routing loop")
	}

	n1.Shutdown()
	n2.Shutdown()
	n1.BackendWait()
	n2.BackendWait()
}

func TestLotsOfPings(t *testing.T) {
	numBackboneNodes := 3
	numLeafNodesPerBackbone := 3

	nodes := []*Netceptor{}
	for i := 0; i < numBackboneNodes; i++ {
		nodes = append(nodes, New(context.Background(), fmt.Sprintf("backbone_%d", i)))
	}
	for i := 0; i < numBackboneNodes; i++ {
		for j := 0; j < i; j++ {
			b1, err := NewExternalBackend()
			if err == nil {
				err = nodes[i].AddBackend(b1)
			}
			if err != nil {
				t.Fatal(err)
			}
			b2, err := NewExternalBackend()
			if err == nil {
				err = nodes[j].AddBackend(b2)
			}
			if err != nil {
				t.Fatal(err)
			}
			c1, c2, err := socketpair.New("unix")
			if err != nil {
				t.Fatal(err)
			}
			b1.NewConnection(MessageConnFromNetConn(c1), true)
			b2.NewConnection(MessageConnFromNetConn(c2), true)
		}
	}

	for i := 0; i < numBackboneNodes; i++ {
		for j := 0; j < numLeafNodesPerBackbone; j++ {
			b1, err := NewExternalBackend()
			if err == nil {
				err = nodes[i].AddBackend(b1)
			}
			if err != nil {
				t.Fatal(err)
			}
			newNode := New(context.Background(), fmt.Sprintf("leaf_%d_%d", i, j))
			nodes = append(nodes, newNode)
			b2, err := NewExternalBackend()
			if err == nil {
				err = newNode.AddBackend(b2)
			}
			if err != nil {
				t.Fatal(err)
			}
			c1, c2, err := socketpair.New("unix")
			if err != nil {
				t.Fatal(err)
			}
			b1.NewConnection(MessageConnFromNetConn(c1), true)
			b2.NewConnection(MessageConnFromNetConn(c2), true)
		}
	}

	responses := make([][]bool, len(nodes))
	responseLocks := make([][]sync.RWMutex, len(nodes))
	for i := range nodes {
		responses[i] = make([]bool, len(nodes))
		responseLocks[i] = make([]sync.RWMutex, len(nodes))
	}

	errorChan := make(chan error)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	wg := sync.WaitGroup{}
	for i := range nodes {
		for j := range nodes {
			// Need to make copies of these variables to avoid a data race
			i2 := i
			j2 := j
			wg.Add(2)
			go func(sender *Netceptor, recipient *Netceptor, response *bool) {
				pc, err := sender.ListenPacket("")
				if err != nil {
					errorChan <- err

					return
				}
				go func() {
					defer wg.Done()
					for {
						buf := make([]byte, 1024)
						err := pc.SetReadDeadline(time.Now().Add(1 * time.Second))
						if err != nil {
							errorChan <- fmt.Errorf("error in SetReadDeadline: %s", err)

							return
						}
						_, addr, err := pc.ReadFrom(buf)
						if ctx.Err() != nil {
							return
						}
						if err != nil {
							continue
						}
						ncAddr, ok := addr.(Addr)
						if !ok {
							errorChan <- fmt.Errorf("addr was not a Netceptor address")

							return
						}
						if ncAddr.node != recipient.nodeID {
							errorChan <- fmt.Errorf("received response from wrong node")

							return
						}
						t.Logf("%s received response from %s", sender.nodeID, recipient.nodeID)
						responseLocks[i2][j2].Lock()
						*response = true
						responseLocks[i2][j2].Unlock()
					}
				}()
				go func() {
					defer wg.Done()
					buf := []byte("test")
					rAddr := sender.NewAddr(recipient.nodeID, "ping")
					for {
						// Ignore write errors - routes may not be established yet.
						// The loop will keep retrying until the ping succeeds.
						_, _ = pc.WriteTo(buf, rAddr)
						select {
						case <-ctx.Done():
							return
						case <-time.After(100 * time.Millisecond):
						}
						responseLocks[i2][j2].RLock()
						r := *response
						responseLocks[i2][j2].RUnlock()
						if r {
							return
						}
					}
				}()
			}(nodes[i], nodes[j], &responses[i][j])
		}
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			good := true
			for i := range nodes {
				for j := range nodes {
					responseLocks[i][j].RLock()
					r := responses[i][j]
					responseLocks[i][j].RUnlock()
					if !r {
						good = false

						break
					}
				}
				if !good {
					break
				}
			}
			if good {
				t.Log("all pings received")
				cancel()

				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
			}
		}
	}()

	t.Log("waiting for done")
	select {
	case err := <-errorChan:
		t.Fatal(err)
	case <-ctx.Done():
	}
	t.Log("waiting for waitgroup")
	wg.Wait()

	t.Log("shutting down")
	for i := range nodes {
		go nodes[i].Shutdown()
	}
	t.Log("waiting for backends")
	for i := range nodes {
		nodes[i].BackendWait()
	}
}

func TestDuplicateNodeDetection(t *testing.T) {
	// Create Netceptor nodes
	netsize := 4
	nodes := make([]*Netceptor, netsize)
	backends := make([]*ExternalBackend, netsize)
	routingChans := make([]chan map[string]string, netsize)
	logWriter := utils.NewLogCapture()
	defer func() {
		t.Log(logWriter.String())
	}()
	for i := 0; i < netsize; i++ {
		nodes[i] = New(context.Background(), fmt.Sprintf("node%d", i))
		nodes[i].Logger.SetOutput(logWriter)
		routingChans[i] = nodes[i].SubscribeRoutingUpdates()
		var err error
		backends[i], err = NewExternalBackend()
		if err != nil {
			t.Fatal(err)
		}
		err = nodes[i].AddBackend(backends[i])
		if err != nil {
			t.Fatal(err)
		}
	}

	// Connect the nodes in a circular network
	for i := 0; i < netsize; i++ {
		c1, c2, err := socketpair.New("unix")
		if err != nil {
			t.Fatal(err)
		}
		peer := (i + 1) % netsize
		backends[i].NewConnection(MessageConnFromNetConn(c1), true)
		backends[peer].NewConnection(MessageConnFromNetConn(c2), true)
	}

	// Wait for the nodes to establish routing to each other
	knownRoutes := make([]map[string]string, netsize)
	knownRoutesLock := sync.RWMutex{}
	for i := 0; i < netsize; i++ {
		knownRoutes[i] = make(map[string]string)
		go func(i int) {
			for {
				select {
				case routes := <-routingChans[i]:
					knownRoutesLock.Lock()
					knownRoutes[i] = routes
					knownRoutesLock.Unlock()
				case <-nodes[i].context.Done():
					return
				}
			}
		}(i)
	}
	timeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for {
		select {
		case <-timeout.Done():
			t.Fatal("timed out waiting for nodes to connect")
		case <-time.After(200 * time.Millisecond):
		}
		for i := 0; i < netsize; i++ {
			peer := (i + 1) % 3
			knownRoutesLock.RLock()
			_, ok := knownRoutes[i][fmt.Sprintf("node%d", peer)]
			if !ok {
				knownRoutesLock.RUnlock()

				continue
			}
			_, ok = knownRoutes[peer][fmt.Sprintf("node%d", i)]
			if !ok {
				knownRoutesLock.RUnlock()

				continue
			}
			knownRoutesLock.RUnlock()
		}

		break
	}

	// Make sure the new node gets a more recent timestamp than the old one
	time.Sleep(1 * time.Second)

	// Create and connect a new node with a duplicate name
	n := New(context.Background(), "node0")
	n.Logger.SetOutput(logWriter)
	n.Logger.Info("Duplicate node0 has epoch %d\n", n.epoch)
	b, err := NewExternalBackend()
	if err != nil {
		t.Fatal(err)
	}
	err = n.AddBackend(b)
	if err != nil {
		t.Fatal(err)
	}
	c1, c2, err := socketpair.New("unix")
	if err != nil {
		t.Fatal(err)
	}
	b.NewConnection(MessageConnFromNetConn(c1), true)
	backends[netsize/2].NewConnection(MessageConnFromNetConn(c2), true)
	// Wait for duplicate node to self-terminate
	backendCloseChan := make(chan struct{})
	go func() {
		n.BackendWait()
		close(backendCloseChan)
	}()
	select {
	case <-backendCloseChan:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for duplicate node to terminate")
	}

	// Force close the connection to the connected node
	_ = c2.Close()

	// Shut down the rest of the network
	for i := 0; i < netsize; i++ {
		nodes[i].Shutdown()
	}
	for i := 0; i < netsize; i++ {
		nodes[i].BackendWait()
	}

	if !strings.Contains(logWriter.String(), "We are a duplicate node") {
		t.Fatalf("Did not find expected log message from duplicate node.")
	}
}

func TestFirewalling(t *testing.T) {
	lw := &logWriter{
		t: t,
	}
	log.SetOutput(lw)
	defer func() {
		log.SetOutput(os.Stdout)
	}()

	// Create two Netceptor nodes using external backends
	n1 := New(context.Background(), "node1")
	n1.Logger.SetOutput(lw)
	n1.Logger.SetShowTrace(true)
	b1, err := NewExternalBackend()
	if err != nil {
		t.Fatal(err)
	}
	err = n1.AddBackend(b1)
	if err != nil {
		t.Fatal(err)
	}
	n2 := New(context.Background(), "node2")
	n2.Logger.SetOutput(lw)
	n2.Logger.SetShowTrace(true)
	b2, err := NewExternalBackend()
	if err != nil {
		t.Fatal(err)
	}
	err = n2.AddBackend(b2)
	if err != nil {
		t.Fatal(err)
	}

	// Add a firewall to node 1 that rejects messages whose data is "bad"
	err = n1.AddFirewallRules([]FirewallRuleFunc{
		func(md *MessageData) FirewallResult {
			if string(md.Data) == "bad" {
				return FirewallResultReject
			}

			return FirewallResultAccept
		},
	}, true)
	if err != nil {
		t.Fatal(err)
	}

	// Create a Unix socket pair and use it to connect the backends
	c1, c2, err := socketpair.New("unix")
	if err != nil {
		t.Fatal(err)
	}

	// Subscribe for node list updates
	nCh1 := n1.SubscribeRoutingUpdates()
	nCh2 := n2.SubscribeRoutingUpdates()

	// Connect the two nodes
	b1.NewConnection(MessageConnFromNetConn(c1), true)
	b2.NewConnection(MessageConnFromNetConn(c2), true)

	// Wait for the nodes to establish routing to each other
	var routes1 map[string]string
	var routes2 map[string]string
	timeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		select {
		case <-timeout.Done():
			t.Fatal("timed out waiting for nodes to connect")
		case routes1 = <-nCh1:
		case routes2 = <-nCh2:
		}
		if routes1 != nil && routes2 != nil {
			_, ok := routes1["node2"]
			if ok {
				_, ok := routes2["node1"]
				if ok {
					break
				}
			}
		}
	}

	// Set up packet connection
	pc1, err := n1.ListenPacket("testsvc")
	if err != nil {
		t.Fatal(err)
	}
	pc2, err := n2.ListenPacket("")
	if err != nil {
		t.Fatal(err)
	}

	// Subscribe for unreachable messages
	doneChan := make(chan struct{})
	unreach2chan := pc2.SubscribeUnreachable(doneChan)

	// Save received unreachable messages to a variable
	var lastUnreachMsg *UnreachableNotification
	lastUnreachLock := sync.RWMutex{}
	go func() {
		<-timeout.Done()
		close(doneChan)
	}()
	go func() {
		for unreach := range unreach2chan {
			unreach := unreach
			lastUnreachLock.Lock()
			lastUnreachMsg = &unreach
			lastUnreachLock.Unlock()
		}
	}()

	// Send a good message
	lastUnreachMsg = nil
	_, err = pc2.WriteTo([]byte("good"), n2.NewAddr("node1", "testsvc"))
	if err != nil {
		t.Fatal(err)
	}
	err = pc1.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 256)
	n, _, err := pc1.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf[:n]) != "good" {
		t.Fatal("incorrect message received")
	}
	time.Sleep(100 * time.Millisecond)
	if lastUnreachMsg != nil {
		t.Fatalf("unexpected unreachable message received: %v", lastUnreachMsg)
	}

	// Send a bad message
	_, err = pc2.WriteTo([]byte("bad"), n2.NewAddr("node1", "testsvc"))
	if err != nil {
		t.Fatal(err)
	}
	err = pc1.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = pc1.ReadFrom(buf)
	if err != ErrTimeout {
		if err == nil {
			err = fmt.Errorf("received message that should have been firewalled")
		}
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	lastUnreachLock.RLock()
	lum := lastUnreachMsg //nolint:ifshort
	lastUnreachLock.RUnlock()
	if lum == nil {
		t.Fatal("did not receive expected unreachable message")
	}

	// Shut down the network
	n1.Shutdown()
	n2.Shutdown()
	n1.BackendWait()
	n2.BackendWait()
}

func TestAllowedPeers(t *testing.T) {
	lw := &logWriter{
		t: t,
	}
	log.SetOutput(lw)
	defer func() {
		log.SetOutput(os.Stdout)
	}()

	// Create two Netceptor nodes using external backends
	n1 := New(context.Background(), "node1")
	n1.Logger.SetOutput(lw)
	n1.Logger.SetShowTrace(true)
	b1, err := NewExternalBackend()
	if err != nil {
		t.Fatal(err)
	}
	err = n1.AddBackend(b1)
	if err != nil {
		t.Fatal(err)
	}
	n2 := New(context.Background(), "node2")
	n2.Logger.SetOutput(lw)
	n2.Logger.SetShowTrace(true)
	b2, err := NewExternalBackend()
	if err != nil {
		t.Fatal(err)
	}
	err = n2.AddBackend(b2)
	if err != nil {
		t.Fatal(err)
	}

	// Add a firewall to node 1 that rejects messages whose data is "bad"
	err = n1.AddFirewallRules([]FirewallRuleFunc{
		func(md *MessageData) FirewallResult {
			if string(md.Data) == "bad" {
				return FirewallResultReject
			}

			return FirewallResultAccept
		},
	}, true)
	if err != nil {
		t.Fatal(err)
	}

	// Create a Unix socket pair and use it to connect the backends
	c1, c2, err := socketpair.New("unix")
	if err != nil {
		t.Fatal(err)
	}

	// Subscribe for node list updates
	nCh1 := n1.SubscribeRoutingUpdates()
	nCh2 := n2.SubscribeRoutingUpdates()

	// Connect the two nodes
	b1.NewConnection(MessageConnFromNetConn(c1), true)
	b2.NewConnection(MessageConnFromNetConn(c2), true)

	// Wait for the nodes to establish routing to each other
	var routes1 map[string]string
	var routes2 map[string]string
	timeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		select {
		case <-timeout.Done():
			t.Fatal("timed out waiting for nodes to connect")
		case routes1 = <-nCh1:
		case routes2 = <-nCh2:
		}
		if routes1 != nil && routes2 != nil {
			_, ok := routes1["node2"]
			if ok {
				_, ok := routes2["node1"]
				if ok {
					break
				}
			}
		}
	}

	// Set up packet connection
	pc1, err := n1.ListenPacket("testsvc")
	if err != nil {
		t.Fatal(err)
	}
	pc2, err := n2.ListenPacket("")
	if err != nil {
		t.Fatal(err)
	}

	// Subscribe for unreachable messages
	doneChan := make(chan struct{})
	unreach2chan := pc2.SubscribeUnreachable(doneChan)

	// Save received unreachable messages to a variable
	var lastUnreachMsg *UnreachableNotification
	lastUnreachLock := sync.RWMutex{}
	go func() {
		<-timeout.Done()
		close(doneChan)
	}()
	go func() {
		for unreach := range unreach2chan {
			unreach := unreach
			lastUnreachLock.Lock()
			lastUnreachMsg = &unreach
			lastUnreachLock.Unlock()
		}
	}()

	// Send a good message
	lastUnreachMsg = nil
	_, err = pc2.WriteTo([]byte("good"), n2.NewAddr("node1", "testsvc"))
	if err != nil {
		t.Fatal(err)
	}
	err = pc1.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 256)
	n, _, err := pc1.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf[:n]) != "good" {
		t.Fatal("incorrect message received")
	}
	time.Sleep(100 * time.Millisecond)
	if lastUnreachMsg != nil {
		t.Fatalf("unexpected unreachable message received: %v", lastUnreachMsg)
	}

	// Send a bad message
	_, err = pc2.WriteTo([]byte("bad"), n2.NewAddr("node1", "testsvc"))
	if err != nil {
		t.Fatal(err)
	}
	err = pc1.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = pc1.ReadFrom(buf)
	if err != ErrTimeout {
		if err == nil {
			err = fmt.Errorf("received message that should have been firewalled")
		}
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	lastUnreachLock.RLock()
	lum := lastUnreachMsg //nolint:ifshort
	lastUnreachLock.RUnlock()
	if lum == nil {
		t.Fatal("did not receive expected unreachable message")
	}

	// Shut down the network
	n1.Shutdown()
	n2.Shutdown()
	n1.BackendWait()
	n2.BackendWait()
}

func TestSetMaxConnectionIdleTime(t *testing.T) {
	t.Parallel()
	node := New(context.Background(), "node1")
	defer node.Shutdown()
	err := node.SetMaxConnectionIdleTime("60s")
	if err != nil {
		t.Fatal(err)
	}
	time, err := time.ParseDuration("60s")
	if err != nil {
		t.Fatalf("Failed to parse duration: %v", err)
	}
	if node.MaxConnectionIdleTime() != time {
		t.Fatal("setter behaved incorrectly")
	}
}

func TestSetBadMaxConnectionIdleTime(t *testing.T) {
	t.Parallel()
	node := New(context.Background(), "node1")
	defer node.Shutdown()
	err := node.SetMaxConnectionIdleTime("60d")
	if err == nil {
		t.Fatal("this should have failed out, as we're passing in an invalid date-string to SetMaxConnectionIdleTime")
	}
}

func TestTooShortSetMaxConnectionIdleTime(t *testing.T) {
	t.Parallel()
	node := New(context.Background(), "node1")
	defer node.Shutdown()
	err := node.SetMaxConnectionIdleTime("60us")
	if err == nil {
		t.Fatal("this should have failed out, as we're passing in an invalid time object that violates the logic in SetMaxConnectionIdleTime")
	}
}

func TestTracerReturnsNewConnectionTracer(t *testing.T) {
	t.Setenv("QLOGDIR", "/tmp/")
	trace := qlog.DefaultConnectionTracer(context.Background(), true, quic.ConnectionID{})
	if trace == nil {
		t.Fatalf("tracer should return a newConnectionTracer when QLOGDIR environment variable is defined but got %v", trace)
	}
}

func TestTracerDoesNotReturnsNewConnectionTracer(t *testing.T) {
	t.Setenv("QLOGDIR", "")
	trace := qlog.DefaultConnectionTracer(context.Background(), true, quic.ConnectionID{})
	if trace != nil {
		t.Fatalf("tracer should return nil when QLOGDIR environment variable is not defined but got %v", trace)
	}
}

// TestTracerCreatesCorrectFilePath tests that QUIC tracing does not depend on
// the QLOGDIR having a trailing slash character.
func TestTracerCreatesCorrectFilePath(t *testing.T) {
	connID := quic.ConnectionIDFromBytes([]byte{0x01, 0x02})
	expectedFilename := "/tmp/0102_client.sqlog"

	tests := []struct {
		name          string
		qlogDirectory string
	}{
		{
			name:          "QLOGDIR without trailing slash character",
			qlogDirectory: "/tmp",
		},
		{
			name:          "QLOGDIR with trailing slash character",
			qlogDirectory: "/tmp/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("QLOGDIR", tt.qlogDirectory)
			trace := qlog.DefaultConnectionTracer(context.Background(), true, connID)
			if trace == nil {
				t.Fatal("tracer should not be nil when QLOGDIR is set")
			}

			_, err := os.Stat(expectedFilename)
			if os.IsNotExist(err) {
				t.Errorf("tracer should create file but did not exist. Expected: %s, Got: %v", expectedFilename, err)
			} else {
				_ = os.Remove(expectedFilename)
			}
		})
	}
}

// TestTracerCreatesNonEmptyFiles tests that qlog files are correctly written to
// when QUIC tracing is enabled.
func TestTracerCreatesNonEmptyFiles(t *testing.T) {
	// Make temporary directory to hold qlog files
	qlogDirectory, err := os.MkdirTemp("", "receptor-qlogs-*")
	if err != nil {
		t.Fatalf("Error creating temp directory: %v", err)
	}
	defer func() {
		err := os.RemoveAll(qlogDirectory)
		if err != nil {
			t.Errorf("Error removing temp directory '%s': %v", qlogDirectory, err)
		}
	}()

	// Set QLOGDIR environment variable to enable tracing
	os.Setenv("QLOGDIR", qlogDirectory)
	defer func() {
		os.Unsetenv("QLOGDIR")
	}()

	// Capture Go's log output because quic-go calls log.Printf() when it logs the
	// "exporting qlog failed" error message
	var logBuffer bytes.Buffer
	log.SetOutput(&logBuffer)
	defer func() {
		log.SetOutput(os.Stderr)
	}()

	// Create a netceptor instance and attempt to dial a service that does not exist
	node1 := New(context.Background(), "node1")

	// Dial error is expected and ignored - the test only verifies qlog file creation.
	// The service "testsvc" does not exist, so the dial may fail, but qlog files
	// should still be created during the connection attempt.
	conn, _ := node1.Dial("node1", "testsvc", nil)
	if conn != nil {
		conn.Close()
	}

	node1.Shutdown()

	// Verify qlog trace files exist in the temp directory
	foundAtLeastOneQlogFile := false
	err = filepath.Walk(qlogDirectory, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if info.Size() <= 0 {
			foundAtLeastOneQlogFile = true

			return fmt.Errorf("QLog trace file was empty: %s", path)
		}
		foundAtLeastOneQlogFile = true

		return nil
	})
	if err != nil {
		t.Errorf("Error verifying qlog trace files: %v", err)
	}
	if !foundAtLeastOneQlogFile {
		t.Error("Did not find any trace files in QLOGDIR")
	}

	// Verify the "exporting qlog failed ... file already closed" error was not logged
	logs := strings.Split(logBuffer.String(), "\n")
	logCapture := utils.NewLogCapture(logs...)
	if logCapture.Contains("exporting qlog failed", "file already closed") {
		t.Error("Node logs contained error about qlog file already closed")
	}
}

// TestRunProtocolExistingConnWithCanceledContext tests the condition in runProtocol
// where an existing connection has a canceled context by calling the real runProtocol function.
func TestRunProtocolExistingConnWithCanceledContext(t *testing.T) {
	// Set up using helper.
	ctx := context.Background()
	s, logCapture := createNetceptorWithLogCapture(t, "test-node")
	remoteNodeID := "existing-node"

	// Create canceled context and existing connection using helper.
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel it immediately
	createExistingConnectionWithContext(s, remoteNodeID, canceledCtx, cancel)

	// Create mock session using helper.
	mockSession, bi := createMockSessionAndBackendInfo(t, s, remoteNodeID)

	// Call runProtocol.
	go func() {
		err := s.runProtocol(ctx, mockSession, bi)
		if err != nil {
			t.Logf("runProtocol finished with: %v", err)
		}
	}()

	// Wait for processing.
	time.Sleep(200 * time.Millisecond)

	// Verify new connection was established.
	s.connLock.RLock()
	newConn, exists := s.connections[remoteNodeID]
	s.connLock.RUnlock()

	if !exists {
		t.Error("Expected new connection to be established after removing canceled connection")
	}
	if exists && newConn.Context.Err() != nil {
		t.Error("Expected new connection to have valid context")
	}

	// Verify context error was logged using helper.
	if !logCapture.Contains("Context for existing connection error", "context canceled") {
		t.Error("Expected error message about context for existing connection was not logged")
	}
}

// TestRunProtocolLogsContextErrorForExistingConnection specifically tests that
// the "Context for existing connection error" message is logged when an existing
// connection has a context error.
func TestRunProtocolLogsContextErrorForExistingConnection(t *testing.T) {
	// Test different types of context errors using table-driven approach
	testCases := []struct {
		name        string
		createCtx   func() (context.Context, context.CancelFunc)
		expectedMsg string
	}{
		{
			name: "canceled context",
			createCtx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // Cancel immediately

				return ctx, cancel
			},
			expectedMsg: "context canceled",
		},
		{
			name: "timeout context",
			createCtx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
				time.Sleep(2 * time.Millisecond) // Ensure it times out

				return ctx, cancel
			},
			expectedMsg: "context deadline exceeded",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up using helper (fresh instance for each subtest).
			ctx := context.Background()
			s, logCapture := createNetceptorWithLogCapture(t, "test-node")
			remoteNodeID := "error-node-" + tc.name

			// Create context with error.
			errorCtx, cancel := tc.createCtx()
			defer cancel()

			// Create existing connection using helper.
			createExistingConnectionWithContext(s, remoteNodeID, errorCtx, cancel)

			// Create mock session using helper.
			mockSession, bi := createMockSessionAndBackendInfo(t, s, remoteNodeID)

			// Call runProtocol.
			go func() {
				err := s.runProtocol(ctx, mockSession, bi)
				if err != nil {
					t.Logf("runProtocol finished with: %v", err)
				}
			}()

			// Wait for processing.
			time.Sleep(200 * time.Millisecond)

			// Verify new connection was established.
			s.connLock.RLock()
			newConn, exists := s.connections[remoteNodeID]
			s.connLock.RUnlock()

			if !exists {
				t.Error("Expected new connection to be established after removing connection with context error")
			}
			if exists && newConn.Context.Err() != nil {
				t.Error("Expected new connection to have valid context")
			}

			// Verify the specific error message was logged using helper.
			if !logCapture.Contains("Context for existing connection error", tc.expectedMsg) {
				t.Errorf("Expected to find log message containing 'Context for existing connection error' and '%s'", tc.expectedMsg)
			}
		})
	}
}

// TestRunProtocolLogsConnectionRemoval tests that connections are properly removed
// when a connection context is canceled (functionality test rather than log message test).
func TestRunProtocolLogsConnectionRemoval(t *testing.T) {
	// Set up a Netceptor instance.
	ctx := context.Background()
	s := New(ctx, "test-node")

	remoteNodeID := "removal-test-node"

	// Create mock session with routing update.
	mockSession := newMockBackendSession()

	// Send the routing update message to establish connection.
	mockSession.recvData <- newRoutingUpdateMessage(t, s, remoteNodeID, "removal-test-update")

	bi := &BackendInfo{
		connectionCost: 1.0,
		nodeCost:       make(map[string]float64),
		allowedPeers:   []string{remoteNodeID}, // Allow the remote node.
	}

	// Create a cancelable context for this test.
	testCtx, cancel := context.WithCancel(ctx)

	// Start runProtocol in a goroutine.
	errChan := make(chan error, 1)
	go func() {
		err := s.runProtocol(testCtx, mockSession, bi)
		errChan <- err
	}()

	// Wait a moment for the connection to be established.
	time.Sleep(50 * time.Millisecond)

	// Verify connection was established.
	s.connLock.RLock()
	_, connectionExists := s.connections[remoteNodeID]
	s.connLock.RUnlock()
	if !connectionExists {
		t.Fatal("Expected connection to be established")
	}

	// Cancel the context to trigger the connection removal.
	cancel()

	// Wait for runProtocol to complete.
	select {
	case err := <-errChan:
		// We expect no error (nil) when context is canceled normally.
		if err != nil {
			t.Logf("runProtocol returned error (this may be expected): %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("runProtocol did not complete within timeout")
	}

	// Wait for cleanup to complete.
	time.Sleep(100 * time.Millisecond)

	// Verify the connection was properly removed.
	s.connLock.RLock()
	_, connectionStillExists := s.connections[remoteNodeID]
	s.connLock.RUnlock()

	if connectionStillExists {
		t.Error("Expected connection to be removed after context cancellation")
	}
}

// TestRunProtocolRemovesExistingConnectionWithCanceledContext verifies that when
// runProtocol detects an existing connection with a canceled context, it properly
// removes that connection from s.connections and allows the new connection to proceed.
func TestRunProtocolRemovesExistingConnectionWithCanceledContext(t *testing.T) {
	ctx := t.Context()
	s, _ := createNetceptorWithLogCapture(t, "test-node")
	const remoteNodeID = "leak-test-node"

	// Seed the connections map with a connection whose context is already canceled.
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	createExistingConnectionWithContext(s, remoteNodeID, canceledCtx, cancel)

	before := snapshotConnection(s, remoteNodeID)
	if !before.exists {
		t.Fatal("Existing connection should be in the connections map")
	}
	if before.conn.Context.Err() == nil {
		t.Fatal("Existing connection context should be canceled")
	}

	// runProtocol should detect the canceled connection, remove it, and replace it.
	mockSession, bi := createMockSessionAndBackendInfo(t, s, remoteNodeID)
	done := runProtocolAsync(s, ctx, mockSession, bi)
	t.Cleanup(func() {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("runProtocol did not exit after context cancellation")
		}
	})
	waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
	defer waitCancel()
	if !utils.CheckUntilTimeout(waitCtx, 5*time.Millisecond, func() bool {
		c := snapshotConnection(s, remoteNodeID)

		return c.exists && c.conn != before.conn
	}) {
		t.Fatal("timed out waiting for the replacement connection")
	}

	after := snapshotConnection(s, remoteNodeID)
	if !after.exists {
		t.Fatal("Expected a new connection to be established after removing canceled connection")
	}
	if after.count != before.count {
		t.Errorf("Expected connection count to remain the same (%d), but got %d", before.count, after.count)
	}
	if after.conn.Context.Err() != nil {
		t.Error("The replacement connection should have a valid (non-canceled) context")
	}
	if after.conn == before.conn {
		t.Error("The replacement connection should be a different connection object than the original")
	}

	// A second attempt with the same node ID must now fail because a valid connection exists.
	mockSession2, _ := createMockSessionAndBackendInfo(t, s, remoteNodeID)
	err := s.runProtocol(ctx, mockSession2, bi)
	if err == nil {
		t.Fatal("Expected second connection attempt to fail due to existing valid connection")
	}
	if !strings.Contains(err.Error(), "it connected using a node ID we are already connected to") {
		t.Errorf("Expected second connection to be rejected due to existing connection, got: %v", err)
	}

	final := snapshotConnection(s, remoteNodeID)
	if final.count != 1 {
		t.Errorf("Expected exactly 1 connection after cleanup, but got %d", final.count)
	}
	if final.exists && final.conn.Context.Err() != nil {
		t.Error("Final connection should have valid context")
	}
}

// Helper functions for test setup to reduce code duplication.

// createNetceptorWithLogCapture creates a Netceptor instance with log capture setup.
func createNetceptorWithLogCapture(t *testing.T, nodeID string) (*Netceptor, *utils.LogCapture) {
	t.Helper()
	ctx := context.Background()
	s := New(ctx, nodeID)
	t.Cleanup(s.Shutdown)
	logCapture := utils.NewLogCapture()
	s.Logger.SetOutput(logCapture)

	return s, logCapture
}

// createMockSessionAndBackendInfo creates a mock session and backend info for testing.
func createMockSessionAndBackendInfo(t *testing.T, s *Netceptor, remoteNodeID string) (*mockBackendSession, *BackendInfo) {
	mockSession := newMockBackendSession()

	// Create routing update message.
	mockSession.recvData <- newRoutingUpdateMessage(t, s, remoteNodeID, "test-update-"+remoteNodeID)

	bi := &BackendInfo{
		connectionCost: 1.0,
		nodeCost:       make(map[string]float64),
		allowedPeers:   []string{remoteNodeID},
	}

	return mockSession, bi
}

// createExistingConnectionWithContext creates an existing connection with the given context.
func createExistingConnectionWithContext(s *Netceptor, remoteNodeID string, ctx context.Context, cancel context.CancelFunc) {
	existingConn := &connInfo{
		Context:          ctx,
		CancelFunc:       cancel,
		ReadChan:         make(chan []byte),
		WriteChan:        make(chan []byte),
		Cost:             1.0,
		lastReceivedLock: &sync.RWMutex{},
		logger:           s.Logger,
	}

	s.connLock.Lock()
	if s.connections == nil {
		s.connections = make(map[string]*connInfo)
	}
	s.connections[remoteNodeID] = existingConn
	s.connLock.Unlock()
}

// connSnapshot is a consistent view of the connections map for a single node ID.
type connSnapshot struct {
	conn   *connInfo
	exists bool
	count  int
}

// snapshotConnection returns the connection for nodeID (if any) and the total connection count.
func snapshotConnection(s *Netceptor, nodeID string) connSnapshot {
	s.connLock.RLock()
	defer s.connLock.RUnlock()

	conn, exists := s.connections[nodeID]

	return connSnapshot{conn: conn, exists: exists, count: len(s.connections)}
}

// runProtocolAsync runs runProtocol in a goroutine so the test can observe its
// side effects. The returned channel receives the result once the context is
// canceled (t.Context() is canceled before the test's cleanup functions run);
// callers should drain it in a cleanup to ensure the goroutine has exited.
func runProtocolAsync(s *Netceptor, ctx context.Context, session BackendSession, bi *BackendInfo) <-chan error {
	done := make(chan error, 1)
	go func() {
		done <- s.runProtocol(ctx, session, bi)
	}()

	return done
}

// mockBackendSession implements BackendSession for testing.
type mockBackendSession struct {
	sendData chan []byte
	recvData chan []byte
	closed   chan struct{}
	mutex    sync.Mutex
	isClosed bool
}

func (m *mockBackendSession) Send(data []byte) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.isClosed {
		return fmt.Errorf("session closed")
	}
	select {
	case m.sendData <- data:

		return nil
	default:

		return fmt.Errorf("send buffer full")
	}
}

func (m *mockBackendSession) Recv(timeout time.Duration) ([]byte, error) {
	m.mutex.Lock()
	if m.isClosed {
		m.mutex.Unlock()

		return nil, fmt.Errorf("session closed")
	}
	m.mutex.Unlock()

	select {
	case data := <-m.recvData:

		return data, nil
	case <-time.After(timeout):

		return nil, ErrTimeout
	case <-m.closed:

		return nil, fmt.Errorf("session closed")
	}
}

func (m *mockBackendSession) Close() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if !m.isClosed {
		m.isClosed = true
		close(m.closed)
	}

	return nil
}

// slowBackendSession is a BackendSession whose Send blocks for a fixed delay,
// simulating a slow or congested mesh link.
type slowBackendSession struct {
	delay  time.Duration
	closed chan struct{}
}

func (s *slowBackendSession) Send(_ []byte) error {
	select {
	case <-time.After(s.delay):
		return nil
	case <-s.closed:
		return fmt.Errorf("session closed")
	}
}

func (s *slowBackendSession) Recv(timeout time.Duration) ([]byte, error) {
	select {
	case <-time.After(timeout):
		return nil, ErrTimeout
	case <-s.closed:
		return nil, fmt.Errorf("session closed")
	}
}

func (s *slowBackendSession) Close() error {
	close(s.closed)

	return nil
}

// blockableBackendSession is a BackendSession whose first Send blocks until the
// test signals unblockFirst; subsequent Sends return immediately.
type blockableBackendSession struct {
	firstSendStarted chan struct{}
	unblockFirst     chan struct{}
	once             sync.Once
}

func (s *blockableBackendSession) Send(_ []byte) error {
	s.once.Do(func() {
		close(s.firstSendStarted)
		<-s.unblockFirst
	})

	return nil
}

func (s *blockableBackendSession) Recv(timeout time.Duration) ([]byte, error) {
	time.Sleep(timeout)

	return nil, ErrTimeout
}

func (s *blockableBackendSession) Close() error { return nil }

// TestProtoWriterDoesNotDropBufferedMessagesOnContextCancel verifies that when
// the context is cancelled while protoWriter is blocked in sess.Send, messages
// already enqueued in the buffered WriteChan are not silently dropped. Senders
// received nil from their channel send and believe delivery succeeded; losing
// those messages is a correctness bug.
func TestProtoWriterDoesNotDropBufferedMessagesOnContextCancel(t *testing.T) {
	const bufSize = writeChanBufferSize

	firstSendStarted := make(chan struct{})
	unblockFirst := make(chan struct{})

	sess := &blockableBackendSession{
		firstSendStarted: firstSendStarted,
		unblockFirst:     unblockFirst,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := New(ctx, "test-node")
	ci := &connInfo{
		Context:          ctx,
		CancelFunc:       cancel,
		ReadChan:         make(chan []byte),
		WriteChan:        make(chan []byte, bufSize),
		lastReceivedLock: &sync.RWMutex{},
		logger:           s.Logger,
	}

	protoWriterDone := make(chan struct{})
	go func() {
		defer close(protoWriterDone)
		ci.protoWriter(sess)
	}()

	// Enqueue bufSize+1 messages: protoWriter consumes the first into the blocking
	// Send; the remaining bufSize sit in the channel. The loop blocks on the last
	// send until protoWriter drains a slot, so all messages are enqueued by the
	// time <-firstSendStarted fires.
	for i := range bufSize + 1 {
		ci.WriteChan <- fmt.Appendf(nil, "msg-%d", i)
	}

	// Wait until protoWriter is confirmed inside Send before cancelling, so the
	// buffer is full and the race window is deterministic.
	<-firstSendStarted
	cancel()
	close(unblockFirst) // Send(msg-0) returns; protoWriter re-enters the select

	select {
	case <-protoWriterDone:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("protoWriter did not exit after context cancellation")
	}

	if remaining := len(ci.WriteChan); remaining > 0 {
		t.Errorf("protoWriter silently dropped %d buffered message(s) on context cancellation", remaining)
	}
}

// TestProtoWriterLogsDroppedMessagesOnContextCancel verifies that when protoWriter
// drains buffered messages on context cancellation it emits a warning with the
// drop count rather than discarding them silently.
func TestProtoWriterLogsDroppedMessagesOnContextCancel(t *testing.T) {
	const bufSize = writeChanBufferSize

	firstSendStarted := make(chan struct{})
	unblockFirst := make(chan struct{})

	sess := &blockableBackendSession{
		firstSendStarted: firstSendStarted,
		unblockFirst:     unblockFirst,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := New(ctx, "test-node")

	var logBuf bytes.Buffer
	s.Logger.SetOutput(&logBuf)

	ci := &connInfo{
		Context:          ctx,
		CancelFunc:       cancel,
		ReadChan:         make(chan []byte),
		WriteChan:        make(chan []byte, bufSize),
		lastReceivedLock: &sync.RWMutex{},
		logger:           s.Logger,
	}

	protoWriterDone := make(chan struct{})
	go func() {
		defer close(protoWriterDone)
		ci.protoWriter(sess)
	}()

	for i := range bufSize + 1 {
		ci.WriteChan <- fmt.Appendf(nil, "msg-%d", i)
	}

	<-firstSendStarted
	cancel()
	close(unblockFirst)

	select {
	case <-protoWriterDone:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("protoWriter did not exit after context cancellation")
	}

	// protoWriter has exited; no further writes to logBuf can occur.
	logged := logBuf.String()
	if !strings.Contains(logged, "queued message(s): connection closed before delivery") {
		t.Errorf("expected a drop warning in the log, got: %q", logged)
	}
}

// TestWriteChanBufferDecouplesSendersFromProtoWriter verifies that the buffered
// WriteChan allows senders to enqueue without blocking on a slow protoWriter.
// It uses non-blocking sends: if the buffer is too small (or absent), sends
// fall through to the default arm and are counted as blocked.
func TestWriteChanBufferDecouplesSendersFromProtoWriter(t *testing.T) {
	const bufferSize = writeChanBufferSize

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := New(ctx, "test-node")

	sess := &slowBackendSession{delay: time.Hour, closed: make(chan struct{})}
	defer sess.Close()

	ci := &connInfo{
		Context:          ctx,
		CancelFunc:       cancel,
		ReadChan:         make(chan []byte),
		WriteChan:        make(chan []byte, bufferSize),
		lastReceivedLock: &sync.RWMutex{},
		logger:           s.Logger,
	}
	go ci.protoWriter(sess)

	blocked := 0
	for range bufferSize {
		select {
		case ci.WriteChan <- []byte("payload"):
		default:
			blocked++
		}
	}

	if blocked > 0 {
		t.Errorf("%d/%d sends blocked: buffer did not decouple senders from the slow writer", blocked, bufferSize)
	}
}

// newRoutingUpdateMessage builds a network-encoded routing update message
// from the given peer node, as the peer would send it at handshake time.
func newRoutingUpdateMessage(t *testing.T, s *Netceptor, nodeID, updateID string) []byte {
	t.Helper()
	routingUpdate := &routingUpdate{
		NodeID:         nodeID,
		ForwardingNode: nodeID,
		UpdateEpoch:    1,
		UpdateSequence: 1,
		Connections:    make(map[string]float64),
		UpdateID:       updateID,
	}
	msgBytes, err := s.translateStructToNetwork(MsgTypeRoute, routingUpdate)
	if err != nil {
		t.Fatalf("Failed to create routing update message: %v", err)
	}

	return msgBytes
}

// newMockBackendSession creates a mock session with buffered channels.
func newMockBackendSession() *mockBackendSession {
	return &mockBackendSession{
		sendData: make(chan []byte, 10),
		recvData: make(chan []byte, 10),
		closed:   make(chan struct{}),
	}
}
