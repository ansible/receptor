package mesh

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/ansible/receptor/pkg/backends"
	"github.com/ansible/receptor/pkg/controlsvc"
	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/types"
	"github.com/ansible/receptor/pkg/workceptor"
	"github.com/ansible/receptor/tests/utils"
)

// LibMesh represents a single Receptor mesh network, used for test simulations.
type LibMesh struct {
	Name      string // Only used for generating test names
	nodes     map[string]*LibNode
	DataDir   string
	LogWriter *utils.TestLogWriter
	Context   context.Context
}

// NewLibMesh constructs a new LibMesh.
func NewLibMesh() LibMesh {
	baseDir := filepath.Join(os.TempDir(), "receptor-testing")
	os.Mkdir(baseDir, 0o700)

	err := os.MkdirAll(baseDir, 0o755)
	if err != nil {
		panic(err)
	}

	tempdir, err := os.MkdirTemp(baseDir, "mesh-")
	if err != nil {
		panic(err)
	}

	return LibMesh{
		nodes:     make(map[string]*LibNode),
		LogWriter: utils.NewTestLogWriter(),
		DataDir:   tempdir,
		Context:   context.Background(),
	}
}

// m.NewLibNode constructs a node with the name passed as the argument.
func (m *LibMesh) NewLibNode(nodeID string) *LibNode {
	node := LibNode{
		Config: types.NodeCfg{
			ID:      nodeID,
			DataDir: m.DataDir,
		},
		ControlSocket: filepath.Join(m.DataDir, nodeID+".sock"),
		backends:      make(map[string]BackendInfo),
	}

	m.nodes[nodeID] = &node

	return &node
}

func (m *LibMesh) Start(_ string) error {
	var err error

	// Bootstrap nodes
	for _, node := range m.GetNodes() {
		err = node.StartLocalServices()
		if err != nil {
			return err
		}

		// Comment out the line below to print test logs to stdout.
		// Note that some assertions will fail by doing this.
		node.netceptorInstance.Logger.SetOutput(m.LogWriter)
	}

	// Start listeners first, we connect below
	for _, node := range m.GetNodes() {
		err = node.StartListeners()
		if err != nil {
			return err
		}
	}

	// Establish outbound connections
	for _, node := range m.GetNodes() {
		err = node.EstablishRemoteConnections()
		if err != nil {
			return err
		}
	}

	return nil
}

// GetNodes returns a list of nodes.
func (m *LibMesh) GetNodes() map[string]*LibNode {
	return m.nodes
}

// GetDataDir returns the path to the data directory for this mesh.
func (m *LibMesh) GetDataDir() string {
	return m.DataDir
}

// Shutdown stops all running Netceptors and their backends.
func (m *LibMesh) Destroy() {
	for _, node := range m.GetNodes() {
		node.Destroy()
	}
}

// WaitForShutdown Waits for all running Netceptors and their backends to stop.
func (m LibMesh) WaitForShutdown() {
	for _, node := range m.GetNodes() {
		node.WaitForShutdown()
	}
}

// CheckConnections returns true if the connections defined in our mesh definition are
// consistent with the connections made by the nodes.
func (m LibMesh) CheckConnections() bool {
	statusList, err := m.Status()
	if err != nil {
		return false
	}

	expectedConnections := make(map[string]map[string]float64)
	actualConnections := make(map[string]map[string]float64)

	for nodeID := range m.GetNodes() {
		expectedConnections[nodeID] = map[string]float64{}
		actualConnections[nodeID] = map[string]float64{}
	}

	for nodeID, node := range m.GetNodes() {
		for _, connection := range node.Connections {
			backend := connection.RemoteNode.backends[connection.Protocol]
			cost := backend.connectionCost
			nodeCost, ok := backend.nodeCost[nodeID]
			if ok {
				cost = nodeCost
			}
			expectedConnections[nodeID][connection.RemoteNode.GetID()] = cost
			expectedConnections[connection.RemoteNode.GetID()][nodeID] = cost
		}
	}

	for _, nodeStatus := range statusList {
		for _, connection := range nodeStatus.Connections {
			actualConnections[nodeStatus.NodeID][connection.NodeID] = connection.Cost
		}
	}

	return reflect.DeepEqual(actualConnections, expectedConnections)
}

