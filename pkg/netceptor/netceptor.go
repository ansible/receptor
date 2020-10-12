// Package netceptor is the networking layer of Receptor.
package netceptor

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	priorityQueue "github.com/jupp0r/go-priority-queue"
	"github.com/minio/highwayhash"
	"github.com/project-receptor/receptor/pkg/logger"
	"github.com/project-receptor/receptor/pkg/randstr"
	"github.com/project-receptor/receptor/pkg/tickrunner"
	"github.com/project-receptor/receptor/pkg/utils"
	"io"
	"math"
	"reflect"
	"strings"
	"sync"
	"time"
)

// MTU is the largest message sendable over the Netecptor network
const MTU = 16384

// RouteUpdateTime is the interval at which regular route updates will be sent
const RouteUpdateTime = 10 * time.Second

// ServiceAdTime is the interval at which regular service advertisements will be sent
const ServiceAdTime = 60 * time.Second

// SeenUpdateExpireTime is the age after which routing update IDs can be discarded
const SeenUpdateExpireTime = 1 * time.Hour

// MaxForwardingHops is the maximum number of times that Netceptor will forward a data packet
const MaxForwardingHops = 30

// MainInstance is the global instance of Netceptor instantiated by the command-line main() function
var MainInstance *Netceptor

// ErrorFunc is a function parameter used to process errors. The boolean parameter
// indicates whether the error is fatal (i.e. the associated process is going to exit).
type ErrorFunc func(error, bool)

// ErrTimeout is returned for an expired deadline.
var ErrTimeout error = &TimeoutError{}

// TimeoutError is returned for an expired deadline.
type TimeoutError struct{}

// Error returns a string describing the error.
func (e *TimeoutError) Error() string { return "i/o timeout" }

// Timeout returns true if this error was a timeout.
func (e *TimeoutError) Timeout() bool { return true }

// Temporary returns true if a retry is likely a good idea.
func (e *TimeoutError) Temporary() bool { return true }

// Backend is the interface for back-ends that the Receptor network can run over
type Backend interface {
	Start(context.Context) (chan BackendSession, error)
}

// BackendSession is the interface for a single session of a back-end.
// Backends must be DATAGRAM ORIENTED, meaning that Recv() must return
// whole packets sent by Send(). If the underlying protocol is stream
// oriented, then the backend must deal with any required buffering.
type BackendSession interface {
	Send([]byte) error
	Recv(time.Duration) ([]byte, error) // Must return netceptor.ErrTimeout if the timeout is exceeded
	Close() error
}

// Netceptor is the main object of the Receptor mesh network protocol
type Netceptor struct {
	nodeID                 string
	allowedPeers           []string
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
	routingPathCosts       map[string]float64
	listenerLock           *sync.RWMutex
	listenerRegistry       map[string]*PacketConn
	sendRouteFloodChan     chan time.Duration
	updateRoutingTableChan chan time.Duration
	context                context.Context
	cancelFunc             context.CancelFunc
	hashLock               *sync.RWMutex
	nameHashes             map[uint64]string
	reservedServices       map[string]func(*messageData) error
	serviceAdsLock         *sync.RWMutex
	serviceAdsReceived     map[string]map[string]*ServiceAdvertisement
	sendServiceAdsChan     chan time.Duration
	backendWaitGroup       sync.WaitGroup
	backendCount           int
	networkName            string
	serverTLSConfigs       map[string]*tls.Config
	clientTLSConfigs       map[string]*tls.Config
	unreachableBroker      *utils.Broker
	routingUpdateBroker    *utils.Broker
}

// ConnStatus holds information about a single connection in the Status struct.
type ConnStatus struct {
	NodeID string
	Cost   float64
}

// Status is the struct returned by Netceptor.Status().  It represents a public
// view of the internal status of the Netceptor object.
type Status struct {
	NodeID               string
	Connections          []*ConnStatus
	RoutingTable         map[string]string
	Advertisements       []*ServiceAdvertisement
	KnownConnectionCosts map[string]map[string]float64
}

const (
	// MsgTypeData is a normal data-containing message
	MsgTypeData = 0
	// MsgTypeRoute is a routing update
	MsgTypeRoute = 1
	// MsgTypeServiceAdvertisement is an advertisement for a service
	MsgTypeServiceAdvertisement = 2
	// MsgTypeReject indicates a rejection (closure) of a backend connection
	MsgTypeReject = 3
)

const (
	// ProblemServiceUnknown occurs when a message arrives for a non-listening service
	ProblemServiceUnknown = "service unknown"
	// ProblemExpiredInTransit occurs when a message's HopsToLive expires in transit
	ProblemExpiredInTransit = "message expired"
)

type messageData struct {
	FromNode    string
	FromService string
	ToNode      string
	ToService   string
	HopsToLive  byte
	Data        []byte
}

