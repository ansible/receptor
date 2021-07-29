package mesh

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/fortytw2/leaktest"
	"github.com/project-receptor/receptor/tests/functional/lib/mesh"
	"github.com/project-receptor/receptor/tests/functional/lib/receptorcontrol"
	"github.com/project-receptor/receptor/tests/functional/lib/utils"
)

func checkSkipKube(t *testing.T) {
	if skip := os.Getenv("SKIP_KUBE"); skip == "1" {
		t.Skip("Kubernetes tests are set to skip, unset SKIP_KUBE to run them")
	}
}

func TestWork(t *testing.T) {
	t.Parallel()
	home := os.Getenv("HOME")
	echoSleepLong := map[interface{}]interface{}{
		"work-command": map[interface{}]interface{}{
			"workType": "echosleeplong",
			"command":  "bash",
			"params":   "-c \"for i in {1..5}; do echo $i; sleep 3;done\"",
		},
	}
	echoSleepShort := map[interface{}]interface{}{
		"work-command": map[interface{}]interface{}{
			"workType": "echosleepshort",
			"command":  "bash",
			"params":   "-c \"for i in {1..5}; do echo $i;done\"",
		},
	}
	kubeEchoSleepLong := map[interface{}]interface{}{
		"work-kubernetes": map[interface{}]interface{}{
			"workType":   "echosleeplong",
			"authmethod": "kubeconfig",
			"namespace":  "default",
			"kubeconfig": filepath.Join(home, ".kube/config"),
			"image":      "alpine",
			"command":    "sh -c \"for i in `seq 1 5`; do echo $i; sleep 3;done\"",
		},
	}
	kubeEchoSleepShort := map[interface{}]interface{}{
		"work-kubernetes": map[interface{}]interface{}{
			"workType":   "echosleepshort",
			"authmethod": "kubeconfig",
			"namespace":  "default",
			"kubeconfig": filepath.Join(home, ".kube/config"),
			"image":      "alpine",
			"command":    "sh -c \"for i in `seq 1 5`; do echo $i;done\"",
		},
	}
	testTable := []struct {
		testGroup    string
		shortCommand map[interface{}]interface{}
		longCommand  map[interface{}]interface{}
	}{
		{"normal_worker", echoSleepShort, echoSleepLong},
		{"kube_worker", kubeEchoSleepShort, kubeEchoSleepLong},
	}
	for _, subtest := range testTable {
		testGroup := subtest.testGroup
		shortCommand := subtest.shortCommand
		longCommand := subtest.longCommand
		// Setup our mesh yaml data
		workSetup := func(testName string) (map[string]*receptorcontrol.ReceptorControl, *mesh.CLIMesh, []byte) {
			data := mesh.YamlData{}
			data.Nodes = make(map[string]*mesh.YamlNode)
			expectedResults := []byte("1\n2\n3\n4\n5\n")
			// Setup certs
			caKey, caCrt, err := utils.GenerateCA("ca", "localhost")
			if err != nil {
				t.Fatal(err)
			}
			key1, crt1, err := utils.GenerateCertWithCA("node1", caKey, caCrt, "node1", nil, []string{"node1"})
			if err != nil {
				t.Fatal(err)
			}
			key2, crt2, err := utils.GenerateCertWithCA("node2", caKey, caCrt, "node2", nil, []string{"node2"})
			if err != nil {
				t.Fatal(err)
			}
			key3, crt3, err := utils.GenerateCertWithCA("node1wrongCN", caKey, caCrt, "node1wrongCN", nil, []string{"node1wrongCN"})
			if err != nil {
				t.Fatal(err)
			}
			// Generate a mesh with 3 nodes
			data.Nodes["node2"] = &mesh.YamlNode{
				Connections: map[string]mesh.YamlConnection{},
				Nodedef: []interface{}{
					map[interface{}]interface{}{
						"tcp-listener": map[interface{}]interface{}{
							"cost": 1.0,
							"nodecost": map[interface{}]interface{}{
								"node1": 1.0,
								"node3": 1.0,
							},
						},
					},
					map[interface{}]interface{}{
						"tls-server": map[interface{}]interface{}{
							"name":              "control_tls",
							"cert":              crt2,
							"key":               key2,
							"requireclientcert": true,
							"clientcas":         caCrt,
						},
					},
					map[interface{}]interface{}{
						"control-service": map[interface{}]interface{}{
							"service": "control",
							"tls":     "control_tls",
						},
					},
					shortCommand,
				},
			}
			data.Nodes["node1"] = &mesh.YamlNode{
				Connections: map[string]mesh.YamlConnection{
					"node2": {
						Index: 0,
					},
				},
				Nodedef: []interface{}{
					map[interface{}]interface{}{
						"tls-client": map[interface{}]interface{}{
							"name":               "tlsclient",
							"rootcas":            caCrt,
							"insecureskipverify": false,
							"cert":               crt1,
							"key":                key1,
						},
					},
					map[interface{}]interface{}{
						"tls-client": map[interface{}]interface{}{
							"name":               "tlsclientwrongCN",
							"rootcas":            caCrt,
							"insecureskipverify": false,
							"cert":               crt3,
							"key":                key3,
						},
					},
				},
			}
			data.Nodes["node3"] = &mesh.YamlNode{
				Connections: map[string]mesh.YamlConnection{
					"node2": {
						Index: 0,
					},
				},
				Nodedef: []interface{}{
					longCommand,
					shortCommand,
				},
			}

			m, err := mesh.NewCLIMeshFromYaml(data, testName)
			if err != nil {
				t.Fatal(err)
			}

			ctx, _ := context.WithTimeout(context.Background(), 60*time.Second)
			err = m.WaitForReady(ctx)
			if err != nil {
				t.Fatal(err)
			}

			nodes := m.Nodes()
			controllers := make(map[string]*receptorcontrol.ReceptorControl)
			for k := range nodes {
				controller := receptorcontrol.New()
				err = controller.Connect(nodes[k].ControlSocket())
				if err != nil {
					t.Fatal(err)
				}
				controllers[k] = controller
			}
			return controllers, m, expectedResults
		}

		tearDown := func(controllers map[string]*receptorcontrol.ReceptorControl, m *mesh.CLIMesh) {
			defer m.WaitForShutdown()
			defer m.Destroy()
			defer func() {
				for _, controller := range controllers {
					controller.Close()
				}
			}()
		}

		assertFilesReleased := func(ctx context.Context, nodeDir, nodeID, unitID string) error {
			workPath := filepath.Join(nodeDir, "datadir", nodeID, unitID)
			check := func() bool {
				_, err := os.Stat(workPath)
				return os.IsNotExist(err)
			}
			if !utils.CheckUntilTimeout(ctx, 3000*time.Millisecond, check) {
				return fmt.Errorf("unitID %s on %s did not release", unitID, nodeID)
			}
			return nil
		}

		assertStdoutFizeSize := func(ctx context.Context, nodeDir, nodeID, unitID string, waitUntilSize int) error {
			stdoutFilename := filepath.Join(nodeDir, "datadir", nodeID, unitID, "stdout")
			check := func() bool {
				_, err := os.Stat(stdoutFilename)
				if os.IsNotExist(err) {
					return false
				}
				fstat, _ := os.Stat(stdoutFilename)
				return int(fstat.Size()) >= waitUntilSize
			}
			if !utils.CheckUntilTimeout(ctx, 3000*time.Millisecond, check) {
				return fmt.Errorf("file size not correct for %s", stdoutFilename)
			}
			return nil
		}

		t.Run(testGroup+"/work submit with tlsclient", func(t *testing.T) {
			// tests work submit via json
			// tests connecting to remote control service with tlsclient
			// tests that having a ttl that never times out (10 hours) works fine
			t.Parallel()
			if strings.Contains(t.Name(), "kube") {
				checkSkipKube(t)
			}
			controllers, m, _ := workSetup(t.Name())
			defer tearDown(controllers, m)

			command := `{"command":"work","subcommand":"submit","worktype":"echosleepshort","tlsclient":"tlsclient","node":"node2","params":"", "ttl":"10h"}`
			unitID, err := controllers["node1"].WorkSubmitJSON(command)
			if err != nil {
				t.Fatal(err)
			}
			ctx, _ := context.WithTimeout(context.Background(), 20*time.Second)
			err = controllers["node1"].AssertWorkSucceeded(ctx, unitID)
			if err != nil {
				t.Fatal(err)
			}
		})

		t.Run(testGroup+"/work submit with incorrect tlsclient CN", func(t *testing.T) {
			// tests that submitting work with wrong cert CN immediately fails the job
			// also tests that releasing a job that has not been started on remote
			// will not attempt to connect to remote
			t.Parallel()
			if strings.Contains(t.Name(), "kube") {
				checkSkipKube(t)
			}
			controllers, m, _ := workSetup(t.Name())
			defer tearDown(controllers, m)
			nodes := m.Nodes()

			command := `{"command":"work","subcommand":"submit","worktype":"echosleepshort","tlsclient":"tlsclientwrongCN","node":"node2","params":""}`
			unitID, err := controllers["node1"].WorkSubmitJSON(command)
			if err != nil {
				t.Fatal(err)
			}
			ctx, _ := context.WithTimeout(context.Background(), 20*time.Second)
			err = controllers["node1"].AssertWorkFailed(ctx, unitID)
			if err != nil {
				t.Fatal(err)
			}
			_, err = controllers["node1"].WorkRelease(unitID)
			if err != nil {
				t.Fatal(err)
			}
			ctx, _ = context.WithTimeout(context.Background(), 10*time.Second)
			err = controllers["node1"].AssertWorkReleased(ctx, unitID)
			if err != nil {
				t.Fatal(err)
			}
			ctx, _ = context.WithTimeout(context.Background(), 20*time.Second)
			err = assertFilesReleased(ctx, nodes["node1"].Dir(), "node1", unitID)
			if err != nil {
				t.Fatal(err)
			}
		})

		t.Run(testGroup+"/start remote work with ttl", func(t *testing.T) {
			t.Parallel()
			if strings.Contains(t.Name(), "kube") {
				checkSkipKube(t)
			}
			controllers, m, _ := workSetup(t.Name())
			defer tearDown(controllers, m)
			nodes := m.Nodes()

			nodes["node2"].Shutdown()
			nodes["node2"].WaitForShutdown()

			command := `{"command":"work","subcommand":"submit","worktype":"echosleepshort","tlsclient":"tlsclient","node":"node2","params":"","ttl":"5s"}`
			unitID, err := controllers["node1"].WorkSubmitJSON(command)
			if err != nil {
				t.Fatal(err)
			}
			ctx, _ := context.WithTimeout(context.Background(), 20*time.Second)
			err = controllers["node1"].AssertWorkTimedOut(ctx, unitID)
			if err != nil {
				t.Fatal(err)
			}
			_, err = controllers["node1"].WorkRelease(unitID)
			if err != nil {
				t.Fatal(err)
			}
			ctx, _ = context.WithTimeout(context.Background(), 10*time.Second)
			err = controllers["node1"].AssertWorkReleased(ctx, unitID)
			if err != nil {
				t.Fatal(err)
			}
			ctx, _ = context.WithTimeout(context.Background(), 20*time.Second)
			err = assertFilesReleased(ctx, nodes["node1"].Dir(), "node1", unitID)
			if err != nil {
				t.Fatal(err)
			}
		})

		t.Run(testGroup+"/cancel then release remote work", func(t *testing.T) {
			// also tests that release still works after control service restarts
			t.Parallel()
			if strings.Contains(t.Name(), "kube") {
				checkSkipKube(t)
			}
			controllers, m, _ := workSetup(t.Name())
			defer tearDown(controllers, m)
			nodes := m.Nodes()

			unitID, err := controllers["node1"].WorkSubmit("node3", "echosleeplong")
			if err != nil {
				t.Fatal(err)
			}
			ctx, _ := context.WithTimeout(context.Background(), 20*time.Second)
			err = controllers["node1"].AssertWorkRunning(ctx, unitID)
			if err != nil {
				t.Fatal(err)
			}
			_, err = controllers["node1"].WorkCancel(unitID)
			if err != nil {
				t.Fatal(err)
			}
			ctx, _ = context.WithTimeout(context.Background(), 10*time.Second)
			err = controllers["node1"].AssertWorkCancelled(ctx, unitID)
			if err != nil {
				t.Fatal(err)
			}
			workStatus, err := controllers["node1"].GetWorkStatus(unitID)
			if err != nil {
				t.Fatal(err)
			}
			remoteUnitID, ok := workStatus.ExtraData.(map[string]interface{})["RemoteUnitID"].(string)
			if !ok {
				panic("value is not the correct type")
			}
			if remoteUnitID == "" {
				t.Errorf("remoteUnitID should not be empty")
			}
			nodes["node1"].Shutdown()
			nodes["node1"].WaitForShutdown()
			err = nodes["node1"].Start()
			if err != nil {
				t.Fatal(err)
			}
			ctx, _ = context.WithTimeout(context.Background(), 10*time.Second)
			err = m.WaitForReady(ctx)
			if err != nil {
				t.Fatal(err)
			}
			err = controllers["node1"].Close()
			if err != nil {
				t.Fatal(err)
			}
			err = controllers["node1"].Reconnect()
			if err != nil {
				t.Fatal(err)
			}
			_, err = controllers["node1"].WorkRelease(unitID)
			if err != nil {
				t.Fatal(err)
			}
			ctx, _ = context.WithTimeout(context.Background(), 30*time.Second)
			err = controllers["node1"].AssertWorkReleased(ctx, unitID)
			if err != nil {
				t.Fatal(err)
			}
			ctx, _ = context.WithTimeout(context.Background(), 10*time.Second)
			err = assertFilesReleased(ctx, nodes["node1"].Dir(), "node1", unitID)
			if err != nil {
				t.Fatal(err)
			}
			ctx, _ = context.WithTimeout(context.Background(), 10*time.Second)
			err = assertFilesReleased(ctx, nodes["node3"].Dir(), "node3", remoteUnitID)
			if err != nil {
				t.Fatal(err)
			}
		})

		t.Run(testGroup+"/work submit while remote node is down", func(t *testing.T) {
			t.Parallel()
			if strings.Contains(t.Name(), "kube") {
				checkSkipKube(t)
			}
			controllers, m, _ := workSetup(t.Name())
			defer tearDown(controllers, m)
			nodes := m.Nodes()

			nodes["node3"].Shutdown()
			nodes["node3"].WaitForShutdown()
			unitID, err := controllers["node1"].WorkSubmit("node3", "echosleepshort")
			if err != nil {
				t.Fatal(err)
			}
			ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
			err = controllers["node1"].AssertWorkPending(ctx, unitID)
			if err != nil {
				t.Fatal(err)
			}
			err = nodes["node3"].Start()
			if err != nil {
				t.Fatal(err)
			}
			// Wait for node3 to join the mesh again
			ctx, _ = context.WithTimeout(context.Background(), 60*time.Second)
			err = m.WaitForReady(ctx)
			if err != nil {
				t.Fatal(err)
			}
			ctx, _ = context.WithTimeout(context.Background(), 60*time.Second)
			err = controllers["node1"].AssertWorkSucceeded(ctx, unitID)
			if err != nil {
				t.Fatal(err)
			}
		})

		t.Run(testGroup+"/work streaming resumes when relay node restarts", func(t *testing.T) {
			t.Parallel()
			if strings.Contains(t.Name(), "kube") {
				checkSkipKube(t)
			}
			controllers, m, expectedResults := workSetup(t.Name())
			defer tearDown(controllers, m)
			nodes := m.Nodes()

			unitID, err := controllers["node1"].WorkSubmit("node3", "echosleeplong")
			if err != nil {
				t.Fatal(err)
			}
			ctx, _ := context.WithTimeout(context.Background(), 20*time.Second)
			err = controllers["node1"].AssertWorkRunning(ctx, unitID)
			if err != nil {
				t.Fatal(err)
			}
			ctx, _ = context.WithTimeout(context.Background(), 20*time.Second)
			err = assertStdoutFizeSize(ctx, nodes["node1"].Dir(), "node1", unitID, 1)
			if err != nil {
				t.Fatal(err)
			}
			err = controllers["node1"].AssertWorkResults(unitID, expectedResults[:1])
			if err != nil {
				t.Fatal(err)
			}
			nodes["node2"].Shutdown()
			nodes["node2"].WaitForShutdown()
			if err := nodes["node2"].Start(); err != nil {
				t.Fatal(err)
			}
			// Wait for node2 to join the mesh again
			ctx, _ = context.WithTimeout(context.Background(), 60*time.Second)
			err = m.WaitForReady(ctx)
			if err != nil {
				t.Fatal(err)
			}
			ctx, _ = context.WithTimeout(context.Background(), 30*time.Second)
			err = controllers["node1"].AssertWorkSucceeded(ctx, unitID)
			if err != nil {
				t.Fatal(err)
			}
			ctx, _ = context.WithTimeout(context.Background(), 30*time.Second)
			err = assertStdoutFizeSize(ctx, nodes["node1"].Dir(), "node1", unitID, 10)
			if err != nil {
				t.Fatal(err)
			}
			err = controllers["node1"].AssertWorkResults(unitID, expectedResults)
			if err != nil {
				t.Fatal(err)
			}
		})
		t.Run(testGroup+"/results on restarted node", func(t *testing.T) {
			t.Parallel()
			if strings.Contains(t.Name(), "kube") {
				checkSkipKube(t)
			}
			controllers, m, expectedResults := workSetup(t.Name())
			defer tearDown(controllers, m)
			nodes := m.Nodes()

			unitID, err := controllers["node1"].WorkSubmit("node3", "echosleeplong")
			if err != nil {
				t.Fatal(err)
			}
			ctx, _ := context.WithTimeout(context.Background(), 20*time.Second)
			err = controllers["node1"].AssertWorkRunning(ctx, unitID)
			if err != nil {
				t.Fatal(err)
			}
			nodes["node3"].Shutdown()
			nodes["node3"].WaitForShutdown()
			err = nodes["node3"].Start()
			if err != nil {
				t.Fatal(err)
			}
			// Wait for node3 to join the mesh again
			ctx, _ = context.WithTimeout(context.Background(), 60*time.Second)
			err = m.WaitForReady(ctx)
			if err != nil {
				t.Fatal(err)
			}
			ctx, _ = context.WithTimeout(context.Background(), 60*time.Second)
			err = controllers["node1"].AssertWorkSucceeded(ctx, unitID)
			if err != nil {
				t.Fatal(err)
			}
			err = controllers["node1"].AssertWorkResults(unitID, expectedResults)
			if err != nil {
				t.Fatal(err)
			}
		})
		t.Run(testGroup+"/work submit and release to non-existent node", func(t *testing.T) {
			t.Parallel()
			if strings.Contains(t.Name(), "kube") {
				checkSkipKube(t)
			}
			controllers, m, _ := workSetup(t.Name())
			defer tearDown(controllers, m)
			nodes := m.Nodes()

			// submit work from node1 to non-existent-node
			// node999 was never initialised
			unitID, err := controllers["node1"].WorkSubmit("node999", "echosleeplong")
			if err != nil {
				t.Fatal(err)
			}

			// wait for 10 seconds, and check if the work is in pending state
			ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
			err = controllers["node1"].AssertWorkPending(ctx, unitID)
			if err != nil {
				t.Fatal(err)
			}

			nodes["node1"].Shutdown()
			nodes["node1"].WaitForShutdown()
			err = nodes["node1"].Start()
			if err != nil {
				t.Fatal(err)
			}
			ctx, _ = context.WithTimeout(context.Background(), 10*time.Second)
			err = m.WaitForReady(ctx)
			if err != nil {
				t.Fatal(err)
			}
			err = controllers["node1"].Close()
			if err != nil {
				t.Fatal(err)
			}
			err = controllers["node1"].Reconnect()
			if err != nil {
				t.Fatal(err)
			}

			// release the work on node1
			_, err = controllers["node1"].WorkRelease(unitID)
			if err != nil {
				t.Fatal(err)
			}
			ctx, _ = context.WithTimeout(context.Background(), 15*time.Second)
			err = controllers["node1"].AssertWorkReleased(ctx, unitID)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRuntimeParams(t *testing.T) {
	echoCommand := map[interface{}]interface{}{
		"workType":           "echo",
		"command":            "echo",
		"params":             "",
		"allowruntimeparams": true,
	}

	data := mesh.YamlData{}
	data.Nodes = make(map[string]*mesh.YamlNode)
	data.Nodes["node0"] = &mesh.YamlNode{
		Connections: map[string]mesh.YamlConnection{},
		Nodedef: []interface{}{
			map[interface{}]interface{}{
				"tcp-listener": map[interface{}]interface{}{},
			},
			map[interface{}]interface{}{
				"work-command": echoCommand,
			},
		},
	}

	m, err := mesh.NewCLIMeshFromYaml(data, t.Name())
	if err != nil {
		t.Fatal(err)
	}

	ctx, _ := context.WithTimeout(context.Background(), 60*time.Second)
	err = m.WaitForReady(ctx)
	if err != nil {
		t.Fatal(err)
	}
	nodes := m.Nodes()
	controller := receptorcontrol.New()
	err = controller.Connect(nodes["node0"].ControlSocket())
	if err != nil {
		t.Fatal(err)
	}
	command := `{"command":"work","subcommand":"submit","worktype":"echo","node":"node0","params":"it worked!"}`
	unitID, err := controller.WorkSubmitJSON(command)
	if err != nil {
		t.Fatal(err)
	}
	err = controller.AssertWorkSucceeded(ctx, unitID)
	if err != nil {
		t.Fatal(err)
	}

	err = controller.AssertWorkResults(unitID, []byte("it worked!"))
	if err != nil {
		t.Fatal(err)
	}
}

func TestKubeRuntimeParams(t *testing.T) {
	checkSkipKube(t)
	home := os.Getenv("HOME")
	configfilename := filepath.Join(home, ".kube/config")
	reader, err := os.Open(configfilename)
	if err != nil {
		t.Fatal(err)
	}
	buf, err := ioutil.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	kubeconfig := string(buf)
	kubeconfig = strings.ReplaceAll(kubeconfig, "\n", "\\n")
	data := mesh.YamlData{}
	data.Nodes = make(map[string]*mesh.YamlNode)
	data.Nodes["node0"] = &mesh.YamlNode{
		Connections: map[string]mesh.YamlConnection{},
		Nodedef: []interface{}{
			map[interface{}]interface{}{
				"tcp-listener": map[interface{}]interface{}{},
			},
			map[interface{}]interface{}{
				"work-kubernetes": map[interface{}]interface{}{
					"workType":         "echo",
					"authmethod":       "runtime",
					"namespace":        "default",
					"allowruntimepod":  true,
					"allowruntimeauth": true,
				},
			},
		},
	}
	m, err := mesh.NewCLIMeshFromYaml(data, t.Name())
	if err != nil {
		t.Fatal(err)
	}
	ctx, _ := context.WithTimeout(context.Background(), 60*time.Second)
	err = m.WaitForReady(ctx)
	if err != nil {
		t.Fatal(err)
	}
	nodes := m.Nodes()
	controller := receptorcontrol.New()
	err = controller.Connect(nodes["node0"].ControlSocket())
	if err != nil {
		t.Fatal(err)
	}
	command := fmt.Sprintf(`{"command": "work", "subcommand": "submit", "node": "localhost", "worktype": "echo", "secret_kube_pod": "---\napiVersion: v1\nkind: Pod\nspec:\n  containers:\n  - name: worker\n    image: centos:8\n    command:\n    - bash\n    args:\n    - \"-c\"\n    - for i in {1..5}; do echo $i;done\n", "secret_kube_config": "%s"}`, kubeconfig)
	unitID, err := controller.WorkSubmitJSON(command)
	if err != nil {
		t.Fatal(err)
	}
	err = controller.AssertWorkSucceeded(ctx, unitID)
	if err != nil {
		t.Fatal(err)
	}
	err = controller.AssertWorkResults(unitID, []byte("1\n2\n3\n4\n5\n"))
	if err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeParamsNotAllowed(t *testing.T) {
	echoCommand := map[interface{}]interface{}{
		"workType":           "echo",
		"command":            "echo",
		"params":             "",
		"allowruntimeparams": false,
	}

	data := mesh.YamlData{}
	data.Nodes = make(map[string]*mesh.YamlNode)
	data.Nodes["node0"] = &mesh.YamlNode{
		Connections: map[string]mesh.YamlConnection{},
		Nodedef: []interface{}{
			map[interface{}]interface{}{
				"tcp-listener": map[interface{}]interface{}{},
			},
			map[interface{}]interface{}{
				"work-command": echoCommand,
			},
		},
	}

	m, err := mesh.NewCLIMeshFromYaml(data, t.Name())
	if err != nil {
		t.Fatal(err)
	}

	ctx, _ := context.WithTimeout(context.Background(), 60*time.Second)
	err = m.WaitForReady(ctx)
	if err != nil {
		t.Fatal(err)
	}
	nodes := m.Nodes()
	controller := receptorcontrol.New()
	err = controller.Connect(nodes["node0"].ControlSocket())
	if err != nil {
		t.Fatal(err)
	}
	command := `{"command":"work","subcommand":"submit","worktype":"echo","node":"node0","params":"it worked!"}`
	_, err = controller.WorkSubmitJSON(command)
	if err == nil {
		t.Fatal("Expected work submit to fail but it succeeded")
	}
}

func TestKubeContainerFailure(t *testing.T) {
	checkSkipKube(t)
	home := os.Getenv("HOME")
	command := map[interface{}]interface{}{
		"work-kubernetes": map[interface{}]interface{}{
			"workType":   "kubejob",
			"authmethod": "kubeconfig",
			"namespace":  "default",
			"kubeconfig": filepath.Join(home, ".kube/config"),
			"image":      "alpine",
			"command":    "thiscommandwontexist",
		},
	}

	data := mesh.YamlData{}
	data.Nodes = make(map[string]*mesh.YamlNode)
	data.Nodes["node0"] = &mesh.YamlNode{
		Connections: map[string]mesh.YamlConnection{},
		Nodedef: []interface{}{
			map[interface{}]interface{}{
				"tcp-listener": map[interface{}]interface{}{},
			},
			command,
		},
	}
	m, err := mesh.NewCLIMeshFromYaml(data, t.Name())
	if err != nil {
		t.Fatal(err)
	}

	ctx, _ := context.WithTimeout(context.Background(), 60*time.Second)
	err = m.WaitForReady(ctx)
	if err != nil {
		t.Fatal(err)
	}
	nodes := m.Nodes()
	controller := receptorcontrol.New()
	err = controller.Connect(nodes["node0"].ControlSocket())
	if err != nil {
		t.Fatal(err)
	}
	job := `{"command":"work","subcommand":"submit","worktype":"kubejob","node":"node0"}`
	unitID, err := controller.WorkSubmitJSON(job)
	if err != nil {
		t.Fatal(err)
	}
	ctx, _ = context.WithTimeout(context.Background(), 20*time.Second)
	err = controller.AssertWorkFailed(ctx, unitID)
	if err != nil {
		t.Fatal("Expected work to fail but it succeeded")
	}
}
