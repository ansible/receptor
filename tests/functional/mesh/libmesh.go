package mesh

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"github.com/project-receptor/receptor/pkg/backends"
	"github.com/project-receptor/receptor/pkg/controlsvc"
	"github.com/project-receptor/receptor/pkg/netceptor"
	"github.com/project-receptor/receptor/tests/functional/lib/receptorcontrol"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// LibNode holds a Netceptor, this layer of abstraction might be unnecessary and
// go away later
type LibNode struct {
	NetceptorInstance      *netceptor.Netceptor
	Backends               []netceptor.Backend
	controlServer          *controlsvc.Server
	controlSocket          string
	controlServerCanceller context.CancelFunc
	serverTLSConfigs       map[string]*tls.Config
	clientTLSConfigs       map[string]*tls.Config
}

// LibMesh contains a list of Nodes and the yaml definition that created them
type LibMesh struct {
	nodes          map[string]*LibNode
	MeshDefinition *YamlData
	dataDir        string
}

// Error handler that gets called for backend errors
func handleError(err error, fatal bool) {
	fmt.Printf("Error: %s\n", err)
	if fatal {
		os.Exit(1)
	}
}

// NewLibNode builds a node with the name passed as the argument
func NewLibNode(name string) *LibNode {
	n1 := netceptor.New(context.Background(), name, nil)
	return &LibNode{
		NetceptorInstance: n1,
		serverTLSConfigs:  make(map[string]*tls.Config),
		clientTLSConfigs:  make(map[string]*tls.Config),
	}
}

// Status returns the status of the node
func (n *LibNode) Status() (*netceptor.Status, error) {
	status := n.NetceptorInstance.Status()
	return &status, nil
}

// ControlSocket Returns the path to the controlsocket
func (n *LibNode) ControlSocket() string {
	return n.controlSocket
}

// Shutdown shuts the node down
func (n *LibNode) Shutdown() {
	n.controlServerCanceller()
	n.NetceptorInstance.Shutdown()
}

// WaitForShutdown Waits for the node to shutdown completely
func (n *LibNode) WaitForShutdown() {
	n.NetceptorInstance.BackendWait()
}

// Nodes Returns a list of nodes
func (m *LibMesh) Nodes() map[string]Node {
	nodes := make(map[string]Node)
	for k, v := range m.nodes {
		nodes[k] = v
	}
	return nodes
}

// TCPListen helper function to create and start a TCPListener
// This might be an unnecessary abstraction and maybe should be deleted
func (n *LibNode) TCPListen(address string, cost float64, tlsCfg *tls.Config) error {
	b1, err := backends.NewTCPListener(address, nil)
	if err != nil {
		return err
	}
	n.Backends = append(n.Backends, b1)
	err = n.NetceptorInstance.AddBackend(b1, cost, nil)
	return err
}

// TCPDial helper function to create and start a TCPDialer
// This might be an unnecessary abstraction and maybe should be deleted
func (n *LibNode) TCPDial(address string, cost float64, tlsCfg *tls.Config) error {
	b1, err := backends.NewTCPDialer(address, true, nil)
	if err != nil {
		return err
	}
	err = n.NetceptorInstance.AddBackend(b1, cost, nil)
	return err
}

// UDPListen helper function to create and start a UDPListener
// This might be an unnecessary abstraction and maybe should be deleted
func (n *LibNode) UDPListen(address string, cost float64) error {
	b1, err := backends.NewUDPListener(address)
	if err != nil {
		return err
	}
	n.Backends = append(n.Backends, b1)
	err = n.NetceptorInstance.AddBackend(b1, cost, nil)
	return err
}

// UDPDial helper function to create and start a UDPDialer
// This might be an unnecessary abstraction and maybe should be deleted
func (n *LibNode) UDPDial(address string, cost float64) error {
	b1, err := backends.NewUDPDialer(address, true)
	if err != nil {
		return err
	}
	err = n.NetceptorInstance.AddBackend(b1, cost, nil)
	return err
}

// WebsocketListen helper function to create and start a WebsocketListener
// This might be an unnecessary abstraction and maybe should be deleted
func (n *LibNode) WebsocketListen(address string, cost float64, tlsCfg *tls.Config) error {
	// TODO: Add support for TLS
	b1, err := backends.NewWebsocketListener(address, nil)
	if err != nil {
		return err
	}
	n.Backends = append(n.Backends, b1)
	err = n.NetceptorInstance.AddBackend(b1, cost, nil)
	return err
}

