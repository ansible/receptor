package mesh

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/workceptor"
	"github.com/ansible/receptor/tests/utils"
)

const signingPolicyOutput = "signing-policy-ok\n"

// signingPolicyCase is one row of the signed-work behavior matrix: whether the
// client asked for signing, which keys exist, whether the target work type
// verifies, and the expected outcome.
type signingPolicyCase struct {
	name      string
	signWork  bool
	originKey bool
	targetKey bool
	verify    bool
	wantRun   bool
	wantErr   string
}

func (tc signingPolicyCase) originNode() string {
	if tc.originKey {
		return "origin-signed"
	}

	return "origin-plain"
}

func (tc signingPolicyCase) targetNode() string {
	key := "nokey"
	if tc.targetKey {
		key = "key"
	}
	verify := "noverify"
	if tc.verify {
		verify = "verify"
	}

	return fmt.Sprintf("target-%s-%s", key, verify)
}

var signingPolicyCases = []signingPolicyCase{
	// Unsigned submissions never consult either key, so only the target's
	// verifysignature policy decides whether they are accepted.
	{name: "unsigned no keys no verify", wantRun: true},
	{name: "unsigned no keys verify", verify: true, wantErr: "signature is empty"},
	{name: "unsigned target key no verify", targetKey: true, wantRun: true},
	{name: "unsigned target key verify", targetKey: true, verify: true, wantErr: "signature is empty"},
	{name: "unsigned origin key no target key no verify", originKey: true, wantRun: true},
	{name: "unsigned origin key no target key verify", originKey: true, verify: true, wantErr: "signature is empty"},
	{name: "unsigned both keys no verify", originKey: true, targetKey: true, wantRun: true},
	{name: "unsigned both keys verify", originKey: true, targetKey: true, verify: true, wantErr: "signature is empty"},

	// A signed submission without a signing key fails on the origin before the
	// target sees it.
	{name: "signed no origin key no target key no verify", signWork: true, wantErr: "cannot sign work"},
	{name: "signed no origin key no target key verify", signWork: true, verify: true, wantErr: "cannot sign work"},
	{name: "signed no origin key target key no verify", signWork: true, targetKey: true, wantErr: "cannot sign work"},
	{name: "signed no origin key target key verify", signWork: true, targetKey: true, verify: true, wantErr: "cannot sign work"},

	// A signed submission is rejected by a target that does not verify, and by
	// a verifying target that has no key.
	{name: "signed origin key no target key no verify", signWork: true, originKey: true, wantErr: "did not expect a signature"},
	{name: "signed origin key no target key verify", signWork: true, originKey: true, verify: true, wantErr: "verifying key not specified"},
	{name: "signed origin key target key no verify", signWork: true, originKey: true, targetKey: true, wantErr: "did not expect a signature"},
	{name: "signed both keys verify", signWork: true, originKey: true, targetKey: true, verify: true, wantRun: true},
}

