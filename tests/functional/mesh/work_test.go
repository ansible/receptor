package mesh

import (
	"context"
	_ "github.com/fortytw2/leaktest"
	"github.com/project-receptor/receptor/tests/functional/lib/mesh"
	"github.com/project-receptor/receptor/tests/functional/lib/receptorcontrol"
	"github.com/project-receptor/receptor/tests/functional/lib/utils"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWork(t *testing.T) {
	t.Parallel()
	// Setup our mesh yaml data
	workSetup := func(testName string) (*receptorcontrol.ReceptorControl, *mesh.CLIMesh, []byte) {
		data := mesh.YamlData{}
		data.Nodes = make(map[string]*mesh.YamlNode)
		echoSleepLong := map[interface{}]interface{}{
			"workType": "echosleeplong",
			"command":  "bash",
			"params":   "-c \"for i in {1..5}; do echo $i; sleep 3;done\"",
		}
		echoSleepShort := map[interface{}]interface{}{
			"workType": "echosleepshort",
			"command":  "bash",
			"params":   "-c \"for i in {1..5}; do echo $i;done\"",
		}
		expectedResults := []byte("1\n2\n3\n4\n5\n")
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
			},
		}
		data.Nodes["node1"] = &mesh.YamlNode{
			Connections: map[string]mesh.YamlConnection{
				"node2": mesh.YamlConnection{
					Index: 0,
				},
			},
			Nodedef: []interface{}{},
		}
		data.Nodes["node3"] = &mesh.YamlNode{
			Connections: map[string]mesh.YamlConnection{
				"node2": mesh.YamlConnection{
					Index: 0,
				},
			},
			Nodedef: []interface{}{
				map[interface{}]interface{}{
					"work-command": echoSleepLong,
				},
				map[interface{}]interface{}{
					"work-command": echoSleepShort,
				},
			},
		}

		m, err := mesh.NewCLIMeshFromYaml(data, filepath.Join(mesh.TestBaseDir, testName))
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
		err = controller.Connect(nodes["node1"].ControlSocket())
		if err != nil {
			t.Fatal(err)
		}
		return controller, m, expectedResults
	}

	tearDown := func(controller *receptorcontrol.ReceptorControl, m *mesh.CLIMesh) {
		defer m.WaitForShutdown()
		defer m.Destroy()
		defer controller.Close()
	}

	assertFilesReleased := func(nodeDir, nodeID, unitID string) {
		workPath := filepath.Join(nodeDir, "datadir", nodeID, unitID)
		_, err := os.Stat(workPath)
		if !os.IsNotExist(err) {
			t.Errorf("unitID %s on %s did not release", unitID, nodeID)
		}
	}

	assertStdoutFizeSize := func(ctx context.Context, nodeDir, nodeID, unitID string, waitUntilSize int) {
		stdoutFilename := filepath.Join(nodeDir, "datadir", nodeID, unitID, "stdout")
		check := func() bool {
			_, err := os.Stat(stdoutFilename)
			if os.IsNotExist(err) {
				return false
			}
			fstat, _ := os.Stat(stdoutFilename)
			if int(fstat.Size()) >= waitUntilSize {
				return true
			}
			return false
		}
		if !utils.CheckUntilTimeout(ctx, 3000*time.Millisecond, check) {
			t.Errorf("file size not correct for %s", stdoutFilename)
		}
	}

	t.Run("cancel then release remote work", func(t *testing.T) {
		t.Parallel()
		controller, m, _ := workSetup(t.Name())
		defer tearDown(controller, m)
		nodes := m.Nodes()

		unitID, err := controller.WorkSubmit("node3", "echosleeplong")
		if err != nil {
			t.Fatal(err)
		}
		ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
		err = controller.AssertWorkRunning(ctx, unitID)
		if err != nil {
			t.Fatal(err)
		}
		_, err = controller.WorkCancel(unitID)
		if err != nil {
			t.Fatal(err)
		}
		ctx, _ = context.WithTimeout(context.Background(), 10*time.Second)
		err = controller.AssertWorkCancelled(ctx, unitID)
		if err != nil {
			t.Fatal(err)
		}
		workStatus, err := controller.GetWorkStatus(unitID)
		if err != nil {
			t.Fatal(err)
		}
		remoteUnitID := workStatus.ExtraData.(map[string]interface{})["RemoteUnitID"].(string)
		if remoteUnitID == "" {
			t.Errorf("remoteUnitID should not be empty")
		}

		_, err = controller.WorkRelease(unitID)
		if err != nil {
			t.Fatal(err)
		}
		ctx, _ = context.WithTimeout(context.Background(), 10*time.Second)
		err = controller.AssertWorkReleased(ctx, unitID)
		if err != nil {
			t.Fatal(err)
		}
		assertFilesReleased(nodes["node1"].Dir(), "node1", unitID)
		assertFilesReleased(nodes["node3"].Dir(), "node3", remoteUnitID)
	})

	t.Run("work submit while remote node is down", func(t *testing.T) {
		t.Parallel()
		controller, m, _ := workSetup(t.Name())
		defer tearDown(controller, m)
		nodes := m.Nodes()

		nodes["node3"].Shutdown()
		nodes["node3"].WaitForShutdown()
		unitID, err := controller.WorkSubmit("node3", "echosleepshort")
		if err != nil {
			t.Fatal(err)
		}
		ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
		err = controller.AssertWorkPending(ctx, unitID)
		err = nodes["node3"].Start()
		if err != nil {
			t.Fatal(err)
		}
		// Wait for node3 to join the mesh again
		ctx, _ = context.WithTimeout(context.Background(), 60*time.Second)
		err = m.WaitForReady(ctx)

		ctx, _ = context.WithTimeout(context.Background(), 30*time.Second)
		err = controller.AssertWorkSucceeded(ctx, unitID)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("work streaming resumes when relay node restarts", func(t *testing.T) {
		t.Parallel()
		controller, m, expectedResults := workSetup(t.Name())
		defer tearDown(controller, m)
		nodes := m.Nodes()

		unitID, err := controller.WorkSubmit("node3", "echosleeplong")
		if err != nil {
			t.Fatal(err)
		}
		ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
		err = controller.AssertWorkRunning(ctx, unitID)
		if err != nil {
			t.Fatal(err)
		}
		ctx, _ = context.WithTimeout(context.Background(), 20*time.Second)
		assertStdoutFizeSize(ctx, nodes["node1"].Dir(), "node1", unitID, 1)
		err = controller.AssertWorkResults(unitID, expectedResults[:1])
		if err != nil {
			t.Fatal(err)
		}
		nodes["node2"].Shutdown()
		nodes["node2"].WaitForShutdown()
		nodes["node2"].Start()
		// Wait for node2 to join the mesh again
		ctx, _ = context.WithTimeout(context.Background(), 60*time.Second)
		err = m.WaitForReady(ctx)

		ctx, _ = context.WithTimeout(context.Background(), 30*time.Second)
		err = controller.AssertWorkSucceeded(ctx, unitID)
		if err != nil {
			t.Fatal(err)
		}
		ctx, _ = context.WithTimeout(context.Background(), 30*time.Second)
		assertStdoutFizeSize(ctx, nodes["node1"].Dir(), "node1", unitID, 10)
		err = controller.AssertWorkResults(unitID, expectedResults)
		if err != nil {
			t.Fatal(err)
		}
	})
}
