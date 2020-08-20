package mesh

import (
	"bytes"
	_ "github.com/fortytw2/leaktest"
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
			mesh, err := NewCLIMeshFromFile(filename)
			if err != nil {
				t.Fatal(err)
			}
			defer mesh.Shutdown()
			defer mesh.WaitForShutdown()
			t.Logf("waiting for mesh")
			err = mesh.WaitForReady(60000)
			if err != nil {
				t.Fatal(err)
			}
			// Test that each Node can ping each Node
			for _, nodeSender := range mesh.Nodes() {
				controller := receptorcontrol.New()
				t.Logf("connecting to %s", nodeSender.ControlSocket())
				err = controller.Connect(nodeSender.ControlSocket())
				if err != nil {
					t.Fatal(err)
				}
				for nodeIDResponder := range mesh.Nodes() {
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
			mesh, err := NewCLIMeshFromFile(filename)
			if err != nil {
				t.Fatal(err)
			}
			defer mesh.Shutdown()
			defer mesh.WaitForShutdown()
			yamlDat, err := ioutil.ReadFile(filename)
			if err != nil {
				t.Fatal(err)
			}

			data := YamlData{}

			err = yaml.Unmarshal(yamlDat, &data)
			if err != nil {
				t.Fatal(err)
			}
			connectionsReady := false
			for timeout := 10000; timeout > 0 && !connectionsReady; connectionsReady = mesh.CheckConnections() {
				time.Sleep(100 * time.Millisecond)
				timeout -= 100
			}
			if connectionsReady == false {
				t.Error("Timed out while waiting for connections:")
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
			mesh, err := NewLibMeshFromFile(filename)
			if err != nil {
				t.Fatal(err)
			}
			err = mesh.WaitForReady(60000)
			if err != nil {
				t.Fatal(err)
			}
			mesh.Shutdown()
			mesh.WaitForShutdown()

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

func TestTCPSSLConnections(t *testing.T) {
	t.Parallel()
	// Setup our mesh yaml data
	data := YamlData{}
	data.Nodes = make(map[string]*YamlNode)

	// Generate a mesh where each node n is connected to only n+1 and n-1
	// if they exist
	data.Nodes["node1"] = &YamlNode{
		Connections: map[string]int{},
		Nodedef: []interface{}{
			map[interface{}]interface{}{
				"tls-server": map[interface{}]interface{}{
					"name": "cert1",
					"key":  "certs/private1.key",
					"cert": "certs/public1.crt",
				},
			},
			map[interface{}]interface{}{
				"tcp-listener": map[interface{}]interface{}{
					"tls": "cert1",
				},
			},
		},
	}
	data.Nodes["node2"] = &YamlNode{
		Connections: map[string]int{
			"node1": 1,
		},
		Nodedef: []interface{}{
			map[interface{}]interface{}{
				"tls-server": map[interface{}]interface{}{
					"name": "cert2",
					"key":  "certs/private2.key",
					"cert": "certs/public2.crt",
				},
			},
			map[interface{}]interface{}{
				"tcp-listener": map[interface{}]interface{}{
					"tls": "cert2",
				},
			},
		},
	}
	data.Nodes["node3"] = &YamlNode{
		Connections: map[string]int{
			"node2": 1,
		},
		Nodedef: []interface{}{
			map[interface{}]interface{}{
				"tcp-listener": map[interface{}]interface{}{},
			},
		},
	}
	mesh, err := NewCLIMeshFromYaml(data)
	if err != nil {
		t.Fatal(err)
	}
	defer mesh.Shutdown()
	defer mesh.WaitForShutdown()

	err = mesh.WaitForReady(60000)
	if err != nil {
		t.Fatal(err)
	}
	// Test that each Node can ping each Node
	for _, nodeSender := range mesh.Nodes() {
		controller := receptorcontrol.New()
		err = controller.Connect(nodeSender.ControlSocket())
		if err != nil {
			t.Fatal(err)
		}
		for nodeIDResponder := range mesh.Nodes() {
			response, err := controller.Ping(nodeIDResponder)
			if err != nil {
				t.Error(err)
			}
			t.Logf("%v", response)
		}
		controller.Close()
	}

}

func benchmarkLinearMeshStartup(totalNodes int, b *testing.B) {
	for i := 0; i < b.N; i++ {
		// Setup our mesh yaml data
		b.StopTimer()
		data := YamlData{}
		data.Nodes = make(map[string]*YamlNode)

		// Generate a mesh where each node n is connected to only n+1 and n-1
		// if they exist
		for i := 0; i < totalNodes; i++ {
			connections := make(map[string]int)
			nodeID := "Node" + strconv.Itoa(i)
			if i > 0 {
				prevNodeID := "Node" + strconv.Itoa(i-1)
				connections[prevNodeID] = 0
			}
			data.Nodes[nodeID] = &YamlNode{
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
		mesh, err := NewLibMeshFromYaml(data)
		if err != nil {
			b.Fatal(err)
		}
		err = mesh.WaitForReady(60000)
		if err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		mesh.Shutdown()
		mesh.WaitForShutdown()
		b.StartTimer()
	}
}

func BenchmarkLinearMeshStartup100(b *testing.B) {
	benchmarkLinearMeshStartup(100, b)
}

func BenchmarkLinearMeshStartup10(b *testing.B) {
	benchmarkLinearMeshStartup(10, b)
}
