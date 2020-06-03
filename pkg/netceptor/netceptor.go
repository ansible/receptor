// Package netceptor is the networking layer of Receptor.
package netceptor

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/ghjm/sockceptor/pkg/debug"
	"github.com/ghjm/sockceptor/pkg/randstr"
	"github.com/ghjm/sockceptor/pkg/tickrunner"
	priorityQueue "github.com/jupp0r/go-priority-queue"
	"github.com/minio/highwayhash"
	"math"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

// MTU is the largest message sendable over the Netecptor network
const MTU = 16384

// RouteUpdateTime is the interval at which regular route updates will be sent
const RouteUpdateTime = 10 * time.Second

// ErrorFunc is a function parameter used to process errors. The boolean parameter
// indicates whether the error is fatal (i.e. the associated process is going to exit).
type ErrorFunc func(error, bool)

// BackendSessFunc is a function run by a backend, that runs the Netceptor protocol (or some other protocol)
type BackendSessFunc func(BackendSession) error

// Backend is the interface for back-ends that the Receptor network can run over
type Backend interface {
	Start(sessFunc BackendSessFunc, errFunc ErrorFunc)
}

// MainInstance is the global instance of Netceptor instantiated by the command-line main() function
var MainInstance *Netceptor

// BackendSession is the interface for a single session of a back-end
// Backends must be DATAGRAM ORIENTED, meaning that Recv() must return
// whole packets sent by Send(). If the underlying protocol is stream
// oriented, then the backend must deal with any required buffering.
type BackendSession interface {
	Send([]byte) error
	Recv() ([]byte, error)
	Close() error
}

// Netceptor is the main object of the Receptor mesh network protocol
type Netceptor struct {
	nodeID                 string
	epoch                  uint64
	sequence               uint64
	connLock               *sync.RWMutex
	connections            map[string]*connInfo
	knownNodeLock          *sync.RWMutex
	seenUpdates            map[string]time.Time
	knownNodeInfo          map[string]*nodeInfo
	knownConnectionCosts   map[string]map[string]float64
	routingTableLock       *sync.RWMutex
	routingTable           map[string]string
	listenerLock           *sync.RWMutex
	listenerRegistry       map[string]*PacketConn
	sendRouteFloodChan     chan time.Duration
	updateRoutingTableChan chan time.Duration
	shutdownChans          []chan bool
	hashLock               *sync.RWMutex
	nameHashes             map[uint64]string
	reservedServices       map[string]func(*messageData) error
}

// Status is the struct returned by Netceptor.Status().  It represents a public
// view of the internal status of the Netceptor object.
type Status struct {
	NodeID       string
	Connections  []string
	RoutingTable map[string]string
}

const (
	// MsgTypeData is a normal data-containing message
	MsgTypeData = 0
	// MsgTypeRoute is a routing update
	MsgTypeRoute = 1
)

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
	ErrorChan        chan error
	lastReceivedData time.Time
}

type nodeInfo struct {
	Epoch    uint64
	Sequence uint64
}

type routingUpdate struct {
	NodeID         string
	UpdateID       string
	UpdateEpoch    uint64
	UpdateSequence uint64
	Connections    []string
	ForwardingNode string
}

var backendWaitGroup = sync.WaitGroup{}
var backendCount int32 = 0

// AddBackend adds a backend to the wait group
func AddBackend() {
	backendWaitGroup.Add(1)
	atomic.AddInt32(&backendCount, 1)
}

// DoneBackend signals to the wait group that the backend is done
func DoneBackend() {
	backendWaitGroup.Done()
}

// BackendCount returns the number of backends that ever registered
func BackendCount() int32 {
	return backendCount
}

// BackendWait waits for the backend wait group
func BackendWait() {
	backendWaitGroup.Wait()
}

