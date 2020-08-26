package mesh

import (
	"context"
	"errors"
	"github.com/project-receptor/receptor/pkg/netceptor"
	"github.com/project-receptor/receptor/tests/functional/lib/receptorcontrol"
	"github.com/project-receptor/receptor/tests/functional/lib/utils"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"time"
)

// CLINode holds a Netceptor, this layer of abstraction might be unnecessary and
// go away later
type CLINode struct {
	receptorCmd    *exec.Cmd
	nodeDir        string
	yamlConfigPath string
	yamlConfig     []interface{}
	controlSocket  string
}

// CLIMesh contains a list of Nodes and the yaml definition that created them
type CLIMesh struct {
	nodes          map[string]*CLINode
	MeshDefinition *YamlData
	dataDir        string
}

// NewCLINode builds a node with the name passed as the argument
func NewCLINode(name string) *CLINode {
	return &CLINode{
		receptorCmd:   nil,
		controlSocket: "",
	}
}

// Status returns the status of the node using the control socket to query the
// node
func (n *CLINode) Status() (*netceptor.Status, error) {
	controller := receptorcontrol.New()
	err := controller.Connect(n.controlSocket)
	if err != nil {
		return nil, err
	}
	defer controller.Close()
	status, err := controller.Status()
	if err != nil {
		return nil, err
	}
	return status, nil
}

// ControlSocket Returns the path to the controlsocket
func (n *CLINode) ControlSocket() string {
	return n.controlSocket
}

// Shutdown kills the receptor process and puts its ports back into the pool to
// be reallocated once it's shutdown
func (n *CLINode) Shutdown() {
	n.receptorCmd.Process.Kill()
	go func() {
		n.receptorCmd.Wait()
		for _, i := range n.yamlConfig {
			m, ok := i.(map[interface{}]interface{})
			if !ok {
				continue
			}
			for k, v := range m {
				if k == "tcp-listener" {
					vMap, _ := v.(map[interface{}]interface{})
					port, _ := strconv.Atoi(vMap["port"].(string))
					utils.FreeTCPPort(port)
				}
				if k == "ws-listener" {
					vMap, _ := v.(map[interface{}]interface{})
					port, _ := strconv.Atoi(vMap["port"].(string))
					utils.FreeTCPPort(port)
				}
				if k == "udp-listener" {
					vMap, _ := v.(map[interface{}]interface{})
					port, _ := strconv.Atoi(vMap["port"].(string))
					utils.FreeUDPPort(port)
				}
			}
		}
	}()
}

// WaitForShutdown Waits for the receptor process to finish
func (n *CLINode) WaitForShutdown() {
	n.receptorCmd.Wait()
}

// Nodes Returns a list of nodes
func (m *CLIMesh) Nodes() map[string]Node {
	nodes := make(map[string]Node)
	for k, v := range m.nodes {
		nodes[k] = v
	}
	return nodes
}

// NewCLIMeshFromFile Takes a filename of a file with a yaml description of a mesh, loads it and
// calls NewMeshFromYaml on it
func NewCLIMeshFromFile(filename string) (Mesh, error) {
	yamlDat, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	MeshDefinition := YamlData{}

	err = yaml.Unmarshal(yamlDat, &MeshDefinition)
	if err != nil {
		return nil, err
	}

	return NewCLIMeshFromYaml(MeshDefinition)
}

