package mesh

import (
	"context"

	"github.com/project-receptor/receptor/pkg/netceptor"
)

// Node Defines the interface for nodes made using the CLI, Library, and
// containers
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
// containers
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

// TCRuleYaml represents the rules to apply to the receptor container interface, note this only has
// effect on docker receptor nodes, and podman receptor nodes run by root
type TCRuleYaml struct {
	// Adds delay to the interface
	Delay string
	// Adds jitter to the interface, can only be used if Delay is set
	Jitter string
	// Adds a percentage of loss to the interface
	Loss string
	// Adds a percentage of out of order packets
	Reordering string
	// Adds a percentage of duplicated packets
	Duplication string
	// Corrupts a percentage of packets
	Corrupt string
}

// YamlNode describes how a single node should be represented in yaml
type YamlNode struct {
	Connections map[string]YamlConnection
	TCRules     *TCRuleYaml
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
