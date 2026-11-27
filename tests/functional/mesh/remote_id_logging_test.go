package mesh

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/netceptor"
	testsutils "github.com/ansible/receptor/tests/utils"
)

var (
	remoteIDFixturesOnce                   sync.Once
	remoteIDFixturesErr                    error
	remoteIDCAKey, remoteIDCACrt           string
	remoteIDControlKey, remoteIDControlCrt string
)

// remoteIDFixtures returns the shared CA and the control node's server
// certificate for the remote_id TLS tests, generating them once per test
// binary rather than once per test.
func remoteIDFixtures(t *testing.T) (caKey, caCrt, controlKey, controlCrt string) {
	t.Helper()

	remoteIDFixturesOnce.Do(func() {
		remoteIDCAKey, remoteIDCACrt, remoteIDFixturesErr = testsutils.GenerateCA("ca", "localhost")
		if remoteIDFixturesErr != nil {
			return
		}
		remoteIDControlKey, remoteIDControlCrt, remoteIDFixturesErr = testsutils.GenerateCertWithCA(
			"control", remoteIDCAKey, remoteIDCACrt, "control", []string{"localhost"}, []string{"control"})
	})
	if remoteIDFixturesErr != nil {
		t.Fatalf("failed to generate remote_id TLS fixtures: %v", remoteIDFixturesErr)
	}

	return remoteIDCAKey, remoteIDCACrt, remoteIDControlKey, remoteIDControlCrt
}

// mustGenerateCert signs a certificate with the given CA, failing the test on
// error.
func mustGenerateCert(t *testing.T, name, caKey, caCrt, commonName string, dnsNames, nodeIDs []string) (string, string) {
	t.Helper()

	key, crt, err := testsutils.GenerateCertWithCA(name, caKey, caCrt, commonName, dnsNames, nodeIDs)
	if err != nil {
		t.Fatalf("failed to generate certificate %s: %v", name, err)
	}

	return key, crt
}

// dialSpec describes one dialAndExpect case.
type dialSpec struct {
	serverCfg string
	svcName   string
	clientCfg string
	captureID string
	want      []string
}

// dialAndExpect starts an in-memory TLS service on control and dials it from
// execA, then asserts the expected log lines on the captured node. The
// handshake rejection may be reported during or just after the handshake, so
// Dial's error is ignored and the log line is the assertion.
func dialAndExpect(t *testing.T, control, execA *LibNode, spec dialSpec, captures map[string]*testsutils.LogCapture) {
	t.Helper()

	serverTLS, err := control.GetNetceptor().GetServerTLSConfig(spec.serverCfg)
	if err != nil {
		t.Fatalf("failed to prepare server TLS config: %v", err)
	}
	ln, err := control.GetNetceptor().Listen(spec.svcName, serverTLS)
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()
	go func() {
		if conn, err := ln.Accept(); err == nil {
			_ = conn.Close()
		}
	}()

	clientTLS, err := execA.GetNetceptor().GetClientTLSConfig(spec.clientCfg, "control", netceptor.ExpectedHostnameTypeReceptor)
	if err != nil {
		t.Fatalf("failed to prepare client TLS config: %v", err)
	}
	if conn, err := execA.GetNetceptor().Dial("control", spec.svcName, clientTLS); err == nil {
		_ = conn.CloseConnection()
	}

	captures[spec.captureID].WaitFor(t, 15*time.Second, spec.want...)
}

// startRemoteIDMesh starts a two-node mesh, capturing the requested nodes'
// logs. configure runs before Start so TLS configs are registered in time.
// After readiness it asserts the handshake-assigned remote_id on each captured
// node, so tests can rely on it being present when they trigger their line.
func startRemoteIDMesh(t *testing.T, captureIDs []string, configure func(t *testing.T, control, execA *LibNode)) (*LibMesh, map[string]*testsutils.LogCapture) {
	t.Helper()

	m := twoNodeMesh()
	nodes := m.GetNodes()

	captures := make(map[string]*testsutils.LogCapture, len(captureIDs))
	for _, id := range captureIDs {
		captures[id] = testsutils.NewLogCapture()
	}
	m.AfterInit = func(ns map[string]*LibNode) {
		for id, capture := range captures {
			ns[id].netceptorInstance.Logger.SetOutput(capture)
		}
	}

	if configure != nil {
		configure(t, nodes["control"], nodes["execution-a"])
	}

	if err := m.Start(t.Name()); err != nil {
		t.Fatalf("failed to start mesh: %v", err)
	}
	t.Cleanup(m.WaitForShutdown)
	t.Cleanup(m.Destroy)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := m.WaitForReady(ctx); err != nil {
		t.Fatalf("mesh did not become ready: %v (data dir %s)", err, m.DataDir)
	}

	for id, capture := range captures {
		peer := "execution-a"
		if id == "execution-a" {
			peer = "control"
		}
		capture.WaitFor(t, 10*time.Second, `"remote_id":"`+peer+`"`)
	}

	return m, captures
}