// NewCLIMeshFromYaml takes a yaml mesh description and returns a mesh of nodes
// listening and dialing as defined in the yaml
func NewCLIMeshFromYaml(MeshDefinition YamlData) (*CLIMesh, error) {
	mesh := &CLIMesh{}
	// Setup the mesh directory
	baseDir := filepath.Join(os.TempDir(), "receptor-testing")
	// Ignore the error, if the dir already exists thats fine
	os.Mkdir(baseDir, 0755)
	tempdir, err := ioutil.TempDir(baseDir, "mesh-*")
	if err != nil {
		return nil, err
	}
	mesh.dataDir = tempdir

	// HERE BE DRAGONS OF THE TYPE SYSTEMS
	nodes := make(map[string]*CLINode)

	// We must start listening on all our nodes before we start dialing so
	// there's something to dial into
	for k := range MeshDefinition.Nodes {
		node := NewCLINode(k)
		tempdir, err = ioutil.TempDir(mesh.dataDir, k+"-*")
		if err != nil {
			return nil, err
		}
		node.nodeDir = tempdir
		// Keep track of if we need to add an attribute for the node id or if
		// it already exists
		needsIDAttr := true
		for attrkey, attr := range MeshDefinition.Nodes[k].Nodedef {
			attrMap := attr.(map[interface{}]interface{})
			for k, v := range attrMap {
				k = k.(string)
				if k == "tcp-listener" || k == "udp-listener" || k == "ws-listener" {
					vMap, ok := v.(map[interface{}]interface{})
					if !ok {
						vMap = make(map[interface{}]interface{})
					}
					if k == "tcp-listener" || k == "ws-listener" {
						_, ok = vMap["port"]
						if !ok {
							vMap["port"] = strconv.Itoa(utils.ReserveTCPPort())
						}
						attrMap[k] = vMap
					} else if k == "udp-listener" {
						_, ok = vMap["port"]
						if !ok {
							vMap["port"] = strconv.Itoa(utils.ReserveUDPPort())
						}
						attrMap[k] = vMap
					}
				} else if k == "node" {
					vMap, _ := v.(map[interface{}]interface{})
					_, ok := vMap["id"]
					if ok {
						needsIDAttr = false
					}
				}
			}
			MeshDefinition.Nodes[k].Nodedef[attrkey] = attrMap
		}
		if needsIDAttr {
			idYaml := make(map[interface{}]interface{})
			nodeYaml := make(map[interface{}]interface{})
			nodeYaml["id"] = k
			nodeYaml["datadir"] = filepath.Join(node.nodeDir, "datadir")
			os.Mkdir(nodeYaml["datadir"].(string), 0755)
			idYaml["node"] = nodeYaml
			MeshDefinition.Nodes[k].Nodedef = append(MeshDefinition.Nodes[k].Nodedef, idYaml)
		}
		logYaml := make(map[interface{}]interface{})
		levelYaml := make(map[interface{}]interface{})
		levelYaml["level"] = "debug"
		logYaml["log-level"] = levelYaml
		MeshDefinition.Nodes[k].Nodedef = append(MeshDefinition.Nodes[k].Nodedef, logYaml)
		nodes[k] = node
	}
	for k := range MeshDefinition.Nodes {
		for connNode, connYaml := range MeshDefinition.Nodes[k].Connections {
			index := connYaml.Index
			TLS := connYaml.TLS
			attr := MeshDefinition.Nodes[connNode].Nodedef[index]
			attrMap, ok := attr.(map[interface{}]interface{})
			listener, ok := attrMap["tcp-listener"]
			if ok {
				dialerYaml := make(map[interface{}]interface{})
				listenerMap, ok := listener.(map[interface{}]interface{})
				if !ok {
					return nil, errors.New("Listener object is not a map")
				}
				peerYaml := make(map[interface{}]interface{})
				bindaddr, ok := listenerMap["bindaddr"].(string)
				var addr string
				if ok {
					addr = bindaddr + ":" + listenerMap["port"].(string)
				} else {
					addr = "127.0.0.1:" + listenerMap["port"].(string)
				}
				peerYaml["address"] = addr
				peerYaml["cost"] = getListenerCost(listenerMap, k)

				if TLS != "" {
					peerYaml["tls"] = TLS
				}
				dialerYaml["tcp-peer"] = peerYaml
				MeshDefinition.Nodes[k].Nodedef = append(MeshDefinition.Nodes[k].Nodedef, dialerYaml)
			}
			listener, ok = attrMap["udp-listener"]
			if ok {
				dialerYaml := make(map[interface{}]interface{})
				listenerMap, ok := listener.(map[interface{}]interface{})
				if !ok {
					return nil, errors.New("Listener object is not a map")
				}
				peerYaml := make(map[interface{}]interface{})
				bindaddr, ok := listenerMap["bindaddr"].(string)
				var addr string
				if ok {
					addr = bindaddr + ":" + listenerMap["port"].(string)
				} else {
					addr = "127.0.0.1:" + listenerMap["port"].(string)
				}
				peerYaml["address"] = addr
				peerYaml["cost"] = getListenerCost(listenerMap, k)
				dialerYaml["udp-peer"] = peerYaml
				MeshDefinition.Nodes[k].Nodedef = append(MeshDefinition.Nodes[k].Nodedef, dialerYaml)
			}
			listener, ok = attrMap["ws-listener"]
			if ok {
				dialerYaml := make(map[interface{}]interface{})
				listenerMap, ok := listener.(map[interface{}]interface{})
				if !ok {
					return nil, errors.New("Listener object is not a map")
				}
				peerYaml := make(map[interface{}]interface{})

				proto := "ws://"
				tlsName, tlsEnabled := listenerMap["tls"].(string)
				if tlsEnabled && tlsName != "" {
					proto = "wss://"
				}

				bindaddr, ok := listenerMap["bindaddr"].(string)
				var addr string
				if ok {
					addr = proto + bindaddr + ":" + listenerMap["port"].(string)
				} else {
					addr = proto + "127.0.0.1:" + listenerMap["port"].(string)
				}
				peerYaml["address"] = addr
				peerYaml["cost"] = getListenerCost(listenerMap, k)
				if TLS != "" {
					peerYaml["tls"] = TLS
				}
				dialerYaml["ws-peer"] = peerYaml
				MeshDefinition.Nodes[k].Nodedef = append(MeshDefinition.Nodes[k].Nodedef, dialerYaml)
			}
		}
	}

	// Setup the controlsvc and sockets
	for k, node := range nodes {
		node.controlSocket = filepath.Join(node.nodeDir, "controlsock")
		controlServiceYaml := make(map[interface{}]interface{})
		tmp := make(map[interface{}]interface{})
		tmp["filename"] = node.controlSocket
		controlServiceYaml["control-service"] = tmp
		MeshDefinition.Nodes[k].Nodedef = append(MeshDefinition.Nodes[k].Nodedef, controlServiceYaml)
	}

	for k, node := range nodes {
		node.yamlConfig = MeshDefinition.Nodes[k].Nodedef
		strData, err := yaml.Marshal(node.yamlConfig)
		if err != nil {
			return nil, err
		}
		nodedefPath := filepath.Join(node.nodeDir, "nodedef.yaml")
		ioutil.WriteFile(nodedefPath, strData, 0644)
		node.receptorCmd = exec.Command("receptor", "--config", nodedefPath)
		stdout, err := os.Create(filepath.Join(node.nodeDir, "stdout"))
		if err != nil {
			return nil, err
		}
		stderr, err := os.Create(filepath.Join(node.nodeDir, "stderr"))
		if err != nil {
			return nil, err
		}
		node.receptorCmd.Stdout = stdout
		node.receptorCmd.Stderr = stderr
		node.receptorCmd.Start()
	}
	mesh.nodes = nodes
	mesh.MeshDefinition = &MeshDefinition

	failedMesh := make(chan bool, 1)
	time.Sleep(100 * time.Millisecond)
	for _, node := range mesh.nodes {
		go func() {
			node.WaitForShutdown()
			failedMesh <- true
		}()
		select {
		case <-failedMesh:
			mesh.Shutdown()
			mesh.WaitForShutdown()
			return nil, errors.New("Failed to create mesh")
		case <-time.After(time.Until(time.Now().Add(100 * time.Millisecond))):
		}
	}

	return mesh, nil
}

