package mesh

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/project-receptor/receptor/pkg/backends"
	"github.com/project-receptor/receptor/pkg/netceptor"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net"
	"os"
	"reflect"
	"time"
)

// Node holds a Netceptor, this layer of abstraction might be unnecessary and
// go away later
type Node struct {
	NetceptorInstance *netceptor.Netceptor
	Backends          []netceptor.Backend
}

// Mesh contains a list of Nodes and the yaml definition that created them
type Mesh struct {
	Nodes          map[string]Node
	MeshDefinition *YamlData
}

// YamlData is the top level structure that defines how our yaml mesh data should be
// represented
type YamlData struct {
	Nodes map[string]*YamlNode
}

// YamlNode describes how a single node should be represented in yaml
type YamlNode struct {
	Connections map[string]float64
	Listen      []*YamlListener
	Name        string
}

// YamlListener describes how a single listener should be represented in yaml
type YamlListener struct {
	Cost     float64
	Addr     string
	Protocol string
	// Filenames to a ssl key and cert, relative to the executable, for tests that is
	// in the directory of the test source
	Sslkey  string
	Sslcert string
}

// Error handler that gets called for backend errors
func handleError(err error, fatal bool) {
	fmt.Printf("Error: %s\n", err)
	if fatal {
		os.Exit(1)
	}
}

// NewNode builds a node with the name passed as the argument
func NewNode(name string) Node {
	n1 := netceptor.New(context.Background(), name, nil)
	return Node{
		NetceptorInstance: n1,
	}
}

// TCPListen helper function to create and start a TCPListener
// This might be an unnecessary abstraction and maybe should be deleted
func (n *Node) TCPListen(address string, cost float64, tlsCfg *tls.Config) error {
	b1, err := backends.NewTCPListener(address, nil)
	if err != nil {
		return err
	}
	n.Backends = append(n.Backends, b1)
	err = n.NetceptorInstance.AddBackend(b1, cost)
	return err
}

// TCPDial helper function to create and start a TCPDialer
// This might be an unnecessary abstraction and maybe should be deleted
func (n *Node) TCPDial(address string, cost float64, tlsCfg *tls.Config) error {
	b1, err := backends.NewTCPDialer(address, true, nil)
	if err != nil {
		return err
	}
	err = n.NetceptorInstance.AddBackend(b1, cost)
	return err
}

// UDPListen helper function to create and start a UDPListener
// This might be an unnecessary abstraction and maybe should be deleted
func (n *Node) UDPListen(address string, cost float64) error {
	b1, err := backends.NewUDPListener(address)
	if err != nil {
		return err
	}
	n.Backends = append(n.Backends, b1)
	err = n.NetceptorInstance.AddBackend(b1, cost)
	return err
}

// UDPDial helper function to create and start a UDPDialer
// This might be an unnecessary abstraction and maybe should be deleted
func (n *Node) UDPDial(address string, cost float64) error {
	b1, err := backends.NewUDPDialer(address, true)
	if err != nil {
		return err
	}
	err = n.NetceptorInstance.AddBackend(b1, cost)
	return err
}

// WebsocketListen helper function to create and start a WebsocketListener
// This might be an unnecessary abstraction and maybe should be deleted
func (n *Node) WebsocketListen(address string, cost float64, tlsCfg *tls.Config) error {
	// TODO: Add support for TLS
	b1, err := backends.NewWebsocketListener(address, nil)
	if err != nil {
		return err
	}
	n.Backends = append(n.Backends, b1)
	err = n.NetceptorInstance.AddBackend(b1, cost)
	return err
}

// WebsocketDial helper function to create and start a WebsocketDialer
// This might be an unnecessary abstraction and maybe should be deleted
func (n *Node) WebsocketDial(address string, cost float64, tlsCfg *tls.Config) error {
	// TODO: Add support for TLS and extra headers
	b1, err := backends.NewWebsocketDialer(address, nil, "", true)
	if err != nil {
		return err
	}
	err = n.NetceptorInstance.AddBackend(b1, cost)
	return err
}