// CheckKnownConnectionCosts returns true if every node has the same view of the connections in the mesh.
func (m *LibMesh) CheckKnownConnectionCosts() bool {
	meshStatus, err := m.Status()
	if err != nil {
		return false
	}
	// If the mesh is empty we are done
	if len(meshStatus) == 0 {
		return true
	}

	knownConnectionCosts := meshStatus[0].KnownConnectionCosts
	for _, status := range meshStatus {
		if !reflect.DeepEqual(status.KnownConnectionCosts, knownConnectionCosts) {
			return false
		}
	}

	return true
}

// CheckRoutes returns true if every node has a route to every other node.
func (m *LibMesh) CheckRoutes() bool {
	meshStatus, err := m.Status()
	if err != nil {
		return false
	}
	for _, status := range meshStatus {
		// loop over m.MeshDefinition.Nodes instead... check for NodeConfig.ID, fall back to key
		for _, node := range m.GetNodes() {
			// Dont check a route to ourselves
			if status.NodeID == node.GetID() {
				continue
			}
			_, ok := status.RoutingTable[node.GetID()]
			if !ok {
				return false
			}
		}
	}

	return true
}

// CheckControlSockets Checks if the Control sockets in the mesh are all running and accepting
// connections.
func (m *LibMesh) CheckControlSockets() bool {
	for _, node := range m.GetNodes() {
		controller := NewReceptorControl()
		if controller.Connect(node.GetControlSocket()) != nil {
			node.netceptorInstance.Logger.Warning("%s: failed to connect to control socket", node.GetID())

			return false
		}
		controller.Close()
	}

	return true
}

// WaitForReady Waits for connections and routes to converge.
func (m *LibMesh) WaitForReady(ctx context.Context) error {
	sleepInterval := 1 * time.Second
	if !utils.CheckUntilTimeout(ctx, sleepInterval, m.CheckControlSockets) {
		return errors.New("timed out while waiting for control sockets")
	}
	if !utils.CheckUntilTimeout(ctx, sleepInterval, m.CheckConnections) {
		return errors.New("timed out while waiting for Connections")
	}
	if !utils.CheckUntilTimeout(ctx, sleepInterval, m.CheckKnownConnectionCosts) {
		return errors.New("timed out while checking Connection Costs")
	}
	if !utils.CheckUntilTimeout(ctx, sleepInterval, m.CheckRoutes) {
		return errors.New("timed out while waiting for routes to converge")
	}

	return nil
}

// Status returns a list of statuses from the contained netceptors.
func (m *LibMesh) Status() ([]*netceptor.Status, error) {
	out := []*netceptor.Status{}
	for _, node := range m.GetNodes() {
		status, err := node.Status()
		if err != nil {
			return nil, err
		}
		out = append(out, status)
	}

	return out, nil
}

// LibNode represents a node (it's configuration and running services).
type LibNode struct {
	Config                 types.NodeCfg
	Connections            []Connection
	ListenerCfgs           map[listenerName]ListenerCfg
	netceptorInstance      *netceptor.Netceptor
	workceptorInstance     *workceptor.Workceptor
	backends               map[string]BackendInfo
	controlServer          *controlsvc.Server
	ControlSocket          string
	controlServerCanceller context.CancelFunc
	controlerServerContext context.Context
	controlServerTLS       string
	workerConfigs          []workceptor.WorkerConfig
	TLSServerConfigs       []*netceptor.TLSServerConfig
	TLSClientConfigs       []*netceptor.TLSClientConfig
	WorkSigningKey         *workceptor.SigningKeyPrivateCfg
	WorkVerificationKey    *workceptor.VerifyingKeyPublicCfg
}

type listenerName string

type (
	workPlugin string // "kube" or "command"
	workType   string // identifier for an instance of work-kubernetes or work-command
)

// Status returns the status of the node.
func (n *LibNode) Status() (*netceptor.Status, error) {
	status := n.netceptorInstance.Status()

	return &status, nil
}

// GetControlSocket returns the path to the controlsocket.
func (n *LibNode) GetControlSocket() string {
	return n.ControlSocket
}

// GetDataDir returns the path to the directory where data is stored for this node.
func (n *LibNode) GetDataDir() string {
	return n.Config.DataDir
}

// GetID returns the ID (name) of this node.
func (n *LibNode) GetID() string {
	return n.Config.ID
}