// WebsocketDial helper function to create and start a WebsocketDialer
// This might be an unnecessary abstraction and maybe should be deleted
func (n *LibNode) WebsocketDial(address string, cost float64, tlsCfg *tls.Config) error {
	// TODO: Add support for TLS and extra headers
	b1, err := backends.NewWebsocketDialer(address, nil, "", true)
	if err != nil {
		return err
	}
	err = n.NetceptorInstance.AddBackend(b1, cost, nil)
	return err
}

// NewLibMeshFromFile Takes a filename of a file with a yaml description of a mesh, loads it and
// calls NewMeshFromYaml on it
func NewLibMeshFromFile(filename string) (Mesh, error) {
	yamlDat, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	MeshDefinition := YamlData{}

	err = yaml.Unmarshal(yamlDat, &MeshDefinition)
	if err != nil {
		return nil, err
	}

	return NewLibMeshFromYaml(MeshDefinition)
}

// NewLibMeshFromYaml takes a yaml mesh description and returns a mesh of nodes
// listening and dialing as defined in the yaml
func NewLibMeshFromYaml(MeshDefinition YamlData) (*LibMesh, error) {
	mesh := &LibMesh{}
	// Setup the mesh directory
	baseDir := filepath.Join(os.TempDir(), "receptor-testing")
	// Ignore the error, if the dir already exists thats fine
	os.Mkdir(baseDir, 0755)
	tempdir, err := ioutil.TempDir(baseDir, "mesh-*")
	if err != nil {
		return nil, err
	}
	mesh.dataDir = tempdir

	nodes := make(map[string]*LibNode)
	// We must start listening on all our nodes before we start dialing so
	// there's something to dial into
	for k := range MeshDefinition.Nodes {
		node := NewLibNode(k)
		for _, attr := range MeshDefinition.Nodes[k].Nodedef {
			attrMap := attr.(map[interface{}]interface{})
			for k, v := range attrMap {
				k = k.(string)
				if k == "tls-client" {
					vMap := v.(map[interface{}]interface{})
					// Taken from pkg/netceptor/tlsconfig.go
					tlscfg := &tls.Config{}

					if vMap["cert"] != "" || vMap["key"] != "" {
						if vMap["cert"] == "" || vMap["key"] == "" {
							return nil, fmt.Errorf("cert and key must both be supplied or neither")
						}
						certbytes, err := ioutil.ReadFile(vMap["cert"].(string))
						if err != nil {
							return nil, err
						}
						keybytes, err := ioutil.ReadFile(vMap["key"].(string))
						if err != nil {
							return nil, err
						}
						cert, err := tls.X509KeyPair(certbytes, keybytes)
						if err != nil {
							return nil, err
						}
						tlscfg.Certificates = []tls.Certificate{cert}
					}

					if vMap["rootcas"] != "" {
						bytes, err := ioutil.ReadFile(vMap["rootcas"].(string))
						if err != nil {
							return nil, fmt.Errorf("error reading root CAs file: %s", err)
						}

						rootCAs := x509.NewCertPool()
						rootCAs.AppendCertsFromPEM(bytes)
						tlscfg.RootCAs = rootCAs
					}

					tlscfg.InsecureSkipVerify = vMap["insecureskipverify"].(bool)

					node.clientTLSConfigs[vMap["name"].(string)] = tlscfg
				} else if k == "tls-server" {
					vMap := v.(map[interface{}]interface{})
					// Taken from pkg/netceptor/tlsconfig.go
					tlscfg := &tls.Config{}

					certbytes, err := ioutil.ReadFile(vMap["cert"].(string))
					if err != nil {
						return nil, err
					}
					keybytes, err := ioutil.ReadFile(vMap["key"].(string))
					if err != nil {
						return nil, err
					}
					cert, err := tls.X509KeyPair(certbytes, keybytes)
					if err != nil {
						return nil, err
					}

					tlscfg.Certificates = []tls.Certificate{cert}

					if vMap["clientcas"] != nil {
						bytes, err := ioutil.ReadFile(vMap["clientcas"].(string))
						if err != nil {
							return nil, fmt.Errorf("error reading client CAs file: %s", err)
						}
						clientCAs := x509.NewCertPool()
						clientCAs.AppendCertsFromPEM(bytes)
						tlscfg.ClientCAs = clientCAs
					}

					if vMap["requireclientcert"] != nil {
						tlscfg.ClientAuth = tls.RequireAndVerifyClientCert
					} else if vMap["clientcas"] != nil {
						tlscfg.ClientAuth = tls.VerifyClientCertIfGiven
					} else {
						tlscfg.ClientAuth = tls.NoClientCert
					}

					node.serverTLSConfigs[vMap["name"].(string)] = tlscfg
				}
			}
		}

		for attrkey, attr := range MeshDefinition.Nodes[k].Nodedef {
			attrMap := attr.(map[interface{}]interface{})
			for k, v := range attrMap {
				k = k.(string)
				if k == "tcp-listener" || k == "udp-listener" || k == "ws-listener" {
					vMap := v.(map[interface{}]interface{})
					port, ok := vMap["port"].(string)
					if !ok {
						port = "0"
					}
					costStr, ok := vMap["cost"].(string)
					if !ok {
						costStr = "1.0"
					}
					cost, err := strconv.ParseFloat(costStr, 64)
					if err != nil {
						return nil, fmt.Errorf("Unable to determine cost for %s", k)
					}
					bindaddr, ok := vMap["bindaddr"].(string)
					if !ok {
						bindaddr = "127.0.0.1"
					}
					address := bindaddr + ":" + port

					var tls *tls.Config
					tlsName, ok := vMap["tls"].(string)
					if !ok {
						tls = nil
					} else {
						tls = node.serverTLSConfigs[tlsName]
					}
					if k == "tcp-listener" {
						err = node.TCPListen(address, cost, tls)
					} else if k == "udp-listener" {
						err = node.UDPListen(address, cost)
					} else if k == "ws-listener" {
						err = node.WebsocketListen(address, cost, tls)
					}
					if err != nil {
						return nil, err
					}
					switch backend := node.Backends[len(node.Backends)-1].(type) {
					case *backends.UDPListener:
						address = backend.LocalAddr().String()
					case *backends.TCPListener:
						address = backend.Addr().String()
					case *backends.WebsocketListener:
						address = backend.Addr().String()
					}
					// Store the address back in the meshdef so we can retrieve
					// it later
					port = strings.Split(address, ":")[1]
					bindaddr = strings.Split(address, ":")[0]
					vMap["port"] = port
					vMap["bindaddr"] = bindaddr
					attrMap[k] = vMap
				} else if k == "node" {
					vMap, _ := v.(map[interface{}]interface{})
					id, ok := vMap["id"]
					if ok {
						if k != id {
							return nil, fmt.Errorf("Node id and key in mesh definition must match, key: %s, id: %s", k, id)
						}
					}
				}
			}
			MeshDefinition.Nodes[k].Nodedef[attrkey] = attrMap
		}
		nodes[k] = node
	}
	for k := range MeshDefinition.Nodes {
		node := nodes[k]
		for connNode, index := range MeshDefinition.Nodes[k].Connections {
			attr := MeshDefinition.Nodes[connNode].Nodedef[index]
			attrMap, ok := attr.(map[interface{}]interface{})
			listener, ok := attrMap["tcp-listener"]
			if ok {
				listenerMap, ok := listener.(map[interface{}]interface{})
				if !ok {
					return nil, errors.New("Listener object is not a map")
				}
				costStr, ok := listenerMap["cost"].(string)
				if !ok {
					costStr = "1.0"
				}

				cost, err := strconv.ParseFloat(costStr, 64)
				if err != nil {
					return nil, fmt.Errorf("Unable to determine cost for %s", k)
				}

				addr := listenerMap["bindaddr"].(string) + ":" + listenerMap["port"].(string)
				err = node.TCPDial(addr, cost, nil)
				if err != nil {
					return nil, err
				}
			}
			listener, ok = attrMap["udp-listener"]
			if ok {
				listenerMap, ok := listener.(map[interface{}]interface{})
				if !ok {
					return nil, errors.New("Listener object is not a map")
				}
				costStr, ok := listenerMap["cost"].(string)
				if !ok {
					costStr = "1.0"
				}

				cost, err := strconv.ParseFloat(costStr, 64)
				if err != nil {
					return nil, fmt.Errorf("Unable to determine cost for %s", k)
				}

				addr := listenerMap["bindaddr"].(string) + ":" + listenerMap["port"].(string)
				err = node.UDPDial(addr, cost)
				if err != nil {
					return nil, err
				}
			}
			listener, ok = attrMap["ws-listener"]
			if ok {
				listenerMap, ok := listener.(map[interface{}]interface{})
				if !ok {
					return nil, errors.New("Listener object is not a map")
				}
				costStr, ok := listenerMap["cost"].(string)
				if !ok {
					costStr = "1.0"
				}

				cost, err := strconv.ParseFloat(costStr, 64)
				if err != nil {
					return nil, fmt.Errorf("Unable to determine cost for %s", k)
				}

				addr := "ws://" + listenerMap["bindaddr"].(string) + ":" + listenerMap["port"].(string)
				err = node.WebsocketDial(addr, cost, nil)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	// Setup the controlsvc and sockets
	for k, node := range nodes {
		ctx, canceller := context.WithCancel(context.Background())
		node.controlServerCanceller = canceller

		baseDir := filepath.Join(os.TempDir(), "receptor-testing")
		// Ignore the error, if the dir already exists thats fine
		os.Mkdir(baseDir, 0700)
		tempdir, err := ioutil.TempDir(mesh.dataDir, k+"-*")
		if err != nil {
			return nil, err
		}
		node.controlSocket = filepath.Join(tempdir, "controlsock")

		node.controlServer = controlsvc.New(true, node.NetceptorInstance)
		err = node.controlServer.RunControlSvc(ctx, "control", nil, node.controlSocket, os.FileMode(0600))
		if err != nil {
			return nil, err
		}
	}
	mesh.nodes = nodes
	mesh.MeshDefinition = &MeshDefinition
	return mesh, nil
}

// Shutdown stops all running Netceptors and their backends
func (m *LibMesh) Shutdown() {
	for _, node := range m.nodes {
		node.Shutdown()
	}
}

// WaitForShutdown Waits for all running Netceptors and their backends to stop
func (m *LibMesh) WaitForShutdown() {
	for _, node := range m.nodes {
		node.WaitForShutdown()
	}
}

// CheckConnections returns true if the connections defined in our mesh definition are
// consistent with the connections made by the nodes
func (m *LibMesh) CheckConnections() bool {
	statusList, err := m.Status()
	if err != nil {
		return false
	}
	for _, status := range statusList {
		actualConnections := map[string]float64{}
		for _, connection := range status.Connections {
			actualConnections[connection.NodeID] = connection.Cost
		}
		expectedConnections := map[string]float64{}
		for k := range m.MeshDefinition.Nodes[status.NodeID].Connections {
			expectedConnections[k] = 1.0
		}
		for nodeID, node := range m.MeshDefinition.Nodes {
			if nodeID == status.NodeID {
				continue
			}
			for k := range node.Connections {
				if k == status.NodeID {
					expectedConnections[nodeID] = 1.0
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

// CheckRoutes returns true if every node has a route to every other node
func (m *LibMesh) CheckRoutes() bool {
	meshStatus, err := m.Status()
	if err != nil {
		return false
	}
	for _, status := range meshStatus {
		for nodeID := range m.nodes {
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

// CheckControlSockets Checks if the Control sockets in the mesh are all running and accepting
// connections
func (m *LibMesh) CheckControlSockets() bool {
	for _, node := range m.nodes {
		controller := receptorcontrol.New()
		if controller.Connect(node.ControlSocket()) != nil {
			return false
		}
		controller.Close()
	}
	return true
}

// WaitForReady Waits for connections and routes to converge
func (m *LibMesh) WaitForReady(timeout float64) error {
	// TODO: Add a poll interval parameter
	socketsReady := m.CheckControlSockets()
	for ; timeout > 0 && !socketsReady; socketsReady = m.CheckControlSockets() {
		time.Sleep(1 * time.Millisecond)
		timeout--
	}
	if socketsReady == false {
		return errors.New("Timed out while waiting for sockets")
	}

	connectionsReady := m.CheckConnections()
	for ; timeout > 0 && !connectionsReady; connectionsReady = m.CheckConnections() {
		time.Sleep(1 * time.Millisecond)
		timeout--
	}
	if connectionsReady == false {
		return errors.New("Timed out while waiting for connections")
	}

	costsConsistent := m.CheckKnownConnectionCosts()
	for ; timeout > 0 && !costsConsistent; costsConsistent = m.CheckKnownConnectionCosts() {
		time.Sleep(1 * time.Millisecond)
		timeout--
	}
	if costsConsistent == false {
		return errors.New("Timed out while waiting for connection costs to converge")
	}

	routesReady := m.CheckRoutes()
	for ; timeout > 0 && !routesReady; routesReady = m.CheckRoutes() {
		time.Sleep(1 * time.Millisecond)
		timeout--
	}
	if costsConsistent == false {
		return errors.New("Timed out while waiting for every node to have a route to every other node")
	}

	return nil
}

// Status returns a list of statuses from the contained netceptors
func (m *LibMesh) Status() ([]*netceptor.Status, error) {
	var out []*netceptor.Status
	for _, node := range m.nodes {
		status, err := node.Status()
		if err != nil {
			return nil, err
		}
		out = append(out, status)
	}
	return out, nil
}
