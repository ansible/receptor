// Netceptor is the networking layer of Receptor.
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

type ErrorFunc func(error)
type BackendSessFunc func(BackendSession) error

type Backend interface {
	Start(sessFunc BackendSessFunc, errFunc ErrorFunc)
}

type BackendSession interface {
	Send([]byte) error
	Recv() ([]byte, error)
	Close() error
}

type Listener struct {
	s          		*Netceptor
	service    		string
	acceptChan      chan *Conn
	srLock			*sync.Mutex
	sessionRegistry map[string]*Conn
	open			bool
}

type Conn struct {
	li            *Listener
	localService  string
	remoteNode    string
	remoteService string
	RecvChan      chan *MessageData
	open          bool
}

type MessageData struct {
	FromNode    string
	FromService string
	ToNode      string
	ToService   string
	Data        []byte
}

type Netceptor struct {
	nodeId                 string
	epoch                  int64
	sequence               int64
	structLock             *sync.RWMutex
	connections            map[string]*connInfo
	seenUpdates            map[string]time.Time
	knownNodeInfo          map[string]*nodeInfo
	knownConnectionCosts   map[string]map[string]float64
	routingTable           map[string]string
	listenerRegistry       map[string]*Listener
	sendRouteFloodChan     chan time.Duration
	updateRoutingTableChan chan time.Duration
	shutdownChans          []chan bool
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
	NodeId         string
	UpdateId       string
	UpdateEpoch    int64
	UpdateSequence int64
	Connections    []string
	ForwardingNode string
}

func New(NodeId string) *Netceptor {
	s := Netceptor{
		nodeId:                 NodeId,
		epoch:                  time.Now().Unix(),
		structLock:             &sync.RWMutex{},
		connections:            make(map[string]*connInfo),
		seenUpdates:            make(map[string]time.Time),
		knownNodeInfo:          make(map[string]*nodeInfo),
		knownConnectionCosts:   make(map[string]map[string]float64),
		routingTable:           make(map[string]string),
		listenerRegistry:       make(map[string]*Listener),
		shutdownChans:			make([]chan bool, 4),
	}
	s.updateRoutingTableChan = tickrunner.Run(s.updateRoutingTable, time.Hour * 24, time.Second * 1, s.shutdownChans[1])
	s.sendRouteFloodChan = tickrunner.Run(s.floodRoutingUpdate, time.Second * 10, time.Millisecond * 100, s.shutdownChans[2])
	go s.monitorConnectionAging(s.shutdownChans[3])
	return &s
}

func (s *Netceptor) Shutdown() {
	s.structLock.RLock()
	defer s.structLock.RUnlock()
	for i := range s.shutdownChans {
		s.shutdownChans[i] <- true
	}
}

func (s *Netceptor) monitorConnectionAging(shutdownChan chan bool) {
	for {
		select {
		case <- time.After(5 * time.Second):
			timedOut := make([]chan error, 0)
			s.structLock.RLock()
			for i := range s.connections {
				if time.Since(s.connections[i].lastReceivedData) > 22 * time.Second {
					timedOut = append(timedOut, s.connections[i].ErrorChan)
				}
			}
			s.structLock.RUnlock()
			for i := range timedOut {
				debug.Printf("Timing out connection")
				timedOut[i] <- fmt.Errorf("connection timed out")
			}
		case <- shutdownChan:
			return
		}
	}
}

