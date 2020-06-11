package workceptor

import (
	"fmt"
	"github.com/ghjm/sockceptor/pkg/cmdline"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// commandUnit implements the WorkUnit interface
type commandUnit struct {
	command        string
	cmd            *exec.Cmd
	done           *bool
	stdinFilename  string
	stdoutFilename string
}

// cmdWaiter hangs around and waits for the command to be done because apparently you
// can't safely call exec.Cmd.Exited() unless you already know the command has exited.
func cmdWaiter(cmd *commandUnit) {
	_ = cmd.cmd.Wait()
	*cmd.done = true
}

// Start launches a job with given parameters.  It returns an identifier string and an error.
func (cw *commandUnit) Start(params string, stdinFilename string) error {
	var cmd *exec.Cmd
	if params == "" {
		cmd = exec.Command(cw.command)
	} else {
		cmd = exec.Command(cw.command, strings.Split(params, " ")...)
	}
	stdin, err := os.Open(stdinFilename)
	if err != nil {
		return err
	}
	cmd.Stdin = stdin
	stdout, err := ioutil.TempFile(os.TempDir(), "receptor-stdout*.tmp")
	if err != nil {
		return err
	}
	cw.stdoutFilename, err = filepath.Abs(stdout.Name())
	if err != nil {
		return err
	}
	cmd.Stdout = stdout
	cmd.Stderr = stdout
	done := false
	cw.done = &done
	err = cmd.Start()
	if err != nil {
		return err
	}
	cw.cmd = cmd
	go cmdWaiter(cw)
	return nil
}

// Status returns the status of a previously job identified by the identifier.
// The return values are running (bool), status detail, and error.
func (cw *commandUnit) Status() (state int, detail string, err error) {
	if cw.done != nil && *cw.done {
		if cw.cmd.ProcessState.Success() {
			return WorkStateSucceeded, cw.cmd.ProcessState.String(), nil
		}
		return WorkStateFailed, cw.cmd.ProcessState.String(), nil
	}
	if cw.cmd != nil {
		return WorkStateRunning, fmt.Sprintf("Running: PID %d", cw.cmd.Process.Pid), nil
	}
	return WorkStatePending, "Not started yet", nil
}

// Release releases resources associated with a job, including cancelling it if running.
func (cw *commandUnit) Release() error {
	if cw.cmd != nil && !*cw.done {
		err := cw.cmd.Process.Kill()
		if err != nil {
			return err
		}
	}
	err1 := os.Remove(cw.stdinFilename)
	err2 := os.Remove(cw.stdoutFilename)
	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	return nil
}

// Results returns the results of the job.
func (cw *commandUnit) Results() (results io.Reader, err error) {
	return nil, fmt.Errorf("not implemented yet")
}

// Marshal returns a binary representation of this object
func (cw *commandUnit) Marshal() ([]byte, error) {
	return nil, fmt.Errorf("not implemented yet")
}

// **************************************************************************
// Command line
// **************************************************************************

// CommandCfg is the cmdline configuration object for a worker that runs a command
type CommandCfg struct {
	Service string `required:"true" description:"Local Receptor service name to bind to"`
	Command string `required:"true" description:"Command to run to process units of work"`
}

func (cfg CommandCfg) newWorker() WorkType {
	return &commandUnit{
		command: cfg.Command,
	}
}

func (cfg CommandCfg) unmarshalWorker(data []byte) (WorkType, error) {
	return nil, fmt.Errorf("not implemented yet")
}

// Run runs the action
func (cfg CommandCfg) Run() error {
	err := MainInstance().RegisterWorker(cfg.Service, cfg.newWorker, cfg.unmarshalWorker)
	return err
}

func init() {
	cmdline.AddConfigType("work-command", "Run a worker using an external command", CommandCfg{}, false, workersSection)
}