// Start will start local services (netceptor, workceptor, controlsvc),
// then start any listeners, finally establishing any remote connections.
// Note that this requires remote nodes to be running since we need to detect which
// random port was assigned to the backend. This is typically only used when calling Shutdown
// in the tests. When starting the mesh for the first time we loop over nodes in 2 phases,
// first calling StartListeners and then EstablishRemoteConnections.
func (n *LibNode) Start() error {
	var err error

	err = n.StartLocalServices()
	if err != nil {
		return err
	}

	err = n.StartListeners()
	if err != nil {
		return err
	}

	err = n.EstablishRemoteConnections()
	if err != nil {
		return err
	}

	return nil
}

// StartListeners loops over n.ListenerCfgs, which is an interface that wraps
// TCPListenerCfg, UDPListenerCfg, and WebsocketListenerCfg and starts listening
// on the appropriate protocol.
func (n *LibNode) StartListeners() error {
	var bi *BackendInfo
	var err error

	for _, listenerCfg := range n.ListenerCfgs {
		switch lcfg := listenerCfg.(type) {
		case *backends.TCPListenerCfg:
			bi, err = n.TCPListen(listenerCfg)

			// Record what address we are listening on so we can reuse it if we restart this node
			lcfg.BindAddr = bi.listener.GetAddr()
		case *backends.UDPListenerCfg:
			bi, err = n.UDPListen(listenerCfg)

			// Record what address we are listening on so we can reuse it if we restart this node
			lcfg.BindAddr = bi.listener.GetAddr()
		case *backends.WebsocketListenerCfg:
			bi, err = n.WebsocketListen(listenerCfg)

			// Record what address we are listening on so we can reuse it if we restart this node
			lcfg.BindAddr = bi.listener.GetAddr()
		default:
			err = fmt.Errorf("unknown listener type: %s", reflect.TypeOf(lcfg))
		}

		if err != nil {
			return err
		}
	}

	return nil
}