// Shutdown stops all running Netceptors and their backends
func (m *CLIMesh) Shutdown() {
	for _, node := range m.nodes {
		node.Shutdown()
	}
}

// WaitForShutdown Waits for all running Netceptors and their backends to stop
func (m *CLIMesh) WaitForShutdown() {
	for _, node := range m.nodes {
		node.WaitForShutdown()
	}
}

// CheckConnections returns true if the connections defined in our mesh definition are
// consistent with the connections made by the nodes
func (m *CLIMesh) CheckConnections() bool {
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
		for k, connYaml := range m.MeshDefinition.Nodes[status.NodeID].Connections {
			index := connYaml.Index
			configItemYaml, ok := m.MeshDefinition.Nodes[k].Nodedef[index].(map[interface{}]interface{})
			listenerYaml, ok := configItemYaml["tcp-listener"].(map[interface{}]interface{})
			if ok {
				expectedConnections[k] = getListenerCost(listenerYaml, status.NodeID)
				continue
			}
			listenerYaml, ok = configItemYaml["udp-listener"].(map[interface{}]interface{})
			if ok {
				expectedConnections[k] = getListenerCost(listenerYaml, status.NodeID)
				continue
			}
			listenerYaml, ok = configItemYaml["ws-listener"].(map[interface{}]interface{})
			if ok {
				expectedConnections[k] = getListenerCost(listenerYaml, status.NodeID)
				continue
			}
		}
		for nodeID, node := range m.MeshDefinition.Nodes {
			if nodeID == status.NodeID {
				continue
			}
			for k := range node.Connections {
				if k == status.NodeID {
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
func (m *CLIMesh) CheckKnownConnectionCosts() bool {
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
func (m *CLIMesh) CheckRoutes() bool {
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
func (m *CLIMesh) CheckControlSockets() bool {
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
func (m *CLIMesh) WaitForReady(ctx context.Context) error {
	sleepInterval := 100 * time.Millisecond
	if !utils.CheckUntilTimeout(ctx, m.CheckControlSockets, sleepInterval) {
		return errors.New("Timed out while waiting for control sockets")
	}
	if !utils.CheckUntilTimeout(ctx, m.CheckConnections, sleepInterval) {
		return errors.New("Timed out while waiting for Connections")
	}
	if !utils.CheckUntilTimeout(ctx, m.CheckKnownConnectionCosts, sleepInterval) {
		return errors.New("Timed out while checking Connection Costs")
	}
	if !utils.CheckUntilTimeout(ctx, m.CheckRoutes, sleepInterval) {
		return errors.New("Timed out while waiting for routes to converge")
	}
	return nil
}

// Status returns a list of statuses from the contained netceptors
func (m *CLIMesh) Status() ([]*netceptor.Status, error) {
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
