package mesh

import (
	"context"
	"encoding/json"
	"net"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/services"
	net_interface "github.com/ansible/receptor/pkg/services/interfaces"
	"github.com/ansible/receptor/pkg/workceptor"
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

// captureNetLib wraps services.NetTCPWrapper, sending the listener it creates to ch.
type captureNetLib struct {
	services.NetTCPWrapper
	ch chan net.Listener
}

func (c *captureNetLib) Listen(network, addr string) (net.Listener, error) {
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), network, addr)
	if err == nil {
		c.ch <- ln
	}

	return ln, err
}

// captureNetterUDP wraps services.NetUDPWrapper, sending the connection it
// creates to ch so the test can close the proxy listener.
type captureNetterUDP struct {
	services.NetUDPWrapper
	ch chan net_interface.UDPConnInterface
}

func (c *captureNetterUDP) ListenUDP(network string, laddr *net.UDPAddr) (net_interface.UDPConnInterface, error) {
	conn, err := net.ListenUDP(network, laddr)
	if err == nil {
		c.ch <- conn
	}

	return conn, err
}

// waitForLogLine polls until logsFn returns logs containing a line with substr.
// Use it before assertLogLineHasNodeID when the log write may lag behind the
// action that triggers it.
func waitForLogLine(t *testing.T, ctx context.Context, logsFn func() string, substr string) {
	t.Helper()

	if !utils.CheckUntilTimeout(ctx, 100*time.Millisecond, func() bool {
		return strings.Contains(logsFn(), substr)
	}) {
		t.Fatalf("no %q log line captured\nlogs:\n%s", substr, logsFn())
	}
}

// assertLogLineHasNodeID fails if logs has no line containing substr, or if any
// such line lacks a structured node_id suffix.
func assertLogLineHasNodeID(t *testing.T, logs, substr string) {
	t.Helper()

	if !strings.Contains(logs, substr) {
		t.Fatalf("no %q log line captured\nlogs:\n%s", substr, logs)
	}
	for _, line := range strings.Split(logs, "\n") {
		if !strings.Contains(line, substr) {
			continue
		}
		if !strings.Contains(line, `"node_id"`) {
			t.Errorf("%q log line missing node_id: %s", substr, line)
		}
	}
}

// TestControlSvcLogsHaveNodeID verifies that the "Running control service" log
// line emitted by controlsvc.go carries node_id in its structured suffix.
func TestControlSvcLogsHaveNodeID(t *testing.T) {
	t.Parallel()

	m := NewLibMesh()

	control := m.NewLibNode("control")
	control.ListenerCfgs = map[listenerName]ListenerCfg{
		listenerName("tcp"): newListenerCfg("tcp", "", 1, nil),
	}

	execA := m.NewLibNode("execution-a")
	execA.Connections = []Connection{{RemoteNode: control, Protocol: "tcp"}}

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

	assertLogLineHasNodeID(t, m.LogWriter.String(), "Running control service")
}

