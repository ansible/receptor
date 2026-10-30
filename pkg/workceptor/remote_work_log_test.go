//go:build !no_workceptor
// +build !no_workceptor

package workceptor_test

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/utils"
	"github.com/ansible/receptor/pkg/workceptor"
	"github.com/ansible/receptor/pkg/workceptor/mock_workceptor"
	testutils "github.com/ansible/receptor/tests/utils"
	"github.com/prep/socketpair"
	"go.uber.org/mock/gomock"
)

const (
	localNode    = "node1"
	remoteIDNode = "node2"
)

// remoteID returns the structured remote_id suffix the handshake assigns to
// the shared logger.
func remoteID(node string) string {
	return `"remote_id":"` + node + `"`
}

// connectTwoNetceptorNodes creates two real Netceptor nodes connected via
// external backends over a Unix socket pair, and returns the local node with
// its log output captured. After the handshake, the local node's shared
// logger carries the remote peer's identity. The remote-work logging tests
// below assert only that this remote_id suffix is still present: while the
// global logger mutation is in place, the suffix names whichever peer
// handshook last, so these tests guard against its loss rather than prove it
// names this unit's peer.
func connectTwoNetceptorNodes(t *testing.T, ctx context.Context, localNode, remoteNode string) (*netceptor.Netceptor, *netceptor.Netceptor, *testutils.LogCapture) {
	t.Helper()

	n1 := netceptor.New(ctx, localNode)
	logCapture := testutils.NewLogCapture()
	n1.Logger.SetOutput(logCapture)

	n2 := netceptor.New(ctx, remoteNode)
	n2.Logger.SetOutput(io.Discard)

	b1, err := netceptor.NewExternalBackend()
	if err != nil {
		t.Fatal(err)
	}
	if err := n1.AddBackend(b1); err != nil {
		t.Fatal(err)
	}

	b2, err := netceptor.NewExternalBackend()
	if err != nil {
		t.Fatal(err)
	}
	if err := n2.AddBackend(b2); err != nil {
		t.Fatal(err)
	}

	// Subscribe before connecting: the initial routing-update flood fires
	// as soon as the backends are linked, and the next one only lands at
	// defaultRouteUpdateTime, which would exhaust the timeout below.
	routesChan := n1.SubscribeRoutingUpdates()

	c1, c2, err := socketpair.New("unix")
	if err != nil {
		t.Fatal(err)
	}
	b1.NewConnection(netceptor.MessageConnFromNetConn(c1), true)
	b2.NewConnection(netceptor.MessageConnFromNetConn(c2), true)
	t.Cleanup(func() {
		_ = c1.Close()
		_ = c2.Close()
	})
	t.Cleanup(n1.Shutdown)
	t.Cleanup(n2.Shutdown)

	// Wait for the local node to establish routing to the remote node.
	timeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	for {
		select {
		case <-timeout.Done():
			t.Fatalf("timed out waiting for %s to connect to %s; logs:\n%s", localNode, remoteNode, logCapture.String())
		case routes, ok := <-routesChan:
			if !ok {
				t.Fatalf("routing update subscription closed before %s connected to %s; logs:\n%s", localNode, remoteNode, logCapture.String())
			}
			if routes != nil {
				if _, ok := routes[remoteNode]; ok {
					return n1, n2, logCapture
				}
			}
		}
	}
}

// startFakeControlListener advertises a "control" service on n2 and hands each
// accepted connection to handle.
func startFakeControlListener(t *testing.T, n2 *netceptor.Netceptor, handle func(net.Conn)) {
	t.Helper()
	listener, err := n2.ListenAndAdvertise("control", nil, nil)
	if err != nil {
		t.Fatalf("could not listen on control service: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				// A dialer that closes before streaming its init byte
				// surfaces here as a per-connection error; keep serving.
				select {
				case <-listener.DoneChan:
					return
				default:
					continue
				}
			}
			go handle(conn)
		}
	}()
}

// startFakeControlService advertises a "control" service on n2 that greets the
// caller with the node ID and delegates each request line to handle. handle
// returns false to close the connection after the request.
func startFakeControlService(t *testing.T, n2 *netceptor.Netceptor, node string, handle func(conn net.Conn, req string) bool) {
	t.Helper()
	startFakeControlListener(t, n2, func(conn net.Conn) {
		defer conn.Close()
		if _, err := fmt.Fprintf(conn, "%s\n", node); err != nil {
			return
		}
		reader := bufio.NewReader(conn)
		for {
			req, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			if !handle(conn, strings.TrimSpace(req)) {
				return
			}
		}
	})
}

