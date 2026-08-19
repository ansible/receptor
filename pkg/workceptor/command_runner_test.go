package workceptor

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"

	"github.com/ansible/receptor/pkg/netceptor"
)

// setupCommandRunnerTest creates a Workceptor backed by a real netceptor,
// sets MainInstance, and returns a cancellable context, the unit directory
// path, and a cleanup func that cancels the context and tears down resources.
func setupCommandRunnerTest(t *testing.T) (context.Context, string, func()) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())

	tmpdir := t.TempDir()

	nc := netceptor.New(ctx, "test-cmd-runner")
	w, err := New(ctx, nc, tmpdir)
	if err != nil {
		cancel()
		nc.Shutdown()
		t.Fatal(err)
	}

	originalMainInstance := MainInstance
	MainInstance = w

	unitdir := path.Join(tmpdir, "unit1")
	if err := os.MkdirAll(unitdir, 0o700); err != nil {
		cancel()
		nc.Shutdown()
		MainInstance = originalMainInstance
		t.Fatal(err)
	}

	cleanup := func() {
		cancel()
		MainInstance = originalMainInstance
		nc.Shutdown()
	}

	return ctx, unitdir, cleanup
}

// subprocessTestCmd builds an exec.Cmd that re-invokes this test binary
// with the subprocess env vars set. The supplied context controls the
// lifetime of the subprocess: if the context is cancelled (e.g. test
// timeout), the subprocess is killed automatically.
func subprocessTestCmd(ctx context.Context, t *testing.T, testName string, unitdir string, command string, params string) *exec.Cmd {
	t.Helper()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^"+testName+"$", "-test.v") //nolint:gosec // G702: re-invoking test binary is intentional

	// Filter out RECEPTOR_PAYLOAD_TRACE_LEVEL to ensure we always exercise
	// the standard stdin code path rather than the payload-debug branch.
	var env []string
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "RECEPTOR_PAYLOAD_TRACE_LEVEL=") {
			env = append(env, e)
		}
	}
	env = append(env,
		"TEST_COMMAND_RUNNER_SUBPROCESS=1",
		"TEST_COMMAND_RUNNER_COMMAND="+command,
		"TEST_COMMAND_RUNNER_PARAMS="+params,
		"TEST_COMMAND_RUNNER_UNITDIR="+unitdir,
	)
	cmd.Env = env

	return cmd
}

// testCommandRunnerSubprocess runs inside the re-exec'd subprocess.
// commandRunner calls os.Exit on completion, so deferred cleanup will not
// execute in the normal path. Explicit cleanup is only reachable when
// commandRunner returns an early error (before cmd.Start).
func testCommandRunnerSubprocess(t *testing.T) {
	t.Helper()
	command := os.Getenv("TEST_COMMAND_RUNNER_COMMAND")
	params := os.Getenv("TEST_COMMAND_RUNNER_PARAMS")
	unitdir := os.Getenv("TEST_COMMAND_RUNNER_UNITDIR")

	ctx, cancel := context.WithCancel(context.Background())

	nc := netceptor.New(ctx, "test-subprocess")

	tmpdir := t.TempDir()

	w, err := New(ctx, nc, tmpdir)
	if err != nil {
		cancel()
		nc.Shutdown()
		t.Fatalf("failed to create workceptor: %v", err)
	}
	MainInstance = w

	// commandRunner calls os.Exit on completion; this point is only
	// reached when commandRunner returns an error before starting the process.
	err = commandRunner(command, params, unitdir)
	cancel()
	nc.Shutdown()
	if err != nil {
		t.Fatalf("commandRunner error: %v", err)
	}
}

func TestCombineParams(t *testing.T) {
	tests := []struct {
		name       string
		baseParams string
		userParams string
		expected   string
	}{
		{name: "both empty", baseParams: "", userParams: "", expected: ""},
		{name: "only baseParams", baseParams: "--verbose", userParams: "", expected: "--verbose"},
		{name: "only userParams", baseParams: "", userParams: "--debug", expected: "--debug"},
		{name: "both present", baseParams: "--verbose", userParams: "--debug", expected: "--verbose --debug"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := combineParams(tt.baseParams, tt.userParams)
			if got != tt.expected {
				t.Errorf("combineParams(%q, %q) = %q, want %q", tt.baseParams, tt.userParams, got, tt.expected)
			}
		})
	}
}

// TestCommandRunnerBadShlexParams covers the shlex.Split error path.
func TestCommandRunnerBadShlexParams(t *testing.T) {
	_, unitdir, cleanup := setupCommandRunnerTest(t)
	defer cleanup()

	echoBin, err := exec.LookPath("echo")
	if err != nil {
		t.Skipf("echo not found in PATH: %v", err)
	}

	err = commandRunner(echoBin, "\"unterminated", unitdir)
	if err == nil {
		t.Fatal("expected error from shlex.Split for unterminated quote, got nil")
	}
}

