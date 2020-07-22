package mesh

import (
	"github.com/project-receptor/receptor/pkg/netceptor"
)

// Node Defines the interface for nodes made using the CLI, Library, and
// eventually Docker
type Node interface {
	Status() (*netceptor.Status, error)
	ControlSocket() string
	Shutdown()
	WaitForShutdown()
}

// Mesh Defines the interface for meshes made using the CLI, Library, and
// eventually Docker
type Mesh interface {
	Nodes() map[string]Node
	Status() ([]*netceptor.Status, error)
	CheckConnections() bool
	CheckKnownConnectionCosts() bool
	CheckRoutes() bool
	WaitForReady(float64) error
	Shutdown()
	WaitForShutdown()
}

// YamlData is the top level structure that defines how our yaml mesh data should be
// represented
type YamlData struct {
	Nodes map[string]*YamlNode
}

// YamlNode describes how a single node should be represented in yaml
type YamlNode struct {
	Connections map[string]int
	Nodedef     []interface{}
}
