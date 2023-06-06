package mesh

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ansible/receptor/tests/utils"
)

func workSetup(workPluginName workPlugin, t *testing.T) (map[string]*ReceptorControl, *LibMesh, []byte) {
	checkSkipKube(t)

	m := workTestMesh(workPluginName)

	err := m.Start(t.Name())
	if err != nil {
		t.Fatal(err)
	}

	ctx, _ := context.WithTimeout(context.Background(), 120*time.Second)
	err = m.WaitForReady(ctx)
	if err != nil {
		t.Fatal(err, m.DataDir)
	}

	nodes := m.GetNodes()
	controllers := make(map[string]*ReceptorControl)
	for k := range nodes {
		controller := NewReceptorControl()
		err = controller.Connect(nodes[k].GetControlSocket())
		if err != nil {
			t.Fatal(err, m.DataDir)
		}
		controllers[k] = controller
	}

	return controllers, m, []byte("1\n2\n3\n4\n5\n")
}

func assertFilesReleased(ctx context.Context, nodeDir, nodeID, unitID string) error {
	workPath := filepath.Join(nodeDir, "datadir", nodeID, unitID)
	check := func() bool {
		_, err := os.Stat(workPath)

		return os.IsNotExist(err)
	}
	if !utils.CheckUntilTimeout(ctx, 5*time.Second, check) {
		return fmt.Errorf("unitID %s on %s did not release", unitID, nodeID)
	}

	return nil
}

func assertStdoutFizeSize(ctx context.Context, dataDir, nodeID, unitID string, waitUntilSize int) error {
	stdoutFilename := filepath.Join(dataDir, nodeID, unitID, "stdout")
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

func checkSkipKube(t *testing.T) {
	if skip := os.Getenv("SKIP_KUBE"); skip == "1" {
		t.Skip("Kubernetes tests are set to skip, unset SKIP_KUBE to run them")
	}
}