type connInfo struct {
	ReadChan         chan []byte
	WriteChan        chan []byte
	Context          context.Context
	CancelFunc       context.CancelFunc
	Cost             float64
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
	Connections    map[string]float64
	ForwardingNode string
}

// ServiceAdvertisement is the data associated with a service advertisement
type ServiceAdvertisement struct {
	NodeID  string
	Service string
	Time    time.Time
	Tags    map[string]string
}

// serviceAdvertisementFull is the whole message from the network
type serviceAdvertisementFull struct {
	*ServiceAdvertisement
	Cancel bool
}

// UnreachableMessage is the on-the-wire data associated with an unreachable message
type UnreachableMessage struct {
	FromNode    string
	ToNode      string
	FromService string
	ToService   string
	Problem     string
}

// UnreachableNotification includes additional information returned from SubscribeUnreachable
type UnreachableNotification struct {
	UnreachableMessage
	ReceivedFromNode string
}

var networkNames = make([]string, 0)
var networkNamesLock = sync.Mutex{}

//makeNetworkName returns a network name that is unique within global scope
func makeNetworkName(nodeID string) string {
	networkNamesLock.Lock()
	defer networkNamesLock.Unlock()
	nameCounter := 1
	proposedName := fmt.Sprintf("netceptor-%s", nodeID)
	for {
		good := true
		for i := range networkNames {
			if networkNames[i] == proposedName {
				good = false
				break
			}
		}
		if good {
			networkNames = append(networkNames, proposedName)
			return proposedName
		}
		nameCounter++
		proposedName = fmt.Sprintf("netceptor-%s-%d", nodeID, nameCounter)
	}
}

// New constructs a new Receptor network protocol instance
func New(ctx context.Context, NodeID string, AllowedPeers []string) *Netceptor {
	s := Netceptor{
		nodeID:                 NodeID,
		allowedPeers:           AllowedPeers,
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
		routingPathCosts:       make(map[string]float64),
		listenerLock:           &sync.RWMutex{},
		listenerRegistry:       make(map[string]*PacketConn),
		sendRouteFloodChan:     nil,
		updateRoutingTableChan: nil,
		hashLock:               &sync.RWMutex{},
		nameHashes:             make(map[uint64]string),
		serviceAdsLock:         &sync.RWMutex{},
		serviceAdsReceived:     make(map[string]map[string]*ServiceAdvertisement),
		sendServiceAdsChan:     nil,
		backendWaitGroup:       sync.WaitGroup{},
		backendCount:           0,
		networkName:            makeNetworkName(NodeID),
		clientTLSConfigs:       make(map[string]*tls.Config),
		serverTLSConfigs:       make(map[string]*tls.Config),
	}
	s.reservedServices = map[string]func(*messageData) error{
		"ping":    s.handlePing,
		"unreach": s.handleUnreachable,
	}
	if ctx == nil {
		ctx = context.Background()
	}
	s.clientTLSConfigs["default"] = &tls.Config{}
	s.addNameHash(NodeID)
	s.context, s.cancelFunc = context.WithCancel(ctx)
	s.unreachableBroker = utils.NewBroker(s.context, reflect.TypeOf(UnreachableNotification{}))
	s.routingUpdateBroker = utils.NewBroker(s.context, reflect.TypeOf(map[string]string{}))
	s.updateRoutingTableChan = tickrunner.Run(s.context, s.updateRoutingTable, time.Hour*24, time.Millisecond*100)
	s.sendRouteFloodChan = tickrunner.Run(s.context, s.sendRoutingUpdate, RouteUpdateTime, time.Millisecond*100)
	s.sendServiceAdsChan = tickrunner.Run(s.context, s.sendServiceAds, ServiceAdTime, time.Second*5)
	go s.monitorConnectionAging()
	go s.expireSeenUpdates()
	return &s
}

// NewAddr generates a Receptor network address from a node ID and service name
func (s *Netceptor) NewAddr(node string, service string) Addr {
	return Addr{
		network: s.networkName,
		node:    node,
		service: service,
	}
}

// Context returns the context for this Netceptor instance
func (s *Netceptor) Context() context.Context {
	return s.context
}

// Shutdown shuts down a Netceptor instance
func (s *Netceptor) Shutdown() {
	s.cancelFunc()
}

// NodeID returns the local Node ID of this Netceptor instance
func (s *Netceptor) NodeID() string {
	return s.nodeID
}

// AddBackend adds a backend to the Netceptor system
func (s *Netceptor) AddBackend(backend Backend, connectionCost float64, nodeCost map[string]float64) error {
	sessChan, err := backend.Start(s.context)
	if err != nil {
		return err
	}
	s.backendWaitGroup.Add(1)
	s.backendCount++
	go func() {
		defer s.backendWaitGroup.Done()
		for {
			select {
			case sess, ok := <-sessChan:
				if ok {
					s.backendWaitGroup.Add(1)
					go func() {
						err := s.runProtocol(sess, connectionCost, nodeCost)
						s.backendWaitGroup.Done()
						if err != nil {
							logger.Error("Backend error: %s\n", err)
						}
					}()
				} else {
					return
				}
			case <-s.context.Done():
				return
			}
		}
	}()
	return nil
}

