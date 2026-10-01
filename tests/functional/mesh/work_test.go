package mesh

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/workceptor"
	"github.com/ansible/receptor/tests/utils"
)

func TestWorkSubmitWithTLSClient(t *testing.T) {
	t.Parallel()

	for _, plugin := range workPlugins {
		plugin := plugin

		t.Run(string(plugin), func(t *testing.T) {
			t.Parallel()
			controllers, m, expectedResults := workSetup(plugin, t)

			defer m.WaitForShutdown()
			defer m.Destroy()

			command := `{"command":"work","subcommand":"submit","worktype":"echosleepshort","tlsclient":"client","node":"node2","params":"", "ttl":"10h"}`
			unitID, err := controllers["node1"].WorkSubmitJSON(command)
			if err != nil {
				t.Fatal(err, m.DataDir)
			}
			ctx1, cancel1 := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel1()

			err = controllers["node1"].AssertWorkSucceeded(ctx1, unitID)
			if err != nil {
				t.Fatal(err, m.DataDir)
			}

			err = controllers["node1"].AssertWorkResults(unitID, expectedResults)
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}
			err = controllers["node2"].AssertWorkResults(unitID, expectedResults)
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}
		})
	}
}

// Tests that submitting work with wrong cert CN immediately fails the job
// also tests that releasing a job that has not been started on remote
// will not attempt to connect to remote.
func TestWorkSubmitWithIncorrectTLSClient(t *testing.T) {
	t.Parallel()

	for _, plugin := range workPlugins {
		plugin := plugin
		t.Run(string(plugin), func(t *testing.T) {
			t.Parallel()
			controllers, m, _ := workSetup(plugin, t)
			nodes := m.GetNodes()

			command := `{"command":"work","subcommand":"submit","worktype":"echosleepshort","tlsclient":"tlsclientwrongCN","node":"node2","params":""}`
			unitID, err := controllers["node1"].WorkSubmitJSON(command)
			if err != nil {
				t.Fatal(err)
			}

			ctx1, cancel1 := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel1()

			err = controllers["node1"].AssertWorkFailed(ctx1, unitID)
			if err != nil {
				t.Fatal(err)
			}

			_, err = controllers["node1"].WorkRelease(unitID)
			if err != nil {
				t.Fatal(err)
			}

			ctx2, cancel2 := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel2()

			err = controllers["node1"].AssertWorkReleased(ctx2, unitID)
			if err != nil {
				t.Fatal(err)
			}

			ctx3, cancel3 := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel3()

			err = assertFilesReleased(ctx3, nodes["node1"].GetDataDir(), "node1", unitID)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestStartRemoteWorkWithTTL(t *testing.T) {
	t.Parallel()

	for _, plugin := range workPlugins {
		plugin := plugin

		t.Run(string(plugin), func(t *testing.T) {
			t.Parallel()
			controllers, m, _ := workSetup(plugin, t)

			defer func() {
				t.Log(m.LogWriter.String())
			}()

			nodes := m.GetNodes()

			nodes["node2"].Shutdown()

			command := `{"command":"work","subcommand":"submit","worktype":"echosleepshort","tlsclient":"client","node":"node2","params":"","ttl":"5s"}`
			unitID, err := controllers["node1"].WorkSubmitJSON(command)
			if err != nil {
				t.Fatal(err)
			}
			ctx1, cancel1 := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel1()

			err = controllers["node1"].AssertWorkTimedOut(ctx1, unitID)
			if err != nil {
				t.Fatal(err)
			}
			_, err = controllers["node1"].WorkRelease(unitID)
			if err != nil {
				t.Fatal(err)
			}
			ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel2()

			err = controllers["node1"].AssertWorkReleased(ctx2, unitID)
			if err != nil {
				t.Fatal(err)
			}

			ctx3, cancel3 := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel3()
			err = assertFilesReleased(ctx3, nodes["node1"].GetDataDir(), "node1", unitID)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCancelThenReleaseRemoteWork(t *testing.T) {
	t.Parallel()

	for _, plugin := range workPlugins {
		plugin := plugin

		t.Run(string(plugin), func(t *testing.T) {
			t.Parallel()
			controllers, m, _ := workSetup(plugin, t)

			defer func() {
				t.Log(m.LogWriter.String())
			}()

			nodes := m.GetNodes()

			unitID, err := controllers["node1"].WorkSubmit("node3", "echosleeplong")
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}
			ctx1, cancel1 := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel1()

			err = controllers["node1"].AssertWorkRunning(ctx1, unitID)
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}
			_, err = controllers["node1"].WorkCancel(unitID)
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}

			ctx2, cancel2 := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel2()

			err = controllers["node1"].AssertWorkCancelled(ctx2, unitID)
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}
			workStatus, err := controllers["node1"].GetWorkStatus(unitID)
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}
			remoteUnitID := workStatus.ExtraData.(map[string]interface{})["RemoteUnitID"].(string)
			if remoteUnitID == "" {
				t.Errorf("remoteUnitID should not be empty")
			}
			nodes["node1"].Shutdown()
			err = nodes["node1"].Start()
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}

			ctx3, cancel3 := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel3()
			err = m.WaitForReady(ctx3)
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}
			err = controllers["node1"].Close()
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}
			err = controllers["node1"].Reconnect()
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}
			_, err = controllers["node1"].WorkRelease(unitID)
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}
			ctx4, cancel4 := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel4()

			err = controllers["node1"].AssertWorkReleased(ctx4, unitID)
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}

			ctx5, cancel5 := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel5()

			err = assertFilesReleased(ctx5, nodes["node1"].GetDataDir(), "node1", unitID)
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}

			ctx6, cancel6 := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel6()

			err = assertFilesReleased(ctx6, nodes["node3"].GetDataDir(), "node3", remoteUnitID)
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}
		})
	}
}