// EstablishRemoteConnections discovers which address a remote backend is listening on
// and then dials out to it.
func (n *LibNode) EstablishRemoteConnections() error {
	for _, connection := range n.Connections {
		backend := connection.RemoteNode.backends[connection.Protocol]
		host, _, err := net.SplitHostPort(backend.bindAddr)
		dialAddr := backend.listener.GetAddr()

		if err != nil {
			return err
		}

		tlscfg, err := n.netceptorInstance.GetClientTLSConfig(connection.TLS, host, netceptor.ExpectedHostnameTypeDNS)
		if err != nil {
			return err
		}

		connectionCost := backend.connectionCost
		nodeCost, ok := backend.nodeCost[n.GetID()]
		if ok {
			connectionCost = nodeCost
		}
		switch connection.Protocol {
		case "tcp":
			err = n.TCPDial(dialAddr, connectionCost, tlscfg)
			if err != nil {
				return err
			}
		case "udp":
			err = n.UDPDial(dialAddr, connectionCost)
			if err != nil {
				return err
			}
		case "ws":
			proto := "wss://"
			if tlscfg == nil {
				proto = "ws://"
			}

			err = n.WebSocketDial(proto+dialAddr, connectionCost, tlscfg)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Shutdown stops the node and waits for it to exit.
func (n *LibNode) Shutdown() {
	n.Destroy()
	n.WaitForShutdown()

	// Forces a new instance of netceptor to get created when we restart.
	// This is necessary because we allow for pre-assigning the netceptor instance
	// so we can simulate duplicate nodes in TestDuplicateNodes.
	n.netceptorInstance = nil
}

// Destroy instructs the node to stop its services.
func (n *LibNode) Destroy() {
	n.controlServerCanceller()
	n.netceptorInstance.Shutdown()
}

// WaitForShutdown Waits for the node to shutdown completely.
func (n *LibNode) WaitForShutdown() {
	n.netceptorInstance.BackendWait()
}

// TCPListen takes a ListenerCfg (backends.TCPListenerCfg) and listens for TCP traffic.
func (n *LibNode) TCPListen(listenerCfg ListenerCfg) (*BackendInfo, error) {
	tlsCfg, err := n.netceptorInstance.GetServerTLSConfig(listenerCfg.GetTLS())
	if err != nil {
		return nil, err
	}

	backend, err := backends.NewTCPListener(listenerCfg.GetAddr(), tlsCfg, n.netceptorInstance.Logger)
	if err != nil {
		return nil, err
	}

	cost := listenerCfg.GetCost()
	nodeCost := listenerCfg.GetNodeCost()

	err = n.netceptorInstance.AddBackend(
		backend,
		netceptor.BackendConnectionCost(cost),
		netceptor.BackendNodeCost(nodeCost),
	)
	if err != nil {
		return nil, err
	}

	bi := BackendInfo{
		protocol:       "tcp",
		bindAddr:       listenerCfg.GetAddr(),
		connectionCost: cost,
		nodeCost:       nodeCost,
		listener:       backend,
	}

	n.backends[bi.protocol] = bi

	return &bi, nil
}

// TCPDial registers a new netceptor.Backend that will dial a remote node via TCP.
func (n *LibNode) TCPDial(address string, cost float64, tlsCfg *tls.Config) error {
	b1, err := backends.NewTCPDialer(address, true, tlsCfg, n.netceptorInstance.Logger)
	if err != nil {
		return err
	}
	err = n.netceptorInstance.AddBackend(b1, netceptor.BackendConnectionCost(cost))

	return err
}

// UDPListen takes a ListenerCfg (backends.UDPListenerCfg) and listens for UDP traffic.
func (n *LibNode) UDPListen(listenerCfg ListenerCfg) (*BackendInfo, error) {
	backend, err := backends.NewUDPListener(listenerCfg.GetAddr(), n.netceptorInstance.Logger)
	if err != nil {
		return nil, err
	}

	cost := listenerCfg.GetCost()
	nodeCost := listenerCfg.GetNodeCost()

	err = n.netceptorInstance.AddBackend(
		backend,
		netceptor.BackendConnectionCost(cost),
		netceptor.BackendNodeCost(nodeCost),
	)
	if err != nil {
		return nil, err
	}

	bi := BackendInfo{
		protocol:       "udp",
		bindAddr:       listenerCfg.GetAddr(),
		connectionCost: cost,
		nodeCost:       nodeCost,
		listener:       backend,
	}

	n.backends[bi.protocol] = bi

	return &bi, nil
}

// UDPDial registers a new netceptor.Backend that will dial a remote node via UDP.
func (n *LibNode) UDPDial(address string, cost float64) error {
	b1, err := backends.NewUDPDialer(address, true, n.netceptorInstance.Logger)
	if err != nil {
		return err
	}
	err = n.netceptorInstance.AddBackend(b1, netceptor.BackendConnectionCost(cost))

	return err
}

// WebsocketListen takes a ListenerCfg (backends.WebsocketListenerCfg) and listens for Websocket traffic.
func (n *LibNode) WebsocketListen(listenerCfg ListenerCfg) (*BackendInfo, error) {
	tlsCfg, err := n.netceptorInstance.GetServerTLSConfig(listenerCfg.GetTLS())
	if err != nil {
		return nil, err
	}

	backend, err := backends.NewWebsocketListener(listenerCfg.GetAddr(), tlsCfg, n.netceptorInstance.Logger, nil, nil)
	if err != nil {
		return nil, err
	}

	cost := listenerCfg.GetCost()
	nodeCost := listenerCfg.GetNodeCost()

	err = n.netceptorInstance.AddBackend(
		backend,
		netceptor.BackendConnectionCost(cost),
		netceptor.BackendNodeCost(nodeCost),
	)
	if err != nil {
		return nil, err
	}

	bi := BackendInfo{
		protocol:       "ws",
		bindAddr:       listenerCfg.GetAddr(),
		connectionCost: cost,
		nodeCost:       nodeCost,
		listener:       backend,
	}

	n.backends[bi.protocol] = bi

	return &bi, nil
}

// WebSocketDial registers a new netceptor.Backend that will dial a remote node via a WebSocket.
func (n *LibNode) WebSocketDial(address string, cost float64, tlsCfg *tls.Config) error {
	b1, err := backends.NewWebsocketDialer(address, tlsCfg, "", true, n.netceptorInstance.Logger, nil)
	if err != nil {
		return err
	}
	err = n.netceptorInstance.AddBackend(b1, netceptor.BackendConnectionCost(cost))

	return err
}

func (n *LibNode) StartLocalServices() error {
	// This conditional only exists to give TestDuplicateNodes a way
	// to simulate a duplicate node on the mesh.
	if n.netceptorInstance == nil {
		n.netceptorInstance = netceptor.New(context.Background(), n.GetID())
	}

	ctx, canceller := context.WithCancel(context.Background())
	n.controlerServerContext = ctx
	n.controlServerCanceller = canceller
	n.controlServer = controlsvc.New(true, n.netceptorInstance)

	err := n.configureFirewallRules()
	if err != nil {
		return err
	}

	err = n.configureTLS()
	if err != nil {
		return err
	}

	tlsCfg, err := n.netceptorInstance.GetServerTLSConfig(n.controlServerTLS)
	if err != nil {
		return err
	}

	n.workceptorInstance, err = workceptor.New(n.netceptorInstance.Context(), n.netceptorInstance, n.GetDataDir())
	if err != nil {
		return err
	}

	err = n.configureWorkSigning()
	if err != nil {
		return err
	}

	err = n.workceptorInstance.RegisterWithControlService(n.controlServer)
	if err != nil {
		return err
	}

	err = n.configureWorkers()
	if err != nil {
		return err
	}

	err = n.controlServer.RunControlSvc(n.controlerServerContext, "control", tlsCfg, n.ControlSocket, os.FileMode(0o600), "", nil)
	if err != nil {
		return err
	}

	return nil
}

func (n *LibNode) configureFirewallRules() error {
	rules, err := netceptor.ParseFirewallRules(n.Config.FirewallRules)
	if err != nil {
		return err
	}

	err = n.netceptorInstance.AddFirewallRules(rules, true)
	if err != nil {
		return err
	}

	return nil
}

func (n *LibNode) configureTLS() error {
	for _, c := range n.TLSServerConfigs {
		tlscfg, err := c.PrepareTLSServerConfig(n.netceptorInstance)
		if err != nil {
			return err
		}

		err = n.netceptorInstance.SetServerTLSConfig(c.Name, tlscfg)
		if err != nil {
			return err
		}
	}

	for _, c := range n.TLSClientConfigs {
		tlscfg, pinnedFingerprints, err := c.PrepareTLSClientConfig(n.netceptorInstance)
		if err != nil {
			return err
		}

		err = n.netceptorInstance.SetClientTLSConfig(c.Name, tlscfg, pinnedFingerprints)
		if err != nil {
			return err
		}
	}

	return nil
}

func (n *LibNode) configureWorkers() error {
	for _, cfg := range n.workerConfigs {
		err := n.workceptorInstance.RegisterWorker(cfg.GetWorkType(), cfg.NewWorker, cfg.GetVerifySignature())
		if err != nil {
			return err
		}
	}

	return nil
}

func (n *LibNode) configureWorkSigning() error {
	if n.WorkSigningKey != nil {
		duration, err := n.WorkSigningKey.PrepareSigningKeyPrivateCfg()
		if err != nil {
			return err
		}

		if duration != nil {
			n.workceptorInstance.SigningExpiration = *duration
		}

		n.workceptorInstance.SigningKey = n.WorkSigningKey.PrivateKey
	}

	if n.WorkVerificationKey != nil {
		err := n.WorkVerificationKey.PrepareVerifyingKeyPublicCfg()
		if err != nil {
			return err
		}

		n.workceptorInstance.VerifyingKey = n.WorkVerificationKey.PublicKey
	}

	return nil
}

// Connection is an abstraction that ultimately results in a new running netceptor.Backend.
type Connection struct {
	RemoteNode *LibNode
	Protocol   string
	TLS        string
}

type ListenerCfg interface {
	GetCost() float64
	GetNodeCost() map[string]float64
	GetAddr() string
	GetTLS() string
}

type NativeBackend interface {
	netceptor.Backend
	GetAddr() string
	GetTLS() *tls.Config
}

type BackendInfo struct {
	protocol       string
	bindAddr       string
	connectionCost float64
	nodeCost       map[string]float64
	listener       NativeBackend
}

func newListenerCfg(proto string, tls string, cost float64, nodeCost map[string]float64) ListenerCfg {
	switch proto {
	case "tcp":
		return &backends.TCPListenerCfg{BindAddr: "localhost:0", TLS: tls, Cost: cost, NodeCost: nodeCost}
	case "udp":
		return &backends.UDPListenerCfg{BindAddr: "localhost:0", Cost: cost, NodeCost: nodeCost}
	case "ws":
		return &backends.WebsocketListenerCfg{BindAddr: "localhost:0", TLS: tls, Cost: cost, NodeCost: nodeCost}
	}

	return nil
}