// New constructs a new Receptor network protocol instance
func New(NodeID string) *Netceptor {
	s := Netceptor{
		nodeID:                 NodeID,
		epoch:                  uint64(time.Now().Unix()),
		sequence:               0,
		connLock:               &sync.RWMutex{},
		connections:            make(map[string]*connInfo),
		knownNodeLock:          &sync.RWMutex{},
		seenUpdates:            make(map[string]time.Time),
		knownNodeInfo:          make(map[string]*nodeInfo),
		knownConnectionCosts:   make(map[string]map[string]float64),
		routingTableLock:       &sync.RWMutex{},
		routingTable:           make(map[string]string),
		listenerLock:           &sync.RWMutex{},
		listenerRegistry:       make(map[string]*PacketConn),
		sendRouteFloodChan:     nil,
		updateRoutingTableChan: nil,
		shutdownChans:          make([]chan bool, 4),
		hashLock:               &sync.RWMutex{},
		nameHashes:             make(map[uint64]string),
	}
	s.reservedServices = map[string]func(*messageData) error{
		"ping": s.handlePing,
	}

	s.addNameHash(NodeID)
	s.updateRoutingTableChan = tickrunner.Run(s.updateRoutingTable, time.Hour*24, time.Second*1, s.shutdownChans[1])
	s.sendRouteFloodChan = tickrunner.Run(s.sendRoutingUpdate, RouteUpdateTime, time.Millisecond*100, s.shutdownChans[2])
	go s.monitorConnectionAging(s.shutdownChans[3])
	return &s
}

// Shutdown shuts down a Receptor network protocol instance
func (s *Netceptor) Shutdown() {
	for i := range s.shutdownChans {
		s.shutdownChans[i] <- true
	}
}

// Status returns the current state of the Netceptor object
func (s *Netceptor) Status() Status {
	conns := make([]string, 0)
	for conn := range s.connections {
		conns = append(conns, conn)
	}
	routes := make(map[string]string)
	for k, v := range s.routingTable {
		routes[k] = v
	}
	return Status{
		NodeID:       s.nodeID,
		Connections:  conns,
		RoutingTable: routes,
	}
}

// Watches connections and expires any that haven't seen traffic in too long
func (s *Netceptor) monitorConnectionAging(shutdownChan chan bool) {
	for {
		select {
		case <-time.After(5 * time.Second):
			timedOut := make([]chan error, 0)
			s.connLock.RLock()
			for i := range s.connections {
				if time.Since(s.connections[i].lastReceivedData) > (2*RouteUpdateTime + 1*time.Second) {
					timedOut = append(timedOut, s.connections[i].ErrorChan)
				}
			}
			s.connLock.RUnlock()
			for i := range timedOut {
				debug.Printf("Timing out connection\n")
				timedOut[i] <- fmt.Errorf("connection timed out")
			}
		case <-shutdownChan:
			return
		}
	}
}

// Recalculates the next-hop table based on current knowledge of the network
func (s *Netceptor) updateRoutingTable() {
	s.knownNodeLock.RLock()
	defer s.knownNodeLock.RUnlock()
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
	s.routingTableLock.Lock()
	defer s.routingTableLock.Unlock()
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
	s.printRoutingTable()
}

// Forwards a message to all neighbors, possibly excluding one
func (s *Netceptor) flood(message []byte, excludeConn string) {
	s.connLock.RLock()
	writeChans := make([]chan []byte, 0)
	for conn, connInfo := range s.connections {
		if conn != excludeConn {
			writeChans = append(writeChans, connInfo.WriteChan)
		}
	}
	s.connLock.RUnlock()
	for i := range writeChans {
		i := i
		go func() { writeChans[i] <- message }()
	}
}

var zerokey = make([]byte, 32)

func (s *Netceptor) addNameHash(name string) uint64 {
	h, _ := highwayhash.New64(zerokey)
	_, _ = h.Write([]byte(name))
	hv := h.Sum64()
	s.hashLock.Lock()
	defer s.hashLock.Unlock()
	_, ok := s.nameHashes[hv]
	if !ok {
		s.nameHashes[hv] = name
	}
	return hv
}