func TestWorkSubmitWhileRemoteNodeIsDown(t *testing.T) {
	t.Parallel()

	for _, plugin := range workPlugins {
		plugin := plugin

		t.Run(string(plugin), func(t *testing.T) {
			t.Parallel()
			controllers, m, expectedResults := workSetup(plugin, t)
			nodes := m.GetNodes()

			nodes["node3"].Shutdown()
			unitID, err := controllers["node1"].WorkSubmit("node3", "echosleepshort")
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}

			ctx1, cancel1 := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel1()

			err = controllers["node1"].AssertWorkPending(ctx1, unitID)
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}

			err = nodes["node3"].Start()
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}

			// Wait for node3 to join the mesh again
			ctx2, cancel2 := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel2()

			err = m.WaitForReady(ctx2)
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}

			ctx3, cancel3 := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel3()

			err = controllers["node1"].AssertWorkSucceeded(ctx3, unitID)
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}

			err = controllers["node1"].AssertWorkResults(unitID, expectedResults)
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}
		})
	}
}

func TestWorkStreamingResumesWhenRelayNodeRestarts(t *testing.T) {
	t.Parallel()

	for _, plugin := range workPlugins {
		plugin := plugin

		t.Run(string(plugin), func(t *testing.T) {
			t.Parallel()
			controllers, m, expectedResults := workSetup(plugin, t)
			nodes := m.GetNodes()

			unitID, err := controllers["node1"].WorkSubmit("node3", "echosleeplong")
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}
			ctx1, cancel1 := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel1()

			err = controllers["node1"].AssertWorkRunning(ctx1, unitID)
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}
			ctx2, cancel2 := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel2()

			err = assertStdoutFizeSize(ctx2, nodes["node1"].GetDataDir(), "node1", unitID, 1)
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}
			err = controllers["node1"].AssertWorkResults(unitID, expectedResults[:1])
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}
			nodes["node2"].Shutdown()
			nodes["node2"].Start()
			// Wait for node2 to join the mesh again
			ctx3, cancel3 := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel3()

			err = m.WaitForReady(ctx3)
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}

			ctx4, cancel4 := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel4()

			err = controllers["node1"].AssertWorkSucceeded(ctx4, unitID)
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}
			ctx5, cancel5 := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel5()

			err = assertStdoutFizeSize(ctx5, nodes["node1"].GetDataDir(), "node1", unitID, 10)
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}
			err = controllers["node1"].AssertWorkResults(unitID, expectedResults)
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}
		})
	}
}

