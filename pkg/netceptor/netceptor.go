// Package netceptor is the networking layer of Receptor.
package netceptor

import (
	"bytes"
	"encoding/json"
	"fmt"
	priorityQueue "github.com/jupp0r/go-priority-queue"
	"github.org/ghjm/sockceptor/pkg/debug"
	"github.org/ghjm/sockceptor/pkg/randstr"
	"github.org/ghjm/sockceptor/pkg/tickrunner"
	"math"
	"reflect"
	"sync"
	"time"
)

// MTU is the largest message sendable over the Netecptor network
const MTU = 16384

// RouteUpdateTime is the interval at which regular route updates will be sent
const RouteUpdateTime = 10 * time.Second

// ErrorFunc is a function parameter used to process errors
type ErrorFunc func(error)

// BackendSessFunc is a function run by a backend, that runs the Netceptor protocol (or some other protocol)
type BackendSessFunc func(BackendSession) error

// Backend is the interface for back-ends that the Receptor network can run over
type Backend interface {
	Start(sessFunc BackendSessFunc, errFunc ErrorFunc)
}

// BackendSession is the interface for a single session of a back-end
type BackendSession interface {
	Send([]byte) error
	Recv() ([]byte, error)
	Close() error
}

// Netceptor is the main object of the Receptor mesh network protocol
type Netceptor struct {
	nodeID               string
	epoch                int64
	sequence             int64
	structLock           *sync.RWMutex
	connections          map[string]*connInfo
	seenUpdates          map[string]time.Time
	knownNodeInfo        map[string]*nodeInfo
	knownConnectionCosts map[string]map[string]float64
	routingTable         map[string]string
	listenerRegistry     map[string]*PacketConn
	sendRouteFloodChan   chan time.Duration
	updateRoutingTableChan chan time.Duration
	shutdownChans          []chan bool
}

type messageData struct {
	FromNode    string
	FromService string
	ToNode      string
	ToService   string
	Data        []byte
}

type connInfo struct {
	ReadChan         chan []byte
	WriteChan        chan []byte
	ErrorChan 		 chan error
	lastReceivedData time.Time
}

type nodeInfo struct {
	Epoch    int64
	Sequence int64
}

type routingUpdate struct {
	NodeID         string
	UpdateID       string
	UpdateEpoch    int64
	UpdateSequence int64
	Connections    []string
	ForwardingNode string
}

// New constructs a new Receptor network protocol instance
func New(NodeID string) *Netceptor {
	s := Netceptor{
		nodeID:               NodeID,
		epoch:                time.Now().Unix(),
		structLock:           &sync.RWMutex{},
		connections:          make(map[string]*connInfo),
		seenUpdates:          make(map[string]time.Time),
		knownNodeInfo:        make(map[string]*nodeInfo),
		knownConnectionCosts: make(map[string]map[string]float64),
		routingTable:         make(map[string]string),
		listenerRegistry:     make(map[string]*PacketConn),
		shutdownChans:        make([]chan bool, 4),
	}
	s.updateRoutingTableChan = tickrunner.Run(s.updateRoutingTable, time.Hour * 24, time.Second * 1, s.shutdownChans[1])
	s.sendRouteFloodChan = tickrunner.Run(s.floodRoutingUpdate, RouteUpdateTime, time.Millisecond * 100, s.shutdownChans[2])
	go s.monitorConnectionAging(s.shutdownChans[3])
	return &s
}

// Shutdown shuts down a Receptor network protocol instance
func (s *Netceptor) Shutdown() {
	s.structLock.RLock()
	defer s.structLock.RUnlock()
	for i := range s.shutdownChans {
		s.shutdownChans[i] <- true
	}
}

// Watches connections and expires any that haven't seen traffic in too long
func (s *Netceptor) monitorConnectionAging(shutdownChan chan bool) {
	for {
		select {
		case <- time.After(5 * time.Second):
			timedOut := make([]chan error, 0)
			s.structLock.RLock()
			for i := range s.connections {
				if time.Since(s.connections[i].lastReceivedData) > (2 *RouteUpdateTime + 1 * time.Second) {
					timedOut = append(timedOut, s.connections[i].ErrorChan)
				}
			}
			s.structLock.RUnlock()
			for i := range timedOut {
				debug.Printf("Timing out connection\n")
				timedOut[i] <- fmt.Errorf("connection timed out")
			}
		case <- shutdownChan:
			return
		}
	}
}