// newSigningPolicyMesh builds one mesh containing both origins and all four
// target configurations. In the direct topology each origin connects to every
// target; in the hop topology both origins reach the targets only through a
// shared hub, so every submission crosses a relay node.
func newSigningPolicyMesh(t *testing.T, topology, privateKey, publicKey string) LibMesh {
	t.Helper()

	m := NewLibMesh()

	targetCfgs := []struct {
		id     string
		key    bool
		verify bool
	}{
		{id: "target-nokey-noverify"},
		{id: "target-nokey-verify", verify: true},
		{id: "target-key-noverify", key: true},
		{id: "target-key-verify", key: true, verify: true},
	}

	targets := make(map[string]*LibNode, len(targetCfgs))
	for _, cfg := range targetCfgs {
		node := m.NewLibNode(cfg.id)
		node.ListenerCfgs = map[listenerName]ListenerCfg{
			listenerName("tcp"): newListenerCfg("tcp", "", 1, nil),
		}
		node.workerConfigs = []workceptor.WorkerConfig{
			workceptor.CommandWorkerCfg{
				WorkType:        "echo",
				Command:         "bash",
				Params:          "-c 'echo signing-policy-ok'",
				VerifySignature: cfg.verify,
			},
		}
		if cfg.key {
			node.WorkVerificationKey = &workceptor.VerifyingKeyPublicCfg{PublicKey: publicKey}
		}
		targets[cfg.id] = node
	}

	var hub *LibNode
	if topology == "hop" {
		hub = m.NewLibNode("hub")
		hub.ListenerCfgs = map[listenerName]ListenerCfg{
			listenerName("tcp"): newListenerCfg("tcp", "", 1, nil),
		}
		for _, node := range targets {
			hub.Connections = append(hub.Connections, Connection{RemoteNode: node, Protocol: "tcp"})
		}
	}

	originCfgs := []struct {
		id  string
		key bool
	}{
		{id: "origin-plain"},
		{id: "origin-signed", key: true},
	}
	for _, cfg := range originCfgs {
		node := m.NewLibNode(cfg.id)
		if cfg.key {
			node.WorkSigningKey = &workceptor.SigningKeyPrivateCfg{PrivateKey: privateKey}
		}
		if topology == "hop" {
			node.Connections = []Connection{{RemoteNode: hub, Protocol: "tcp"}}
		} else {
			for _, target := range targets {
				node.Connections = append(node.Connections, Connection{RemoteNode: target, Protocol: "tcp"})
			}
		}
	}

	return m
}

// TestWorkSigningPolicyMatrix pins the behavior of every signed/unsigned and
// key/verify combination, in both a direct mesh and through a relay node. The
// cases where work runs also assert that results are readable from the
// submitting node, which is where a local verification key used to break
// unsigned remote work.
func TestWorkSigningPolicyMatrix(t *testing.T) {
	t.Parallel()

	privateKey, publicKey, err := utils.GenerateRSAPair()
	if err != nil {
		t.Fatal(err)
	}

	for _, topology := range []string{"direct", "hop"} {
		topology := topology
		t.Run(topology, func(t *testing.T) {
			t.Parallel()

			m := newSigningPolicyMesh(t, topology, privateKey, publicKey)
			if err := m.Start(t.Name()); err != nil {
				t.Fatal(err)
			}
			defer m.WaitForShutdown()
			defer m.Destroy()

			readyCtx, readyCancel := context.WithTimeout(context.Background(), 180*time.Second)
			defer readyCancel()
			if err := m.WaitForReady(readyCtx); err != nil {
				t.Fatal(err, m.GetDataDir())
			}

			for _, tc := range signingPolicyCases {
				tc := tc
				t.Run(tc.name, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
					defer cancel()

					controller := NewReceptorControl()
					if err := controller.Connect(m.GetNodes()[tc.originNode()].GetControlSocket()); err != nil {
						t.Fatal(err, m.GetDataDir())
					}
					defer func() { _ = controller.Close() }()

					signwork := ""
					if tc.signWork {
						signwork = `,"signwork":"true"`
					}
					job := fmt.Sprintf(`{"command":"work","subcommand":"submit","worktype":"echo","node":%q%s}`,
						tc.targetNode(), signwork)

					unitID, err := controller.WorkSubmitJSON(job)
					if !tc.wantRun {
						if err == nil {
							t.Fatalf("submit unexpectedly succeeded with unit %s", unitID)
						}
						if !strings.Contains(err.Error(), tc.wantErr) {
							t.Fatalf("submit error = %q, want it to contain %q", err, tc.wantErr)
						}

						return
					}
					if err != nil {
						t.Fatal(err, m.GetDataDir())
					}
					if err := controller.AssertWorkSucceeded(ctx, unitID); err != nil {
						t.Fatal(err, m.GetDataDir())
					}
					if err := controller.AssertWorkResults(unitID, []byte(signingPolicyOutput)); err != nil {
						t.Fatal(err, m.GetDataDir())
					}
				})
			}
		})
	}
}