// TestRemoteIDTLSMissingClientCert triggers the server-side
// "RVF failed: peer certificate missing" line on control. It happens before the
// Receptor handshake, so no remote_id is known; the line is attributed to the
// transport peer via remote_addr instead.
func TestRemoteIDTLSMissingClientCert(t *testing.T) {
	t.Parallel()

	m, captures := startRemoteIDMesh(t, []string{"control"}, func(t *testing.T, control, execA *LibNode) {
		_, caCrt, srvKey, srvCrt := remoteIDFixtures(t)

		control.TLSServerConfigs = append(control.TLSServerConfigs, &netceptor.TLSServerConfig{
			Name: "srv-missing", Key: srvKey, Cert: srvCrt,
			ClientCAs: caCrt,
		})
		execA.TLSClientConfigs = append(execA.TLSClientConfigs, &netceptor.TLSClientConfig{
			Name: "cli-nocert", RootCAs: caCrt,
		})
	})

	control := m.GetNodes()["control"]
	execA := m.GetNodes()["execution-a"]

	bi, err := control.TCPListen(newListenerCfg("tcp", "srv-missing", 1, nil))
	if err != nil {
		t.Fatalf("failed to start TLS listener: %v", err)
	}
	clientTLS, err := execA.GetNetceptor().GetClientTLSConfig("cli-nocert", "localhost", netceptor.ExpectedHostnameTypeDNS)
	if err != nil {
		t.Fatalf("failed to prepare client TLS config: %v", err)
	}
	if err := execA.TCPDial(bi.listener.GetAddr(), 1.0, clientTLS); err != nil {
		t.Fatalf("failed to start TLS dial: %v", err)
	}

	captures["control"].WaitFor(t, 15*time.Second,
		"RVF failed: peer certificate missing",
		`"remote_addr":"`)
}

// TestRemoteIDTLSPinnedClientCertMismatch triggers the server-side pinned
// fingerprint failure on control.
func TestRemoteIDTLSPinnedClientCertMismatch(t *testing.T) {
	t.Parallel()

	m, captures := startRemoteIDMesh(t, []string{"control"}, func(t *testing.T, control, execA *LibNode) {
		caKey, caCrt, srvKey, srvCrt := remoteIDFixtures(t)
		cliKey, cliCrt := mustGenerateCert(t, "execution-a", caKey, caCrt, "execution-a", nil, []string{"execution-a"})

		control.TLSServerConfigs = append(control.TLSServerConfigs, &netceptor.TLSServerConfig{
			Name: "srv-pinned", Key: srvKey, Cert: srvCrt,
			ClientCAs:         caCrt,
			RequireClientCert: true,
			PinnedClientCert:  []string{strings.Repeat("00", 32)},
		})
		execA.TLSClientConfigs = append(execA.TLSClientConfigs, &netceptor.TLSClientConfig{
			Name: "cli-cert", Key: cliKey, Cert: cliCrt, RootCAs: caCrt,
		})
	})

	control := m.GetNodes()["control"]
	execA := m.GetNodes()["execution-a"]

	bi, err := control.TCPListen(newListenerCfg("tcp", "srv-pinned", 1, nil))
	if err != nil {
		t.Fatalf("failed to start TLS listener: %v", err)
	}
	clientTLS, err := execA.GetNetceptor().GetClientTLSConfig("cli-cert", "localhost", netceptor.ExpectedHostnameTypeDNS)
	if err != nil {
		t.Fatalf("failed to prepare client TLS config: %v", err)
	}
	if err := execA.TCPDial(bi.listener.GetAddr(), 1.0, clientTLS); err != nil {
		t.Fatalf("failed to start TLS dial: %v", err)
	}

	captures["control"].WaitFor(t, 15*time.Second,
		"presented certificate does not match any pinned fingerprint",
		`"remote_id":"execution-a"`)
}

// TestRemoteIDTLSVerifyFailure triggers the client-side "RVF failed verify"
// line on execution-a by dialing control's auto-certified service with a client
// config that trusts no roots.
func TestRemoteIDTLSVerifyFailure(t *testing.T) {
	t.Parallel()

	m, captures := startRemoteIDMesh(t, []string{"execution-a"}, func(t *testing.T, _, execA *LibNode) {
		execA.TLSClientConfigs = append(execA.TLSClientConfigs, &netceptor.TLSClientConfig{
			Name: "svc-badchain",
		})
	})

	execA := m.GetNodes()["execution-a"]

	clientTLS, err := execA.GetNetceptor().GetClientTLSConfig("svc-badchain", "control", netceptor.ExpectedHostnameTypeReceptor)
	if err != nil {
		t.Fatalf("failed to prepare client TLS config: %v", err)
	}
	// The failure may surface during or just after the handshake; the log line
	// is the assertion.
	if conn, err := execA.GetNetceptor().Dial("control", "control", clientTLS); err == nil {
		_ = conn.CloseConnection()
	}

	captures["execution-a"].WaitFor(t, 15*time.Second,
		"RVF failed verify",
		`"remote_id":"control"`)
}

