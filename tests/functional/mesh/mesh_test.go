package mesh

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/backends"
	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/tests/utils"
	_ "github.com/fortytw2/leaktest"
)

// Test that a mesh starts and that connections are what we expect and that
// each node's view of the mesh converges.
func TestMeshStartup(t *testing.T) {
	meshDefinitions := map[string]*LibMesh{
		"tcp": flatMesh("tcp"),
		"udp": flatMesh("udp"),
		"ws":  flatMesh("ws"),
	}

	t.Parallel()
	for protocol, m := range meshDefinitions {
		m := m
		protocol := protocol

		testName := protocol + "/" + m.Name
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			err := m.Start(t.Name())

			defer func() {
				m.Destroy()
				m.WaitForShutdown()
				t.Log(m.LogWriter.String())
			}()

			if err != nil {
				t.Fatal(err)
			}

			ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel1()

			err = m.WaitForReady(ctx1)
			if err != nil {
				t.Fatal(err)
			}

			// Test that each Node can ping each Node
			for _, node := range m.GetNodes() {
				controller := NewReceptorControl()
				err = controller.Connect(node.GetControlSocket())
				if err != nil {
					t.Fatalf("Error connecting to controller: %s", err)
				}
				for _, remoteNode := range m.GetNodes() {
				retryloop:
					for i := 30; i > 0; i-- {
						_, err := controller.Ping(remoteNode.GetID())
						switch {
						case err == nil:

							break retryloop
						case i != 1:
							t.Logf("Error pinging %s: %s. Retrying", remoteNode.GetID(), err)

							continue
						default:
							t.Fatalf("Error pinging %s: %s", remoteNode.GetID(), err)
						}
					}
				}
			}
		})
	}
}

// Test that traceroute works.
func TestTraceroute(t *testing.T) {
	meshDefinitions := map[string]*LibMesh{
		"tcp": treeMesh("tcp"),
		"udp": treeMesh("udp"),
		"ws":  treeMesh("ws"),
	}

	t.Parallel()
	for protocol, m := range meshDefinitions {
		m := m
		protocol := protocol

		testName := protocol + "/" + m.Name
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			defer func() {
				t.Log(m.LogWriter.String())
			}()
			defer m.WaitForShutdown()
			defer m.Destroy()

			err := m.Start(t.Name())
			if err != nil {
				t.Fatal(err)
			}

			ctx1, cancel1 := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel1()

			err = m.WaitForReady(ctx1)
			if err != nil {
				t.Fatal(err)
			}

			controlNode := m.GetNodes()["controller"]
			controller := NewReceptorControl()
			err = controller.Connect(controlNode.GetControlSocket())
			if err != nil {
				t.Fatal(err)
			}
			_, err = controller.WriteStr("traceroute node7\n")
			if err != nil {
				t.Fatal(err)
			}
			jsonData, err := controller.ReadAndParseJSON()
			if err != nil {
				t.Fatal(err)
			}
			err = controller.Close()
			if err != nil {
				t.Fatal(err)
			}
			for key := range jsonData {
				value := jsonData[key]
				valMap, ok := value.(map[string]interface{})
				if !ok {
					t.Fatal("traceroute returned invalid result")
				}
				_, ok = valMap["Error"]
				if ok {
					t.Fatalf("traceroute returned error: %s", valMap["Error"])
				}
			}
			expectedHops := []struct {
				key  string
				from string
			}{
				{"0", "controller"},
				{"1", "node5"},
				{"2", "node7"},
			}
			if len(jsonData) != len(expectedHops) {
				t.Fatal("traceroute has wrong number of hops")
			}
			for i := range expectedHops {
				eh := expectedHops[i]
				var fromStr string
				result, ok := jsonData[eh.key]
				if ok {
					var resultMap map[string]interface{}
					resultMap, ok = result.(map[string]interface{})
					if ok {
						var fromIf interface{}
						fromIf, ok = resultMap["From"]
						if ok {
							fromStr, ok = fromIf.(string)
						}
					}
				}
				if !ok {
					t.Fatalf("hop %s not in result data or not in expected format", eh.key)
				}
				if fromStr != eh.from {
					t.Fatalf("hop %s should be %s but is actually %s", eh.key, eh.from, fromStr)
				}
			}
		})
	}
}