// waitForControlDial dials until the control service is reachable.
func waitForControlDial(t *testing.T, n1 *netceptor.Netceptor, node string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var lastErr error
	ready := testutils.CheckUntilTimeout(ctx, 50*time.Millisecond, func() bool {
		dialCtx, dialCancel := context.WithTimeout(context.Background(), time.Second)
		defer dialCancel()
		conn, err := n1.DialContext(dialCtx, node, "control", nil)
		if err != nil {
			lastErr = err

			return false
		}
		_ = conn.CloseConnection()

		return true
	})
	if !ready {
		t.Fatalf("timed out waiting for control service on %s: %v", node, lastErr)
	}
}

// waitForControlService waits for the advertisement to propagate and verifies
// that the service greets the caller with the node ID.
func waitForControlService(t *testing.T, n1 *netceptor.Netceptor, node string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var lastErr error
	greeted := testutils.CheckUntilTimeout(ctx, 50*time.Millisecond, func() bool {
		dialCtx, dialCancel := context.WithTimeout(context.Background(), time.Second)
		defer dialCancel()
		conn, err := n1.DialContext(dialCtx, node, "control", nil)
		if err != nil {
			lastErr = err

			return false
		}
		defer func() { _ = conn.CloseConnection() }()
		_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		hello, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			lastErr = err

			return false
		}
		if !strings.Contains(hello, node) {
			lastErr = fmt.Errorf("control service on %s greeted with %q", node, strings.TrimSpace(hello))

			return false
		}

		return true
	})
	if !greeted {
		t.Fatalf("timed out waiting for control service on %s: %v", node, lastErr)
	}
}