// Ping a node and wait for a response
func (n *Node) Ping(node string) (map[string]interface{}, error) {
	shutdownctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pc, err := n.NetceptorInstance.ListenPacket("")
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = pc.Close()
	}()
	startTime := time.Now()
	replyChan := make(chan net.Addr)
	go func() {
		buf := make([]byte, 8)
		_, addr, err := pc.ReadFrom(buf)
		if err == nil {
			replyChan <- addr
		}
	}()

	sendErrChan := make(chan error)
	go func() {
		_, err = pc.WriteTo([]byte{}, netceptor.NewAddr(node, "ping"))
		if err != nil {
			sendErrChan <- err
			return
		}
		select {
		case <-time.After(100 * time.Millisecond):
			_, err = pc.WriteTo([]byte{}, netceptor.NewAddr(node, "ping"))
			if err != nil {
				sendErrChan <- err
				return
			}
		case <-shutdownctx.Done():
			return
		}
	}()
	cfr := make(map[string]interface{})
	select {
	case addr := <-replyChan:
		cfr["Result"] = fmt.Sprintf("Reply from %s in %s\n", addr.String(), time.Since(startTime))
		cfr["Node"] = addr.String()
		cfr["Time"] = time.Since(startTime)
	case <-shutdownctx.Done():
		cfr["Result"] = "Timed out waiting for response"
		return cfr, err
	case err := <-sendErrChan:
		cfr["Result"] = err
		return cfr, err
	}
	return cfr, nil
}

// NewMeshFromFile Takes a filename of a file with a yaml description of a mesh, loads it and
// calls NewMeshFromYaml on it
func NewMeshFromFile(filename string) (*Mesh, error) {
	yamlDat, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	MeshDefinition := YamlData{}

	err = yaml.Unmarshal(yamlDat, &MeshDefinition)
	if err != nil {
		return nil, err
	}

	return NewMeshFromYaml(&MeshDefinition)
}

