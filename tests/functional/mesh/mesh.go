package mesh

import (
	"context"
	"github.com/project-receptor/receptor/pkg/netceptor"
	"os"
	"path/filepath"
)

// Node Defines the interface for nodes made using the CLI, Library, and
// eventually Docker
type Node interface {
	Dir() string
	Status() (*netceptor.Status, error)
	ControlSocket() string
	Start() error
	Shutdown()
	Destroy()
	WaitForShutdown()
}

// Mesh Defines the interface for meshes made using the CLI, Library, and
// eventually Docker
type Mesh interface {
	Dir() string
	Nodes() map[string]Node
	Status() ([]*netceptor.Status, error)
	CheckConnections() bool
	CheckKnownConnectionCosts() bool
	CheckRoutes() bool
	WaitForReady(context.Context) error
	Destroy()
	WaitForShutdown()
}

// YamlData is the top level structure that defines how our yaml mesh data should be
// represented
type YamlData struct {
	Nodes map[string]*YamlNode
}

// YamlConnection represents a meta connection object that gets used to
// generate the peer config for receptor
type YamlConnection struct {
	Index int
	TLS   string
}

// YamlNode describes how a single node should be represented in yaml
type YamlNode struct {
	Connections map[string]YamlConnection
	Nodedef     []interface{}
}

func getListenerCost(listenerYaml map[interface{}]interface{}, nodeID string) float64 {
	var cost float64
	nodecostYaml, ok := listenerYaml["nodecost"].(map[interface{}]interface{})
	if ok {
		cost, ok = nodecostYaml[nodeID].(float64)
		if !ok {
			cost, ok = listenerYaml["cost"].(float64)
			if !ok {
				cost = 1.0
			}
		}
	} else {
		cost = 1.0
	}
	return cost
}

// TestBaseDir holds the base directory that all permanent test logs should go in
var TestBaseDir string

// ControlSocketBaseDir holds the base directory for controlsockets, control sockets
// have a limited path length, therefore we cant always put them along side the
// node they are attached to
var ControlSocketBaseDir string

func init() {
	TestBaseDir = filepath.Join(os.TempDir(), "receptor-testing")
	os.Mkdir(TestBaseDir, 0700)
	ControlSocketBaseDir = filepath.Join(TestBaseDir, "controlsockets")
	os.Mkdir(ControlSocketBaseDir, 0700)
}
