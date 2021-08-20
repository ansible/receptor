package mesh

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/tests/functional/lib/receptorcontrol"
	"github.com/ansible/receptor/tests/functional/lib/utils"
	"gopkg.in/yaml.v2"
)

var containerImage string

var (
	containerRunner        string
	containerComposeRunner string
)

func init() {
	containerImage = os.Getenv("CONTAINER_IMAGE")
	if containerImage == "" {
		containerImage = "receptor-tc"
	}

	containerRunner = os.Getenv("CONTAINERCMD")
	if containerRunner == "" {
		containerRunner = "podman"
	}

	containerComposeRunner = containerRunner + "-compose"
}

// ContainerNode holds a Netceptor, this layer of abstraction might be unnecessary and
// go away later.
type ContainerNode struct {
	dir                   string
	yamlConfig            []interface{}
	externalControlSocket string
	containerName         string
	TCRules               *TCRuleYaml
}

// ContainerMesh contains a list of Nodes and the yaml definition that created them.
type ContainerMesh struct {
	nodes          map[string]*ContainerNode
	MeshDefinition *YamlData
	dir            string
}

// NewContainerNode builds a node with the name passed as the argument.
func NewContainerNode(name string) *ContainerNode {
	return &ContainerNode{}
}

// Dir returns the basedir which contains all of the node data.
func (n *ContainerNode) Dir() string {
	return n.dir
}

