//+build linux

package workceptor

import (
	"fmt"
	"github.com/ghjm/sockceptor/pkg/cmdline"
	"github.com/ghjm/sockceptor/pkg/debug"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"
)

// commandUnit implements the WorkUnit interface
type commandUnit struct {
	command string
	cmd     *exec.Cmd
	done    *bool
}

func termThenKill(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = cmd.Process.Signal(os.Interrupt)
	time.Sleep(1 * time.Second)
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

// commandRunner is run in a separate process, to monitor the subprocess and report back metadata
func commandRunner(command string, params string, unitdir string) error {
	err := saveState(unitdir, WorkStatePending, "Not started yet")
	if err != nil {
		return err
	}
	var cmd *exec.Cmd
	if params == "" {
		cmd = exec.Command(command)
	} else {
		cmd = exec.Command(command, strings.Split(params, " ")...)
	}
	termChan := make(chan os.Signal)
	sigKilled := false
	go func() {
		<-termChan
		sigKilled = true
		termThenKill(cmd)
		_ = saveState(unitdir, WorkStateFailed, "Killed")
		os.Exit(-1)
	}()
	signal.Notify(termChan, os.Interrupt, os.Kill)
	stdin, err := os.Open(path.Join(unitdir, "stdin"))
	if err != nil {
		return err
	}
	cmd.Stdin = stdin
	stdout, err := os.OpenFile(path.Join(unitdir, "stdout"), os.O_CREATE+os.O_WRONLY, 0700)
	if err != nil {
		return err
	}
	cmd.Stdout = stdout
	cmd.Stderr = stdout
	err = cmd.Start()
	if err != nil {
		return err
	}
	_ = saveState(unitdir, WorkStateRunning, fmt.Sprintf("Running: PID %d", cmd.Process.Pid))
	err = cmd.Wait()
	if err != nil {
		if sigKilled {
			time.Sleep(50 * time.Millisecond)
		} else {
			_ = saveState(unitdir, WorkStateFailed, fmt.Sprintf("Error: %s", err))
		}
		return err
	}
	if cmd.ProcessState.Success() {
		_ = saveState(unitdir, WorkStateSucceeded, cmd.ProcessState.String())
	} else {
		_ = saveState(unitdir, WorkStateFailed, cmd.ProcessState.String())
	}
	os.Exit(cmd.ProcessState.ExitCode())
	return nil
}

// cmdWaiter hangs around and waits for the command to be done because apparently you
// can't safely call exec.Cmd.Exited() unless you already know the command has exited.
func cmdWaiter(cw *commandUnit) {
	_ = cw.cmd.Wait()
	*cw.done = true
}

// Start launches a job with given parameters.
func (cw *commandUnit) Start(params string, unitdir string) error {
	cw.cmd = exec.Command(os.Args[0], "--command-runner",
		fmt.Sprintf("command=%s", cw.command),
		fmt.Sprintf("params=%s", params),
		fmt.Sprintf("unitdir=%s", unitdir))
	cw.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
	done := false
	cw.done = &done
	err := cw.cmd.Start()
	if err != nil {
		return err
	}
	go cmdWaiter(cw)
	return nil
}

// Cancel releases resources associated with a job, including cancelling it if running.
func (cw *commandUnit) Cancel() error {
	if cw.cmd != nil && !*cw.done {
		err := cw.cmd.Process.Signal(os.Interrupt)
		if err != nil {
			return err
		}
	}
	return nil
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

// Run runs the action
func (cfg CommandCfg) Run() error {
	err := MainInstance.RegisterWorker(cfg.Service, cfg.newWorker)
	return err
}

// CommandRunnerCfg is a hidden command line option for a command runner process
type CommandRunnerCfg struct {
	Command string `required:"true"`
	Params  string `required:"true"`
	UnitDir string `required:"true"`
}

// Run runs the action
func (cfg CommandRunnerCfg) Run() error {
	err := commandRunner(cfg.Command, cfg.Params, cfg.UnitDir)
	if err != nil {
		debug.Printf("Command runner exited with error: %s\n", err)
		os.Exit(-1)
	} else {
		os.Exit(0)
	}
	return nil
}

func init() {
	cmdline.AddConfigType("work-command", "Run a worker using an external command", CommandCfg{}, false, false, false, workersSection)
	cmdline.AddConfigType("command-runner", "Wrapper around a process invocation", CommandRunnerCfg{}, false, true, true, nil)
}