func (s *Netceptor) updateRoutingTable() {
	s.structLock.Lock()
	defer s.structLock.Unlock()
	debug.Printf("Re-calculating routing table\n")

	// Dijkstra's algorithm
	Q := priorityQueue.New()
	Q.Insert(s.nodeId, 0.0)
	cost := make(map[string]float64)
	prev := make(map[string]string)
	for node := range s.knownConnectionCosts {
		if node == s.nodeId {
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
			if prev[p] == s.nodeId {
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

func (s *Netceptor) forwardMessage(md *MessageData) error {
	nextHop, ok := s.routingTable[md.ToNode]
	if ! ok { return fmt.Errorf("no route to node") }
	s.structLock.RLock()
	writeChan := s.connections[nextHop].WriteChan
	s.structLock.RUnlock()
	debug.Printf("Forwarding message to %s via %s\n", md.ToNode, nextHop)
	if writeChan != nil {
		message, err := s.makeMessage("send", md)
		if err != nil { return err }
		writeChan <- message
		return nil
	} else {
		return fmt.Errorf("could not write to node")
	}
}

func (s *Netceptor) SendMessage(fromService string, toNode string, toService string, data []byte) error {
	md := &MessageData{
		FromNode:    s.nodeId,
		FromService: fromService,
		ToNode:      toNode,
		ToService:   toService,
		Data:        data,
	}
	return s.handleMessageData(md)
}

func (s *Netceptor) getEphemeralService() string {
	for {
		service := randstr.RandomString(128)
		_, ok := s.listenerRegistry[service]
		if !ok {
			return service
		}
	}
}

func (s *Netceptor) Listen(service string) (*Listener, error) {
	s.structLock.Lock()
	defer s.structLock.Unlock()
	if service == "" {
		service = s.getEphemeralService()
	} else {
		_, ok := s.listenerRegistry[service]
		if ok {
			return nil, fmt.Errorf("service %s is already listening", service)
		}
	}
	li := Listener{
		s:               s,
		service:         service,
		acceptChan:      make(chan *Conn),
		srLock:          &sync.Mutex{},
		sessionRegistry: make(map[string]*Conn),
		open:            true,
	}
	s.listenerRegistry[service] = &li
	return &li, nil
}

func (s *Netceptor) Dial(node string, service string) (*Conn, error) {
	s.structLock.Lock()
	defer s.structLock.Unlock()
	lservice := s.getEphemeralService()
	li := Listener{
		s:               s,
		service:         lservice,
		acceptChan:      nil,
		srLock:          &sync.Mutex{},
		sessionRegistry: make(map[string]*Conn),
		open:            true,
	}
	s.listenerRegistry[lservice] = &li
	li.srLock.Lock()
	defer li.srLock.Unlock()
	nc := Conn{
		li:            &li,
		localService:  lservice,
		remoteNode:    node,
		remoteService: service,
		RecvChan:      make(chan *MessageData),
		open:          true,
	}
	li.sessionRegistry[service] = &nc
	return &nc, nil
}

func(li *Listener) Accept() *Conn {
	nc := <- li.acceptChan
	li.srLock.Lock()
	defer li.srLock.Unlock()
	li.sessionRegistry[nc.localService] = nc
	return nc
}

func (li *Listener) Close() {
	li.s.structLock.Lock()
	defer li.s.structLock.Unlock()
	li.open = false
	delete(li.s.listenerRegistry, li.service)
}

func(nc *Conn) Send(data []byte) error {
	if ! nc.open || ! nc.li.open {
		return fmt.Errorf("cannot send on closed connection")
	}
	return nc.li.s.SendMessage(nc.localService, nc.remoteNode, nc.remoteService, data)
}

func(nc *Conn) Recv() ([]byte, error) {
	if ! nc.open || ! nc.li.open {
		return nil, fmt.Errorf("cannot receive on closed connection")
	}
	m := <- nc.RecvChan
	return m.Data, nil
}

func (nc *Conn) LocalService() string {
	return nc.localService
}

func (nc *Conn) RemoteNode() string {
	return nc.remoteNode
}

func (nc *Conn) RemoteService() string {
	return nc.remoteService
}

func(nc *Conn) Close() {
	nc.open = false
}

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

func (s *Netceptor) makeRoutingUpdate() ([]byte, error) {
	s.sequence += 1
	s.structLock.RLock()
	conns := make([]string, len(s.connections))
	i := 0
	for conn := range s.connections {
		conns[i] = conn
		i++
	}
	s.structLock.RUnlock()
	update := routingUpdate{
		NodeId:         s.nodeId,
		UpdateId:       randstr.RandomString(128),
		UpdateEpoch:    s.epoch,
		UpdateSequence: s.sequence,
		Connections:    conns,
		ForwardingNode: s.nodeId,
	}
	message, err := s.makeMessage("route", update); if err != nil {
		return nil, err
	}
	return message, nil
}

func (s *Netceptor) floodRoutingUpdate() {
	debug.Printf("Sending routing update\n")
	message, err := s.makeRoutingUpdate()
	if err != nil { return }
	s.flood(message, "")
}

func (s *Netceptor) handleRoutingUpdate(ri *routingUpdate, recvConn string) {
	debug.Printf("Received routing update from %s via %s\n", ri.NodeId, recvConn)
	if ri.NodeId == s.nodeId || ri.NodeId == "" { return }
	s.structLock.RLock()
	_, ok := s.seenUpdates[ri.UpdateId]
	s.structLock.RUnlock()
	if ok { return }
	s.structLock.Lock()
	s.seenUpdates[ri.UpdateId] = time.Now()
	ni, ok := s.knownNodeInfo[ri.NodeId]
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
	if ! reflect.DeepEqual(conns, s.knownConnectionCosts[ri.NodeId]) {
		changed = true
	}
	s.knownNodeInfo[ri.NodeId] = ni
	s.knownConnectionCosts[ri.NodeId] = conns
	for conn := range s.knownConnectionCosts {
		if conn == s.nodeId {
			continue
		}
		_, ok = conns[conn]
		if ! ok {
			delete(s.knownConnectionCosts[conn], ri.NodeId)
		}
	}
	s.structLock.Unlock()
	ri.ForwardingNode = s.nodeId
	message, err := s.makeMessage("route", ri)
	if err != nil { return }
	s.flood(message, recvConn)
	if changed {
		s.updateRoutingTableChan <- 0
	}
}

func(s *Netceptor) handleMessageData(md *MessageData) error {
	if md.ToNode == s.nodeId {
		li, ok := s.listenerRegistry[md.ToService]; if ! ok {
			return fmt.Errorf("received message for unknown service")
		}
		nc, ok := li.sessionRegistry[md.FromService]
		if !ok {
			nc = &Conn{
				li:            li,
				localService:  md.ToService,
				remoteNode:    md.FromNode,
				remoteService: md.FromService,
				RecvChan:      make(chan *MessageData),
				open:          true,
			}
			li.sessionRegistry[md.FromService] = nc
			li.acceptChan <- nc
		}
		nc.RecvChan <- md
		return nil
	} else {
		return s.forwardMessage(md)
	}
}

func (s *Netceptor) handleSend(data []byte) error {
	md := &MessageData{}
	err := json.Unmarshal(data, md)
	if err != nil { return err }
	return s.handleMessageData(md)
}

func (ci *connInfo) protoReader(sess BackendSession) {
	for {
		message, err := sess.Recv()
		ci.lastReceivedData = time.Now()
		if err != nil {
			debug.Printf("UDP receiving error %s\n", err)
			ci.ErrorChan <- err
			return
		}
		ci.ReadChan <- message
	}
}

func (ci *connInfo) protoWriter(sess BackendSession) {
	for {
		message, more := <- ci.WriteChan
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

func (s *Netceptor) sendInitialRoutingUpdates(ci *connInfo, initDoneChan chan bool) {
	count := 0
	for {
		ri, err := s.makeRoutingUpdate(); if err != nil {
			debug.Printf("Error Sending initial routing message: %s\n", err)
			return
		}
		debug.Printf("Sending initial routing message\n")
		ci.WriteChan <- ri
		count += 1
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
	remoteNodeId := ""
	for {
		select {
		case message := <- ci.ReadChan:
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
					remoteNodeId = ri.ForwardingNode
					debug.Printf("Connection established with %s\n", remoteNodeId)
					s.structLock.Lock()
					s.connections[remoteNodeId] = ci
					_, ok := s.knownConnectionCosts[s.nodeId]
					if ! ok {
						s.knownConnectionCosts[s.nodeId] = make(map[string]float64)
					}
					s.knownConnectionCosts[s.nodeId][remoteNodeId] = 1.0
					s.structLock.Unlock()
					s.updateRoutingTableChan <- 0
					s.sendRouteFloodChan <- 0
					defer func() {
						s.structLock.Lock()
						delete(s.connections, remoteNodeId)
						delete(s.knownConnectionCosts[remoteNodeId], s.nodeId)
						delete(s.knownConnectionCosts[s.nodeId], remoteNodeId)
						s.structLock.Unlock()
						s.updateRoutingTableChan <- 0
						s.sendRouteFloodChan <- 0
					}()
				}
				s.handleRoutingUpdate(ri, remoteNodeId)
			} else if established {
				if command == "send" {
					_ = s.handleSend(data)
				} else {
					debug.Printf("Unknown command: %s, Data: %s\n", command, data)
				}
			}
		case err := <- ci.ErrorChan:
			return err
		case <- s.shutdownChans[0]:
			return nil
		}
	}
}

func (s *Netceptor) RunBackend(b Backend, errf func(error)) {
	b.Start(s.runProtocol, errf)
}