// Package netceptor is the networking layer of Receptor.
package netceptor

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/randstr"
	"github.com/ansible/receptor/pkg/tickrunner"
	"github.com/ansible/receptor/pkg/utils"
	priorityQueue "github.com/jupp0r/go-priority-queue"
	"github.com/minio/highwayhash"
)

// defaultMTU is the largest message sendable over the Netceptor network.
const defaultMTU = 16384

// defaultRouteUpdateTime is the interval at which regular route updates will be sent.
const defaultRouteUpdateTime = 10 * time.Second

// defaultServiceAdTime is the interval at which regular service advertisements will be sent.
const defaultServiceAdTime = 60 * time.Second

// defaultSeenUpdateExpireTime is the age after which routing update IDs can be discarded.
const defaultSeenUpdateExpireTime = 1 * time.Hour

// defaultMaxForwardingHops is the maximum number of times that Netceptor will forward a data packet.
const defaultMaxForwardingHops = 30

// defaultMaxConnectionIdleTime is the maximum time a connection can go without data before we consider it failed.
const defaultMaxConnectionIdleTime = 2*defaultRouteUpdateTime + 1*time.Second

// MainInstance is the global instance of Netceptor instantiated by the command-line main() function.
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

// Backend is the interface for back-ends that the Receptor network can run over.
type Backend interface {
	Start(context.Context, *sync.WaitGroup) (chan BackendSession, error)
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

// FirewallRuleFunc is a function that takes a message and returns a firewall decision.
type FirewallRuleFunc func(*MessageData) FirewallResult

// FirewallResult enumerates the actions that can be taken as a result of a firewall rule.
type FirewallResult int

const (
	// FirewallResultContinue continues processing further rules (no result).
	FirewallResultContinue FirewallResult = iota
	// FirewallResultAccept accepts the message for normal processing.
	FirewallResultAccept
	// FirewallResultReject denies the message, sending an unreachable message to the originator.
	FirewallResultReject
	// FirewallResultDrop denies the message silently, leaving the originator to time out.
	FirewallResultDrop
)

// Netceptor is the main object of the Receptor mesh network protocol.
type Netceptor struct {
	nodeID                 string
	mtu                    int
	routeUpdateTime        time.Duration
	serviceAdTime          time.Duration
	seenUpdateExpireTime   time.Duration
	maxForwardingHops      byte
	maxConnectionIdleTime  time.Duration
	workCommands           []WorkCommand
	epoch                  uint64
	sequence               uint64
	connLock               *sync.RWMutex
	connections            map[string]*connInfo
	knownNodeLock          *sync.RWMutex
	knownNodeInfo          map[string]*nodeInfo
	seenUpdatesLock        *sync.RWMutex
	seenUpdates            map[string]time.Time
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
	reservedServices       map[string]func(*MessageData) error
	serviceAdsLock         *sync.RWMutex
	serviceAdsReceived     map[string]map[string]*ServiceAdvertisement
	sendServiceAdsChan     chan time.Duration
	backendWaitGroup       sync.WaitGroup
	backendCount           int
	backendCancel          []context.CancelFunc
	networkName            string
	serverTLSConfigs       map[string]*tls.Config
	clientTLSConfigs       map[string]*tls.Config
	unreachableBroker      *utils.Broker
	routingUpdateBroker    *utils.Broker
	firewallLock           *sync.RWMutex
	firewallRules          []FirewallRuleFunc
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
	// MsgTypeData is a normal data-containing message.
	MsgTypeData = 0
	// MsgTypeRoute is a routing update.
	MsgTypeRoute = 1
	// MsgTypeServiceAdvertisement is an advertisement for a service.
	MsgTypeServiceAdvertisement = 2
	// MsgTypeReject indicates a rejection (closure) of a backend connection.
	MsgTypeReject = 3
)

const (
	// ProblemServiceUnknown occurs when a message arrives for a non-listening service.
	ProblemServiceUnknown = "service unknown"
	// ProblemExpiredInTransit occurs when a message's HopsToLive expires in transit.
	ProblemExpiredInTransit = "message expired"
	// ProblemRejected occurs when a packet is rejected by a firewall rule.
	ProblemRejected = "blocked by firewall"
)

// MessageData contains a single message packet from the network.
type MessageData struct {
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
	NodeID             string
	UpdateID           string
	UpdateEpoch        uint64
	UpdateSequence     uint64
	Connections        map[string]float64
	ForwardingNode     string
	SuspectedDuplicate uint64
}

const (
	// ConnTypeDatagram indicates a packetconn (datagram) service listener.
	ConnTypeDatagram = 0
	// ConnTypeStream indicates a conn (stream) service listener, without a user-defined TLS.
	ConnTypeStream = 1
	// ConnTypeStreamTLS indicates the service listens on a packetconn connection, with a user-defined TLS.
	ConnTypeStreamTLS = 2
)

// WorkCommand tracks available work types and whether they verify work submissions.
type WorkCommand struct {
	WorkType string
	// Secure true means receptor will verify the signature of the work submit payload
	Secure bool
}

// ServiceAdvertisement is the data associated with a service advertisement.
type ServiceAdvertisement struct {
	NodeID       string
	Service      string
	Time         time.Time
	ConnType     byte
	Tags         map[string]string
	WorkCommands []WorkCommand
}

// serviceAdvertisementFull is the whole message from the network.
type serviceAdvertisementFull struct {
	*ServiceAdvertisement
	Cancel bool
}

// UnreachableMessage is the on-the-wire data associated with an unreachable message.
type UnreachableMessage struct {
	FromNode    string
	ToNode      string
	FromService string
	ToService   string
	Problem     string
}

// UnreachableNotification includes additional information returned from SubscribeUnreachable.
type UnreachableNotification struct {
	UnreachableMessage
	ReceivedFromNode string
}

var (
	networkNames     = make([]string, 0)
	networkNamesLock = sync.Mutex{}
)

// makeNetworkName returns a network name that is unique within global scope.
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

// NewWithConsts constructs a new Receptor network protocol instance, specifying operational constants.
func NewWithConsts(ctx context.Context, nodeID string,
	mtu int, routeUpdateTime time.Duration, serviceAdTime time.Duration, seenUpdateExpireTime time.Duration,
	maxForwardingHops byte, maxConnectionIdleTime time.Duration) *Netceptor {
	s := Netceptor{
		nodeID:                 nodeID,
		mtu:                    mtu,
		routeUpdateTime:        routeUpdateTime,
		serviceAdTime:          serviceAdTime,
		seenUpdateExpireTime:   seenUpdateExpireTime,
		maxForwardingHops:      maxForwardingHops,
		maxConnectionIdleTime:  maxConnectionIdleTime,
		epoch:                  uint64(time.Now().Unix()*(1<<24)) + uint64(rand.Intn(1<<24)),
		sequence:               0,
		connLock:               &sync.RWMutex{},
		connections:            make(map[string]*connInfo),
		knownNodeLock:          &sync.RWMutex{},
		knownNodeInfo:          make(map[string]*nodeInfo),
		seenUpdatesLock:        &sync.RWMutex{},
		seenUpdates:            make(map[string]time.Time),
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
		backendCancel:          nil,
		networkName:            makeNetworkName(nodeID),
		clientTLSConfigs:       make(map[string]*tls.Config),
		serverTLSConfigs:       make(map[string]*tls.Config),
		firewallLock:           &sync.RWMutex{},
	}
	s.reservedServices = map[string]func(*MessageData) error{
		"ping":    s.handlePing,
		"unreach": s.handleUnreachable,
	}
	if ctx == nil {
		ctx = context.Background()
	}
	s.clientTLSConfigs["default"] = &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	s.addNameHash(nodeID)
	s.context, s.cancelFunc = context.WithCancel(ctx)
	s.unreachableBroker = utils.NewBroker(s.context, reflect.TypeOf(UnreachableNotification{}))
	s.routingUpdateBroker = utils.NewBroker(s.context, reflect.TypeOf(map[string]string{}))
	s.updateRoutingTableChan = tickrunner.Run(s.context, s.updateRoutingTable, time.Hour*24, time.Millisecond*100)
	s.sendRouteFloodChan = tickrunner.Run(s.context, func() { s.sendRoutingUpdate(0) }, s.routeUpdateTime, time.Millisecond*100)
	if s.serviceAdTime > 0 {
		s.sendServiceAdsChan = tickrunner.Run(s.context, s.sendServiceAds, s.serviceAdTime, time.Second*5)
	} else {
		s.sendServiceAdsChan = make(chan time.Duration)
		go func() {
			for {
				select {
				case <-s.sendServiceAdsChan:
					// do nothing
				case <-s.context.Done():
					return
				}
			}
		}()
	}
	go s.monitorConnectionAging()
	go s.expireSeenUpdates()

	return &s
}

// New constructs a new Receptor network protocol instance.
func New(ctx context.Context, nodeID string) *Netceptor {
	return NewWithConsts(ctx, nodeID, defaultMTU, defaultRouteUpdateTime, defaultServiceAdTime,
		defaultSeenUpdateExpireTime, defaultMaxForwardingHops, defaultMaxConnectionIdleTime)
}

// NewAddr generates a Receptor network address from a node ID and service name.
func (s *Netceptor) NewAddr(node string, service string) Addr {
	return Addr{
		network: s.networkName,
		node:    node,
		service: service,
	}
}

// Context returns the context for this Netceptor instance.
func (s *Netceptor) Context() context.Context {
	return s.context
}

// Shutdown shuts down a Netceptor instance.
func (s *Netceptor) Shutdown() {
	s.cancelFunc()
}

// NetceptorDone returns the channel for the netceptor context.
func (s *Netceptor) NetceptorDone() <-chan struct{} {
	return s.context.Done()
}

// NodeID returns the local Node ID of this Netceptor instance.
func (s *Netceptor) NodeID() string {
	return s.nodeID
}

// MTU returns the configured MTU of this Netceptor instance.
func (s *Netceptor) MTU() int {
	return s.mtu
}

// RouteUpdateTime returns the configured RouteUpdateTime of this Netceptor instance.
func (s *Netceptor) RouteUpdateTime() time.Duration {
	return s.routeUpdateTime
}

// ServiceAdTime returns the configured ServiceAdTime of this Netceptor instance.
func (s *Netceptor) ServiceAdTime() time.Duration {
	return s.serviceAdTime
}

// SeenUpdateExpireTime returns the configured SeenUpdateExpireTime of this Netceptor instance.
func (s *Netceptor) SeenUpdateExpireTime() time.Duration {
	return s.seenUpdateExpireTime
}

// MaxForwardingHops returns the configured MaxForwardingHops of this Netceptor instance.
func (s *Netceptor) MaxForwardingHops() byte {
	return s.maxForwardingHops
}

// MaxConnectionIdleTime returns the configured MaxConnectionIdleTime of this Netceptor instance.
func (s *Netceptor) MaxConnectionIdleTime() time.Duration {
	return s.maxConnectionIdleTime
}

type backendInfo struct {
	connectionCost float64
	nodeCost       map[string]float64
	allowedPeers   []string
}

// BackendConnectionCost is a modifier for AddBackend, which sets the global connection cost.
func BackendConnectionCost(cost float64) func(*backendInfo) {
	return func(bi *backendInfo) {
		bi.connectionCost = cost
	}
}

// BackendNodeCost is a modifier for AddBackend, which sets the per-node connection costs.
func BackendNodeCost(nodeCost map[string]float64) func(*backendInfo) {
	return func(bi *backendInfo) {
		bi.nodeCost = nodeCost
	}
}

// BackendAllowedPeers is a modifier for AddBackend, which sets the list of peers allowed to connect.
func BackendAllowedPeers(peers []string) func(*backendInfo) {
	return func(bi *backendInfo) {
		bi.allowedPeers = peers
	}
}

// AddBackend adds a backend to the Netceptor system.
func (s *Netceptor) AddBackend(backend Backend, modifiers ...func(*backendInfo)) error {
	bi := &backendInfo{
		connectionCost: 1.0,
		nodeCost:       nil,
		allowedPeers:   nil,
	}
	for _, mod := range modifiers {
		mod(bi)
	}
	ctxBackend, cancel := context.WithCancel(s.context)
	s.backendCancel = append(s.backendCancel, cancel)
	// Start() runs a go routine that attempts establish a session over this
	// backend. For listeners, each time a peer dials this backend, sessChan is
	// written to, resulting in multiple ongoing sessions at once.
	sessChan, err := backend.Start(ctxBackend, &s.backendWaitGroup)
	if err != nil {
		return err
	}
	s.backendWaitGroup.Add(1)
	s.backendCount++
	// Outer go routine -- this go routine waits for new sessions to be written to the sessChan and
	// starts the runProtocol() for that session
	go func() {
		runProtocolWg := sync.WaitGroup{}
		defer func() {
			// First wait for all session protocols to finish (the inner go routines)
			// for this backend before exiting this outer go routine.
			// It is important that the inner go routine is on a separate wait group
			// from the outer go routine.
			runProtocolWg.Wait()
			s.backendWaitGroup.Done()
		}()
		for {
			select {
			case sess, ok := <-sessChan:
				if ok {
					runProtocolWg.Add(1)
					// Inner go routine -- start the runProtocol loop for the new session
					// that was just passed to sessChan (which was written to from the
					// Start() method above)
					go func() {
						defer runProtocolWg.Done()
						err := s.runProtocol(ctxBackend, sess, bi)
						if err != nil {
							logger.Error("Backend error: %s\n", err)
						}
					}()
				} else {
					return
				}
			case <-ctxBackend.Done():
				return
			}
		}
	}()

	return nil
}

// BackendWait waits for the backend wait group.
func (s *Netceptor) BackendWait() {
	s.backendWaitGroup.Wait()
}

// BackendDone calls Done on the backendWaitGroup.
func (s *Netceptor) BackendDone() {
	s.backendWaitGroup.Done()
}

// BackendCount returns the number of backends that ever registered with this Netceptor.
func (s *Netceptor) BackendCount() int {
	return s.backendCount
}

// CancelBackends stops all backends by calling a context cancel.
func (s *Netceptor) CancelBackends() {
	logger.Debug("Canceling backends")
	for i := range s.backendCancel {
		// a context cancel function
		s.backendCancel[i]()
	}
	s.BackendWait()
	s.backendCancel = nil
	s.backendCount = 0
}

// Status returns the current state of the Netceptor object.
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
				if len(s.workCommands) > 0 {
					adCopy.WorkCommands = s.workCommands
				}
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

// AddFirewallRules adds firewall rules, optionally clearing existing rules first.
func (s *Netceptor) AddFirewallRules(rules []FirewallRuleFunc, clearExisting bool) error {
	s.firewallLock.Lock()
	defer s.firewallLock.Unlock()
	if clearExisting {
		s.firewallRules = nil
	}
	s.firewallRules = append(s.firewallRules, rules...)

	return nil
}

func (s *Netceptor) addLocalServiceAdvertisement(service string, connType byte, tags map[string]string) {
	s.serviceAdsLock.Lock()
	defer s.serviceAdsLock.Unlock()
	n, ok := s.serviceAdsReceived[s.nodeID]
	if !ok {
		n = make(map[string]*ServiceAdvertisement)
		s.serviceAdsReceived[s.nodeID] = n
	}
	n[service] = &ServiceAdvertisement{
		NodeID:   s.nodeID,
		Service:  service,
		Time:     time.Now(),
		ConnType: connType,
		Tags:     tags,
	}
	s.sendServiceAdsChan <- 0
}

func (s *Netceptor) removeLocalServiceAdvertisement(service string) error {
	s.serviceAdsLock.Lock()
	defer s.serviceAdsLock.Unlock()
	n, ok := s.serviceAdsReceived[s.nodeID]
	connType := n[service].ConnType
	if ok {
		delete(n, service)
	}
	sa := &serviceAdvertisementFull{
		ServiceAdvertisement: &ServiceAdvertisement{
			NodeID:   s.nodeID,
			Service:  service,
			Time:     time.Now(),
			ConnType: connType,
			Tags:     nil,
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

// Send a single service broadcast.
func (s *Netceptor) sendServiceAd(si *ServiceAdvertisement) error {
	logger.Debug("Sending service advertisement: %v\n", si)
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

// Send advertisements for all advertised services.
func (s *Netceptor) sendServiceAds() {
	ads := make([]ServiceAdvertisement, 0)
	s.listenerLock.RLock()
	for sn := range s.listenerRegistry {
		if s.listenerRegistry[sn].advertise {
			sa := ServiceAdvertisement{
				NodeID:   s.nodeID,
				Service:  sn,
				Time:     time.Now(),
				ConnType: s.listenerRegistry[sn].connType,
				Tags:     s.listenerRegistry[sn].adTags,
			}
			if svcType, ok := sa.Tags["type"]; ok {
				if svcType == "Control Service" {
					if len(s.workCommands) > 0 {
						sa.WorkCommands = s.workCommands
					}
				}
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

// Watches connections and expires any that haven't seen traffic in too long.
func (s *Netceptor) monitorConnectionAging() {
	for {
		select {
		case <-time.After(5 * time.Second):
			timedOut := make([]context.CancelFunc, 0)
			s.connLock.RLock()
			for i := range s.connections {
				if time.Since(s.connections[i].lastReceivedData) > s.maxConnectionIdleTime {
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

// Expires old updates from the seenUpdates table.
func (s *Netceptor) expireSeenUpdates() {
	for {
		select {
		case <-time.After(s.seenUpdateExpireTime / 2):
			thresholdTime := time.Now().Add(-s.seenUpdateExpireTime)
			s.seenUpdatesLock.Lock()
			for id := range s.seenUpdates {
				if s.seenUpdates[id].Before(thresholdTime) {
					delete(s.seenUpdates, id)
				}
			}
			s.seenUpdatesLock.Unlock()
		case <-s.context.Done():
			return
		}
	}
}

// Re-calculates the next-hop table based on current knowledge of the network.
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

// SubscribeRoutingUpdates subscribes for messages when the routing table is changed.
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

// Forwards a message to all neighbors, possibly excluding one.
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

// GetServerTLSConfig retrieves a server TLS config by name.
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

// AddWorkCommand records a work command so it can be included in service announcements.
func (s *Netceptor) AddWorkCommand(command string, secure bool) error {
	if command == "" {
		return fmt.Errorf("must provide a name")
	}
	wC := WorkCommand{WorkType: command, Secure: secure}
	s.workCommands = append(s.workCommands, wC)

	return nil
}

// SetServerTLSConfig stores a server TLS config by name.
func (s *Netceptor) SetServerTLSConfig(name string, config *tls.Config) error {
	if name == "" {
		return fmt.Errorf("must provide a name")
	}
	s.serverTLSConfigs[name] = config

	return nil
}

// ReceptorCertNameError is the error produced when Receptor certificate name verification fails.
type ReceptorCertNameError struct {
	ValidNodes   []string
	ExpectedNode string
}

func (rce ReceptorCertNameError) Error() string {
	if len(rce.ValidNodes) == 0 {
		return fmt.Sprintf("x509: certificate is not valid for any Receptor node IDs, but wanted to match %s",
			rce.ExpectedNode)
	}
	var plural string
	if len(rce.ValidNodes) > 1 {
		plural = "s"
	}

	return fmt.Sprintf("x509: certificate is valid for Receptor node ID%s %s, not %s",
		plural, strings.Join(rce.ValidNodes, ", "), rce.ExpectedNode)
}

const (
	// VerifyServer indicates we are the client, verifying a server.
	VerifyServer = 1
	// VerifyClient indicates we are the server, verifying a client.
	VerifyClient = 2
)

// receptorVerifyFunc generates a function that verifies a Receptor node ID.
func (s *Netceptor) receptorVerifyFunc(tlscfg *tls.Config, expectedNodeID string,
	verifyType int) func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	oldVerifyFunc := tlscfg.VerifyPeerCertificate

	return func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		if oldVerifyFunc != nil {
			err := oldVerifyFunc(rawCerts, verifiedChains)
			if err != nil {
				return err
			}
		}
		certs := make([]*x509.Certificate, len(rawCerts))
		for i, asn1Data := range rawCerts {
			cert, err := x509.ParseCertificate(asn1Data)
			if err != nil {
				logger.Error("RVF failed to parse: %s", err)

				return fmt.Errorf("failed to parse certificate from server: " + err.Error())
			}
			certs[i] = cert
		}
		var opts x509.VerifyOptions
		switch verifyType {
		case VerifyServer:
			opts = x509.VerifyOptions{
				Intermediates: x509.NewCertPool(),
				Roots:         tlscfg.RootCAs,
				CurrentTime:   time.Now(),
				KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			}
		case VerifyClient:
			opts = x509.VerifyOptions{
				Intermediates: x509.NewCertPool(),
				Roots:         tlscfg.ClientCAs,
				CurrentTime:   time.Now(),
				KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			}
		default:
			return fmt.Errorf("invalid verification type: must be client or server")
		}

		for _, cert := range certs[1:] {
			opts.Intermediates.AddCert(cert)
		}
		var err error
		_, err = certs[0].Verify(opts)
		if err != nil {
			logger.Error("RVF failed verify: %s\nRootCAs: %v\nServerName: %s", err, tlscfg.RootCAs, tlscfg.ServerName)

			return err
		}
		var receptorNames []string
		receptorNames, err = utils.ReceptorNames(certs[0].Extensions)
		if err != nil {
			logger.Error("RVF failed to get ReceptorNames: %s", err)

			return err
		}
		found := false
		for _, receptorName := range receptorNames {
			if receptorName == expectedNodeID {
				found = true

				break
			}
		}
		if !found {
			logger.Error("RVF ReceptorNameError: %s", err)

			return ReceptorCertNameError{ValidNodes: receptorNames, ExpectedNode: expectedNodeID}
		}

		return nil
	}
}

// GetClientTLSConfig retrieves a client TLS config by name.  Supported host name types
// are dns and receptor.
func (s *Netceptor) GetClientTLSConfig(name string, expectedHostName string, expectedHostNameType string) (*tls.Config, error) {
	if name == "" {
		return nil, nil
	}
	tlscfg, ok := s.clientTLSConfigs[name]
	if !ok {
		return nil, fmt.Errorf("unknown TLS config %s", name)
	}
	tlscfg = tlscfg.Clone()
	switch {
	case tlscfg.InsecureSkipVerify:
		// noop
	case expectedHostNameType == "receptor":
		tlscfg.InsecureSkipVerify = true
		tlscfg.VerifyPeerCertificate = s.receptorVerifyFunc(tlscfg, expectedHostName, VerifyServer)
	default:
		tlscfg.ServerName = expectedHostName
	}

	return tlscfg, nil
}

// SetClientTLSConfig stores a client TLS config by name.
func (s *Netceptor) SetClientTLSConfig(name string, config *tls.Config) error {
	if name == "" {
		return fmt.Errorf("must provide a name")
	}
	s.clientTLSConfigs[name] = config

	return nil
}

// All-zero seed for deterministic highwayhash.
var zerokey = make([]byte, 32)

// Hash a name and add it to the lookup table.
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

// Looks up a name given a hash received from the network.
func (s *Netceptor) getNameFromHash(namehash uint64) (string, error) {
	s.hashLock.RLock()
	defer s.hashLock.RUnlock()
	name, ok := s.nameHashes[namehash]
	if !ok {
		return "", fmt.Errorf("hash not found")
	}

	return name, nil
}

// Given a string, returns a fixed-length buffer right-padded with null (0) bytes.
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

// Given a fixed-length buffer, returns a string excluding any null (0) bytes on the right.
func fixedLenBytesFromString(s string, l int) []byte {
	bytes := make([]byte, l)
	copy(bytes, s)

	return bytes
}

// Translates an incoming message from wire protocol to MessageData object.
func (s *Netceptor) translateDataToMessage(data []byte) (*MessageData, error) {
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
	md := &MessageData{
		FromNode:    fromNode,
		FromService: fromService,
		ToNode:      toNode,
		ToService:   toService,
		HopsToLive:  data[1],
		Data:        data[36:],
	}

	return md, nil
}

// Translates an outgoing message from a MessageData object to wire protocol.
func (s *Netceptor) translateDataFromMessage(msg *MessageData) ([]byte, error) {
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

// Forwards a message to its next hop.
func (s *Netceptor) forwardMessage(md *MessageData) error {
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

// Generates and sends a message over the Receptor network, specifying HopsToLive.
func (s *Netceptor) sendMessageWithHopsToLive(fromService string, toNode string, toService string, data []byte, hopsToLive byte) error {
	if len(fromService) > 8 || len(toService) > 8 {
		return fmt.Errorf("service name too long")
	}
	if strings.EqualFold(toNode, "localhost") {
		toNode = s.nodeID
	}
	md := &MessageData{
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

// Generates and sends a message over the Receptor network.
func (s *Netceptor) sendMessage(fromService string, toNode string, toService string, data []byte) error {
	return s.sendMessageWithHopsToLive(fromService, toNode, toService, data, s.maxForwardingHops)
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

// Constructs a routing update message.
func (s *Netceptor) makeRoutingUpdate(suspectedDuplicate uint64) *routingUpdate {
	s.sequence++
	s.connLock.RLock()
	conns := make(map[string]float64)
	for conn := range s.connections {
		conns[conn] = s.connections[conn].Cost
	}
	s.connLock.RUnlock()
	update := &routingUpdate{
		NodeID:             s.nodeID,
		UpdateID:           randstr.RandomString(8),
		UpdateEpoch:        s.epoch,
		UpdateSequence:     s.sequence,
		Connections:        conns,
		ForwardingNode:     s.nodeID,
		SuspectedDuplicate: suspectedDuplicate,
	}

	return update
}

// Translates an arbitrary struct to a network message.
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
func (s *Netceptor) sendRoutingUpdate(suspectedDuplicate uint64) {
	if len(s.connections) == 0 {
		return
	}
	ru := s.makeRoutingUpdate(suspectedDuplicate)
	sb := make([]string, 0)
	for conn := range ru.Connections {
		sb = append(sb, fmt.Sprintf("%s(%.2f)", conn, ru.Connections[conn]))
	}
	if suspectedDuplicate == 0 {
		logger.Debug("Sending routing update %s. Connections: %s\n", ru.UpdateID, strings.Join(sb, " "))
	} else {
		logger.Warning("Sending duplicate node notification %s. Connections: %s\n", ru.UpdateID, strings.Join(sb, " "))
	}
	message, err := s.translateStructToNetwork(MsgTypeRoute, ru)
	if err != nil {
		return
	}
	s.flood(message, "")
}

// Processes a routing update received from a connection.
func (s *Netceptor) handleRoutingUpdate(ri *routingUpdate, recvConn string) {
	if ri.NodeID == "" {
		// Our peer is still trying to initialize
		return
	}
	if ri.NodeID == s.nodeID {
		if ri.UpdateEpoch == s.epoch {
			return
		}
		if ri.SuspectedDuplicate == s.epoch {
			// We are the duplicate!
			logger.Error("We are a duplicate node with ID %s and epoch %d.  Shutting down.\n", s.nodeID, s.epoch)
			s.Shutdown()

			return
		}
		if ri.UpdateEpoch > s.epoch {
			// Update has our node ID but a newer epoch - so if clocks are in sync they are a duplicate
			logger.Error("Duplicate node ID %s detected via %s\n", ri.NodeID, recvConn)
			// Send routing update noting our suspicion
			s.sendRoutingUpdate(ri.UpdateEpoch)

			return
		}

		return
	}
	s.seenUpdatesLock.Lock()
	_, ok := s.seenUpdates[ri.UpdateID]
	if ok {
		s.seenUpdatesLock.Unlock()

		return
	}
	s.seenUpdates[ri.UpdateID] = time.Now()
	s.seenUpdatesLock.Unlock()
	if ri.SuspectedDuplicate != 0 {
		logger.Warning("Node %s with epoch %d sent update %s suspecting a duplicate node with epoch %d\n", ri.NodeID, ri.UpdateEpoch, ri.UpdateID, ri.SuspectedDuplicate)
		s.knownNodeLock.Lock()
		ni, ok := s.knownNodeInfo[ri.NodeID]
		if ok {
			if ni.Epoch == ri.SuspectedDuplicate {
				s.knownNodeInfo[ri.NodeID].Epoch = ri.UpdateEpoch
				s.knownNodeInfo[ri.NodeID].Sequence = ri.UpdateSequence
			}
		}
		s.knownNodeLock.Unlock()
	} else {
		logger.Debug("Received routing update %s from %s via %s\n", ri.UpdateID, ri.NodeID, recvConn)
		s.knownNodeLock.Lock()
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
		if changed {
			s.updateRoutingTableChan <- 100 * time.Millisecond
		}
	}
	ri.ForwardingNode = s.nodeID
	message, err := s.translateStructToNetwork(MsgTypeRoute, ri)
	if err != nil {
		return
	}
	s.flood(message, recvConn)
}

// Handles a ping request.
func (s *Netceptor) handlePing(md *MessageData) error {
	return s.sendMessage("ping", md.FromNode, md.FromService, []byte{})
}

// Handles an unreachable response.
func (s *Netceptor) handleUnreachable(md *MessageData) error {
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

// Sends an unreachable response.
func (s *Netceptor) sendUnreachable(toNode string, message *UnreachableMessage) error {
	bytes, err := json.Marshal(message)
	if err != nil {
		return err
	}
	err = s.sendMessage("unreach", toNode, "unreach", bytes)
	if err != nil {
		return err
	}

	return nil
}

// Dispatches a message to a reserved service.  Returns true if handled, false otherwise.
func (s *Netceptor) dispatchReservedService(md *MessageData) (bool, error) {
	svc, ok := s.reservedServices[md.ToService]
	if ok {
		return true, svc(md)
	}

	return false, nil
}

// Handles incoming data and dispatches it to a service listener.
func (s *Netceptor) handleMessageData(md *MessageData) error {
	// Check firewall rules for this packet
	s.firewallLock.RLock()
	result := FirewallResultAccept
	for _, rule := range s.firewallRules {
		result = rule(md)
		if result != FirewallResultContinue {
			break
		}
	}
	s.firewallLock.RUnlock()
	switch result {
	case FirewallResultAccept:
		// do nothing
	case FirewallResultDrop:
		return nil
	case FirewallResultReject:
		if md.FromService != "unreach" {
			_ = s.sendUnreachable(md.FromNode, &UnreachableMessage{
				FromNode:    md.FromNode,
				ToNode:      md.ToNode,
				FromService: md.FromService,
				ToService:   md.ToService,
				Problem:     ProblemRejected,
			})
		}

		return nil
	}

	// If the destination is local, then dispatch the message to a service
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

	// The destination is non-local, so forward the message.
	return s.forwardMessage(md)
}

// GetServiceInfo returns the advertising info, if any, for a service on a node.
func (s *Netceptor) GetServiceInfo(nodeID string, service string) (*ServiceAdvertisement, bool) {
	s.serviceAdsLock.RLock()
	defer s.serviceAdsLock.RUnlock()
	n, ok := s.serviceAdsReceived[nodeID]
	if !ok {
		return nil, false
	}
	svc, ok := n[service]
	if !ok {
		return nil, false
	}
	svcCopy := *svc

	return &svcCopy, true
}

// Handles an incoming service advertisement.
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

// Goroutine to send data from the backend to the connection's ReadChan.
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

// Goroutine to send data from the connection's WriteChan to the backend.
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

// Continuously sends routing updates to let the other end know who we are on initial connection.
func (s *Netceptor) sendInitialConnectMessage(ci *connInfo, initDoneChan chan bool) {
	count := 0
	for {
		ri, err := s.translateStructToNetwork(MsgTypeRoute, s.makeRoutingUpdate(0))
		if err != nil {
			logger.Error("Error Sending initial connection message: %s\n", err)

			return
		}
		logger.Debug("Sending initial connection message\n")
		select {
		case ci.WriteChan <- ri:
		case <-ci.Context.Done():
			return
		case <-initDoneChan:
			return
		}
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

func (s *Netceptor) sendRejectMessage(ci *connInfo) {
	rejMsg, err := s.translateStructToNetwork(MsgTypeReject, make([]string, 0))
	if err != nil {
		select {
		case <-ci.Context.Done():
		case ci.WriteChan <- rejMsg:
		}
	}
}

func (s *Netceptor) sendAndLogConnectionRejection(remoteNodeID string, ci *connInfo, reason string) error {
	s.sendRejectMessage(ci)

	return fmt.Errorf("rejected connection with node %s because %s", remoteNodeID, reason)
}

// Main Netceptor protocol loop.
func (s *Netceptor) runProtocol(ctx context.Context, sess BackendSession, bi *backendInfo) error {
	if bi.connectionCost <= 0.0 {
		return fmt.Errorf("connection cost must be positive")
	}
	established := false
	remoteEstablished := false
	remoteNodeID := ""
	connectionCost := bi.connectionCost
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
			case <-ctx.Done():
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
	ci.Context, ci.CancelFunc = context.WithCancel(ctx)
	go ci.protoReader(sess)
	go ci.protoWriter(sess)
	initDoneChan := make(chan bool)
	go s.sendInitialConnectMessage(ci, initDoneChan)
	for {
		select {
		case data := <-ci.ReadChan:
			msgType := data[0]
			if established {
				switch msgType {
				case MsgTypeData:
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
				case MsgTypeRoute:
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
				case MsgTypeServiceAdvertisement:
					err := s.handleServiceAdvertisement(data, remoteNodeID)
					if err != nil {
						logger.Error("Error handling service advertisement: %s\n", err)

						continue
					}
				case MsgTypeReject:
					logger.Warning("Received a rejection message from peer.")

					return fmt.Errorf("remote node rejected the connection")
				default:
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
					if remoteNodeID == s.nodeID {
						return s.sendAndLogConnectionRejection(remoteNodeID, ci, "it tried to connect using our own node ID")
					}
					remoteNodeAccepted := true
					s.connLock.RLock()
					for conn := range s.connections {
						if remoteNodeID == conn {
							remoteNodeAccepted = false

							break
						}
					}
					s.connLock.RUnlock()
					if !remoteNodeAccepted {
						return s.sendAndLogConnectionRejection(remoteNodeID, ci, "it connected using a node ID we are already connected to")
					}
					if bi.allowedPeers != nil {
						remoteNodeAccepted = false
						for i := range bi.allowedPeers {
							if bi.allowedPeers[i] == remoteNodeID {
								remoteNodeAccepted = true

								break
							}
						}
					}
					if !remoteNodeAccepted {
						return s.sendAndLogConnectionRejection(remoteNodeID, ci, "it is not in the allowed peers list")
					}

					remoteNodeCost, ok := bi.nodeCost[remoteNodeID]
					if ok {
						ci.Cost = remoteNodeCost
						connectionCost = remoteNodeCost
					}

					// Establish the connection
					select {
					case initDoneChan <- true:
					case <-ctx.Done():
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
					case <-ctx.Done():
						return nil
					}
					select {
					case s.updateRoutingTableChan <- 0:
					case <-ctx.Done():
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