// Test that a mesh starts and that connections are what we expect.
//
//nolint:tparallel
func TestMeshShutdown(t *testing.T) {
	// !!!!!!!!!!
	// This test is intentionally set to not run in parallel with the other tests
	// since it is checking to see that all ports are appropriately released.
	// !!!!!!!!!!

	meshDefinitions := map[string]*LibMesh{
		"tcp": randomMesh("tcp"),
		"udp": randomMesh("udp"),
		"ws":  randomMesh("ws"),
	}
	for protocol, m := range meshDefinitions {
		m := m
		protocol := protocol

		testName := protocol + "/" + m.Name
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			defer func() {
				t.Log(m.LogWriter.String())
			}()

			err := m.Start(t.Name())
			if err != nil {
				t.Fatal(err)
			}

			ctx1, cancel1 := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel1()
			err = m.WaitForReady(ctx1)

			if err != nil {
				t.Fatal(err)
			}

			m.Destroy()
			m.WaitForShutdown()

			var lsofProto string
			switch protocol {
			case "tcp":
				lsofProto = "TCP"
			case "ws":
				lsofProto = "TCP"
			case "udp":
				lsofProto = "UDP"
			}

			// Check that the connections are closed
			pid := os.Getpid()
			done := false
			var out bytes.Buffer
			for timeout := 10 * time.Second; timeout > 0 && !done; {
				out = bytes.Buffer{}
				cmd := exec.Command("lsof", "-tap", fmt.Sprint(pid), "-i", lsofProto)
				cmd.Stdout = &out
				cmd.Run()
				if !strings.Contains(out.String(), fmt.Sprint(pid)) {
					done = true

					break
				}
				time.Sleep(100 * time.Millisecond)
				timeout -= 100 * time.Millisecond
			}
			if done == false {
				t.Errorf("Timed out while waiting for backends to close:%s\n", out.String())
			}
		})
	}
}

func TestCosts(t *testing.T) {
	t.Parallel()
	m := NewLibMesh()

	defer func() {
		t.Log(m.LogWriter.String())
	}()

	node1 := m.NewLibNode("node1")
	node1.ListenerCfgs = map[listenerName]ListenerCfg{
		"tcp": &backends.TCPListenerCfg{
			BindAddr: "127.0.0.1:0",
			Cost:     4.5,
			NodeCost: map[string]float64{"node2": 2.6, "node3": 3.2},
		},
	}

	for _, i := range []int{2, 3, 4} {
		nodeID := fmt.Sprintf("node%d", i)
		node := m.NewLibNode(nodeID)
		node.Connections = []Connection{
			{RemoteNode: node1, Protocol: "tcp"},
		}
	}

	err := m.Start(t.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer m.WaitForShutdown()
	defer m.Destroy()

	ctx1, cancel1 := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel1()

	err = m.WaitForReady(ctx1)
	if err != nil {
		t.Fatal(err)
	}
	// Test that each Node can ping each Node
	for _, nodeSender := range m.GetNodes() {
		controller := NewReceptorControl()
		err = controller.Connect(nodeSender.GetControlSocket())
		if err != nil {
			t.Fatal(err)
		}
		for nodeIDResponder := range m.GetNodes() {
			response, err := controller.Ping(nodeIDResponder)
			if err != nil {
				t.Error(err)
			} else {
				t.Logf("%v", response)
			}
		}
		controller.Close()
	}
}

func TestDuplicateNodes(t *testing.T) {
	t.Parallel()
	m := NewLibMesh()

	defer func() {
		t.Log(m.LogWriter.String())
	}()

	node1 := m.NewLibNode("node1")
	node1.ListenerCfgs = map[listenerName]ListenerCfg{
		"tcp": &backends.TCPListenerCfg{
			BindAddr: "127.0.0.1:0",
			Cost:     4.5,
			NodeCost: map[string]float64{"node2": 2.6, "node3": 3.2},
		},
	}

	node2 := m.NewLibNode("node2")
	node2.Connections = []Connection{
		{RemoteNode: node1, Protocol: "tcp"},
	}

	node3 := m.NewLibNode("node3")
	node3.netceptorInstance = netceptor.New(context.Background(), "node2")
	node3.Connections = []Connection{
		{RemoteNode: node1, Protocol: "tcp"},
	}
	// Hack a duplicate node onto the mesh
	delete(m.nodes, "node3")
	m.nodes["node2-dupe"] = node3

	err := m.Start(t.Name())
	defer m.WaitForShutdown()
	defer m.Destroy()

	if err != nil {
		t.Fatal(err)
	}

	ctx1, cancel1 := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel1()

	sleepInterval := 100 * time.Millisecond
	if !utils.CheckUntilTimeout(ctx1, sleepInterval, func() bool {
		return strings.Contains(m.LogWriter.String(), "connected using a node ID we are already connected to")
	}) {
		t.Fatal("duplicate nodes were not expected to exist together")
	}

	time.Sleep(5 * time.Second)
}