// BackendWait waits for the backend wait group
func (s *Netceptor) BackendWait() {
	s.backendWaitGroup.Wait()
}

// BackendCount returns the number of backends that ever registered with this Netceptor
func (s *Netceptor) BackendCount() int {
	return s.backendCount
}

// Status returns the current state of the Netceptor object
func (s *Netceptor) Status() Status {
	s.connLock.RLock()
	conns := make([]*ConnStatus, 0)
	for conn := range s.connections {
		conns = append(conns, &ConnStatus{
			NodeID: conn,
			Cost:   s.connections[conn].Cost,
		})
	}
	s.connLock.RUnlock()
	s.routingTableLock.RLock()
	routes := make(map[string]string)
	for k, v := range s.routingTable {
		routes[k] = v
	}
	s.routingTableLock.RUnlock()
	s.serviceAdsLock.RLock()
	serviceAds := make([]*ServiceAdvertisement, 0)
	for n := range s.serviceAdsReceived {
		for _, ad := range s.serviceAdsReceived[n] {
			adCopy := *ad
			if adCopy.NodeID == s.nodeID {
				adCopy.Time = time.Now()
			}
			serviceAds = append(serviceAds, &adCopy)
		}
	}
	s.serviceAdsLock.RUnlock()
	s.knownNodeLock.RLock()
	knownConnectionCosts := make(map[string]map[string]float64)
	for k1, v1 := range s.knownConnectionCosts {
		knownConnectionCosts[k1] = make(map[string]float64)
		for k2, v2 := range v1 {
			knownConnectionCosts[k1][k2] = v2
		}
	}
	s.knownNodeLock.RUnlock()
	return Status{
		NodeID:               s.nodeID,
		Connections:          conns,
		RoutingTable:         routes,
		Advertisements:       serviceAds,
		KnownConnectionCosts: knownConnectionCosts,
	}
}

// PathCost returns the cost to a given remote node, or an error if the node doesn't exist.
func (s *Netceptor) PathCost(nodeID string) (float64, error) {
	s.routingTableLock.RLock()
	defer s.routingTableLock.RUnlock()
	cost, ok := s.routingPathCosts[nodeID]
	if !ok {
		return 0, fmt.Errorf("node not found")
	}
	return cost, nil
}

func (s *Netceptor) addLocalServiceAdvertisement(service string, tags map[string]string) {
	s.serviceAdsLock.Lock()
	defer s.serviceAdsLock.Unlock()
	n, ok := s.serviceAdsReceived[s.nodeID]
	if !ok {
		n = make(map[string]*ServiceAdvertisement)
		s.serviceAdsReceived[s.nodeID] = n
	}
	n[service] = &ServiceAdvertisement{
		NodeID:  s.nodeID,
		Service: service,
		Time:    time.Now(),
		Tags:    tags,
	}
	s.sendServiceAdsChan <- 0
}

func (s *Netceptor) removeLocalServiceAdvertisement(service string) error {
	s.serviceAdsLock.Lock()
	defer s.serviceAdsLock.Unlock()
	n, ok := s.serviceAdsReceived[s.nodeID]
	if ok {
		delete(n, service)
	}
	sa := &serviceAdvertisementFull{
		ServiceAdvertisement: &ServiceAdvertisement{
			NodeID:  s.nodeID,
			Service: service,
			Time:    time.Now(),
			Tags:    nil,
		},
		Cancel: true,
	}
	data, err := s.translateStructToNetwork(MsgTypeServiceAdvertisement, sa)
	if err != nil {
		return err
	}
	s.flood(data, "")
	return nil
}

// Send a single service broadcast
func (s *Netceptor) sendServiceAd(si *ServiceAdvertisement) error {
	logger.Debug("Sending service advertisement: %s\n", si)
	sf := serviceAdvertisementFull{
		ServiceAdvertisement: si,
		Cancel:               false,
	}
	data, err := s.translateStructToNetwork(MsgTypeServiceAdvertisement, sf)
	if err != nil {
		return err
	}
	s.flood(data, "")
	return nil
}

// Send advertisements for all advertised services
func (s *Netceptor) sendServiceAds() {
	ads := make([]ServiceAdvertisement, 0)
	s.listenerLock.RLock()
	for sn := range s.listenerRegistry {
		if s.listenerRegistry[sn].advertise {
			sa := ServiceAdvertisement{
				NodeID:  s.nodeID,
				Service: sn,
				Time:    time.Now(),
				Tags:    s.listenerRegistry[sn].adTags,
			}
			ads = append(ads, sa)
		}
	}
	s.listenerLock.RUnlock()
	for i := range ads {
		err := s.sendServiceAd(&ads[i])
		if err != nil {
			logger.Error("Error sending service advertisement: %s\n", err)
		}
	}
}

