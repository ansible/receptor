package mesh

import (
	"fmt"

	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/workceptor"
	"github.com/ansible/receptor/tests/utils"
)

func flatMesh(proto string) *LibMesh {
	m := NewLibMesh()

	// Controller has no peers, only a listener
	controllerNodeID := "controller"
	controller := m.NewLibNode(controllerNodeID)
	controller.ListenerCfgs = map[listenerName]ListenerCfg{
		listenerName(proto): newListenerCfg(proto, "", 1, nil),
	}

	i := 1
	// All nodes peer out to "controller"
	for i <= 15 {
		nodeID := fmt.Sprintf("node%d", i)
		node := m.NewLibNode(nodeID)
		node.Connections = []Connection{
			{RemoteNode: controller, Protocol: proto},
		}

		i++
	}

	return &m
}

func randomMesh(proto string) *LibMesh {
	m := NewLibMesh()

	// Controller only has a listener
	controller := m.NewLibNode("controller")
	controller.ListenerCfgs = map[listenerName]ListenerCfg{
		listenerName(proto): newListenerCfg(proto, "", 1, nil),
	}

	node1 := m.NewLibNode("node1")
	node2 := m.NewLibNode("node2")
	node3 := m.NewLibNode("node3")
	node4 := m.NewLibNode("node4")
	node5 := m.NewLibNode("node5")
	node6 := m.NewLibNode("node6")
	node7 := m.NewLibNode("node7")
	node8 := m.NewLibNode("node8")
	node9 := m.NewLibNode("node9")
	node10 := m.NewLibNode("node10")
	node11 := m.NewLibNode("node11")
	node12 := m.NewLibNode("node12")

	// node1 connects to controller
	node1.ListenerCfgs = map[listenerName]ListenerCfg{
		listenerName(proto): newListenerCfg(proto, "", 1, nil),
	}
	node1.Connections = []Connection{
		{RemoteNode: controller, Protocol: proto},
	}

	// node2 connects to node1
	node2.ListenerCfgs = map[listenerName]ListenerCfg{
		listenerName(proto): newListenerCfg(proto, "", 1, nil),
	}

	node2.Connections = []Connection{
		{RemoteNode: node1, Protocol: proto},
	}

	// node3 connects to node4 and node6
	node3.ListenerCfgs = map[listenerName]ListenerCfg{
		listenerName(proto): newListenerCfg(proto, "", 1, nil),
	}

	node3.Connections = []Connection{
		{RemoteNode: node4, Protocol: proto},
		{RemoteNode: node6, Protocol: proto},
	}

	// node4 connects to node2 and node7
	node4.ListenerCfgs = map[listenerName]ListenerCfg{
		listenerName(proto): newListenerCfg(proto, "", 1, nil),
	}

	node4.Connections = []Connection{
		{RemoteNode: node2, Protocol: proto},
		{RemoteNode: node7, Protocol: proto},
	}

	// node5 connects to node8 and node12

	node5.ListenerCfgs = map[listenerName]ListenerCfg{
		listenerName(proto): newListenerCfg(proto, "", 1, nil),
	}

	node5.Connections = []Connection{
		{RemoteNode: node8, Protocol: proto},
		{RemoteNode: node12, Protocol: proto},
	}

	// node6 connects to node10
	node6.ListenerCfgs = map[listenerName]ListenerCfg{
		listenerName(proto): newListenerCfg(proto, "", 1, nil),
	}

	node6.Connections = []Connection{
		{RemoteNode: node10, Protocol: proto},
	}

	// node7 connects to node1 and node3
	node7.ListenerCfgs = map[listenerName]ListenerCfg{
		listenerName(proto): newListenerCfg(proto, "", 1, nil),
	}

	node7.Connections = []Connection{
		{RemoteNode: node1, Protocol: proto},
		{RemoteNode: node3, Protocol: proto},
	}

	// node8 connects to node1
	node8.ListenerCfgs = map[listenerName]ListenerCfg{
		listenerName(proto): newListenerCfg(proto, "", 1, nil),
	}

	node8.Connections = []Connection{
		{RemoteNode: node1, Protocol: proto},
	}

	// node9 connects to node5 and node10
	node9.ListenerCfgs = map[listenerName]ListenerCfg{
		listenerName(proto): newListenerCfg(proto, "", 1, nil),
	}

	node9.Connections = []Connection{
		{RemoteNode: node5, Protocol: proto},
		{RemoteNode: node10, Protocol: proto},
	}

	// node10 connects to node4 and node12
	node10.ListenerCfgs = map[listenerName]ListenerCfg{
		listenerName(proto): newListenerCfg(proto, "", 1, nil),
	}

	node10.Connections = []Connection{
		{RemoteNode: node4, Protocol: proto},
		{RemoteNode: node12, Protocol: proto},
	}

	// node11 connects to node1
	node11.ListenerCfgs = map[listenerName]ListenerCfg{
		listenerName(proto): newListenerCfg(proto, "", 1, nil),
	}

	node11.Connections = []Connection{
		{RemoteNode: node1, Protocol: proto},
	}

	// node12 connects to controller and node11
	node12.ListenerCfgs = map[listenerName]ListenerCfg{
		listenerName(proto): newListenerCfg(proto, "", 1, nil),
	}

	node12.Connections = []Connection{
		{RemoteNode: controller, Protocol: proto},
		{RemoteNode: node11, Protocol: proto},
	}

	return &m
}

