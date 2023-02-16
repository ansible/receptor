package netceptor

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prep/socketpair"
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
	for i := 0; i < netsize; i++ {
		nodes[i] = New(context.Background(), fmt.Sprintf("node%d", i))
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

	for i := 0; i < 5; i++ {
		// Create and connect a new node with a duplicate name
		n := New(context.Background(), "node0")
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
		case <-time.After(120 * time.Second):
			t.Fatal("timed out waiting for duplicate node to terminate")
		}

		// Force close the connection to the connected node
		_ = c2.Close()
	}

	// Shut down the rest of the network
	for i := 0; i < netsize; i++ {
		nodes[i].Shutdown()
	}
	for i := 0; i < netsize; i++ {
		nodes[i].BackendWait()
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
	time, _ := time.ParseDuration("60s")
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