// Watches connections and expires any that haven't seen traffic in too long
func (s *Netceptor) monitorConnectionAging() {
	for {
		select {
		case <-time.After(5 * time.Second):
			timedOut := make([]context.CancelFunc, 0)
			s.connLock.RLock()
			for i := range s.connections {
				if time.Since(s.connections[i].lastReceivedData) > (2*RouteUpdateTime + 1*time.Second) {
					timedOut = append(timedOut, s.connections[i].CancelFunc)
				}
			}
			s.connLock.RUnlock()
			for i := range timedOut {
				logger.Warning("Timing out connection\n")
				timedOut[i]()
			}
		case <-s.context.Done():
			return
		}
	}
}

// Expires old updates from the seenUpdates table
func (s *Netceptor) expireSeenUpdates() {
	for {
		select {
		case <-time.After(SeenUpdateExpireTime / 2):
			thresholdTime := time.Now().Add(-SeenUpdateExpireTime)
			s.knownNodeLock.Lock()
			for id := range s.seenUpdates {
				if s.seenUpdates[id].Before(thresholdTime) {
					delete(s.seenUpdates, id)
				}
			}
			s.knownNodeLock.Unlock()
		case <-s.context.Done():
			return
		}
	}
}

// Recalculates the next-hop table based on current knowledge of the network
func (s *Netceptor) updateRoutingTable() {
	s.knownNodeLock.RLock()
	defer s.knownNodeLock.RUnlock()
	logger.Debug("Re-calculating routing table\n")

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
	s.routingPathCosts = cost
	routingTableCopy := make(map[string]string)
	for k, v := range s.routingTable {
		routingTableCopy[k] = v
	}
	go s.routingUpdateBroker.Publish(routingTableCopy)
	s.printRoutingTable()
}