// TestRemoteIDTLSDialRejections covers TLS configuration rejections that are
// only observable via the log line: the server rejecting execution-a's
// certificate for naming the wrong Receptor node, and the client failing to
// decode a malformed Receptor otherName in control's certificate.
func TestRemoteIDTLSDialRejections(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		configure func(t *testing.T, control, execA *LibNode)
		serverCfg string
		svcName   string
		clientCfg string
		captureID string
		want      []string
	}{
		{
			name: "client certificate name mismatch",
			configure: func(t *testing.T, control, execA *LibNode) {
				t.Helper()
				caKey, caCrt, srvKey, srvCrt := remoteIDFixtures(t)
				badKey, badCrt := mustGenerateCert(t, "wrong-node", caKey, caCrt, "wrong-node", nil, []string{"wrong-node"})

				control.TLSServerConfigs = append(control.TLSServerConfigs, &netceptor.TLSServerConfig{
					Name: "svc-server", Key: srvKey, Cert: srvCrt,
					ClientCAs:         caCrt,
					RequireClientCert: true,
				})
				execA.TLSClientConfigs = append(execA.TLSClientConfigs, &netceptor.TLSClientConfig{
					Name: "bad-client", Key: badKey, Cert: badCrt,
					RootCAs:                caCrt,
					SkipReceptorNamesCheck: true,
				})
			},
			serverCfg: "svc-server",
			svcName:   "tlsvc",
			clientCfg: "bad-client",
			captureID: "control",
			want: []string{
				"x509: certificate is valid for Receptor node ID wrong-node, not execution-a",
				`"remote_id":"execution-a"`,
			},
		},
		{
			name: "malformed receptor names",
			configure: func(t *testing.T, control, execA *LibNode) {
				t.Helper()
				caKey, caCrt, _, _ := remoteIDFixtures(t)
				badKey, badCrt := generateMalformedSANCert(t, caKey, caCrt)

				control.TLSServerConfigs = append(control.TLSServerConfigs, &netceptor.TLSServerConfig{
					Name: "svc-malformed", Key: badKey, Cert: badCrt,
					SkipReceptorNamesCheck: true,
				})
				execA.TLSClientConfigs = append(execA.TLSClientConfigs, &netceptor.TLSClientConfig{
					Name: "svc-client", RootCAs: caCrt,
				})
			},
			serverCfg: "svc-malformed",
			svcName:   "malsvc",
			clientCfg: "svc-client",
			captureID: "execution-a",
			want: []string{
				"RVF failed to get ReceptorNames",
				`"remote_id":"control"`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m, captures := startRemoteIDMesh(t, []string{tc.captureID}, tc.configure)
			dialAndExpect(t, m.GetNodes()["control"], m.GetNodes()["execution-a"],
				dialSpec{
					serverCfg: tc.serverCfg,
					svcName:   tc.svcName,
					clientCfg: tc.clientCfg,
					captureID: tc.captureID,
					want:      tc.want,
				}, captures)
		})
	}
}

// TestRemoteIDCloseConnectionDebug triggers the debug "closing connection from
// service" line on execution-a. It is not parallel because it raises the
// process-wide log level.
func TestRemoteIDCloseConnectionDebug(t *testing.T) {
	m, captures := startRemoteIDMesh(t, []string{"execution-a"}, nil)

	testsutils.EnableDebugLogs(t)

	control := m.GetNodes()["control"]
	execA := m.GetNodes()["execution-a"]

	ln, err := control.GetNetceptor().Listen("echo", nil)
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	accepted := make(chan struct{})
	go func() {
		defer close(accepted)
		if conn, err := ln.Accept(); err == nil {
			defer conn.Close()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var conn *netceptor.Conn
	if !testsutils.CheckUntilTimeout(ctx, 100*time.Millisecond, func() bool {
		conn, err = execA.GetNetceptor().Dial("control", "echo", nil)

		return err == nil
	}) {
		t.Fatalf("failed to dial the echo service: %v", err)
	}
	if err := conn.CloseConnection(); err != nil {
		t.Fatalf("CloseConnection returned error: %v", err)
	}

	captures["execution-a"].WaitFor(t, 10*time.Second,
		"closing connection from service",
		"to control:echo",
		`"remote_id":"control"`)

	<-accepted
}