func (s *Netceptor) getNameFromHash(namehash uint64) (string, error) {
	s.hashLock.RLock()
	defer s.hashLock.RUnlock()
	name, ok := s.nameHashes[namehash]
	if !ok {
		return "", fmt.Errorf("hash not found")
	}
	return name, nil
}

func stringFromFixedLenBytes(bytes []byte) string {
	p := len(bytes) - 1
	for p >= 0 && bytes[p] == 0 {
		p--
	}
	if p < 0 {
		return ""
	}
	return string(bytes[:p+1])
}

func fixedLenBytesFromString(s string, l int) []byte {
	bytes := make([]byte, l)
	copy(bytes, s)
	return bytes
}

// Translates an incoming message from wire protocol to messageData object.
func (s *Netceptor) translateDataToMessage(data []byte) (*messageData, error) {
	if len(data) < 33 {
		return nil, fmt.Errorf("data too short to be a valid message")
	}
	fromNode, err := s.getNameFromHash(binary.BigEndian.Uint64(data[1:9]))
	if err != nil {
		return nil, err
	}
	toNode, err := s.getNameFromHash(binary.BigEndian.Uint64(data[9:17]))
	if err != nil {
		return nil, err
	}
	fromService := stringFromFixedLenBytes(data[17:25])
	toService := stringFromFixedLenBytes(data[25:33])
	md := &messageData{
		FromNode:    fromNode,
		FromService: fromService,
		ToNode:      toNode,
		ToService:   toService,
		Data:        data[33:],
	}
	return md, nil
}

// Translates an outgoing message from a messageData object to wire protocol.
func (s *Netceptor) translateDataFromMessage(msg *messageData) ([]byte, error) {
	data := make([]byte, 33+len(msg.Data))
	data[0] = MsgTypeData
	binary.BigEndian.PutUint64(data[1:9], s.addNameHash(msg.FromNode))
	binary.BigEndian.PutUint64(data[9:17], s.addNameHash(msg.ToNode))
	copy(data[17:25], fixedLenBytesFromString(msg.FromService, 8))
	copy(data[25:33], fixedLenBytesFromString(msg.ToService, 8))
	copy(data[33:], msg.Data)
	return data, nil
}

// Forwards a message to its next hop
func (s *Netceptor) forwardMessage(md *messageData) error {
	nextHop, ok := s.routingTable[md.ToNode]
	if !ok {
		return fmt.Errorf("no route to node")
	}
	s.connLock.RLock()
	c, ok := s.connections[nextHop]
	s.connLock.RUnlock()
	if !ok || c.WriteChan == nil {
		return fmt.Errorf("no connection to next hop")
	}
	message, err := s.translateDataFromMessage(md)
	if err != nil {
		return err
	}
	debug.Tracef("    Forwarding data length %d via %s\n", len(md.Data), nextHop)
	c.WriteChan <- message
	return nil
}

// Generates and sends a message over the Receptor network
func (s *Netceptor) sendMessage(fromService string, toNode string, toService string, data []byte) error {
	if len(fromService) > 8 || len(toService) > 8 {
		return fmt.Errorf("service name too long")
	}
	md := &messageData{
		FromNode:    s.nodeID,
		FromService: fromService,
		ToNode:      toNode,
		ToService:   toService,
		Data:        data,
	}
	debug.Tracef("--- Sending data length %d from %s:%s to %s:%s\n", len(md.Data),
		md.FromNode, md.FromService, md.ToNode, md.ToService)
	return s.handleMessageData(md)
}

// Returns an unused random service name to use as the equivalent of a TCP/IP ephemeral port number.
// Caller must already have s.structLock at least read-locked.
func (s *Netceptor) getEphemeralService() string {
	for {
		service := randstr.RandomString(8)
		_, ok := s.reservedServices[service]
		if ok {
			continue
		}
		_, ok = s.listenerRegistry[service]
		if ok {
			continue
		}
		return service
	}
}