// SubscribeRoutingUpdates subscribes for messages when the routing table is changed
func (s *Netceptor) SubscribeRoutingUpdates() chan map[string]string {
	iChan := s.routingUpdateBroker.Subscribe()
	uChan := make(chan map[string]string)
	go func() {
		for {
			select {
			case msgIf, ok := <-iChan:
				if !ok {
					close(uChan)
					return
				}
				msg, ok := msgIf.(map[string]string)
				if !ok {
					continue
				}
				select {
				case uChan <- msg:
				case <-s.context.Done():
					close(uChan)
					return
				}
			case <-s.context.Done():
				close(uChan)
				return
			}
		}
	}()
	return uChan
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

// GetServerTLSConfig retrieves a server TLS config by name
func (s *Netceptor) GetServerTLSConfig(name string) (*tls.Config, error) {
	if name == "" {
		return nil, nil
	}
	sc, ok := s.serverTLSConfigs[name]
	if !ok {
		return nil, fmt.Errorf("unknown TLS config %s", name)
	}
	return sc.Clone(), nil
}

// SetServerTLSConfig stores a server TLS config by name
func (s *Netceptor) SetServerTLSConfig(name string, config *tls.Config) error {
	if name == "" {
		return fmt.Errorf("must provide a name")
	}
	s.serverTLSConfigs[name] = config
	return nil
}

// GetClientTLSConfig retrieves a client TLS config by name
func (s *Netceptor) GetClientTLSConfig(name string, expectedHostName string) (*tls.Config, error) {
	if name == "" {
		return nil, nil
	}
	cc, ok := s.clientTLSConfigs[name]
	if !ok {
		return nil, fmt.Errorf("unknown TLS config %s", name)
	}
	cc = cc.Clone()
	cc.ServerName = expectedHostName
	return cc, nil
}

// SetClientTLSConfig stores a client TLS config by name
func (s *Netceptor) SetClientTLSConfig(name string, config *tls.Config) error {
	if name == "" {
		return fmt.Errorf("must provide a name")
	}
	s.clientTLSConfigs[name] = config
	return nil
}

// All-zero seed for deterministic highwayhash
var zerokey = make([]byte, 32)

// Hash a name and add it to the lookup table
func (s *Netceptor) addNameHash(name string) uint64 {
	if strings.EqualFold(name, "localhost") {
		name = s.nodeID
	}
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

// Looks up a name given a hash received from the network
func (s *Netceptor) getNameFromHash(namehash uint64) (string, error) {
	s.hashLock.RLock()
	defer s.hashLock.RUnlock()
	name, ok := s.nameHashes[namehash]
	if !ok {
		return "", fmt.Errorf("hash not found")
	}
	return name, nil
}

// Given a string, returns a fixed-length buffer right-padded with null (0) bytes
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

// Given a fixed-length buffer, returns a string excluding any null (0) bytes on the right
func fixedLenBytesFromString(s string, l int) []byte {
	bytes := make([]byte, l)
	copy(bytes, s)
	return bytes
}

// Translates an incoming message from wire protocol to messageData object.
func (s *Netceptor) translateDataToMessage(data []byte) (*messageData, error) {
	if len(data) < 36 {
		return nil, fmt.Errorf("data too short to be a valid message")
	}
	fromNode, err := s.getNameFromHash(binary.BigEndian.Uint64(data[4:12]))
	if err != nil {
		return nil, err
	}
	toNode, err := s.getNameFromHash(binary.BigEndian.Uint64(data[12:20]))
	if err != nil {
		return nil, err
	}
	fromService := stringFromFixedLenBytes(data[20:28])
	toService := stringFromFixedLenBytes(data[28:36])
	md := &messageData{
		FromNode:    fromNode,
		FromService: fromService,
		ToNode:      toNode,
		ToService:   toService,
		HopsToLive:  data[1],
		Data:        data[36:],
	}
	return md, nil
}

// Translates an outgoing message from a messageData object to wire protocol.
func (s *Netceptor) translateDataFromMessage(msg *messageData) ([]byte, error) {
	data := make([]byte, 36+len(msg.Data))
	data[0] = MsgTypeData
	data[1] = msg.HopsToLive
	binary.BigEndian.PutUint64(data[4:12], s.addNameHash(msg.FromNode))
	binary.BigEndian.PutUint64(data[12:20], s.addNameHash(msg.ToNode))
	copy(data[20:28], fixedLenBytesFromString(msg.FromService, 8))
	copy(data[28:36], fixedLenBytesFromString(msg.ToService, 8))
	copy(data[36:], msg.Data)
	return data, nil
}

// Forwards a message to its next hop
func (s *Netceptor) forwardMessage(md *messageData) error {
	if md.HopsToLive <= 0 {
		if md.FromService != "unreach" {
			_ = s.sendUnreachable(md.FromNode, &UnreachableMessage{
				FromNode:    md.FromNode,
				ToNode:      md.ToNode,
				FromService: md.FromService,
				ToService:   md.ToService,
				Problem:     ProblemExpiredInTransit,
			})
		}
		return nil
	}
	s.routingTableLock.RLock()
	nextHop, ok := s.routingTable[md.ToNode]
	s.routingTableLock.RUnlock()
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
	// decrement HopsToLive
	message[1]--
	logger.Trace("    Forwarding data length %d via %s\n", len(md.Data), nextHop)
	c.WriteChan <- message
	return nil
}

// Generates and sends a message over the Receptor network, specifying HopsToLive
func (s *Netceptor) sendMessageWithHopsToLive(fromService string, toNode string, toService string, data []byte, hopsToLive byte) error {
	if len(fromService) > 8 || len(toService) > 8 {
		return fmt.Errorf("service name too long")
	}
	if strings.EqualFold(toNode, "localhost") {
		toNode = s.nodeID
	}
	md := &messageData{
		FromNode:    s.nodeID,
		FromService: fromService,
		ToNode:      toNode,
		ToService:   toService,
		HopsToLive:  hopsToLive,
		Data:        data,
	}
	logger.Trace("--- Sending data length %d from %s:%s to %s:%s\n", len(md.Data),
		md.FromNode, md.FromService, md.ToNode, md.ToService)
	return s.handleMessageData(md)
}

// Generates and sends a message over the Receptor network
func (s *Netceptor) sendMessage(fromService string, toNode string, toService string, data []byte) error {
	return s.sendMessageWithHopsToLive(fromService, toNode, toService, data, MaxForwardingHops)
}

// Returns an unused random service name to use as the equivalent of a TCP/IP ephemeral port number.
func (s *Netceptor) getEphemeralService() string {
	s.listenerLock.RLock()
	defer s.listenerLock.RUnlock()
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

// Prints the routing table.
// The caller must already hold at least a read lock on known connections and routing.
func (s *Netceptor) printRoutingTable() {
	logLevel, _ := logger.GetLogLevelByName("Info")
	if logger.GetLogLevel() < logLevel {
		return
	}
	logger.Log(logLevel, "Known Connections:\n")
	for conn := range s.knownConnectionCosts {
		sb := &strings.Builder{}
		_, _ = fmt.Fprintf(sb, "   %s: ", conn)
		for peer := range s.knownConnectionCosts[conn] {
			_, _ = fmt.Fprintf(sb, "%s(%.2f) ", peer, s.knownConnectionCosts[conn][peer])
		}
		_, _ = fmt.Fprintf(sb, "\n")
		logger.Log(logLevel, sb.String())
	}
	logger.Log(logLevel, "Routing Table:\n")
	for node := range s.routingTable {
		logger.Log(logLevel, "   %s via %s\n", node, s.routingTable[node])
	}
}

// Constructs a routing update message
func (s *Netceptor) makeRoutingUpdate() *routingUpdate {
	s.sequence++
	s.connLock.RLock()
	conns := make(map[string]float64)
	for conn := range s.connections {
		conns[conn] = s.connections[conn].Cost
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

// Translates an arbitrary struct to a network message
func (s *Netceptor) translateStructToNetwork(messageType byte, content interface{}) ([]byte, error) {
	contentBytes, err := json.Marshal(content)
	if err != nil {
		return nil, err
	}
	data := make([]byte, len(contentBytes)+1)
	data[0] = messageType
	copy(data[1:], contentBytes)
	return data, nil
}

// Sends a routing update to all neighbors.
func (s *Netceptor) sendRoutingUpdate() {
	if len(s.connections) == 0 {
		return
	}
	ru := s.makeRoutingUpdate()
	sb := make([]string, 0)
	for conn := range ru.Connections {
		sb = append(sb, fmt.Sprintf("%s(%.2f)", conn, ru.Connections[conn]))
	}
	logger.Debug("Sending routing update. Connections: %s\n", strings.Join(sb, " "))
	message, err := s.translateStructToNetwork(MsgTypeRoute, ru)
	if err != nil {
		return
	}
	s.flood(message, "")
}

// Processes a routing update received from a connection.
func (s *Netceptor) handleRoutingUpdate(ri *routingUpdate, recvConn string) {
	logger.Debug("Received routing update from %s via %s\n", ri.NodeID, recvConn)
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
	if ok {
		if ri.UpdateEpoch < ni.Epoch {
			s.knownNodeLock.Unlock()
			return
		}
		if ri.UpdateEpoch == ni.Epoch && ri.UpdateSequence <= ni.Sequence {
			s.knownNodeLock.Unlock()
			return
		}
	} else {
		s.sendRouteFloodChan <- 0
		ni = &nodeInfo{}
	}
	ni.Epoch = ri.UpdateEpoch
	ni.Sequence = ri.UpdateSequence
	changed := false
	if !reflect.DeepEqual(ri.Connections, s.knownConnectionCosts[ri.NodeID]) {
		changed = true
	}
	_, ok = s.knownNodeInfo[ri.NodeID]
	if !ok {
		_ = s.addNameHash(ri.NodeID)
	}
	s.knownNodeInfo[ri.NodeID] = ni
	if changed {
		s.knownConnectionCosts[ri.NodeID] = make(map[string]float64)
		for k, v := range ri.Connections {
			s.knownConnectionCosts[ri.NodeID][k] = v
		}
		for conn := range s.knownConnectionCosts {
			if conn == s.nodeID {
				continue
			}
			_, ok = ri.Connections[conn]
			if !ok {
				delete(s.knownConnectionCosts[conn], ri.NodeID)
			}
		}
	}
	s.knownNodeLock.Unlock()
	ri.ForwardingNode = s.nodeID
	message, err := s.translateStructToNetwork(MsgTypeRoute, ri)
	if err != nil {
		return
	}
	s.flood(message, recvConn)
	if changed {
		s.updateRoutingTableChan <- 100 * time.Millisecond
	}
}

// Handles a ping request
func (s *Netceptor) handlePing(md *messageData) error {
	return s.sendMessage("ping", md.FromNode, md.FromService, []byte{})
}

// Handles an unreachable response
func (s *Netceptor) handleUnreachable(md *messageData) error {
	unrMsg := UnreachableMessage{}
	err := json.Unmarshal(md.Data, &unrMsg)
	if err != nil {
		return err
	}
	unrData := UnreachableNotification{
		UnreachableMessage: unrMsg,
		ReceivedFromNode:   md.FromNode,
	}
	logger.Warning("Received unreachable message from %s", md.FromNode)
	return s.unreachableBroker.Publish(unrData)
}

// Sends an unreachable response
func (s *Netceptor) sendUnreachable(ToNode string, message *UnreachableMessage) error {
	bytes, err := json.Marshal(message)
	if err != nil {
		return err
	}
	err = s.sendMessage("unreach", ToNode, "unreach", bytes)
	if err != nil {
		return err
	}
	return nil
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
		s.listenerLock.RLock()
		pc, ok := s.listenerRegistry[md.ToService]
		if !ok || pc.context.Err() != nil {
			s.listenerLock.RUnlock()
			if md.FromNode == s.nodeID {
				return fmt.Errorf(ProblemServiceUnknown)
			}
			_ = s.sendUnreachable(md.FromNode, &UnreachableMessage{
				FromNode:    md.FromNode,
				ToNode:      md.ToNode,
				FromService: md.FromService,
				ToService:   md.ToService,
				Problem:     ProblemServiceUnknown,
			})
			return nil
		}
		pc.recvChan <- md
		s.listenerLock.RUnlock()
		return nil
	}
	return s.forwardMessage(md)
}

// GetServiceInfo returns the advertising info, if any, for a service on a node
func (s *Netceptor) GetServiceInfo(NodeID string, Service string) (*ServiceAdvertisement, bool) {
	s.serviceAdsLock.RLock()
	defer s.serviceAdsLock.RUnlock()
	n, ok := s.serviceAdsReceived[NodeID]
	if !ok {
		return nil, false
	}
	svc, ok := n[Service]
	if !ok {
		return nil, false
	}
	svcCopy := *svc
	return &svcCopy, true
}

// Handles an incoming service advertisement
func (s *Netceptor) handleServiceAdvertisement(data []byte, receivedFrom string) error {
	if data[0] != MsgTypeServiceAdvertisement {
		return fmt.Errorf("message is the wrong type")
	}
	si := &serviceAdvertisementFull{}
	err := json.Unmarshal(data[1:], si)
	if err != nil {
		return err
	}
	logger.Debug("Received service advertisement %v\n", si)
	s.serviceAdsLock.Lock()
	defer s.serviceAdsLock.Unlock()
	n, ok := s.serviceAdsReceived[si.NodeID]
	if !ok {
		n = make(map[string]*ServiceAdvertisement)
		s.serviceAdsReceived[si.NodeID] = n
	}
	curSvc, keepCur := n[si.Service]
	if keepCur {
		if si.Time.After(curSvc.Time) {
			keepCur = false
		}
	}
	if keepCur {
		return nil
	}
	if si.Cancel {
		delete(s.serviceAdsReceived[si.NodeID], si.Service)
		if len(s.serviceAdsReceived[si.NodeID]) == 0 {
			delete(s.serviceAdsReceived, si.NodeID)
		}
	} else {
		s.serviceAdsReceived[si.NodeID][si.Service] = si.ServiceAdvertisement
	}
	s.flood(data, receivedFrom)
	return nil
}

// Goroutine to send data from the backend to the connection's ReadChan
func (ci *connInfo) protoReader(sess BackendSession) {
	for {
		buf, err := sess.Recv(1 * time.Second)
		select {
		case <-ci.Context.Done():
			return
		default:
		}
		if err == ErrTimeout {
			continue
		}
		if err != nil {
			if err != io.EOF && ci.Context.Err() == nil {
				logger.Error("Backend receiving error %s\n", err)
			}
			ci.CancelFunc()
			return
		}
		ci.lastReceivedData = time.Now()
		ci.ReadChan <- buf
	}
}

// Goroutine to send data from the connection's WriteChan to the backend
func (ci *connInfo) protoWriter(sess BackendSession) {
	for {
		select {
		case <-ci.Context.Done():
			return
		case message, more := <-ci.WriteChan:
			if !more {
				return
			}
			err := sess.Send(message)
			if err != nil {
				if ci.Context.Err() == nil {
					logger.Error("Backend sending error %s\n", err)
				}
				ci.CancelFunc()
				return
			}
		}

	}
}

// Continuously sends routing updates to let the other end know who we are on initial connection
func (s *Netceptor) sendInitialConnectMessage(ci *connInfo, initDoneChan chan bool) {
	count := 0
	for {
		ri, err := s.translateStructToNetwork(MsgTypeRoute, s.makeRoutingUpdate())
		if err != nil {
			logger.Error("Error Sending initial connection message: %s\n", err)
			return
		}
		logger.Debug("Sending initial connection message\n")
		ci.WriteChan <- ri
		count++
		if count > 10 {
			logger.Warning("Giving up on connection initialization\n")
			ci.CancelFunc()
			return
		}
		select {
		case <-s.context.Done():
			return
		case <-time.After(1 * time.Second):
			continue
		case <-initDoneChan:
			logger.Debug("Stopping initial updates\n")
			return
		}
	}
}

func (s *Netceptor) sendRejectMessage(writeChan chan []byte) {
	rejMsg, err := s.translateStructToNetwork(MsgTypeReject, make([]string, 0))
	if err != nil {
		writeChan <- rejMsg
	}
}

func (s *Netceptor) sendAndLogConnectionRejection(remoteNodeID string, ci *connInfo, reason string) error {
	s.sendRejectMessage(ci.WriteChan)
	return fmt.Errorf("rejected connection with node %s because %s", remoteNodeID, reason)
}

// Main Netceptor protocol loop
func (s *Netceptor) runProtocol(sess BackendSession, connectionCost float64, nodeCost map[string]float64) error {
	if connectionCost <= 0.0 {
		return fmt.Errorf("connection cost must be positive")
	}
	established := false
	remoteEstablished := false
	remoteNodeID := ""
	defer func() {
		_ = sess.Close()
		if established {
			s.connLock.Lock()
			delete(s.connections, remoteNodeID)
			s.connLock.Unlock()
			s.knownNodeLock.Lock()
			delete(s.knownConnectionCosts[remoteNodeID], s.nodeID)
			delete(s.knownConnectionCosts[s.nodeID], remoteNodeID)
			s.knownNodeLock.Unlock()
			done := false
			select {
			case <-s.context.Done():
				done = true
			default:
			}
			if !done {
				s.updateRoutingTableChan <- 0
				s.sendRouteFloodChan <- 0
			}
		}
	}()
	ci := &connInfo{
		ReadChan:  make(chan []byte),
		WriteChan: make(chan []byte),
		Cost:      connectionCost,
	}
	ci.Context, ci.CancelFunc = context.WithCancel(s.context)
	go ci.protoReader(sess)
	go ci.protoWriter(sess)
	initDoneChan := make(chan bool)
	go s.sendInitialConnectMessage(ci, initDoneChan)
	for {
		select {
		case data := <-ci.ReadChan:
			msgType := data[0]
			if established {
				if msgType == MsgTypeData {
					message, err := s.translateDataToMessage(data)
					if err != nil {
						logger.Error("Error translating data to message struct: %s\n", err)
						continue
					}
					logger.Trace("--- Received data length %d from %s:%s to %s:%s via %s\n", len(message.Data),
						message.FromNode, message.FromService, message.ToNode, message.ToService, remoteNodeID)
					err = s.handleMessageData(message)
					if err != nil {
						logger.Error("Error handling message data: %s\n", err)
					}
				} else if msgType == MsgTypeRoute {
					ri := &routingUpdate{}
					err := json.Unmarshal(data[1:], ri)
					if err != nil {
						logger.Error("Error unpacking routing update: %s\n", err)
						continue
					}
					if ri.ForwardingNode != remoteNodeID {
						return s.sendAndLogConnectionRejection(remoteNodeID, ci,
							fmt.Sprintf("remote node ID changed unexpectedly from %s to %s",
								remoteNodeID, ri.NodeID))
					}
					if ri.NodeID == remoteNodeID {
						// This is an update from our direct connection, so do some extra verification
						remoteCost, ok := ri.Connections[s.nodeID]
						if !ok {
							if remoteEstablished {
								return s.sendAndLogConnectionRejection(remoteNodeID, ci, "remote node no longer lists us as a connection")
							}
							// This is a late initialization request from the remote node, so don't process it as a routing update.
							continue
						} else {
							remoteEstablished = true
						}
						if ok && remoteCost != connectionCost {
							return s.sendAndLogConnectionRejection(remoteNodeID, ci, "we disagree about the connection cost")
						}
					}
					s.handleRoutingUpdate(ri, remoteNodeID)
				} else if msgType == MsgTypeServiceAdvertisement {
					err := s.handleServiceAdvertisement(data, remoteNodeID)
					if err != nil {
						logger.Error("Error handling service advertisement: %s\n", err)
						continue
					}
				} else if msgType == MsgTypeReject {
					logger.Warning("Received a rejection message from peer.")
					return fmt.Errorf("remote node rejected the connection")
				} else {
					logger.Warning("Unknown message type %d\n", msgType)
				}
			} else {
				// Connection not established
				if msgType == MsgTypeRoute {
					ri := &routingUpdate{}
					err := json.Unmarshal(data[1:], ri)
					if err != nil {
						logger.Error("Error unpacking routing update: %s\n", err)
						continue
					}
					remoteNodeID = ri.ForwardingNode
					// Decide whether the remote node is acceptable
					remoteNodeAccepted := true
					if s.allowedPeers != nil {
						remoteNodeAccepted = false
						for i := range s.allowedPeers {
							if s.allowedPeers[i] == remoteNodeID {
								remoteNodeAccepted = true
								break
							}
						}
					}
					if !remoteNodeAccepted {
						return s.sendAndLogConnectionRejection(remoteNodeID, ci, "it is not in the accepted connections list")
					}

					remoteNodeCost, ok := nodeCost[remoteNodeID]
					if ok {
						ci.Cost = remoteNodeCost
						connectionCost = remoteNodeCost
					}

					// Establish the connection
					select {
					case initDoneChan <- true:
					case <-s.context.Done():
						return nil
					}
					logger.Info("Connection established with %s\n", remoteNodeID)
					s.addNameHash(remoteNodeID)
					s.connLock.Lock()
					s.connections[remoteNodeID] = ci
					s.connLock.Unlock()
					s.knownNodeLock.Lock()
					_, ok = s.knownConnectionCosts[s.nodeID]
					if !ok {
						s.knownConnectionCosts[s.nodeID] = make(map[string]float64)
					}
					s.knownConnectionCosts[s.nodeID][remoteNodeID] = connectionCost
					_, ok = s.knownConnectionCosts[remoteNodeID]
					if !ok {
						s.knownConnectionCosts[remoteNodeID] = make(map[string]float64)
					}
					s.knownConnectionCosts[remoteNodeID][s.nodeID] = connectionCost
					s.knownNodeLock.Unlock()
					select {
					case s.sendRouteFloodChan <- 0:
					case <-s.context.Done():
						return nil
					}
					select {
					case s.updateRoutingTableChan <- 0:
					case <-s.context.Done():
						return nil
					}
					established = true
				} else if msgType == MsgTypeReject {
					logger.Warning("Received a rejection message from peer.")
					return fmt.Errorf("remote node rejected the connection")
				}
			}
		case <-ci.Context.Done():
			return nil
		}
	}
}