// Recalculates the next-hop table based on current knowledge of the network
func (s *Netceptor) updateRoutingTable() {
	s.structLock.Lock()
	defer s.structLock.Unlock()
	debug.Printf("Re-calculating routing table\n")

	// Dijkstra's algorithm
	Q := priorityQueue.New()
	Q.Insert(s.nodeID, 0.0)
	cost := make(map[string]float64)
	prev := make(map[string]string)
	for node := range s.knownConnectionCosts {
		if node == s.nodeID {
			cost[node] = 0.0
		} else {
			cost[node] = math.MaxFloat64
		}
		prev[node] = ""
		Q.Insert(node, cost[node])
	}
	for Q.Len() > 0 {
		nodeIf, _ := Q.Pop()
		node := fmt.Sprintf("%v", nodeIf)
		for neighbor, edgeCost := range s.knownConnectionCosts[node] {
			pathCost := cost[node] + edgeCost
			if pathCost < cost[neighbor] {
				cost[neighbor] = pathCost
				prev[neighbor] = node
				Q.Insert(neighbor, pathCost)
			}
		}
	}
	s.routingTable = make(map[string]string)
	for dest := range s.knownConnectionCosts {
		p := dest
		for {
			if prev[p] == s.nodeID {
				s.routingTable[dest] = p
				break
			} else if prev[p] == "" {
				break
			}
			p = prev[p]
		}
	}
	go s.printRoutingTable()
}

// Forwards a message to all neighbors, possibly excluding one
func (s *Netceptor) flood(message []byte, excludeConn string) {
	s.structLock.RLock()
	writeChans := make([]chan []byte, 0)
	for conn, connInfo := range s.connections {
		if conn != excludeConn {
			writeChans = append(writeChans, connInfo.WriteChan)
		}
	}
	s.structLock.RUnlock()
	for i := range writeChans {
		i := i
		go func() { writeChans[i] <- message }()
	}
}

func (s *Netceptor) makeMessage(command string, data interface{}) ([]byte, error) {
	dataj, err := json.Marshal(data)
	if err != nil { return []byte{}, err }
	msg := []byte(command)
	msg = append(msg, ' ')
	msg = append(msg, dataj...)
	return msg, nil
}

// Forwards a message to its next hop
func (s *Netceptor) forwardMessage(md *messageData) error {
	nextHop, ok := s.routingTable[md.ToNode]
	if ! ok { return fmt.Errorf("no route to node") }
	s.structLock.RLock()
	writeChan := s.connections[nextHop].WriteChan
	s.structLock.RUnlock()
	debug.Tracef("Forwarding message to %s via %s\n", md.ToNode, nextHop)
	if writeChan != nil {
		message, err := s.makeMessage("send", md)
		if err != nil { return err }
		writeChan <- message
		return nil
	}
	return fmt.Errorf("could not write to node")
}

// Generates and sends a message over the Receptor network
func (s *Netceptor) sendMessage(fromService string, toNode string, toService string, data []byte) error {
	md := &messageData{
		FromNode:    s.nodeID,
		FromService: fromService,
		ToNode:      toNode,
		ToService:   toService,
		Data:        data,
	}
	return s.handleMessageData(md)
}

// Returns an unused random service name to use as the equivalent of a TCP/IP ephemeral port number.
// Caller must already have s.structLock at least read-locked.
func (s *Netceptor) getEphemeralService() string {
	for {
		service := randstr.RandomString(128)
		_, ok := s.listenerRegistry[service]
		if !ok {
			return service
		}
	}
}

// Prints the routing table.  Only used for debugging.
func (s *Netceptor) printRoutingTable() {
	if ! debug.Enable { return }
	s.structLock.RLock()
	defer s.structLock.RUnlock()
	debug.Printf("Known Connections:\n")
	for conn := range s.knownConnectionCosts {
		debug.Printf("   %s: ", conn)
		for peer := range s.knownConnectionCosts[conn] {
			debug.Printf("%s ", peer)
		}
		debug.Printf("\n")
	}
	debug.Printf("Routing Table:\n")
	for node := range s.routingTable {
		debug.Printf("   %s via %s\n", node, s.routingTable[node])
	}
	debug.Printf("\n")
}