func TestResultsOnRestartedNode(t *testing.T) {
	t.Parallel()

	for _, plugin := range workPlugins {
		plugin := plugin

		t.Run(string(plugin), func(t *testing.T) {
			t.Parallel()
			controllers, m, expectedResults := workSetup(plugin, t)
			nodes := m.GetNodes()

			unitID, err := controllers["node1"].WorkSubmit("node3", "echosleeplong")
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}
			ctx1, cancel1 := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel1()
			err = controllers["node1"].AssertWorkRunning(ctx1, unitID)
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}

			nodes["node3"].Shutdown()
			err = nodes["node3"].Start()
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}
			// Wait for node3 to join the mesh again
			ctx2, cancel2 := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel2()

			err = m.WaitForReady(ctx2)
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}

			ctx3, cancel3 := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel3()

			err = controllers["node1"].AssertWorkSucceeded(ctx3, unitID)
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}
			err = controllers["node1"].AssertWorkResults(unitID, expectedResults)
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}
		})
	}
}

func TestWorkSubmitAndReleaseToNonexistentNode(t *testing.T) {
	t.Parallel()

	for _, plugin := range workPlugins {
		plugin := plugin

		t.Run(string(plugin), func(t *testing.T) {
			t.Parallel()
			controllers, m, _ := workSetup(plugin, t)
			nodes := m.GetNodes()

			// submit work from node1 to non-existent-node
			// node999 was never initialised
			unitID, err := controllers["node1"].WorkSubmit("node999", "echosleeplong")
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}

			// wait for 10 seconds, and check if the work is in pending state
			ctx1, cancel1 := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel1()

			err = controllers["node1"].AssertWorkPending(ctx1, unitID)
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}

			nodes["node1"].Shutdown()
			err = nodes["node1"].Start()
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}

			ctx2, cancel2 := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel2()

			err = m.WaitForReady(ctx2)
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}
			err = controllers["node1"].Close()
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}
			err = controllers["node1"].Reconnect()
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}

			// release the work on node1
			_, err = controllers["node1"].WorkRelease(unitID)
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}

			ctx3, cancel3 := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel3()

			err = controllers["node1"].AssertWorkReleased(ctx3, unitID)
			if err != nil {
				t.Fatal(err, m.GetDataDir())
			}
		})
	}
}

func TestRuntimeParams(t *testing.T) {
	m := NewLibMesh()
	node1 := m.NewLibNode("node1")
	node1.workerConfigs = []workceptor.WorkerConfig{
		workceptor.CommandWorkerCfg{
			WorkType:           "echo",
			Command:            "echo",
			AllowRuntimeParams: true,
		},
	}

	err := m.Start(t.Name())
	if err != nil {
		t.Fatal(err, m.GetDataDir())
	}

	ctx1, cancel1 := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel1()
	err = m.WaitForReady(ctx1)
	if err != nil {
		t.Fatal(err, m.GetDataDir())
	}

	nodes := m.GetNodes()
	controllers := make(map[string]*ReceptorControl)
	controllers["node1"] = NewReceptorControl()
	err = controllers["node1"].Connect(nodes["node1"].GetControlSocket())
	if err != nil {
		t.Fatal(err, m.GetDataDir())
	}

	command := `{"command":"work","subcommand":"submit","worktype":"echo","node":"localhost","params":"it worked!"}`
	unitID, err := controllers["node1"].WorkSubmitJSON(command)
	if err != nil {
		t.Fatal(err, m.GetDataDir())
	}

	err = controllers["node1"].AssertWorkSucceeded(ctx1, unitID)
	if err != nil {
		t.Fatal(err, m.GetDataDir())
	}

	err = controllers["node1"].AssertWorkResults(unitID, []byte("it worked!"))
	if err != nil {
		t.Fatal(err, m.GetDataDir())
	}
}