func treeMesh(proto string) *LibMesh {
	m := NewLibMesh()

	// Controller only has a listener
	controller := m.NewLibNode("controller")
	controller.ListenerCfgs = map[listenerName]ListenerCfg{
		listenerName(proto): newListenerCfg(proto, "", 1, nil),
	}

	// node1 connects to controller
	node1 := m.NewLibNode("node1")
	node1.ListenerCfgs = map[listenerName]ListenerCfg{
		listenerName(proto): newListenerCfg(proto, "", 1, nil),
	}

	node1.Connections = []Connection{
		{RemoteNode: controller, Protocol: proto},
	}

	// nodes2-4 connects to node1
	for _, id := range []int{2, 3, 4} {
		node := m.NewLibNode(fmt.Sprintf("node%d", id))
		node.Connections = []Connection{
			{RemoteNode: node1, Protocol: proto},
		}
	}

	// node5 connects to controller
	node5 := m.NewLibNode("node5")
	node5.ListenerCfgs = map[listenerName]ListenerCfg{
		listenerName(proto): newListenerCfg(proto, "", 1, nil),
	}

	node5.Connections = []Connection{
		{RemoteNode: controller, Protocol: proto},
	}

	// node6-8 connects to node5
	for _, id := range []int{6, 7, 8} {
		node := m.NewLibNode(fmt.Sprintf("node%d", id))
		node.Connections = []Connection{
			{RemoteNode: node5, Protocol: proto},
		}
	}

	// node9 connects to controller
	node9 := m.NewLibNode("node9")
	node9.ListenerCfgs = map[listenerName]ListenerCfg{
		listenerName(proto): newListenerCfg(proto, "", 1, nil),
	}

	node9.Connections = []Connection{
		{RemoteNode: controller, Protocol: proto},
	}

	// node10-12 connects to node9
	for _, id := range []int{10, 11, 12} {
		node := m.NewLibNode(fmt.Sprintf("node%d", id))
		node.Connections = []Connection{
			{RemoteNode: node9, Protocol: proto},
		}
	}

	return &m
}

// used in work_test.go.
func workTestMesh(workPluginName workPlugin) *LibMesh {
	caKey, caCrt, err := utils.GenerateCA("ca", "localhost")
	if err != nil {
		panic(err)
	}

	key1, crt1, err := utils.GenerateCertWithCA("node1", caKey, caCrt, "localhost", []string{"localhost"}, []string{"node1"})
	if err != nil {
		panic(err)
	}

	key2, crt2, err := utils.GenerateCertWithCA("node2", caKey, caCrt, "localhost", []string{"localhost"}, []string{"node2"})
	if err != nil {
		panic(err)
	}

	key3, crt3, err := utils.GenerateCertWithCA("node3", caKey, caCrt, "localhost", []string{"localhost"}, []string{"node3"})
	if err != nil {
		panic(err)
	}

	key4, crt4, err := utils.GenerateCertWithCA("node1wrongCN", caKey, caCrt, "node1wrongCN", nil, []string{"node1wrongCN"})
	if err != nil {
		panic(err)
	}

	m := NewLibMesh()

	// node1 -> node2 <- node3
	node1 := m.NewLibNode("node1")
	node2 := m.NewLibNode("node2")
	node3 := m.NewLibNode("node3")

	// node1 dials out to node2
	node1.Connections = []Connection{
		{RemoteNode: node2, Protocol: "tcp", TLS: "client"},
	}
	node1.ListenerCfgs = map[listenerName]ListenerCfg{
		listenerName("tcp"): newListenerCfg("tcp", "", 1, nil),
	}
	node1.TLSClientConfigs = []*netceptor.TLSClientConfig{
		{
			Name:    "client",
			Key:     key1,
			Cert:    crt1,
			RootCAs: caCrt,
		},
		{
			Name:                   "tlsclientwrongCN",
			Key:                    key4,
			Cert:                   crt4,
			RootCAs:                caCrt,
			SkipReceptorNamesCheck: true,
		},
	}

	// node2 has a listener
	node2.workerConfigs = []workceptor.WorkerConfig{workTestConfigs[workPluginName]["echosleepshort"]}
	node2.ListenerCfgs = map[listenerName]ListenerCfg{
		listenerName("tcp"): newListenerCfg("tcp", "server", 1, nil),
	}
	node2.TLSServerConfigs = []*netceptor.TLSServerConfig{
		{
			Name:              "server",
			Key:               key2,
			Cert:              crt2,
			RequireClientCert: true,
			ClientCAs:         caCrt,
		},
	}
	node2.TLSClientConfigs = []*netceptor.TLSClientConfig{
		{
			Name:    "client",
			Key:     key2,
			Cert:    crt2,
			RootCAs: caCrt,
		},
	}
	node2.controlServerTLS = "server"

	// node3 dials out to node2
	node3.Connections = []Connection{
		{RemoteNode: node2, Protocol: "tcp", TLS: "client"},
	}
	node3.workerConfigs = []workceptor.WorkerConfig{
		workTestConfigs[workPluginName]["echosleepshort"],
		workTestConfigs[workPluginName]["echosleeplong"],
		workTestConfigs[workPluginName]["echosleeplong50"],
	}
	node3.TLSClientConfigs = []*netceptor.TLSClientConfig{
		{
			Name:    "client",
			Key:     key3,
			Cert:    crt3,
			RootCAs: caCrt,
		},
	}

	return &m
}