// Constructs a routing update message suitable for sending to the network.
func (s *Netceptor) makeRoutingUpdate() ([]byte, error) {
	s.sequence++
	s.structLock.RLock()
	conns := make([]string, len(s.connections))
	i := 0
	for conn := range s.connections {
		conns[i] = conn
		i++
	}
	s.structLock.RUnlock()
	update := routingUpdate{
		NodeID:         s.nodeID,
		UpdateID:       randstr.RandomString(128),
		UpdateEpoch:    s.epoch,
		UpdateSequence: s.sequence,
		Connections:    conns,
		ForwardingNode: s.nodeID,
	}
	message, err := s.makeMessage("route", update); if err != nil {
		return nil, err
	}
	return message, nil
}

// Sends a routing update to all neighbors.
func (s *Netceptor) floodRoutingUpdate() {
	if len(s.connections) == 0 {
		return
	}
	debug.Printf("Sending routing update\n")
	message, err := s.makeRoutingUpdate()
	if err != nil { return }
	s.flood(message, "")
}

// Processes a routing update received from a connection.
func (s *Netceptor) handleRoutingUpdate(ri *routingUpdate, recvConn string) {
	debug.Printf("Received routing update from %s via %s\n", ri.NodeID, recvConn)
	if ri.NodeID == s.nodeID || ri.NodeID == "" { return }
	s.structLock.RLock()
	_, ok := s.seenUpdates[ri.UpdateID]
	s.structLock.RUnlock()
	if ok { return }
	s.structLock.Lock()
	s.seenUpdates[ri.UpdateID] = time.Now()
	ni, ok := s.knownNodeInfo[ri.NodeID]
	s.structLock.Unlock()
	if ok {
		if ri.UpdateEpoch < ni.Epoch { return }
		if ri.UpdateEpoch == ni.Epoch && ri.UpdateSequence <= ni.Sequence { return }
	} else {
		s.sendRouteFloodChan <- 0
		ni = &nodeInfo{}
	}
	ni.Epoch = ri.UpdateEpoch
	ni.Sequence = ri.UpdateSequence
	conns := make(map[string]float64)
	for conn := range ri.Connections {
		conns[ri.Connections[conn]] = 1.0
	}
	s.structLock.Lock()
	changed := false
	if ! reflect.DeepEqual(conns, s.knownConnectionCosts[ri.NodeID]) {
		changed = true
	}
	s.knownNodeInfo[ri.NodeID] = ni
	s.knownConnectionCosts[ri.NodeID] = conns
	for conn := range s.knownConnectionCosts {
		if conn == s.nodeID {
			continue
		}
		_, ok = conns[conn]
		if ! ok {
			delete(s.knownConnectionCosts[conn], ri.NodeID)
		}
	}
	s.structLock.Unlock()
	ri.ForwardingNode = s.nodeID
	message, err := s.makeMessage("route", ri)
	if err != nil { return }
	s.flood(message, recvConn)
	if changed {
		s.updateRoutingTableChan <- 0
	}
}

// Handles incoming data and dispatches it to a service listener.
func(s *Netceptor) handleMessageData(md *messageData) error {
	if md.ToNode == s.nodeID {
		pc, ok := s.listenerRegistry[md.ToService]; if ! ok {
			return fmt.Errorf("received message for unknown service")
		}
		pc.recvChan <- md
		return nil
	}
	return s.forwardMessage(md)
}

// Translates an incoming message from wire protocol to messageData object.
func (s *Netceptor) translateData(data []byte) (*messageData, error) {
	md := &messageData{}
	err := json.Unmarshal(data, md)
	debug.Tracef("Translated raw data %s to structured data with error %s\n", data, err)
	if err != nil { return nil, err }
	return md, nil
}

// Goroutine to send data from the backend to the connection's ReadChan
func (ci *connInfo) protoReader(sess BackendSession) {
	for {
		buf, err := sess.Recv()
		debug.Tracef("Protocol reader got data %s\n", buf)
		if err != nil {
			debug.Printf("UDP receiving error %s\n", err)
			ci.ErrorChan <- err
			return
		}
		ci.lastReceivedData = time.Now()
		ci.ReadChan <- buf
	}
}

