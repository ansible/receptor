package mesh

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ansible/receptor/tests/utils"
)

// TestNodeIDPresentInAllLogLines verifies that every log line emitted by a mesh
// node — structured or plain — identifies the node that produced it.
func TestNodeIDPresentInAllLogLines(t *testing.T) {
	t.Parallel()

	m := twoNodeMesh()
	if err := m.Start(t.Name()); err != nil {
		t.Fatal(err)
	}
	defer m.WaitForShutdown()
	defer m.Destroy()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := m.WaitForReady(ctx); err != nil {
		t.Fatal(err, m.DataDir)
	}

	rc := NewReceptorControl()
	if err := rc.Connect(m.GetNodes()["control"].GetControlSocket()); err != nil {
		t.Fatalf("control socket connect: %v", err)
	}
	defer rc.Close()
	if _, err := rc.Ping("execution-a"); err != nil {
		t.Fatalf("ping execution-a: %v", err)
	}

	for _, line := range strings.Split(m.LogWriter.String(), "\n") {
		if line == "" {
			continue
		}
		if !strings.Contains(line, `"node_id"`) {
			t.Errorf("log line missing node_id: %s", line)
		}
	}
}

// TestEstablishedConnectionLogsCorrectRemoteID verifies that when two nodes
// successfully connect, the log output records the correct identity of the
// remote peer.
func TestEstablishedConnectionLogsCorrectRemoteID(t *testing.T) {
	t.Parallel()

	m := twoNodeMesh()
	if err := m.Start(t.Name()); err != nil {
		t.Fatal(err)
	}
	defer m.WaitForShutdown()
	defer m.Destroy()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := m.WaitForReady(ctx); err != nil {
		t.Fatal(err, m.DataDir)
	}

	if !strings.Contains(m.LogWriter.String(), `"remote_id":"execution-a"`) {
		t.Errorf("expected at least one log line identifying remote peer execution-a; got:\n%s", m.LogWriter.String())
	}
}

// TestLogLevelFilteringSuppressesLowPriorityMessages verifies that at the
// default log level, low-priority messages (debug, trace) are suppressed while
// normal informational messages are still emitted.
func TestLogLevelFilteringSuppressesLowPriorityMessages(t *testing.T) {
	t.Parallel()

	m := twoNodeMesh()
	if err := m.Start(t.Name()); err != nil {
		t.Fatal(err)
	}
	defer m.WaitForShutdown()
	defer m.Destroy()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := m.WaitForReady(ctx); err != nil {
		t.Fatal(err, m.DataDir)
	}

	logs := m.LogWriter.String()

	for _, line := range strings.Split(logs, "\n") {
		if strings.Contains(line, "DEBUG ") {
			t.Errorf("DEBUG line appeared at default log level: %q", line)
		}
		if strings.Contains(line, "TRACE ") {
			t.Errorf("TRACE line appeared at default log level: %q", line)
		}
	}

	if !strings.Contains(logs, "INFO ") {
		t.Error("expected at least one INFO line in log output, but found none")
	}
}

// TestPerNodeLogWriterIsolation verifies that each node's log output is
// delivered exclusively to its own configured writer and does not appear in
// another node's writer.
func TestPerNodeLogWriterIsolation(t *testing.T) {
	t.Parallel()

	m := twoNodeMesh()

	controlWriter := utils.NewTestLogWriter()
	executionWriter := utils.NewTestLogWriter()
	m.AfterInit = func(nodes map[string]*LibNode) {
		nodes["control"].netceptorInstance.Logger.SetOutput(controlWriter)
		nodes["execution-a"].netceptorInstance.Logger.SetOutput(executionWriter)
	}

	if err := m.Start(t.Name()); err != nil {
		t.Fatal(err)
	}
	defer m.WaitForShutdown()
	defer m.Destroy()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := m.WaitForReady(ctx); err != nil {
		t.Fatal(err, m.DataDir)
	}

	// Poll because log writes to per-node writers may lag behind WaitForReady.
	ctxLog, cancelLog := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelLog()

	if !utils.CheckUntilTimeout(ctxLog, 100*time.Millisecond, func() bool {
		return strings.Contains(controlWriter.String(), `"node_id":"control"`)
	}) {
		t.Errorf("controlWriter missing node_id:control; got:\n%s", controlWriter.String())
	}
	if !utils.CheckUntilTimeout(ctxLog, 100*time.Millisecond, func() bool {
		return strings.Contains(executionWriter.String(), `"node_id":"execution-a"`)
	}) {
		t.Errorf("executionWriter missing node_id:execution-a; got:\n%s", executionWriter.String())
	}
	if strings.Contains(controlWriter.String(), `"node_id":"execution-a"`) {
		t.Error("controlWriter contains output from execution-a (cross-contamination)")
	}
}

