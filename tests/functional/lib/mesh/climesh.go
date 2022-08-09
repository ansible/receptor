package mesh

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/tests/functional/lib/receptorcontrol"
	"github.com/ansible/receptor/tests/functional/lib/utils"
	"gopkg.in/yaml.v2"
)

type Cmd struct {
	*exec.Cmd
	waitLock *sync.Mutex
}

func (c *Cmd) WaitTS() error {
	c.waitLock.Lock()
	defer c.waitLock.Unlock()

	err := c.Wait()

	return err
}

// CLINode holds a Netceptor, this layer of abstraction might be unnecessary and
// go away later.
type CLINode struct {
	receptorCmd   Cmd
	dir           string
	yamlConfig    []interface{}
	controlSocket string
}

// CLIMesh contains a list of Nodes and the yaml definition that created them.
type CLIMesh struct {
	nodes          map[string]*CLINode
	MeshDefinition *YamlData
	dir            string
}

// NewCLINode builds a node with the name passed as the argument.
func NewCLINode(name string) *CLINode {
	return &CLINode{
		controlSocket: "",
	}
}

// Dir returns the basedir which contains all of the node data.
func (n *CLINode) Dir() string {
	return n.dir
}

// Status returns the status of the node using the control socket to query the
// node.
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

// ControlSocket Returns the path to the controlsocket.
func (n *CLINode) ControlSocket() string {
	return n.controlSocket
}

// Shutdown kills the receptor process.
func (n *CLINode) Shutdown() {
	n.receptorCmd.Process.Kill()
}