// Prints the routing table.  Only used for debugging.
// The caller must already hold at least a read lock on known connections and routing.
func (s *Netceptor) printRoutingTable() {
	if !debug.Enable {
		return
	}
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

// Constructs a routing update message
func (s *Netceptor) makeRoutingUpdate() *routingUpdate {
	s.sequence++
	s.connLock.RLock()
	conns := make([]string, len(s.connections))
	i := 0
	for conn := range s.connections {
		conns[i] = conn
		i++
	}
	s.connLock.RUnlock()
	update := &routingUpdate{
		NodeID:         s.nodeID,
		UpdateID:       randstr.RandomString(8),
		UpdateEpoch:    s.epoch,
		UpdateSequence: s.sequence,
		Connections:    conns,
		ForwardingNode: s.nodeID,
	}
	return update
}

// Translates a routing update message to network bytes
func (s *Netceptor) translateRoutingUpdate(update *routingUpdate) ([]byte, error) {
	updateBytes, err := json.Marshal(update)
	if err != nil {
		return nil, err
	}
	data := make([]byte, len(updateBytes)+1)
	data[0] = MsgTypeRoute
	copy(data[1:], updateBytes)
	return data, nil
}

// Sends a routing update to all neighbors.
func (s *Netceptor) sendRoutingUpdate() {
	if len(s.connections) == 0 {
		return
	}
	debug.Printf("Sending routing update\n")
	message, err := s.translateRoutingUpdate(s.makeRoutingUpdate())
	if err != nil {
		return
	}
	s.flood(message, "")
}

// Processes a routing update received from a connection.
func (s *Netceptor) handleRoutingUpdate(ri *routingUpdate, recvConn string) {
	debug.Printf("Received routing update from %s via %s\n", ri.NodeID, recvConn)
	if ri.NodeID == s.nodeID || ri.NodeID == "" {
		return
	}
	s.knownNodeLock.RLock()
	_, ok := s.seenUpdates[ri.UpdateID]
	s.knownNodeLock.RUnlock()
	if ok {
		return
	}
	s.knownNodeLock.Lock()
	s.seenUpdates[ri.UpdateID] = time.Now()
	ni, ok := s.knownNodeInfo[ri.NodeID]
	s.knownNodeLock.Unlock()
	if ok {
		if ri.UpdateEpoch < ni.Epoch {
			return
		}
		if ri.UpdateEpoch == ni.Epoch && ri.UpdateSequence <= ni.Sequence {
			return
		}
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
	s.knownNodeLock.Lock()
	changed := false
	if !reflect.DeepEqual(conns, s.knownConnectionCosts[ri.NodeID]) {
		changed = true
	}
	_, ok = s.knownNodeInfo[ri.NodeID]
	if !ok {
		_ = s.addNameHash(ri.NodeID)
	}
	s.knownNodeInfo[ri.NodeID] = ni
	s.knownConnectionCosts[ri.NodeID] = conns
	for conn := range s.knownConnectionCosts {
		if conn == s.nodeID {
			continue
		}
		_, ok = conns[conn]
		if !ok {
			delete(s.knownConnectionCosts[conn], ri.NodeID)
		}
	}
	s.knownNodeLock.Unlock()
	ri.ForwardingNode = s.nodeID
	message, err := s.translateRoutingUpdate(ri)
	if err != nil {
		return
	}
	s.flood(message, recvConn)
	if changed {
		s.updateRoutingTableChan <- 0
	}
}

// Handles a ping request
func (s *Netceptor) handlePing(md *messageData) error {
	return s.sendMessage("ping", md.FromNode, md.FromService, []byte{})
}

// Dispatches a message to a reserved service.  Returns true if handled, false otherwise.
func (s *Netceptor) dispatchReservedService(md *messageData) (bool, error) {
	svc, ok := s.reservedServices[md.ToService]
	if ok {
		return true, svc(md)
	}
	return false, nil
}

// Handles incoming data and dispatches it to a service listener.
func (s *Netceptor) handleMessageData(md *messageData) error {
	if md.ToNode == s.nodeID {
		handled, err := s.dispatchReservedService(md)
		if err != nil {
			return err
		}
		if handled {
			return nil
		}
		pc, ok := s.listenerRegistry[md.ToService]
		if !ok {
			return fmt.Errorf("received message for unknown service")
		}
		pc.recvChan <- md
		return nil
	}
	return s.forwardMessage(md)
}

// Goroutine to send data from the backend to the connection's ReadChan
func (ci *connInfo) protoReader(sess BackendSession) {
	for {
		buf, err := sess.Recv()
		if err != nil {
			debug.Printf("Backend receiving error %s\n", err)
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
		message, more := <-ci.WriteChan
		if !more {
			return
		}
		err := sess.Send(message)
		if err != nil {
			debug.Printf("Backend sending error %s\n", err)
			ci.ErrorChan <- err
			return
		}
	}
}

// Continuously sends routing updates to let the other end know who we are on initial connection
func (s *Netceptor) sendInitialRoutingUpdates(ci *connInfo, initDoneChan chan bool) {
	count := 0
	for {
		ri, err := s.translateRoutingUpdate(s.makeRoutingUpdate())
		if err != nil {
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
		case <-time.After(1 * time.Second):
		case <-initDoneChan:
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
		ReadChan:  make(chan []byte),
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
		case data := <-ci.ReadChan:
			msgType := data[0]
			if msgType == MsgTypeRoute {
				ri := &routingUpdate{}
				err := json.Unmarshal(data[1:], ri)
				if err != nil {
					debug.Printf("Error unpacking routing update\n")
					continue
				}
				if !established {
					established = true
					initDoneChan <- true
					remoteNodeID = ri.ForwardingNode
					debug.Printf("Connection established with %s\n", remoteNodeID)
					s.addNameHash(remoteNodeID)
					s.connLock.Lock()
					s.connections[remoteNodeID] = ci
					s.connLock.Unlock()
					s.knownNodeLock.Lock()
					_, ok := s.knownConnectionCosts[s.nodeID]
					if !ok {
						s.knownConnectionCosts[s.nodeID] = make(map[string]float64)
					}
					s.knownConnectionCosts[s.nodeID][remoteNodeID] = 1.0
					s.knownNodeLock.Unlock()
					s.updateRoutingTableChan <- 0
					s.sendRouteFloodChan <- 0
					defer func() {
						s.connLock.Lock()
						delete(s.connections, remoteNodeID)
						s.connLock.Unlock()
						s.knownNodeLock.Lock()
						delete(s.knownConnectionCosts[remoteNodeID], s.nodeID)
						delete(s.knownConnectionCosts[s.nodeID], remoteNodeID)
						s.knownNodeLock.Unlock()
						s.updateRoutingTableChan <- 0
						s.sendRouteFloodChan <- 0
					}()
				}
				s.handleRoutingUpdate(ri, remoteNodeID)
			} else if established {
				if msgType == MsgTypeData {
					message, err := s.translateDataToMessage(data)
					if err != nil {
						debug.Printf("Error translating data to message struct\n")
						continue
					}
					debug.Tracef("--- Received data length %d from %s:%s to %s:%s via %s\n", len(message.Data),
						message.FromNode, message.FromService, message.ToNode, message.ToService, remoteNodeID)
					err = s.handleMessageData(message)
					if err != nil {
						debug.Printf("Error handling message data: %s\n", err)
					}
				} else {
					debug.Printf("Unknown message type %d\n", msgType)
				}
			}
		case err := <-ci.ErrorChan:
			return err
		case <-s.shutdownChans[0]:
			return nil
		}
	}
}

// RunBackend runs the Netceptor protocol on a backend object
func (s *Netceptor) RunBackend(b Backend, errf func(error, bool)) {
	b.Start(s.runProtocol, errf)
}