// TestStructuredSuffixIsValidJSON verifies that the structured context block
// appended to log lines is well-formed JSON. This guards against serialization
// regressions that would silently break log parsers and structured ingestion
// pipelines.
func TestStructuredSuffixIsValidJSON(t *testing.T) {
	t.Parallel()

	m := twoNodeMesh()
	if err := m.Start(t.Name()); err != nil {
		t.Fatal(err)
	}
	defer m.WaitForShutdown()
	defer m.Destroy()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := m.WaitForReady(ctx); err != nil {
		t.Fatal(err, m.DataDir)
	}

	checkedAny := false
	for _, line := range strings.Split(m.LogWriter.String(), "\n") {
		if !strings.HasSuffix(line, "}") {
			continue
		}
		idx := strings.LastIndex(line, "{")
		if idx == -1 {
			continue
		}
		checkedAny = true
		var obj map[string]string
		if err := json.Unmarshal([]byte(line[idx:]), &obj); err != nil {
			t.Errorf("suffix block is not valid JSON in line %q: %v", line, err)
		}
	}
	if !checkedAny {
		t.Error("expected at least one log line with a structured suffix, but found none")
	}
}

// TestLogLineHasTimestampAndLevel verifies that every non-empty log line
// carries both a level label and a timestamp. This guards against refactors
// that accidentally drop the timestamp, alter the date format, or remove the
// level prefix.
func TestLogLineHasTimestampAndLevel(t *testing.T) {
	t.Parallel()

	m := twoNodeMesh()
	if err := m.Start(t.Name()); err != nil {
		t.Fatal(err)
	}
	defer m.WaitForShutdown()
	defer m.Destroy()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := m.WaitForReady(ctx); err != nil {
		t.Fatal(err, m.DataDir)
	}

	// Expected format: LEVEL YYYY/MM/DD HH:MM:SS <message>
	lineRe := regexp.MustCompile(`^(INFO|WARNING|ERROR|DEBUG|TRACE) \d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2} `)

	// Every non-empty, non-indented line must carry the required prefix.
	// Indented lines (space or tab) are valid continuations of the preceding
	// entry (e.g. multi-line routing tables, stack traces). A blank line ends
	// any continuation run.
	for _, line := range strings.Split(m.LogWriter.String(), "\n") {
		if line == "" {
			continue
		}
		if lineRe.MatchString(line) {
			continue
		}
		if line[0] == ' ' || line[0] == '\t' {
			continue
		}
		t.Errorf("log line does not match expected LEVEL YYYY/MM/DD HH:MM:SS format: %q", line)
	}
}

// TestNodeReconnectionIsLogged verifies that when a peer disconnects and
// reconnects, the reconnection event is recorded with the correct remote peer
// identity.
func TestNodeReconnectionIsLogged(t *testing.T) {
	t.Parallel()

	m := twoNodeMesh()
	if err := m.Start(t.Name()); err != nil {
		t.Fatal(err)
	}
	defer m.WaitForShutdown()
	defer m.Destroy()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := m.WaitForReady(ctx); err != nil {
		t.Fatal(err, m.DataDir)
	}

	nodes := m.GetNodes()
	execA := nodes["execution-a"]

	execA.Shutdown()
	if err := execA.StartLocalServices(); err != nil {
		t.Fatal(err, m.DataDir)
	}
	execA.netceptorInstance.Logger.SetOutput(m.LogWriter)
	if err := execA.StartListeners(); err != nil {
		t.Fatal(err, m.DataDir)
	}
	if err := execA.EstablishRemoteConnections(); err != nil {
		t.Fatal(err, m.DataDir)
	}

	ctxReconn, cancelReconn := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelReconn()

	if err := m.WaitForReady(ctxReconn); err != nil {
		t.Fatal("mesh did not recover after execution-a restart:", err, m.DataDir)
	}

	// The control node logs one entry per successful connection to execution-a.
	// There should be at least two: the initial connection and the reconnection.
	// Poll because the log write happens in a goroutine that may lag behind the
	// routing convergence that WaitForReady detects.
	ctxLog, cancelLog := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelLog()

	if !utils.CheckUntilTimeout(ctxLog, 100*time.Millisecond, func() bool {
		return strings.Count(m.LogWriter.String(), "Connection established with execution-a") >= 2
	}) {
		t.Errorf("expected at least 2 'Connection established with execution-a' entries (initial + reconnect), got %d\nlogs:\n%s",
			strings.Count(m.LogWriter.String(), "Connection established with execution-a"), m.LogWriter.String())
	}
}

// TestConnectionRejectedPeerIsLogged verifies that when a node's connection is
// rejected by a peer, the rejection is recorded in the log output.
func TestConnectionRejectedPeerIsLogged(t *testing.T) {
	t.Parallel()

	m := NewLibMesh()

	control := m.NewLibNode("control")
	control.ListenerCfgs = map[listenerName]ListenerCfg{
		listenerName("tcp"): newListenerCfg("tcp", "", 1, nil),
	}
	// Restrict the listener to only accept a node that does not exist in this
	// mesh, so execution-a's connection attempt is rejected.
	control.AllowedPeers = []string{"some-other-node"}

	execA := m.NewLibNode("execution-a")
	execA.Connections = []Connection{
		{RemoteNode: control, Protocol: "tcp"},
	}

	if err := m.Start(t.Name()); err != nil {
		t.Fatal(err)
	}
	defer m.WaitForShutdown()
	defer m.Destroy()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if !utils.CheckUntilTimeout(ctx, 500*time.Millisecond, func() bool {
		return strings.Contains(m.LogWriter.String(), "rejected connection with node execution-a because it is not in the allowed peers list")
	}) {
		t.Fatalf("rejection log message not observed\nlogs:\n%s", m.LogWriter.String())
	}
}