func TestKubeRuntimeParams(t *testing.T) {
	checkSkipKube(t)

	m := NewLibMesh()
	node1 := m.NewLibNode("node1")
	node1.workerConfigs = []workceptor.WorkerConfig{
		workceptor.KubeWorkerCfg{
			WorkType:         "echo",
			AuthMethod:       "runtime",
			Namespace:        "default",
			AllowRuntimePod:  true,
			AllowRuntimeAuth: true,
		},
	}

	m.Start(t.Name())

	ctx1, cancel1 := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel1()

	err := m.WaitForReady(ctx1)
	if err != nil {
		t.Fatal(err, m.GetDataDir())
	}
	nodes := m.GetNodes()
	controllers := make(map[string]*ReceptorControl)
	controllers["node1"] = NewReceptorControl()

	err = controllers["node1"].Connect(nodes["node1"].GetControlSocket())
	if err != nil {
		t.Fatal(err, m.GetDataDir())
	}
	submitJSON := new(bytes.Buffer)
	err = json.Compact(submitJSON, []byte(`{
		"command": "work",
		"subcommand": "submit",
		"node": "localhost",
		"worktype": "echo",
		"secret_kube_pod": "%s",
		"secret_kube_config": "%s"
    }`))
	if err != nil {
		t.Fatal(err)
	}

	kubeConfigBytes, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".kube/config")) //nolint:gosec // G703: reading kubeconfig from well-known path
	if err != nil {
		t.Fatal(err)
	}

	// is there a better way to do this?
	kubeConfig := strings.ReplaceAll(string(kubeConfigBytes), "\n", "\\n")

	echoPodBytes, err := os.ReadFile("testdata/echo-pod.yml")
	if err != nil {
		t.Fatal(err)
	}

	// is there a better way to do this?
	echoPod := strings.ReplaceAll(string(echoPodBytes), "\n", "\\n")

	command := fmt.Sprintf(submitJSON.String(), echoPod, kubeConfig)

	unitID, err := controllers["node1"].WorkSubmitJSON(command)
	if err != nil {
		t.Fatal(err, m.GetDataDir())
	}

	err = controllers["node1"].AssertWorkSucceeded(ctx1, unitID)
	if err != nil {
		t.Fatal(err, m.GetDataDir())
	}

	err = controllers["node1"].AssertWorkResults(unitID, []byte("1\n2\n3\n4\n5\n"))
	if err != nil {
		t.Fatal(err, m.GetDataDir())
	}
}

func TestRuntimeParamsNotAllowed(t *testing.T) {
	m := NewLibMesh()
	node1 := m.NewLibNode("node1")
	node1.workerConfigs = []workceptor.WorkerConfig{
		workceptor.CommandWorkerCfg{
			WorkType:           "echo",
			Command:            "echo",
			AllowRuntimeParams: false,
		},
	}

	err := m.Start(t.Name())
	if err != nil {
		t.Fatal(err, m.GetDataDir())
	}

	ctx1, cancel1 := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel1()

	err = m.WaitForReady(ctx1)
	if err != nil {
		t.Fatal(err, m.GetDataDir())
	}
	nodes := m.GetNodes()
	controllers := make(map[string]*ReceptorControl)
	controllers["node1"] = NewReceptorControl()
	err = controllers["node1"].Connect(nodes["node1"].GetControlSocket())
	if err != nil {
		t.Fatal(err, m.GetDataDir())
	}
	command := `{"command":"work","subcommand":"submit","worktype":"echo","node":"node1","params":"it worked!"}`
	_, err = controllers["node1"].WorkSubmitJSON(command)

	if err == nil {
		t.Fatal("Expected this to fail")
	}

	if !strings.Contains(err.Error(), "extra params provided but not allowed") {
		t.Fatal("Did not see the expected error")
	}
}

