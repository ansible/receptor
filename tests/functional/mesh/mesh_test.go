package mesh

import (
	"bytes"
	"context"
	"fmt"
	_ "github.com/fortytw2/leaktest"
	"github.com/project-receptor/receptor/tests/functional/lib/mesh"
	"github.com/project-receptor/receptor/tests/functional/lib/receptorcontrol"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Test that a mesh starts and that connections are what we expect and that
// each node's view of the mesh converges
func TestMeshStartup(t *testing.T) {
	testTable := []struct {
		filename string
	}{
		{"mesh-definitions/flat-mesh-tcp.yaml"},
		{"mesh-definitions/random-mesh-tcp.yaml"},
		{"mesh-definitions/tree-mesh-tcp.yaml"},
		{"mesh-definitions/flat-mesh-udp.yaml"},
		{"mesh-definitions/random-mesh-udp.yaml"},
		{"mesh-definitions/tree-mesh-udp.yaml"},
		{"mesh-definitions/flat-mesh-ws.yaml"},
		{"mesh-definitions/random-mesh-ws.yaml"},
		{"mesh-definitions/tree-mesh-ws.yaml"},
	}
	t.Parallel()
	for _, data := range testTable {
		filename := data.filename
		t.Run(filename, func(t *testing.T) {
			t.Parallel()
			t.Logf("starting mesh")
			m, err := mesh.NewCLIMeshFromFile(filename, t.Name())
			if err != nil {
				t.Fatal(err)
			}
			defer m.WaitForShutdown()
			defer m.Destroy()
			t.Logf("waiting for mesh")
			ctx, _ := context.WithTimeout(context.Background(), 60*time.Second)
			err = m.WaitForReady(ctx)
			if err != nil {
				t.Fatal(err)
			}
			t.Fatal("Testing a failed test")
			// Test that each Node can ping each Node
			for _, nodeSender := range m.Nodes() {
				controller := receptorcontrol.New()
				t.Logf("connecting to %s", nodeSender.ControlSocket())
				err = controller.Connect(nodeSender.ControlSocket())
				if err != nil {
					t.Fatal(err)
				}
				for nodeIDResponder := range m.Nodes() {
					t.Logf("pinging %s", nodeIDResponder)
					response, err := controller.Ping(nodeIDResponder)
					if err != nil {
						t.Error(err)
					}
					t.Logf("%v", response)
				}
				controller.Close()
			}
		})
	}
}

// Test that a mesh starts and that connections are what we expect
func TestMeshConnections(t *testing.T) {
	testTable := []struct {
		filename string
	}{
		{"mesh-definitions/flat-mesh-tcp.yaml"},
		{"mesh-definitions/random-mesh-tcp.yaml"},
		{"mesh-definitions/tree-mesh-tcp.yaml"},
		{"mesh-definitions/flat-mesh-udp.yaml"},
		{"mesh-definitions/random-mesh-udp.yaml"},
		{"mesh-definitions/tree-mesh-udp.yaml"},
		{"mesh-definitions/flat-mesh-ws.yaml"},
		{"mesh-definitions/random-mesh-ws.yaml"},
		{"mesh-definitions/tree-mesh-ws.yaml"},
	}
	t.Parallel()
	for _, data := range testTable {
		filename := data.filename
		t.Run(filename, func(t *testing.T) {
			t.Parallel()
			m, err := mesh.NewCLIMeshFromFile(filename, t.Name())
			if err != nil {
				t.Fatal(err)
			}
			defer m.WaitForShutdown()
			defer m.Destroy()
			yamlDat, err := ioutil.ReadFile(filename)
			if err != nil {
				t.Fatal(err)
			}

			data := mesh.YamlData{}

			err = yaml.Unmarshal(yamlDat, &data)
			if err != nil {
				t.Fatal(err)
			}
			ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
			for connectionsReady := m.CheckConnections(); !connectionsReady; connectionsReady = m.CheckConnections() {
				time.Sleep(100 * time.Millisecond)
				if ctx.Err() != nil {
					t.Error("Timed out while waiting for connections:")
				}
			}
		})
	}
}

// Test that traceroute works
func TestTraceroute(t *testing.T) {
	testTable := []struct {
		filename string
	}{
		{"mesh-definitions/tree-mesh-tcp.yaml"},
		{"mesh-definitions/tree-mesh-udp.yaml"},
		{"mesh-definitions/tree-mesh-ws.yaml"},
	}
	t.Parallel()
	for _, data := range testTable {
		filename := data.filename
		t.Run(filename, func(t *testing.T) {
			t.Parallel()
			m, err := mesh.NewLibMeshFromFile(filename, t.Name())
			if err != nil {
				t.Fatal(err)
			}
			defer m.WaitForShutdown()
			defer m.Destroy()
			ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
			for connectionsReady := m.CheckConnections(); !connectionsReady; connectionsReady = m.CheckConnections() {
				time.Sleep(100 * time.Millisecond)
				if ctx.Err() != nil {
					t.Fatal("Timed out while waiting for connections:")
				}
			}
			controlNode := m.Nodes()["controller"]
			controller := receptorcontrol.New()
			err = controller.Connect(controlNode.ControlSocket())
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
					t.Fatal(fmt.Sprintf("traceroute returned error: %s", valMap["Error"]))
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
					t.Fatal(fmt.Sprintf("hop %s not in result data or not in expected format", eh.key))
				}
				if fromStr != eh.from {
					t.Fatal(fmt.Sprintf("hop %s should be %s but is actually %s", eh.key, eh.from, fromStr))
				}
			}
		})
	}
}