// Status returns the status of the node using the control socket to query the
// node.
func (n *ContainerNode) Status() (*netceptor.Status, error) {
	controller := receptorcontrol.New()
	err := controller.Connect(n.externalControlSocket)
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
func (n *ContainerNode) ControlSocket() string {
	return n.externalControlSocket
}

// Shutdown kills the receptor process.
func (n *ContainerNode) Shutdown() {
	receptorCmd := exec.Command(containerRunner, "stop", n.containerName)
	receptorCmd.Start()
}

// Start writes the the node config to disk and starts the receptor process.
func (n *ContainerNode) Start() error {
	strData, err := yaml.Marshal(n.yamlConfig)
	if err != nil {
		return err
	}
	nodedefPath := filepath.Join(n.dir, "receptor.conf")
	ioutil.WriteFile(nodedefPath, strData, 0o644)
	Cmd := exec.Command(containerRunner, "start", n.containerName)
	output, err := Cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %s\nCombined Output: %s", Cmd.String(), err.Error(), output)
	}
	if n.TCRules != nil {
		args := []string{"exec", "-u", "0", n.containerName, "tc", "qdisc", "add", "dev", "eth0", "root", "netem"}
		if n.TCRules.Delay != "" {
			fmt.Printf("applying tc rule, delay: %s\n", n.TCRules.Delay)
			args = append(args, "delay")
			args = append(args, n.TCRules.Delay)
			if n.TCRules.Jitter != "" {
				args = append(args, n.TCRules.Jitter)
			}
		}
		if n.TCRules.Loss != "" {
			fmt.Printf("applying tc rule, loss: %s\n", n.TCRules.Loss)
			args = append(args, "loss")
			args = append(args, n.TCRules.Loss)
		}
		if n.TCRules.Reordering != "" {
			fmt.Printf("applying tc rule, reordering: %s\n", n.TCRules.Reordering)
			args = append(args, "reordering")
			args = append(args, n.TCRules.Reordering)
		}
		if n.TCRules.Duplication != "" {
			fmt.Printf("applying tc rule, duplication: %s\n", n.TCRules.Duplication)
			args = append(args, "duplication")
			args = append(args, n.TCRules.Duplication)
		}
		if n.TCRules.Corrupt != "" {
			fmt.Printf("applying tc rule, corrupt: %s\n", n.TCRules.Corrupt)
			args = append(args, "corrupt")
			args = append(args, n.TCRules.Corrupt)
		}
		tcCmd := exec.Command(containerRunner, args...)
		output, err = tcCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s failed: %s\nCombined Output: %s", tcCmd.String(), err.Error(), output)
		}
		// Write the script so we can apply these tc rules when launching the
		// mesh manually
		scriptCmdArgs := append([]string{containerRunner}, args...)
		nodeTCScript := "#!/bin/bash\n" + strings.Join(scriptCmdArgs, " ")
		f, err := os.Create(filepath.Join(n.dir, "apply-tc-rules.sh"))
		if err != nil {
			return err
		}
		err = f.Chmod(0o755)
		if err != nil {
			return err
		}
		_, err = f.WriteString(nodeTCScript)
		if err != nil {
			return err
		}
		err = f.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// Destroy kills the receptor process and puts its ports back into the pool to
// be reallocated once it's shutdown.
func (n *ContainerNode) Destroy() {
	n.Shutdown()
	go func() {
		n.WaitForShutdown()
		Cmd := exec.Command(containerRunner, "rm", n.containerName)
		Cmd.Start()
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
func (n *ContainerNode) WaitForShutdown() {
	Cmd := exec.Command(containerRunner, "wait", n.containerName)
	Cmd.Run()
}

// Dir returns the basedir which contains all of the mesh data.
func (m *ContainerMesh) Dir() string {
	return m.dir
}

// Nodes Returns a list of nodes.
func (m *ContainerMesh) Nodes() map[string]Node {
	nodes := make(map[string]Node)
	for k, v := range m.nodes {
		nodes[k] = v
	}

	return nodes
}

// NewContainerMeshFromFile Takes a filename of a file with a yaml description of a mesh, loads it and
// calls NewMeshFromYaml on it.
func NewContainerMeshFromFile(filename, dirSuffix string) (Mesh, error) {
	yamlDat, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	MeshDefinition := YamlData{}

	err = yaml.Unmarshal(yamlDat, &MeshDefinition)
	if err != nil {
		return nil, err
	}

	return NewContainerMeshFromYaml(MeshDefinition, dirSuffix)
}

// NewContainerMeshFromYaml takes a yaml mesh description and returns a mesh of nodes
// listening and dialing as defined in the yaml.
func NewContainerMeshFromYaml(meshDefinition YamlData, dirSuffix string) (*ContainerMesh, error) {
	containerComposeData := make(map[string]interface{})
	containerComposeData["version"] = "2.4"
	// Contains the description of each node for docker/podman-compose
	containerComposeServices := make(map[string]map[string]interface{})
	mesh := &ContainerMesh{}
	baseDir := utils.TestBaseDir
	if dirSuffix != "" {
		baseDir = filepath.Join(utils.TestBaseDir, dirSuffix)
	}
	err := os.MkdirAll(baseDir, 0o755)
	if err != nil {
		return nil, err
	}
	tempdir, err := ioutil.TempDir(baseDir, "mesh-")
	if err != nil {
		return nil, err
	}
	mesh.dir = tempdir

	// Setup a script that will apply each node's tc rules, this is so it's
	// possible to restart a mesh after tests run and apply the same tc rules
	// the tests used, this is not used from within the tests itself
	meshTCScript := `#!/bin/bash
SCRIPT_PATH="$(dirname $0)"
for d in $SCRIPT_PATH/*/; do
	"$d/apply-tc-rules.sh"
done`

	f, err := os.Create(filepath.Join(mesh.dir, "apply-tc-rules.sh"))
	if err != nil {
		return nil, err
	}
	err = f.Chmod(0o755)
	if err != nil {
		return nil, err
	}
	_, err = f.WriteString(meshTCScript)
	if err != nil {
		return nil, err
	}
	err = f.Close()
	if err != nil {
		return nil, err
	}

	// HERE BE DRAGONS OF THE TYPE SYSTEMS
	nodes := make(map[string]*ContainerNode)

	// We must start listening on all our nodes before we start dialing so
	// there's something to dial into
	for k := range meshDefinition.Nodes {
		node := NewContainerNode(k)
		node.TCRules = meshDefinition.Nodes[k].TCRules
		tempdir, err = ioutil.TempDir(mesh.dir, k+"-")
		if err != nil {
			return nil, err
		}
		node.dir = tempdir
		containerComposeServices[k] = make(map[string]interface{})
		containerComposeServices[k]["image"] = containerImage
		configVolume := fmt.Sprintf("%s:/etc/receptor", node.dir)
		certVolume := fmt.Sprintf("%s:%s", utils.CertBaseDir, utils.CertBaseDir)
		serviceVolumes := []string{configVolume, certVolume}
		containerComposeServices[k]["volumes"] = serviceVolumes

		// We need this so we can apply tc rules from inside the container
		containerComposeServices[k]["cap_add"] = []string{"NET_ADMIN"}

		meshID := filepath.Base(mesh.dir)
		nodeID := filepath.Base(node.dir)
		node.containerName = fmt.Sprintf("%s_%s", meshID, nodeID)
		containerComposeServices[k]["container_name"] = node.containerName

		user, err := user.Current()
		if err != nil {
			panic(err)
		}

		if containerRunner == "docker" {
			containerComposeServices[k]["user"] = fmt.Sprintf("%s:%s", user.Uid, user.Gid)
		}

		// Keep track of if we need to add an attribute for the node id or if
		// it already exists
		needsIDAttr := true
		for attrkey, attr := range meshDefinition.Nodes[k].Nodedef {
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
			meshDefinition.Nodes[k].Nodedef[attrkey] = attrMap
		}
		if needsIDAttr {
			idYaml := make(map[interface{}]interface{})
			nodeYaml := make(map[interface{}]interface{})
			nodeYaml["id"] = k
			externalDataDir := filepath.Join(node.dir, "datadir")
			nodeYaml["datadir"] = "/etc/receptor/datadir"
			os.Mkdir(externalDataDir, 0o755)
			idYaml["node"] = nodeYaml
			meshDefinition.Nodes[k].Nodedef = append(meshDefinition.Nodes[k].Nodedef, idYaml)
		}
		logYaml := make(map[interface{}]interface{})
		levelYaml := make(map[interface{}]interface{})
		levelYaml["level"] = "debug"
		logYaml["log-level"] = levelYaml
		meshDefinition.Nodes[k].Nodedef = append(meshDefinition.Nodes[k].Nodedef, logYaml)
		nodes[k] = node
	}
	for k := range meshDefinition.Nodes {
		for connNode, connYaml := range meshDefinition.Nodes[k].Connections {
			index := connYaml.Index
			TLS := connYaml.TLS
			attr := meshDefinition.Nodes[connNode].Nodedef[index]
			attrMap := attr.(map[interface{}]interface{})
			listener, ok := attrMap["tcp-listener"]
			if ok {
				dialerYaml := make(map[interface{}]interface{})
				listenerMap, ok := listener.(map[interface{}]interface{})
				if !ok {
					return nil, errors.New("listener object is not a map")
				}
				peerYaml := make(map[interface{}]interface{})
				bindaddr, ok := listenerMap["bindaddr"].(string)
				var addr string
				if ok {
					addr = bindaddr + ":" + listenerMap["port"].(string)
				} else {
					addr = connNode + ":" + listenerMap["port"].(string)
				}
				peerYaml["address"] = addr
				peerYaml["cost"] = getListenerCost(listenerMap, k)

				if TLS != "" {
					peerYaml["tls"] = TLS
				}
				dialerYaml["tcp-peer"] = peerYaml
				meshDefinition.Nodes[k].Nodedef = append(meshDefinition.Nodes[k].Nodedef, dialerYaml)
			}
			listener, ok = attrMap["udp-listener"]
			if ok {
				dialerYaml := make(map[interface{}]interface{})
				listenerMap, ok := listener.(map[interface{}]interface{})
				if !ok {
					return nil, errors.New("listener object is not a map")
				}
				peerYaml := make(map[interface{}]interface{})
				bindaddr, ok := listenerMap["bindaddr"].(string)
				var addr string
				if ok {
					addr = bindaddr + ":" + listenerMap["port"].(string)
				} else {
					addr = connNode + ":" + listenerMap["port"].(string)
				}
				peerYaml["address"] = addr
				peerYaml["cost"] = getListenerCost(listenerMap, k)
				dialerYaml["udp-peer"] = peerYaml
				meshDefinition.Nodes[k].Nodedef = append(meshDefinition.Nodes[k].Nodedef, dialerYaml)
			}
			listener, ok = attrMap["ws-listener"]
			if ok {
				dialerYaml := make(map[interface{}]interface{})
				listenerMap, ok := listener.(map[interface{}]interface{})
				if !ok {
					return nil, errors.New("listener object is not a map")
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
					addr = proto + connNode + ":" + listenerMap["port"].(string)
				}
				peerYaml["address"] = addr
				peerYaml["cost"] = getListenerCost(listenerMap, k)
				if TLS != "" {
					peerYaml["tls"] = TLS
				}
				dialerYaml["ws-peer"] = peerYaml
				meshDefinition.Nodes[k].Nodedef = append(meshDefinition.Nodes[k].Nodedef, dialerYaml)
			}
		}
	}

	// Setup the controlsvc and sockets
	for k, node := range nodes {
		tempdir, err := ioutil.TempDir(utils.ControlSocketBaseDir, "")
		if err != nil {
			return nil, err
		}
		node.externalControlSocket = filepath.Join(tempdir, "controlsock")
		controlServiceYaml := make(map[interface{}]interface{})
		tmp := make(map[interface{}]interface{})
		tmp["filename"] = node.externalControlSocket
		controlServiceYaml["control-service"] = tmp
		containerComposeServices[k]["volumes"] = append(containerComposeServices[k]["volumes"].([]string), fmt.Sprintf("%s:%s", filepath.Dir(node.externalControlSocket), filepath.Dir(node.externalControlSocket)))
		meshDefinition.Nodes[k].Nodedef = append(meshDefinition.Nodes[k].Nodedef, controlServiceYaml)
	}
	containerComposeData["services"] = containerComposeServices

	containerComposeDataStr, err := yaml.Marshal(containerComposeData)
	if err != nil {
		return nil, err
	}
	nodedefPath := filepath.Join(mesh.dir, "docker-compose.yaml")
	ioutil.WriteFile(nodedefPath, containerComposeDataStr, 0o644)

	containerCompose := exec.Command(containerComposeRunner, "up", "--no-start")
	// Add COMPOSE_PARALLEL_LIMIT=500 to our environment because of
	// https://github.com/docker/compose/issues/7486
	// It's unlikely we'll make meshes larger than 500 containers on a single
	// node, which is why i chose 500
	containerCompose.Env = append(os.Environ(), "COMPOSE_PARALLEL_LIMIT=500")
	containerCompose.Dir = mesh.dir
	output, err := containerCompose.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s failed: %s\nCombined Output: %s", containerCompose.String(), err.Error(), output)
	}

	for k, node := range nodes {
		node.yamlConfig = meshDefinition.Nodes[k].Nodedef
		err = node.Start()
		if err != nil {
			return nil, err
		}
	}
	mesh.nodes = nodes
	mesh.MeshDefinition = &meshDefinition

	failedMesh := make(chan bool, 1)
	time.Sleep(100 * time.Millisecond)
	for _, node := range mesh.nodes {
		go func() {
			node.WaitForShutdown()
			failedMesh <- true
		}()
		select {
		case <-failedMesh:
			mesh.Destroy()
			mesh.WaitForShutdown()

			return nil, errors.New("failed to create mesh")
		case <-time.After(time.Until(time.Now().Add(100 * time.Millisecond))):
		}
	}

	return mesh, nil
}

// Destroy stops all running Netceptors and their backends and frees all
// relevant resources.
func (m *ContainerMesh) Destroy() {
	for _, node := range m.nodes {
		node.Destroy()
	}
}

// WaitForShutdown Waits for all running Netceptors and their backends to stop.
func (m *ContainerMesh) WaitForShutdown() {
	for _, node := range m.nodes {
		node.WaitForShutdown()
	}
}

// CheckConnections returns true if the connections defined in our mesh definition are
// consistent with the connections made by the nodes.
func (m *ContainerMesh) CheckConnections() bool {
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
		if reflect.DeepEqual(actualConnections, expectedConnections) {
			return true
		}
	}

	return false
}

// CheckKnownConnectionCosts returns true if every node has the same view of the connections in the mesh.
func (m *ContainerMesh) CheckKnownConnectionCosts() bool {
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
func (m *ContainerMesh) CheckRoutes() bool {
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
func (m *ContainerMesh) CheckControlSockets() bool {
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
func (m *ContainerMesh) WaitForReady(ctx context.Context) error {
	sleepInterval := 100 * time.Millisecond
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
func (m *ContainerMesh) Status() ([]*netceptor.Status, error) {
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
