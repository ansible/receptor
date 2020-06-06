package workceptor

import (
	"fmt"
	"github.com/ghjm/sockceptor/pkg/cmdline"
	"github.com/ghjm/sockceptor/pkg/debug"
	"github.com/ghjm/sockceptor/pkg/randstr"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

// CommandWorker is a worker type that executes commands
type CommandWorker struct {
	command   string
	processes map[string]*commandInfo
}

type commandInfo struct {
	cmd    *exec.Cmd
	done   *bool
	stdout *os.File
	stderr *os.File
}

// cmdWaiter hangs around and waits for the command to be done because apparently you
// can't safely call exec.Cmd.Exited() unless you already know the command has exited.
func cmdWaiter(cmd *commandInfo) {
	_ = cmd.cmd.Wait()
	*cmd.done = true
}

func tempfile() (*os.File, error) {
	f, err := ioutil.TempFile(os.TempDir(), "receptor*.tmp")
	if err != nil {
		return nil, err
	}
	// Pre-remove the file
	err = os.Remove(f.Name())
	if err != nil {
		debug.Printf("Error pre-removing temp file: %s\n", err)
	}
	return f, nil
}

// Start launches a job with given parameters.  It returns an identifier string and an error.
func (cw *CommandWorker) Start(param string) (string, error) {
	var ident string
	for {
		ident = randstr.RandomString(8)
		_, ok := cw.processes[ident]
		if !ok {
			break
		}
	}
	cmd := exec.Command(cw.command, strings.Split(param, " ")...)
	stdout, err := tempfile()
	if err != nil {
		return "", err
	}
	cmd.Stdout = stdout
	stderr, err := tempfile()
	if err != nil {
		return "", err
	}
	cmd.Stderr = stderr
	err = cmd.Start()
	if err != nil {
		return "", err
	}
	done := false
	ci := &commandInfo{
		cmd:    cmd,
		done:   &done,
		stdout: stdout,
		stderr: stderr,
	}
	go cmdWaiter(ci)
	cw.processes[ident] = ci
	return ident, nil
}

// Status returns the status of a previously job identified by the identifier.
// The return values are running (bool), status detail, and error.
func (cw *CommandWorker) Status(identifier string) (bool, bool, string, error) {
	cmd, ok := cw.processes[identifier]
	if !ok {
		return false, false, "", fmt.Errorf("unknown identifier")
	}
	if !*cmd.done {
		return false, false, fmt.Sprintf("Running: PID %d", cmd.cmd.Process.Pid), nil
	}
	return *cmd.done, cmd.cmd.ProcessState.Success(), cmd.cmd.ProcessState.String(), nil
}

// List lists the tasks known to this
func (cw *CommandWorker) List() ([]string, error) {
	procs := make([]string, 0, len(cw.processes))
	for proc := range cw.processes {
		procs = append(procs, proc)
	}
	return procs, nil
}

// Cancel cancels a running job.
func (cw *CommandWorker) Cancel(identifier string) error {
	cmd, ok := cw.processes[identifier]
	if !ok {
		return fmt.Errorf("unknown identifier")
	}
	return cmd.cmd.Process.Kill()
}

// Get gets an output stream from a job.
func (cw *CommandWorker) Get(identifier string, streamID string) (io.ReadCloser, error) {
	cmd, ok := cw.processes[identifier]
	if !ok {
		return nil, fmt.Errorf("unknown identifier")
	}
	if strings.ToLower(streamID) == "stdout" {
		_, err := cmd.stdout.Seek(0, io.SeekStart)
		if err != nil {
			return nil, err
		}
		return cmd.stdout, nil
	} else if strings.ToLower(streamID) == "stderr" {
		_, err := cmd.stderr.Seek(0, io.SeekStart)
		if err != nil {
			return nil, err
		}
		return cmd.stderr, nil
	}
	return nil, fmt.Errorf("unknown stream identifier")
}

// **************************************************************************
// Command line
// **************************************************************************

// CommandCfg is the cmdline configuration object for a worker that runs a command
type CommandCfg struct {
	Service string `required:"true" description:"Local Receptor service name to bind to"`
	Command string `required:"true" description:"Command to run to process units of work"`
}

// Run runs the action
func (cfg CommandCfg) Run() error {
	err := MainInstance().RegisterWorker(cfg.Service, &CommandWorker{
		command:   cfg.Command,
		processes: make(map[string]*commandInfo),
	})
	return err
}

func init() {
	cmdline.AddConfigType("work-command", "Run a worker using an external command", CommandCfg{}, false, workersSection)
}