// Test that a mesh starts and that connections are what we expect
func TestMeshShutdown(t *testing.T) {
	//defer leaktest.Check(t)()
	testTable := []struct {
		filename string
	}{
		{"mesh-definitions/random-mesh-tcp.yaml"},
		{"mesh-definitions/random-mesh-udp.yaml"},
		{"mesh-definitions/random-mesh-ws.yaml"},
	}
	for _, data := range testTable {
		filename := data.filename
		t.Run(filename, func(t *testing.T) {
			m, err := mesh.NewLibMeshFromFile(filename, t.Name())
			if err != nil {
				t.Fatal(err)
			}
			ctx, _ := context.WithTimeout(context.Background(), 60*time.Second)
			err = m.WaitForReady(ctx)
			if err != nil {
				t.Fatal(err)
			}
			m.Destroy()
			m.WaitForShutdown()

			// Check that the connections are closed
			pid := os.Getpid()
			pidString := "pid=" + strconv.Itoa(pid)
			done := false
			var out bytes.Buffer
			for timeout := 10 * time.Second; timeout > 0 && !done; {
				out = bytes.Buffer{}
				cmd := exec.Command("ss", "-tuanp")
				cmd.Stdout = &out
				err := cmd.Run()
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(out.String(), pidString) {
					done = true
					break
				}
				time.Sleep(100 * time.Millisecond)
				timeout -= 100 * time.Millisecond
			}
			if done == false {
				t.Errorf("Timed out while waiting for backends to close:\n%s", out.String())
			}
		})
	}
}