// newRemoteLogTestEnv builds a Workceptor backed by the handshake-suffixed
// n1 logger, plus a mocked base work unit whose status is controlled by the
// caller. Because w.nc is the real n1, every rw.GetWorkceptor().nc.GetLogger()
// call observes the remote_id set during the handshake.
func newRemoteLogTestEnv(t *testing.T, ctx context.Context, n1 *netceptor.Netceptor, red *workceptor.RemoteExtraData, status workceptor.StatusFileData, unitDir string, lastUpdateError error) (*workceptor.Workceptor, *mock_workceptor.MockBaseWorkUnitForWorkUnit) {
	t.Helper()
	ctrl := gomock.NewController(t)
	base := mock_workceptor.NewMockBaseWorkUnitForWorkUnit(ctrl)
	w, err := workceptor.New(ctx, n1, t.TempDir())
	if err != nil {
		t.Fatalf("could not create workceptor: %v", err)
	}
	t.Cleanup(w.Cancel)

	expectRemoteUnitStatusSources(base, w, red, status)
	base.EXPECT().LastUpdateError().Return(lastUpdateError).AnyTimes()
	if unitDir != "" {
		base.EXPECT().UnitDir().Return(unitDir).AnyTimes()
		base.EXPECT().StatusFileName().Return(filepath.Join(unitDir, "status")).AnyTimes()
	}
	base.EXPECT().UpdateBasicStatus(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	base.EXPECT().UpdateFullStatus(gomock.Any()).Do(func(update func(*workceptor.StatusFileData)) {
		update(&workceptor.StatusFileData{ExtraData: red})
	}).AnyTimes()

	return w, base
}

// remoteMonitorOptions configures a remote monitor test environment.
type remoteMonitorOptions struct {
	// remote is the unit's remote extra data, and status its initial status.
	remote *workceptor.RemoteExtraData
	status workceptor.StatusFileData
	// unitDir, when set, is where the unit's status and stdout files live.
	unitDir string
	// lastUpdateError is what the mocked LastUpdateError returns.
	lastUpdateError error
	// service, when non-nil, advertises a fake control service on the remote
	// node and handles each request line with it.
	service func(conn net.Conn, req string) bool
	// dialOnly skips the hello check, for fake services that do not greet.
	dialOnly bool
}

// remoteMonitorEnv is the shared fixture for remote monitor logging tests: the
// two connected nodes, the log capture, the mocked unit and its job context.
type remoteMonitorEnv struct {
	ctx     context.Context
	capture *testutils.LogCapture
	unit    *mock_workceptor.MockBaseWorkUnitForWorkUnit
	job     *utils.JobContext
}

// newRemoteMonitorEnv connects the two nodes, optionally starts a fake control
// service, builds the mocked unit and creates a job context for the monitor.
func newRemoteMonitorEnv(t *testing.T, opts remoteMonitorOptions) *remoteMonitorEnv {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	n1, n2, capture := connectTwoNetceptorNodes(t, ctx, localNode, remoteIDNode)
	if opts.service != nil {
		startFakeControlService(t, n2, remoteIDNode, opts.service)
		if opts.dialOnly {
			waitForControlDial(t, n1, remoteIDNode)
		} else {
			waitForControlService(t, n1, remoteIDNode)
		}
	}

	_, unit := newRemoteLogTestEnv(t, ctx, n1, opts.remote, opts.status, opts.unitDir, opts.lastUpdateError)
	job := &utils.JobContext{}
	job.NewJob(ctx, 1, false)

	return &remoteMonitorEnv{ctx: ctx, capture: capture, unit: unit, job: job}
}

// startStdout launches monitorRemoteStdout on the shared job context.
func (e *remoteMonitorEnv) startStdout() {
	go workceptor.MonitorRemoteStdoutForTest(e.unit, e.job)
}

// startStatus launches monitorRemoteStatus on the shared job context.
func (e *remoteMonitorEnv) startStatus(forRelease bool) {
	go workceptor.MonitorRemoteStatusForTest(e.unit, e.job, forRelease)
}

// cancel cancels the job context mid-test.
func (e *remoteMonitorEnv) cancel() {
	e.job.Cancel()
}

// wait joins the monitor goroutine.
func (e *remoteMonitorEnv) wait() {
	e.job.Wait()
}

// newRemoteExtraData builds status extra data targeting the remote node.
func newRemoteExtraData() *workceptor.RemoteExtraData {
	return &workceptor.RemoteExtraData{
		RemoteNode:   remoteIDNode,
		RemoteUnitID: "remote-unit",
		RemoteParams: make(map[string]string),
	}
}

// TestRemoteWorkStdoutErrorLogCarriesRemoteID verifies that the workceptor
// "Could not open stdout file" error, emitted while monitoring a remote work
// unit, identifies the remote peer via the structured remote_id suffix. The
// message text names only the local file, so the suffix is the only carrier
// of the peer's identity.
func TestRemoteWorkStdoutErrorLogCarriesRemoteID(t *testing.T) {
	env := newRemoteMonitorEnv(t, remoteMonitorOptions{remote: newRemoteExtraData()})

	// A path whose parent is a regular file can never be opened.
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	env.unit.EXPECT().StdoutFileName().Return(filepath.Join(blocker, "stdout")).AnyTimes()

	env.startStdout()
	env.capture.WaitFor(t, 5*time.Second, "Could not open stdout file", remoteID(remoteIDNode))
	// Join the monitor goroutine so it cannot touch the mock after the
	// test (and the gomock controller) has finished.
	env.wait()
}

// TestRemoteWorkStatusFileReadErrorLogCarriesRemoteID pins the
// "Could not read status file" line emitted by monitorRemoteStdout.
func TestRemoteWorkStatusFileReadErrorLogCarriesRemoteID(t *testing.T) {
	unitDir := t.TempDir()
	env := newRemoteMonitorEnv(t, remoteMonitorOptions{
		remote:  newRemoteExtraData(),
		status:  workceptor.StatusFileData{State: workceptor.WorkStateRunning},
		unitDir: unitDir,
	})
	env.unit.EXPECT().StdoutFileName().Return(filepath.Join(unitDir, "stdout")).AnyTimes()
	env.unit.EXPECT().Load().Return(errors.New("status unreadable")).AnyTimes()

	env.startStdout()
	env.capture.WaitFor(t, 5*time.Second, "Could not read status file", remoteID(remoteIDNode))
	env.wait()
}

// TestRemoteWorkRemoteErrorLogCarriesRemoteID pins the "Remote error" line
// emitted by monitorRemoteStatus.
func TestRemoteWorkRemoteErrorLogCarriesRemoteID(t *testing.T) {
	env := newRemoteMonitorEnv(t, remoteMonitorOptions{
		remote: newRemoteExtraData(),
		status: workceptor.StatusFileData{State: workceptor.WorkStateRunning},
		service: func(conn net.Conn, _ string) bool {
			_, _ = io.WriteString(conn, "ERROR\tboom\n")

			return false
		},
	})

	env.startStatus(false)
	env.capture.WaitFor(t, 5*time.Second, "Remote error", remoteID(remoteIDNode))
	env.wait()
}

// TestRemoteWorkStatusUnmarshalErrorLogCarriesRemoteID pins the
// "Error unmarshalling JSON" line emitted by monitorRemoteStatus.
func TestRemoteWorkStatusUnmarshalErrorLogCarriesRemoteID(t *testing.T) {
	env := newRemoteMonitorEnv(t, remoteMonitorOptions{
		remote: newRemoteExtraData(),
		status: workceptor.StatusFileData{State: workceptor.WorkStateRunning},
		service: func(conn net.Conn, _ string) bool {
			_, _ = io.WriteString(conn, "not-json\n")

			return false
		},
	})

	env.startStatus(false)
	env.capture.WaitFor(t, 5*time.Second, "Error unmarshalling JSON", remoteID(remoteIDNode))
	env.wait()
}

// TestRemoteWorkStatusWriteRetryExceededLogCarriesRemoteID pins the
// "Exceeded retries for updating status file" line emitted by
// monitorRemoteStatus.
func TestRemoteWorkStatusWriteRetryExceededLogCarriesRemoteID(t *testing.T) {
	env := newRemoteMonitorEnv(t, remoteMonitorOptions{
		remote:          newRemoteExtraData(),
		status:          workceptor.StatusFileData{State: workceptor.WorkStateRunning},
		lastUpdateError: errors.New("status write failed"),
		service: func(conn net.Conn, _ string) bool {
			_, _ = io.WriteString(conn, `{"State":1}`+"\n")

			return true
		},
	})

	env.startStatus(false)
	env.capture.WaitFor(t, 10*time.Second, "Exceeded retries for updating status file", remoteID(remoteIDNode))
	env.wait()
}

// TestRemoteWorkCryptoErrorDetailLogCarriesRemoteID pins the bare "%s" crypto
// detail line emitted by handleConnectionError.
func TestRemoteWorkCryptoErrorDetailLogCarriesRemoteID(t *testing.T) {
	red := newRemoteExtraData()
	red.RemoteStarted = false
	env := newRemoteMonitorEnv(t, remoteMonitorOptions{remote: red})

	shouldExit := workceptor.HandleConnectionErrorForTest(env.unit, errors.New("CRYPTO_ERROR: handshake failed"))
	if !shouldExit {
		t.Fatal("expected a crypto error before the remote started to be fatal")
	}

	env.capture.WaitFor(t, 5*time.Second, "TLS error connecting to remote service", remoteID(remoteIDNode))
}

// TestRemoteWorkCreateSignatureErrorLogCarriesRemoteID pins the
// "could not create signature to get results" line emitted by
// monitorRemoteStdout.
func TestRemoteWorkCreateSignatureErrorLogCarriesRemoteID(t *testing.T) {
	unitDir := t.TempDir()
	red := newRemoteExtraData()
	red.SignWork = true
	env := newRemoteMonitorEnv(t, remoteMonitorOptions{
		remote:  red,
		status:  workceptor.StatusFileData{State: workceptor.WorkStateRunning, StdoutSize: 100},
		unitDir: unitDir,
		service: func(_ net.Conn, _ string) bool {
			return true
		},
	})
	env.unit.EXPECT().StdoutFileName().Return(filepath.Join(unitDir, "stdout")).AnyTimes()
	env.unit.EXPECT().Load().Return(nil).AnyTimes()

	env.startStdout()
	env.capture.WaitFor(t, 5*time.Second, "could not create signature to get results", remoteID(remoteIDNode))
	env.wait()
}

// TestRemoteWorkCopyToStdoutErrorLogCarriesRemoteID pins the
// "Could not copy to stdout file" line emitted by monitorRemoteStdout.
func TestRemoteWorkCopyToStdoutErrorLogCarriesRemoteID(t *testing.T) {
	streamStarted := make(chan struct{})
	release := make(chan struct{})
	unitDir := t.TempDir()
	env := newRemoteMonitorEnv(t, remoteMonitorOptions{
		remote:  newRemoteExtraData(),
		status:  workceptor.StatusFileData{State: workceptor.WorkStateRunning, StdoutSize: 1000},
		unitDir: unitDir,
		service: func(conn net.Conn, _ string) bool {
			_, _ = io.WriteString(conn, "Streaming results\n")
			_, _ = io.WriteString(conn, "partial")
			close(streamStarted)
			<-release

			return false
		},
	})
	stdoutPath := filepath.Join(unitDir, "stdout")
	env.unit.EXPECT().StdoutFileName().Return(stdoutPath).AnyTimes()
	env.unit.EXPECT().Load().Return(nil).AnyTimes()

	env.startStdout()

	<-streamStarted
	// Wait until io.Copy has written the streamed bytes before cancelling the
	// read, otherwise the cancellation is observed by the earlier read and the
	// copy error this test pins is never reached.
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer waitCancel()
	if !testutils.CheckUntilTimeout(waitCtx, 5*time.Millisecond, func() bool {
		info, err := os.Stat(stdoutPath)

		return err == nil && info.Size() > 0
	}) {
		t.Fatal("timed out waiting for the stdout copy to start")
	}
	env.cancel()
	close(release)

	env.capture.WaitFor(t, 5*time.Second, "Could not copy to stdout file", remoteID(remoteIDNode))
	env.wait()
}

// TestRemoteWorkInLoopStdoutOpenErrorLogCarriesRemoteID pins the in-loop
// "Could not open stdout file" line emitted by monitorRemoteStdout.
func TestRemoteWorkInLoopStdoutOpenErrorLogCarriesRemoteID(t *testing.T) {
	unitDir := t.TempDir()
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := newRemoteMonitorEnv(t, remoteMonitorOptions{
		remote:  newRemoteExtraData(),
		status:  workceptor.StatusFileData{State: workceptor.WorkStateRunning, StdoutSize: 1000},
		unitDir: unitDir,
		service: func(conn net.Conn, _ string) bool {
			_, _ = io.WriteString(conn, "Streaming results\n")
			_, _ = io.WriteString(conn, "0123456789")

			return false
		},
	})
	// Calls 1 and 2 are the pre-loop open and the first in-loop open. The
	// third call is the second loop iteration's open, which must fail.
	stdoutCalls := 0
	env.unit.EXPECT().StdoutFileName().DoAndReturn(func() string {
		stdoutCalls++
		if stdoutCalls <= 2 {
			return filepath.Join(unitDir, "stdout")
		}

		return filepath.Join(blocker, "stdout")
	}).AnyTimes()
	env.unit.EXPECT().Load().Return(nil).AnyTimes()

	env.startStdout()
	env.capture.WaitFor(t, 10*time.Second, "Could not open stdout file", remoteID(remoteIDNode))
	env.wait()

	// The identical message is also emitted by the pre-loop open, so prove the
	// in-loop branch actually ran.
	if stdoutCalls < 3 {
		t.Errorf("expected at least 3 StdoutFileName calls (pre-loop plus two in-loop opens), got %d", stdoutCalls)
	}
}

// TestRemoteWorkActionErrorLogCarriesRemoteID pins the
// "Error running action function" line emitted by getConnectionAndRun.
func TestRemoteWorkActionErrorLogCarriesRemoteID(t *testing.T) {
	env := newRemoteMonitorEnv(t, remoteMonitorOptions{
		remote: newRemoteExtraData(),
		service: func(_ net.Conn, _ string) bool {
			return true
		},
	})

	actionDone := make(chan struct{})
	_ = workceptor.GetConnectionAndRunForTest(env.unit, env.ctx, false, func(context.Context, net.Conn, *bufio.Reader) error {
		defer close(actionDone)

		return errors.New("action boom")
	}, func() {})

	env.capture.WaitFor(t, 5*time.Second, "Error running action function", remoteID(remoteIDNode))
	// Join the action goroutine so it cannot touch the mock after the test.
	<-actionDone
}

// TestRemoteWorkConnectAndRunErrorLogCarriesRemoteID pins the
// "Error with connect and run" line emitted by cancelOrRelease.
func TestRemoteWorkConnectAndRunErrorLogCarriesRemoteID(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	n1, n2, logCapture := connectTwoNetceptorNodes(t, ctx, localNode, remoteIDNode)

	// Reply with a hello naming the wrong node so ConnectToRemote fails
	// immediately on the mismatch instead of waiting out its hello timeout.
	startFakeControlListener(t, n2, func(conn net.Conn) {
		_, _ = io.WriteString(conn, "wrong-node\n")
		_ = conn.Close()
	})
	waitForControlDial(t, n1, remoteIDNode)

	red := newRemoteExtraData()
	red.RemoteStarted = true
	w, mockBaseWorkUnit := newRemoteLogTestEnv(t, ctx, n1, red, workceptor.StatusFileData{State: workceptor.WorkStateRunning}, "", nil)
	mockBaseWorkUnit.EXPECT().Init(gomock.Any(), "", "", gomock.Any()).AnyTimes()
	mockBaseWorkUnit.EXPECT().SetStatusExtraData(gomock.Any()).AnyTimes()
	mockBaseWorkUnit.EXPECT().Release(true).Return(nil).AnyTimes()

	unit := workceptor.NewRemoteWorker(mockBaseWorkUnit, w, "", "")
	if err := unit.Release(true); err != nil {
		t.Fatal(err)
	}

	logCapture.WaitFor(t, 5*time.Second, "Error with connect and run", remoteID(remoteIDNode))
}