// Start writes the the node config to disk and starts the receptor process.
func (n *CLINode) Start() error {
	strData, err := yaml.Marshal(n.yamlConfig)
	if err != nil {
		return err
	}
	nodedefPath := filepath.Join(n.dir, "nodedef.yaml")
	os.WriteFile(nodedefPath, strData, 0o644)
	n.receptorCmd = Cmd{exec.Command("receptor", "--config", nodedefPath), &sync.Mutex{}}
	stdout, err := os.OpenFile(filepath.Join(n.dir, "stdout"), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	stderr, err := os.OpenFile(filepath.Join(n.dir, "stderr"), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	n.receptorCmd.Stdout = stdout
	n.receptorCmd.Stderr = stderr
	err = n.receptorCmd.Start()

	return err
}

// Destroy kills the receptor process and puts its ports back into the pool to
// be reallocated once it's shutdown.
func (n *CLINode) Destroy() {
	n.Shutdown()
	go func() {
		n.receptorCmd.WaitTS()
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

// WaitForShutdown Waits for the receptor process to finish.
func (n *CLINode) WaitForShutdown() {
	n.receptorCmd.WaitTS()
}

// Dir returns the basedir which contains all of the mesh data.
func (m *CLIMesh) Dir() string {
	return m.dir
}

// Nodes Returns a list of nodes.
func (m *CLIMesh) Nodes() map[string]Node {
	nodes := make(map[string]Node)
	for k, v := range m.nodes {
		nodes[k] = v
	}

	return nodes
}

func createNodedefConnections(meshDefinition *YamlData, existingMesh *CLIMesh) error {
	for k := range meshDefinition.Nodes {
		for connNode, connYaml := range meshDefinition.Nodes[k].Connections {
			index := connYaml.Index
			TLS := connYaml.TLS
			var attr interface{}
			if existingMesh != nil {
				attr = existingMesh.MeshDefinition.Nodes[connNode].NodedefBase[index]
			} else {
				attr = meshDefinition.Nodes[connNode].NodedefBase[index]
			}
			attrMap, _ := attr.(map[interface{}]interface{})
			listener, ok := attrMap["tcp-listener"]
			if ok {
				dialerYaml := make(map[interface{}]interface{})
				listenerMap, ok := listener.(map[interface{}]interface{})
				if !ok {
					return errors.New("listener object is not a map")
				}
				peerYaml := make(map[interface{}]interface{})
				bindaddr, ok := listenerMap["bindaddr"].(string)
				var addr string
				if ok {
					addr = bindaddr + ":" + listenerMap["port"].(string)
				} else {
					addr = "localhost:" + listenerMap["port"].(string)
				}
				peerYaml["address"] = addr
				peerYaml["cost"] = getListenerCost(listenerMap, k)

				if TLS != "" {
					peerYaml["tls"] = TLS
				}
				dialerYaml["tcp-peer"] = peerYaml
				if existingMesh != nil {
					existingMesh.MeshDefinition.Nodes[k].NodedefConnections = append(existingMesh.MeshDefinition.Nodes[k].NodedefConnections, dialerYaml)
				} else {
					meshDefinition.Nodes[k].NodedefConnections = append(meshDefinition.Nodes[k].NodedefConnections, dialerYaml)
				}
			}
			listener, ok = attrMap["udp-listener"]
			if ok {
				dialerYaml := make(map[interface{}]interface{})
				listenerMap, ok := listener.(map[interface{}]interface{})
				if !ok {
					return errors.New("listener object is not a map")
				}
				peerYaml := make(map[interface{}]interface{})
				bindaddr, ok := listenerMap["bindaddr"].(string)
				var addr string
				if ok {
					addr = bindaddr + ":" + listenerMap["port"].(string)
				} else {
					addr = "localhost:" + listenerMap["port"].(string)
				}
				peerYaml["address"] = addr
				peerYaml["cost"] = getListenerCost(listenerMap, k)
				dialerYaml["udp-peer"] = peerYaml
				if existingMesh != nil {
					existingMesh.MeshDefinition.Nodes[k].NodedefConnections = append(existingMesh.MeshDefinition.Nodes[k].NodedefConnections, dialerYaml)
				} else {
					meshDefinition.Nodes[k].NodedefConnections = append(meshDefinition.Nodes[k].NodedefConnections, dialerYaml)
				}
			}

			listener, ok = attrMap["ws-listener"]
			if ok {
				dialerYaml := make(map[interface{}]interface{})
				listenerMap, ok := listener.(map[interface{}]interface{})
				if !ok {
					return errors.New("listener object is not a map")
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
					addr = proto + "localhost:" + listenerMap["port"].(string)
				}
				peerYaml["address"] = addr
				peerYaml["cost"] = getListenerCost(listenerMap, k)
				if TLS != "" {
					peerYaml["tls"] = TLS
				}
				dialerYaml["ws-peer"] = peerYaml
				if existingMesh != nil {
					existingMesh.MeshDefinition.Nodes[k].NodedefConnections = append(existingMesh.MeshDefinition.Nodes[k].NodedefConnections, dialerYaml)
				} else {
					meshDefinition.Nodes[k].NodedefConnections = append(meshDefinition.Nodes[k].NodedefConnections, dialerYaml)
				}
			}
		}
	}

	return nil
}

// NewCLIMeshFromFile Takes a filename of a file with a yaml description of a mesh, loads it and
// calls NewMeshFromYaml on it.
func NewCLIMeshFromFile(filename, dirSuffix string) (Mesh, error) {
	yamlDat, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	MeshDefinition := YamlData{}

	err = yaml.Unmarshal(yamlDat, &MeshDefinition)
	if err != nil {
		return nil, err
	}

	return NewCLIMeshFromYaml(MeshDefinition, dirSuffix)
}

// NewCLIMeshFromYaml takes a yaml mesh description and returns a mesh of nodes
// listening and dialing as defined in the yaml.
func NewCLIMeshFromYaml(meshDefinition YamlData, dirSuffix string) (*CLIMesh, error) {
	mesh := &CLIMesh{}
	// Setup the mesh directory
	baseDir := utils.TestBaseDir
	if dirSuffix != "" {
		baseDir = filepath.Join(utils.TestBaseDir, dirSuffix)
	}
	err := os.MkdirAll(baseDir, 0o755)
	if err != nil {
		return nil, err
	}
	tempdir, err := os.MkdirTemp(baseDir, "mesh-")
	if err != nil {
		return nil, err
	}
	mesh.dir = tempdir

	// HERE BE DRAGONS OF THE TYPE SYSTEMS
	nodes := make(map[string]*CLINode)

	// We must start listening on all our nodes before we start dialing so
	// there's something to dial into
	for k := range meshDefinition.Nodes {
		node := NewCLINode(k)
		tempdir, err = os.MkdirTemp(mesh.dir, k+"-")
		if err != nil {
			return nil, err
		}
		node.dir = tempdir
		// Keep track of if we need to add an attribute for the node id or if
		// it already exists
		needsIDAttr := true
		for attrkey, attr := range meshDefinition.Nodes[k].NodedefBase {
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
			meshDefinition.Nodes[k].NodedefBase[attrkey] = attrMap
		}
		if needsIDAttr {
			idYaml := make(map[interface{}]interface{})
			nodeYaml := make(map[interface{}]interface{})
			nodeYaml["id"] = k
			nodeYaml["datadir"] = filepath.Join(node.dir, "datadir")
			os.Mkdir(nodeYaml["datadir"].(string), 0o755)
			idYaml["node"] = nodeYaml
			meshDefinition.Nodes[k].NodedefBase = append(meshDefinition.Nodes[k].NodedefBase, idYaml)
		}
		logYaml := make(map[interface{}]interface{})
		levelYaml := make(map[interface{}]interface{})
		levelYaml["level"] = "debug"
		logYaml["log-level"] = levelYaml
		meshDefinition.Nodes[k].NodedefBase = append(meshDefinition.Nodes[k].NodedefBase, logYaml)
		nodes[k] = node
	}

	err = createNodedefConnections(&meshDefinition, nil)
	if err != nil {
		return nil, err
	}

	// Setup the controlsvc and sockets
	for k, node := range nodes {
		needsControlService := true
		controlServiceIndex := 0
		for index, attr := range meshDefinition.Nodes[k].NodedefBase {
			attrMap := attr.(map[interface{}]interface{})
			for k, v := range attrMap {
				k = k.(string)
				if k == "control-service" {
					vMap, _ := v.(map[interface{}]interface{})
					csvName, ok := vMap["service"]
					if ok {
						if csvName == "control" {
							_, ok = vMap["filename"].(string)
							if ok {
								return nil, fmt.Errorf("control-service definition should not specify a filename")
							}
							controlServiceIndex = index
							needsControlService = false
						}
					}
				}
			}
		}
		tempdir, err := os.MkdirTemp(utils.ControlSocketBaseDir, "")
		if err != nil {
			return nil, err
		}
		controlSocket := filepath.Join(tempdir, "controlsock")
		node.controlSocket = controlSocket
		if needsControlService {
			tmp := make(map[interface{}]interface{})
			tmp["filename"] = controlSocket
			controlServiceYaml := make(map[interface{}]interface{})
			controlServiceYaml["control-service"] = tmp
			meshDefinition.Nodes[k].NodedefBase = append(meshDefinition.Nodes[k].NodedefBase, controlServiceYaml)
		} else {
			meshDefinition.Nodes[k].NodedefBase[controlServiceIndex].(map[interface{}]interface{})["control-service"].(map[interface{}]interface{})["filename"] = controlSocket
		}
	}

	// nodedef = NodedefBase
	for k := range meshDefinition.Nodes {
		if meshDefinition.Nodes[k].NodedefBase != nil {
			// trying to avoid = due to pointer issues in []interface{}
			meshDefinition.Nodes[k].Nodedef = append(meshDefinition.Nodes[k].Nodedef, meshDefinition.Nodes[k].NodedefBase...)
		}
	}

	// yamlConfig = Nodedef+NodedefConnections
	for k, node := range nodes {
		node.yamlConfig = meshDefinition.Nodes[k].Nodedef
		if meshDefinition.Nodes[k].NodedefConnections != nil {
			node.yamlConfig = append(node.yamlConfig, meshDefinition.Nodes[k].NodedefConnections...)
		}
		err = node.Start()
		if err != nil {
			return nil, err
		}
	}
	mesh.nodes = nodes
	mesh.MeshDefinition = &meshDefinition

	failedMesh := make(chan *CLINode)
	time.Sleep(100 * time.Millisecond)
	for _, node := range mesh.nodes {
		go func(node *CLINode) {
			node.WaitForShutdown()
			// non-blocking send to failedMesh channel
			select {
			case failedMesh <- node:
			default:
			}
		}(node)
	}
	select {
	case node := <-failedMesh:
		mesh.Destroy()
		mesh.WaitForShutdown()

		return nil, fmt.Errorf("failed to create mesh: node %s exited early", node.dir)
	case <-time.After(time.Until(time.Now().Add(100 * time.Millisecond))):
	}

	return mesh, nil
}

func ModifyCLIMeshFromYaml(meshDefinition YamlData, existingMesh CLIMesh) error {
	// setting the current `yamlConfig` for each node as nil
	for existingNodes := range existingMesh.nodes {
		existingMesh.nodes[existingNodes].yamlConfig = nil
	}

	// clear out `[]Nodedef{}`
	for nodes := range existingMesh.MeshDefinition.Nodes {
		existingMesh.MeshDefinition.Nodes[nodes].Nodedef = nil
	}

	// setting `[]NodedefConnections{}` to nil in existing mesh
	for nodes := range existingMesh.MeshDefinition.Nodes {
		existingMesh.MeshDefinition.Nodes[nodes].NodedefConnections = nil
	}

	err := createNodedefConnections(&meshDefinition, &existingMesh)
	if err != nil {
		return err
	}

	// Now we combine NodedefConnections` and `NodedefBase` and assign it to Nodedef
	for k := range existingMesh.MeshDefinition.Nodes {
		if existingMesh.MeshDefinition.Nodes[k].NodedefBase != nil {
			existingMesh.MeshDefinition.Nodes[k].Nodedef = append(existingMesh.MeshDefinition.Nodes[k].Nodedef, existingMesh.MeshDefinition.Nodes[k].NodedefBase...)
		}
		if existingMesh.MeshDefinition.Nodes[k].NodedefConnections != nil {
			existingMesh.MeshDefinition.Nodes[k].Nodedef = append(existingMesh.MeshDefinition.Nodes[k].Nodedef, existingMesh.MeshDefinition.Nodes[k].NodedefConnections...)
		}
	}

	// Now we combine `[]NodedefConnections{}` and `[]Nodedef{}` for each node in the mesh and write to disk
	for k, node := range existingMesh.nodes {
		node.yamlConfig = existingMesh.MeshDefinition.Nodes[k].Nodedef
		strData, err := yaml.Marshal(node.yamlConfig)
		if err != nil {
			return err
		}
		nodedefPath := filepath.Join(node.dir, "nodedef.yaml")
		err = os.WriteFile(nodedefPath, strData, 0o644)
		if err != nil {
			return err
		}
	}

	return nil
}

// Destroy stops all running Netceptors and their backends and frees all
// relevant resources.
func (m *CLIMesh) Destroy() {
	for _, node := range m.nodes {
		node.Destroy()
	}
}

// WaitForShutdown Waits for all running Netceptors and their backends to stop.
func (m *CLIMesh) WaitForShutdown() {
	for _, node := range m.nodes {
		node.WaitForShutdown()
	}
}

// CheckConnections returns true if the connections defined in our mesh definition are
// consistent with the connections made by the nodes.
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
			configItemYaml := m.MeshDefinition.Nodes[k].Nodedef[index].(map[interface{}]interface{})
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
		for nodeID := range m.MeshDefinition.Nodes {
			if nodeID == status.NodeID {
				continue
			}
		}
		if reflect.DeepEqual(actualConnections, expectedConnections) {
			return true
		}
	}

	return false
}

// CheckAdvertisements returns true if the advertisements are recorded in
// a manner consistent with the work-commands defined for the mesh.
func (m *CLIMesh) CheckAdvertisements() bool {
	statusList, err := m.Status()
	if err != nil {
		return false
	}
	for _, status := range statusList {
		actual := map[string][]string{}
		for _, ad := range status.Advertisements {
			if len(ad.WorkCommands) > 0 {
				for _, workCommand := range ad.WorkCommands {
					actual[ad.NodeID] = append(actual[ad.NodeID], workCommand.WorkType)
				}
			}
		}
		expected := map[string][]string{}
		for node := range m.MeshDefinition.Nodes {
			for _, attr := range m.MeshDefinition.Nodes[node].Nodedef {
				attrMap := attr.(map[interface{}]interface{})
				for _, cmd := range []string{"work-command", "work-kubernetes", "work-python"} {
					if v, ok := attrMap[cmd]; ok {
						v, _ := v.(map[interface{}]interface{})
						expected[node] = append(expected[node], v["workType"].(string))
					}
				}
			}
		}
		if reflect.DeepEqual(actual, expected) {
			return true
		}
	}

	return false
}

// CheckKnownConnectionCosts returns true if every node has the same view of the connections in the mesh.
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

// CheckRoutes returns true if every node has a route to every other node.
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
// connections.
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

// WaitForReady Waits for connections and routes to converge.
func (m *CLIMesh) WaitForReady(ctx context.Context) error {
	sleepInterval := 500 * time.Millisecond
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
	if !utils.CheckUntilTimeout(ctx, sleepInterval, m.CheckAdvertisements) {
		return errors.New("timed out while waiting for Advertisements")
	}

	return nil
}

// Status returns a list of statuses from the contained netceptors.
func (m *CLIMesh) Status() ([]*netceptor.Status, error) {
	out := []*netceptor.Status{}
	for _, node := range m.nodes {
		status, err := node.Status()
		if err != nil {
			return nil, err
		}
		out = append(out, status)
	}

	return out, nil
}