// TestWorkceptorLogsHaveNodeID verifies that log lines emitted by workceptor.go
// carry node_id in their structured suffix.
//
// The "Error locating unit" Error fires synchronously in
// StartUnit when the requested unit ID does not exist.
func TestWorkceptorLogsHaveNodeID(t *testing.T) {
	t.Parallel()

	m := twoNodeMesh()
	m.AfterInit = func(nodes map[string]*LibNode) {
		// StartUnit with a nonexistent ID makes scanForUnit log via GetLogger().Error.
		_ = nodes["control"].workceptorInstance.StartUnit("nonexistent-unit-for-nodecheck")
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

	assertLogLineHasNodeID(t, m.LogWriter.String(), "Error locating unit")
}

// TestTCPProxyLogsHaveNodeID verifies that the error logged when a TCP proxy
// cannot reach the target Receptor node carries node_id in its suffix.
func TestTCPProxyLogsHaveNodeID(t *testing.T) {
	t.Parallel()

	m := twoNodeMesh()
	listenerCh := make(chan net.Listener, 1)
	m.AfterInit = func(nodes map[string]*LibNode) {
		n := nodes["control"].GetNetceptor()
		go services.TCPProxyServiceInbound( //nolint:errcheck
			n, "127.0.0.1", 0, nil,
			"nonexistent-node", "echo-svc", nil,
			&captureNetLib{ch: listenerCh},
			nil,
			&services.UtilsTCPWrapper{},
		)
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

	var ln net.Listener
	select {
	case ln = <-listenerCh:
	case <-ctx.Done():
		t.Fatal("timed out waiting for TCP proxy listener")
	}
	defer ln.Close()

	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer conn.Close()

	ctxLog, cancelLog := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelLog()

	waitForLogLine(t, ctxLog, m.LogWriter.String, "error connecting on Receptor network")
	assertLogLineHasNodeID(t, m.LogWriter.String(), "error connecting on Receptor network")
}

// TestUDPProxyLogsHaveNodeID verifies that the error logged when a UDP proxy
// cannot forward a packet to the target Receptor service carries node_id.
func TestUDPProxyLogsHaveNodeID(t *testing.T) {
	t.Parallel()

	m := twoNodeMesh()
	connCh := make(chan net_interface.UDPConnInterface, 1)
	m.AfterInit = func(nodes map[string]*LibNode) {
		n := nodes["control"].GetNetceptor()
		go services.UDPProxyServiceInbound( //nolint:errcheck
			n, "127.0.0.1", 0,
			"nonexistent-node", "nonexistent-svc",
			&captureNetterUDP{ch: connCh},
		)
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

	var uc net_interface.UDPConnInterface
	select {
	case uc = <-connCh:
	case <-ctx.Done():
		t.Fatal("timed out waiting for UDP proxy listener")
	}
	defer uc.Close()

	conn, err := (&net.Dialer{}).DialContext(ctx, "udp", uc.LocalAddr().String())
	if err != nil {
		t.Fatalf("dial udp proxy: %v", err)
	}
	defer conn.Close()
	_, _ = conn.Write([]byte("ping"))

	ctxLog, cancelLog := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelLog()

	waitForLogLine(t, ctxLog, m.LogWriter.String, "Error sending packet on Receptor network")
	assertLogLineHasNodeID(t, m.LogWriter.String(), "Error sending packet on Receptor network")
}

// TestKubeWorkerLogsHaveNodeID verifies that log lines emitted by kubernetes.go
// via GetLogger() carry node_id in their structured suffix.
//
// The "completed successfully" Info line is emitted by KubeLoggingWithReconnect
// once the pod exits 0. It is only reachable through a real Kubernetes API
// server, so this test runs against the KinD cluster that CI provisions and is
// skipped locally when SKIP_KUBE=1.
func TestKubeWorkerLogsHaveNodeID(t *testing.T) {
	t.Parallel()
	checkSkipKube(t)

	m := NewLibMesh()
	node1 := m.NewLibNode("node1")
	node1.workerConfigs = []workceptor.WorkerConfig{
		workTestConfigs["kube"]["echosleepshort"],
	}

	if err := m.Start(t.Name()); err != nil {
		t.Fatal(err)
	}
	defer m.WaitForShutdown()
	defer m.Destroy()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	if err := m.WaitForReady(ctx); err != nil {
		t.Fatal(err, m.DataDir)
	}

	nodes := m.GetNodes()
	controller := NewReceptorControl()
	if err := controller.Connect(nodes["node1"].GetControlSocket()); err != nil {
		t.Fatal(err, m.DataDir)
	}

	unitID, err := controller.WorkStart("echosleepshort")
	if err != nil {
		t.Fatal(err, m.DataDir)
	}

	if err := controller.AssertWorkSucceeded(ctx, unitID); err != nil {
		t.Fatal(err, m.DataDir)
	}

	// The log write happens in a goroutine that may lag behind the status
	// update AssertWorkSucceeded observes, so poll before asserting.
	ctxLog, cancelLog := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelLog()

	waitForLogLine(t, ctxLog, m.LogWriter.String, "completed successfully")
	assertLogLineHasNodeID(t, m.LogWriter.String(), "completed successfully")
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