func TestKubeContainerFailure(t *testing.T) {
	checkSkipKube(t)

	m := NewLibMesh()
	node1 := m.NewLibNode("node1")
	node1.workerConfigs = []workceptor.WorkerConfig{
		workceptor.KubeWorkerCfg{
			WorkType:   "kubejob",
			AuthMethod: "kubeconfig",
			Image:      "alpine",
			KubeConfig: filepath.Join(os.Getenv("HOME"), ".kube/config"),
			Namespace:  "default",
			Command:    "thiscommandwontexist",
		},
	}

	m.Start(t.Name())

	ctx1, cancel1 := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel1()

	err := m.WaitForReady(ctx1)
	if err != nil {
		t.Fatal(err, m.GetDataDir())
	}
	nodes := m.GetNodes()
	controllers := make(map[string]*ReceptorControl)
	controllers["node1"] = NewReceptorControl()
	err = controllers["node1"].Connect(nodes["node1"].GetControlSocket())
	if err != nil {
		t.Fatal(err, m.GetDataDir())
	}
	job := `{"command":"work","subcommand":"submit","worktype":"kubejob","node":"node1"}`
	unitID, err := controllers["node1"].WorkSubmitJSON(job)
	if err != nil {
		t.Fatal(err, m.GetDataDir())
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel2()

	err = controllers["node1"].AssertWorkFailed(ctx2, unitID)
	if err != nil {
		t.Fatal("Expected work to fail but it succeeded")
	}

	status, err := controllers["node1"].GetWorkStatus(unitID)
	if err != nil {
		t.Fatal("Could not check status")
	}

	expected := `executable file not found in $PATH`
	actual := status.Detail
	if !strings.Contains(actual, expected) {
		t.Fatalf("Did not see the expected error. Wanted %s, got: %s", expected, actual)
	}
}

func TestSignedWorkVerification(t *testing.T) {
	t.Parallel()

	privateKey, publicKey, err := utils.GenerateRSAPair()
	if err != nil {
		t.Fatal(err)
	}
	_, publicKeyWrong, err := utils.GenerateRSAPair()
	if err != nil {
		t.Fatal(err)
	}

	m := NewLibMesh()

	node1 := m.NewLibNode("node1")
	node1.WorkSigningKey = &workceptor.SigningKeyPrivateCfg{
		PrivateKey: privateKey,
	}
	node1.ListenerCfgs = map[listenerName]ListenerCfg{
		listenerName("tcp"): newListenerCfg("tcp", "", 1, nil),
	}

	node2 := m.NewLibNode("node2")
	node2.workerConfigs = []workceptor.WorkerConfig{
		workceptor.CommandWorkerCfg{
			WorkType:        "echo",
			Command:         "echo",
			VerifySignature: true,
		},
	}
	node2.WorkVerificationKey = &workceptor.VerifyingKeyPublicCfg{
		PublicKey: publicKey,
	}
	node2.Connections = []Connection{
		{RemoteNode: node1, Protocol: "tcp"},
	}

	node3 := m.NewLibNode("node3")
	node3.workerConfigs = []workceptor.WorkerConfig{
		workceptor.CommandWorkerCfg{
			WorkType:        "echo",
			Command:         "echo",
			VerifySignature: true,
		},
	}
	node3.WorkVerificationKey = &workceptor.VerifyingKeyPublicCfg{
		PublicKey: publicKeyWrong,
	}
	node3.Connections = []Connection{
		{RemoteNode: node1, Protocol: "tcp"},
	}

	err = m.Start(t.Name())
	if err != nil {
		t.Fatal(err)
	}

	ctx1, cancel1 := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel1()

	err = m.WaitForReady(ctx1)
	if err != nil {
		t.Fatal(err, m.GetDataDir())
	}
	nodes := m.GetNodes()
	controllers := make(map[string]*ReceptorControl)
	controllers["node1"] = NewReceptorControl()
	err = controllers["node1"].Connect(nodes["node1"].GetControlSocket())
	if err != nil {
		t.Fatal(err, m.GetDataDir())
	}

	job := `{"command":"work","subcommand":"submit","worktype":"echo","node":"node2", "signwork":"true"}`
	unitID, err := controllers["node1"].WorkSubmitJSON(job)
	if err != nil {
		t.Fatal(err, m.GetDataDir())
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel2()

	err = controllers["node1"].AssertWorkSucceeded(ctx2, unitID)
	if err != nil {
		t.Fatal(err, m.GetDataDir())
	}

	// node3 has the wrong public key to verify work signatures, so the work submission should fail
	job = `{"command":"work","subcommand":"submit","worktype":"echo","node":"node3", "signwork":"true"}`
	_, err = controllers["node1"].WorkSubmitJSON(job)
	if err == nil {
		t.Fatal("expected work submission to fail")
	}

	expected := `could not verify signature: crypto/rsa: verification error`
	actual := err.Error()
	if !strings.Contains(actual, expected) {
		t.Fatalf("Did not see the expected error. Wanted %s, got: %s", expected, actual)
	}
}

func submitRunningWorkForAdoption(t *testing.T, controller *ReceptorControl) string {
	t.Helper()
	unitID, err := controller.WorkSubmit("node3", "echosleeplong")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	if err := controller.AssertWorkRunning(ctx, unitID); err != nil {
		t.Fatal(err)
	}
	status, err := controller.GetWorkStatus(unitID)
	if err != nil {
		t.Fatal(err)
	}
	remoteUnitID, ok := status.ExtraData.(map[string]interface{})["RemoteUnitID"].(string)
	if !ok || remoteUnitID == "" {
		t.Fatal("remote work unit ID is missing")
	}

	return remoteUnitID
}

func adoptAndCheckResults(t *testing.T, controller *ReceptorControl, remoteNode, remoteUnitID string, expectedResults []byte) string {
	t.Helper()
	response, err := controller.WorkAdopt(remoteNode, remoteUnitID)
	if err != nil {
		t.Fatal(err)
	}
	result, ok := response["result"].(string)
	if !ok || (result != "Adopted" && result != "Adopt Pending") {
		t.Fatalf("unexpected adopt response: %#v", response)
	}
	localUnitID, ok := response["unitid"].(string)
	if !ok || localUnitID == "" {
		t.Fatalf("adopt response has no local unit ID: %#v", response)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	if err := controller.AssertWorkSucceeded(ctx, localUnitID); err != nil {
		t.Fatal(err)
	}
	if err := controller.AssertWorkResults(localUnitID, expectedResults); err != nil {
		t.Fatal(err)
	}

	return localUnitID
}

func TestWorkAdopt(t *testing.T) {
	t.Parallel()
	for _, plugin := range workPlugins {
		plugin := plugin
		t.Run(string(plugin), func(t *testing.T) {
			t.Parallel()
			controllers, m, expectedResults := workSetup(plugin, t)
			defer m.WaitForShutdown()
			defer m.Destroy()

			remoteUnitID := submitRunningWorkForAdoption(t, controllers["node1"])
			adoptAndCheckResults(t, controllers["node2"], "node3", remoteUnitID, expectedResults)
			if _, err := controllers["node2"].WorkAdopt("node3", remoteUnitID); err == nil || !strings.Contains(err.Error(), "already in use") {
				t.Fatalf("second adoption error = %v, want already-in-use error", err)
			}
		})
	}
}

func TestWorkAdoptCompletedWork(t *testing.T) {
	t.Parallel()
	controllers, m, expectedResults := workSetup("command", t)
	defer m.WaitForShutdown()
	defer m.Destroy()

	unitID, err := controllers["node1"].WorkSubmit("node3", "echosleepshort")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	if err := controllers["node1"].AssertWorkSucceeded(ctx, unitID); err != nil {
		t.Fatal(err)
	}
	status, err := controllers["node1"].GetWorkStatus(unitID)
	if err != nil {
		t.Fatal(err)
	}
	remoteUnitID := status.ExtraData.(map[string]interface{})["RemoteUnitID"].(string)
	if remoteUnitID == "" {
		t.Fatal("remote work unit ID is missing")
	}
	adoptAndCheckResults(t, controllers["node2"], "node3", remoteUnitID, expectedResults)
}

func TestWorkAdoptNonexistentRemoteUnit(t *testing.T) {
	t.Parallel()
	controllers, m, _ := workSetup("command", t)
	defer m.WaitForShutdown()
	defer m.Destroy()

	response, err := controllers["node2"].WorkAdopt("node3", "does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	localUnitID, ok := response["unitid"].(string)
	if !ok || localUnitID == "" {
		t.Fatalf("adopt response has no local unit ID: %#v", response)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := controllers["node2"].AssertWorkFailed(ctx, localUnitID); err != nil {
		t.Fatal(err)
	}
}

func TestWorkAdoptSignedVerification(t *testing.T) {
	t.Parallel()
	privateKey, publicKey, err := utils.GenerateRSAPair()
	if err != nil {
		t.Fatal(err)
	}
	m := NewLibMesh()
	node1 := m.NewLibNode("node1")
	node1.WorkSigningKey = &workceptor.SigningKeyPrivateCfg{PrivateKey: privateKey}
	node1.ListenerCfgs = map[listenerName]ListenerCfg{listenerName("tcp"): newListenerCfg("tcp", "", 1, nil)}
	node2 := m.NewLibNode("node2")
	node2.workerConfigs = []workceptor.WorkerConfig{
		workceptor.CommandWorkerCfg{WorkType: "echo", Command: "bash", Params: "-c 'echo signed-output'", VerifySignature: true},
	}
	node2.WorkVerificationKey = &workceptor.VerifyingKeyPublicCfg{PublicKey: publicKey}
	node2.ListenerCfgs = map[listenerName]ListenerCfg{listenerName("tcp"): newListenerCfg("tcp", "", 1, nil)}
	node2.Connections = []Connection{{RemoteNode: node1, Protocol: "tcp"}}
	node3 := m.NewLibNode("node3")
	node3.WorkSigningKey = &workceptor.SigningKeyPrivateCfg{PrivateKey: privateKey}
	node3.Connections = []Connection{{RemoteNode: node2, Protocol: "tcp"}}
	if err := m.Start(t.Name()); err != nil {
		t.Fatal(err)
	}
	defer m.WaitForShutdown()
	defer m.Destroy()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	if err := m.WaitForReady(ctx); err != nil {
		t.Fatal(err, m.GetDataDir())
	}
	nodes := m.GetNodes()
	node1Controller := NewReceptorControl()
	if err := node1Controller.Connect(nodes["node1"].GetControlSocket()); err != nil {
		t.Fatal(err)
	}
	node2Controller := NewReceptorControl()
	if err := node2Controller.Connect(nodes["node2"].GetControlSocket()); err != nil {
		t.Fatal(err)
	}
	node3Controller := NewReceptorControl()
	if err := node3Controller.Connect(nodes["node3"].GetControlSocket()); err != nil {
		t.Fatal(err)
	}
	unitID, err := node1Controller.WorkSubmitJSON(`{"command":"work","subcommand":"submit","worktype":"echo","node":"node2","signwork":"true"}`)
	if err != nil {
		t.Fatal(err)
	}
	if err := node1Controller.AssertWorkSucceeded(ctx, unitID); err != nil {
		t.Fatal(err)
	}
	status, err := node1Controller.GetWorkStatus(unitID)
	if err != nil {
		t.Fatal(err)
	}
	remoteUnitID := status.ExtraData.(map[string]interface{})["RemoteUnitID"].(string)
	response, err := node3Controller.WorkAdoptJSON(fmt.Sprintf(`{"command":"work","subcommand":"adopt","node":"node2","unitid":%q,"signwork":"true"}`, remoteUnitID))
	if err != nil {
		t.Fatal(err)
	}
	localUnitID, ok := response["unitid"].(string)
	if !ok || localUnitID == "" {
		t.Fatalf("adopt response has no local unit ID: %#v", response)
	}
	if err := node3Controller.AssertWorkSucceeded(ctx, localUnitID); err != nil {
		t.Fatal(err)
	}
	if err := node3Controller.AssertWorkResults(localUnitID, []byte("signed-output\n")); err != nil {
		t.Fatal(err)
	}
}