// Goroutine to send data from the connection's WriteChan to the backend
func (ci *connInfo) protoWriter(sess BackendSession) {
	for {
		message, more := <- ci.WriteChan
		debug.Tracef("Protocol writer got data %s\n", message)
		if !more {
			return
		}
		err := sess.Send(message)
		if err != nil {
			debug.Printf("UDP sending error %s\n", err)
			ci.ErrorChan <- err
			return
		}
	}
}

// Continuously sends routing updates to let the other end know who we are on initial connection
func (s *Netceptor) sendInitialRoutingUpdates(ci *connInfo, initDoneChan chan bool) {
	count := 0
	for {
		ri, err := s.makeRoutingUpdate(); if err != nil {
			debug.Printf("Error Sending initial routing message: %s\n", err)
			return
		}
		debug.Printf("Sending initial routing message\n")
		ci.WriteChan <- ri
		count++
		if count > 10 {
			debug.Printf("Giving up on connection initialization\n")
			ci.ErrorChan <- fmt.Errorf("initial connection failed")
			return
		}
		select {
		case <- time.After(1 * time.Second):
		case <- initDoneChan:
			debug.Printf("Stopping initial updates\n")
			return
		}
	}
}

// Main Netceptor protocol loop
func (s *Netceptor) runProtocol(sess BackendSession) error {
	defer func() {
		_ = sess.Close()
	}()
	ci := &connInfo{
		ReadChan: make(chan []byte),
		WriteChan: make(chan []byte),
		ErrorChan: make(chan error),
	}
	go ci.protoReader(sess)
	go ci.protoWriter(sess)
	established := false
	initDoneChan := make(chan bool)
	go s.sendInitialRoutingUpdates(ci, initDoneChan)
	remoteNodeID := ""
	for {
		select {
		case message := <- ci.ReadChan:
			debug.Tracef("Got message %s\n", message)
			msgparts := bytes.SplitN(message, []byte(" "), 2)
			command := string(msgparts[0])
			data := []byte("")
			if len(msgparts) > 1 {
				data = msgparts[1]
			}
			if command == "route" {
				ri := &routingUpdate{}
				err := json.Unmarshal(data, ri)
				if err != nil { continue }
				if !established {
					established = true
					initDoneChan <- true
					remoteNodeID = ri.ForwardingNode
					debug.Printf("Connection established with %s\n", remoteNodeID)
					s.structLock.Lock()
					s.connections[remoteNodeID] = ci
					_, ok := s.knownConnectionCosts[s.nodeID]
					if ! ok {
						s.knownConnectionCosts[s.nodeID] = make(map[string]float64)
					}
					s.knownConnectionCosts[s.nodeID][remoteNodeID] = 1.0
					s.structLock.Unlock()
					s.updateRoutingTableChan <- 0
					s.sendRouteFloodChan <- 0
					defer func() {
						s.structLock.Lock()
						delete(s.connections, remoteNodeID)
						delete(s.knownConnectionCosts[remoteNodeID], s.nodeID)
						delete(s.knownConnectionCosts[s.nodeID], remoteNodeID)
						s.structLock.Unlock()
						s.updateRoutingTableChan <- 0
						s.sendRouteFloodChan <- 0
					}()
				}
				s.handleRoutingUpdate(ri, remoteNodeID)
			} else if established {
				if command == "send" {
					md, err := s.translateData(data); if err != nil {
						debug.Printf("Error translating data: %s.  Data was %s\n", err, data)
					} else {
						err := s.handleMessageData(md); if err != nil {
							debug.Printf("Error handling message data: %s\n", err)
						}
					}
				} else {
					debug.Printf("Unknown command in network packet: %s\n", command)
				}
			}
		case err := <- ci.ErrorChan:
			return err
		case <- s.shutdownChans[0]:
			return nil
		}
	}
}

// RunBackend runs the Netceptor protocol on a backend object
func (s *Netceptor) RunBackend(b Backend, errf func(error)) {
	b.Start(s.runProtocol, errf)
}