// NewMeshFromYaml takes a yaml mesh description and returns a mesh of nodes
// listening and dialing as defined in the yaml
// TODO: Add support for websockets
func NewMeshFromYaml(MeshDefinition *YamlData) (*Mesh, error) {
	Nodes := make(map[string]Node)

	// We must start listening on all our nodes before we start dialing so
	// there's something to dial into
	for k := range MeshDefinition.Nodes {
		node := NewNode(MeshDefinition.Nodes[k].Name)
		for _, listener := range MeshDefinition.Nodes[k].Listen {
			var tlsConfig *tls.Config
			var err error
			if listener.Sslcert != "" && listener.Sslkey != "" {
				cert, err := tls.LoadX509KeyPair(listener.Sslcert, listener.Sslkey)
				if err != nil {
					return nil, err
				}
				tlsConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
			}
			if listener.Addr != "" {
				if listener.Protocol == "tcp" {
					err = node.TCPListen(listener.Addr, listener.Cost, tlsConfig)
					if err != nil {
						return nil, err
					}
				} else if listener.Protocol == "udp" {
					err := node.UDPListen(listener.Addr, listener.Cost)
					if err != nil {
						return nil, err
					}
				} else if listener.Protocol == "ws" {
					err := node.WebsocketListen(listener.Addr, listener.Cost, tlsConfig)
					if err != nil {
						return nil, err
					}
				}
			} else {
				retries := 5
				if listener.Protocol == "tcp" {
					for retries > 0 {
						addrString := "127.0.0.1:0"
						err := node.TCPListen(addrString, listener.Cost, tlsConfig)
						if err == nil {
							listener.Addr = node.Backends[len(node.Backends)-1].(*backends.TCPListener).Addr().String()
							break
						}
						retries--
					}
				} else if listener.Protocol == "udp" {
					for retries > 0 {
						addrString := "127.0.0.1:0"
						err := node.UDPListen(addrString, listener.Cost)
						if err == nil {
							listener.Addr = node.Backends[len(node.Backends)-1].(*backends.UDPListener).LocalAddr().String()
							break
						}
						retries--
					}
				} else if listener.Protocol == "ws" {
					for retries > 0 {
						addrString := "127.0.0.1:0"
						err := node.WebsocketListen(addrString, listener.Cost, tlsConfig)
						if err == nil {
							listener.Addr = "ws://" + node.Backends[len(node.Backends)-1].(*backends.WebsocketListener).Addr().String()
							break
						}
						retries--
					}
				}
				if retries == 0 {
					return nil, fmt.Errorf("Failed to connect to %s://%s after trying 5 times", listener.Protocol, listener.Addr)
				}
			}
		}
		Nodes[MeshDefinition.Nodes[k].Name] = node
	}
	for k := range MeshDefinition.Nodes {
		node := Nodes[MeshDefinition.Nodes[k].Name]
		for connNode, cost := range MeshDefinition.Nodes[k].Connections {
			// Update this to choose which listener to dial into
			if MeshDefinition.Nodes[connNode].Listen[0].Protocol == "tcp" {
				err := node.TCPDial(MeshDefinition.Nodes[connNode].Listen[0].Addr, cost, nil)
				if err != nil {
					return nil, err
				}
			} else if MeshDefinition.Nodes[connNode].Listen[0].Protocol == "udp" {
				err := node.UDPDial(MeshDefinition.Nodes[connNode].Listen[0].Addr, cost)
				if err != nil {
					return nil, err
				}
			} else if MeshDefinition.Nodes[connNode].Listen[0].Protocol == "ws" {
				err := node.WebsocketDial(MeshDefinition.Nodes[connNode].Listen[0].Addr, cost, nil)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return &Mesh{
		Nodes,
		MeshDefinition,
	}, nil
}

// Shutdown This is broken and causes the thread to hang, dont use until
// netceptor.Shutdown is fixed
func (m *Mesh) Shutdown() {
	for _, node := range m.Nodes {
		node.NetceptorInstance.Shutdown()
	}
}

// CheckConnections returns true if the connections defined in our mesh definition are
// consistent with the connections made by the nodes
func (m *Mesh) CheckConnections() bool {
	for _, status := range m.Status() {
		actualConnections := map[string]float64{}
		for _, connection := range status.Connections {
			actualConnections[connection.NodeID] = connection.Cost
		}
		expectedConnections := map[string]float64{}
		for k, v := range m.MeshDefinition.Nodes[status.NodeID].Connections {
			expectedConnections[k] = v
		}
		for nodeID, node := range m.MeshDefinition.Nodes {
			if nodeID == status.NodeID {
				continue
			}
			for k, v := range node.Connections {
				if k == status.NodeID {
					expectedConnections[nodeID] = v
				}
			}
		}
		if reflect.DeepEqual(actualConnections, expectedConnections) {
			return true
		}
	}
	return false
}

// CheckKnownConnectionCosts returns true if every node has the same view of the connections in the mesh
func (m *Mesh) CheckKnownConnectionCosts() bool {
	meshStatus := m.Status()
	// If the mesh is empty we are done
	if len(meshStatus) == 0 {
		return true
	}

	knownConnectionCosts := meshStatus[0].KnownConnectionCosts
	for _, status := range m.Status() {
		if !reflect.DeepEqual(status.KnownConnectionCosts, knownConnectionCosts) {
			return false
		}
	}
	return true
}

// CheckRoutes returns true if every node has a route to every other node
func (m *Mesh) CheckRoutes() bool {
	for _, status := range m.Status() {
		for nodeID := range m.Nodes {
			// Dont check a route to ourselves
			if status.NodeID == nodeID {
				continue
			}
			_, ok := status.RoutingTable[nodeID]
			if !ok {
				return false
			}
		}
	}
	return true
}

// WaitForReady Waits for connections and routes to converge
func (m *Mesh) WaitForReady(timeout float64) error {
	// TODO: Add a poll interval parameter
	connectionsReady := m.CheckConnections()
	for ; timeout > 0 && !connectionsReady; connectionsReady = m.CheckConnections() {
		time.Sleep(100 * time.Millisecond)
		timeout -= 100
	}
	if connectionsReady == false {
		return errors.New("Timed out while waiting for connections")
	}

	costsConsistent := m.CheckKnownConnectionCosts()
	for ; timeout > 0 && !costsConsistent; costsConsistent = m.CheckKnownConnectionCosts() {
		time.Sleep(100 * time.Millisecond)
		timeout -= 100
	}
	if costsConsistent == false {
		return errors.New("Timed out while waiting for connection costs to converge")
	}

	routesReady := m.CheckRoutes()
	for ; timeout > 0 && !routesReady; routesReady = m.CheckRoutes() {
		time.Sleep(100 * time.Millisecond)
		timeout -= 100
	}
	if costsConsistent == false {
		return errors.New("Timed out while waiting for every node to have a route to every other node")
	}

	return nil
}

// Status returns a list of statuses from the contained netceptors
func (m *Mesh) Status() []netceptor.Status {
	var out []netceptor.Status
	for _, node := range m.Nodes {
		out = append(out, node.NetceptorInstance.Status())
	}
	return out
}