func TestCosts(t *testing.T) {
	t.Parallel()
	// Setup our mesh yaml data
	data := mesh.YamlData{}
	data.Nodes = make(map[string]*mesh.YamlNode)

	// Generate a mesh where each node n is connected to only n+1 and n-1
	// if they exist
	data.Nodes["node1"] = &mesh.YamlNode{
		Connections: map[string]mesh.YamlConnection{},
		Nodedef: []interface{}{
			map[interface{}]interface{}{
				"tcp-listener": map[interface{}]interface{}{
					"cost": 4.5,
					"nodecost": map[interface{}]interface{}{
						"node2": 2.6,
						"node3": 3.2,
					},
				},
			},
		},
	}
	data.Nodes["node2"] = &mesh.YamlNode{
		Connections: map[string]mesh.YamlConnection{
			"node1": mesh.YamlConnection{
				Index: 0,
			},
		},
		Nodedef: []interface{}{},
	}
	data.Nodes["node3"] = &mesh.YamlNode{
		Connections: map[string]mesh.YamlConnection{
			"node1": mesh.YamlConnection{
				Index: 0,
			},
		},
		Nodedef: []interface{}{},
	}
	data.Nodes["node4"] = &mesh.YamlNode{
		Connections: map[string]mesh.YamlConnection{
			"node1": mesh.YamlConnection{
				Index: 0,
			},
		},
		Nodedef: []interface{}{},
	}
	m, err := mesh.NewCLIMeshFromYaml(data, t.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer m.WaitForShutdown()
	defer m.Destroy()

	ctx, _ := context.WithTimeout(context.Background(), 60*time.Second)
	err = m.WaitForReady(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Test that each Node can ping each Node
	for _, nodeSender := range m.Nodes() {
		controller := receptorcontrol.New()
		err = controller.Connect(nodeSender.ControlSocket())
		if err != nil {
			t.Fatal(err)
		}
		for nodeIDResponder := range m.Nodes() {
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
	// Setup our mesh yaml data
	data := mesh.YamlData{}
	data.Nodes = make(map[string]*mesh.YamlNode)

	// Generate a mesh where each node n is connected to only n+1 and n-1
	// if they exist
	data.Nodes["node1"] = &mesh.YamlNode{
		Connections: map[string]mesh.YamlConnection{},
		Nodedef: []interface{}{
			map[interface{}]interface{}{
				"tcp-listener": map[interface{}]interface{}{},
			},
		},
	}
	data.Nodes["node2"] = &mesh.YamlNode{
		Connections: map[string]mesh.YamlConnection{
			"node1": mesh.YamlConnection{
				Index: 0,
			},
		},
		Nodedef: []interface{}{
			map[interface{}]interface{}{
				"tcp-listener": map[interface{}]interface{}{},
			},
		},
	}
	data.Nodes["node1_dup"] = &mesh.YamlNode{
		Connections: map[string]mesh.YamlConnection{
			"node2": mesh.YamlConnection{
				Index: 0,
			},
		},
		Nodedef: []interface{}{
			map[interface{}]interface{}{
				"node": map[interface{}]interface{}{
					"id": "node1",
				},
			},
		},
	}
	m, err := mesh.NewCLIMeshFromYaml(data, t.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer m.WaitForShutdown()
	defer m.Destroy()

	ctx, _ := context.WithTimeout(context.Background(), 30*time.Second)
	err = m.WaitForReady(ctx)
	if err == nil {
		t.Fatal("duplicate nodes were not expected to exist together")
	}
}

func benchmarkLinearMeshStartup(totalNodes int, b *testing.B) {
	for i := 0; i < b.N; i++ {
		// Setup our mesh yaml data
		b.StopTimer()
		data := mesh.YamlData{}
		data.Nodes = make(map[string]*mesh.YamlNode)

		// Generate a mesh where each node n is connected to only n+1 and n-1
		// if they exist
		for i := 0; i < totalNodes; i++ {
			connections := make(map[string]mesh.YamlConnection)
			nodeID := "Node" + strconv.Itoa(i)
			if i > 0 {
				prevNodeID := "Node" + strconv.Itoa(i-1)
				connections[prevNodeID] = mesh.YamlConnection{
					Index: 0,
				}
			}
			data.Nodes[nodeID] = &mesh.YamlNode{
				Connections: connections,
				Nodedef: []interface{}{
					map[interface{}]interface{}{
						"tcp-listener": map[interface{}]interface{}{},
					},
				},
			}
		}
		b.StartTimer()

		// Reset the Timer because building the yaml data for the mesh may have
		// taken a bit
		m, err := mesh.NewCLIMeshFromYaml(data, b.Name())
		if err != nil {
			b.Fatal(err)
		}
		ctx, _ := context.WithTimeout(context.Background(), 60*time.Second)
		err = m.WaitForReady(ctx)
		if err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		m.Destroy()
		m.WaitForShutdown()
		b.StartTimer()
	}
}

func BenchmarkLinearMeshStartup100(b *testing.B) {
	benchmarkLinearMeshStartup(100, b)
}

func BenchmarkLinearMeshStartup10(b *testing.B) {
	benchmarkLinearMeshStartup(10, b)
}