// TestCommandRunnerMissingStdin covers the stdin open failure.
// Use /usr/bin/true as the binary to execute by commandRunner.
func TestCommandRunnerMissingStdin(t *testing.T) {
	_, unitdir, cleanup := setupCommandRunnerTest(t)
	defer cleanup()

	trueBin, err := exec.LookPath("true")
	if err != nil {
		t.Skipf("true not found in PATH: %v", err)
	}

	// Do not create unitdir/stdin
	err = commandRunner(trueBin, "", unitdir)
	if err == nil {
		t.Fatal("expected error for missing stdin file, got nil")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected fs.ErrNotExist, got: %v", err)
	}
}

// TestCommandRunnerStartFailure covers cmd.Start() failure for a nonexistent binary.
func TestCommandRunnerStartFailure(t *testing.T) {
	tests := []struct {
		name    string
		command string
		params  string
	}{
		{name: "without params", command: "/nonexistent/binary/path", params: ""},
		{name: "with params", command: "/nonexistent/binary", params: "--flag value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, unitdir, cleanup := setupCommandRunnerTest(t)
			defer cleanup()

			if err := os.WriteFile(path.Join(unitdir, "stdin"), []byte{}, 0o600); err != nil {
				t.Fatal(err)
			}

			err := commandRunner(tt.command, tt.params, unitdir)
			if err == nil {
				t.Fatal("expected error from cmd.Start() for nonexistent binary, got nil")
			}
		})
	}
}

// TestCommandRunnerWritesInitialStatus verifies the initial status file is written.
func TestCommandRunnerWritesInitialStatus(t *testing.T) {
	_, unitdir, cleanup := setupCommandRunnerTest(t)
	defer cleanup()

	if err := os.WriteFile(path.Join(unitdir, "stdin"), []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	_ = commandRunner("/nonexistent/binary/for/status/test", "", unitdir)

	statusFile := path.Join(unitdir, "status")
	data, err := os.ReadFile(statusFile)
	if err != nil {
		t.Fatalf("expected status file to exist: %v", err)
	}

	var status StatusFileData
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatalf("failed to unmarshal status file: %v", err)
	}
	if status.State != WorkStatePending {
		t.Errorf("expected State=%d (WorkStatePending), got %d", WorkStatePending, status.State)
	}
	if status.Detail != "Not started yet" {
		t.Errorf("expected Detail=%q, got %q", "Not started yet", status.Detail)
	}
}

// TestCommandRunnerExecution uses the subprocess test pattern to cover
// successful and failed execution paths. commandRunner calls os.Exit,
// so we re-exec the test binary in a subprocess.
func TestCommandRunnerExecution(t *testing.T) {
	if os.Getenv("TEST_COMMAND_RUNNER_SUBPROCESS") == "1" {
		testCommandRunnerSubprocess(t)

		return
	}

	tests := []struct {
		name          string
		binary        string
		params        string
		expectErr     bool
		expectedState int
	}{
		{name: "successful execution", binary: "true", params: "", expectErr: false, expectedState: WorkStateSucceeded},
		{name: "successful execution with params", binary: "echo", params: "hello world", expectErr: false, expectedState: WorkStateSucceeded},
		{name: "failed execution", binary: "false", params: "", expectErr: true, expectedState: WorkStateFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, unitdir, cleanup := setupCommandRunnerTest(t)
			defer cleanup()

			if err := os.WriteFile(path.Join(unitdir, "stdin"), []byte{}, 0o600); err != nil {
				t.Fatal(err)
			}

			bin, err := exec.LookPath(tt.binary)
			if err != nil {
				t.Skipf("%s not found in PATH: %v", tt.binary, err)
			}

			cmd := subprocessTestCmd(ctx, t, "TestCommandRunnerExecution", unitdir, bin, tt.params)
			output, err := cmd.CombinedOutput()
			if tt.expectErr && err == nil {
				t.Fatal("expected subprocess to exit with non-zero code, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Fatalf("subprocess exited with error: %v\noutput: %s", err, output)
			}

			data, err := os.ReadFile(path.Join(unitdir, "status"))
			if err != nil {
				t.Fatalf("failed to read status file: %v", err)
			}
			var status StatusFileData
			if err := json.Unmarshal(data, &status); err != nil {
				t.Fatalf("failed to unmarshal status: %v", err)
			}
			if status.State != tt.expectedState {
				t.Errorf("expected state %d, got %d (detail: %s)", tt.expectedState, status.State, status.Detail)
			}
		})
	}
}

func TestCommandWorkerCfgRun(t *testing.T) {
	tests := []struct {
		name        string
		cfg         CommandWorkerCfg
		expectedErr string
	}{
		{
			name: "VerifySignature with empty VerifyingKey",
			cfg: CommandWorkerCfg{
				WorkType:        "test-cmd",
				Command:         "/bin/echo",
				VerifySignature: true,
			},
			expectedErr: "VerifySignature for work command 'test-cmd' is true, but the work verification public key is not specified",
		},
		{
			name: "Successful registration",
			cfg: CommandWorkerCfg{
				WorkType: "test-cmd-ok",
				Command:  "/bin/echo",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, cleanup := setupCommandRunnerTest(t)
			defer cleanup()

			err := tt.cfg.Run()
			if tt.expectedErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.expectedErr)
				}
				if err.Error() != tt.expectedErr {
					t.Errorf("unexpected error message:\ngot:  %s\nwant: %s", err.Error(), tt.expectedErr)
				}
			}
		})
	}
}
