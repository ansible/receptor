//+build linux

package workceptor

import (
	"fmt"
	"github.com/google/shlex"
	"github.com/project-receptor/receptor/pkg/cmdline"
	"github.com/project-receptor/receptor/pkg/logger"
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
	params  string
	cmd     *exec.Cmd
	done    bool
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

// Returns size of stdout, if it exists, or 0 otherwise
func stdoutSize(unitdir string) int64 {
	stat, err := os.Stat(path.Join(unitdir, "stdout"))
	if err != nil {
		return 0
	}
	return stat.Size()
}

// cmdWaiter hangs around and waits for the command to be done because apparently you
// can't safely call exec.Cmd.Exited() unless you already know the command has exited.
func cmdWaiter(cmd *exec.Cmd, doneChan chan bool) {
	_ = cmd.Wait()
	doneChan <- true
}

// commandRunner is run in a separate process, to monitor the subprocess and report back metadata
func commandRunner(command string, params string, unitdir string) error {
	err := saveStatus(unitdir, WorkStatePending, "Not started yet", 0)
	if err != nil {
		return err
	}
	var cmd *exec.Cmd
	if params == "" {
		cmd = exec.Command(command)
	} else {
		paramList, err := shlex.Split(params)
		if err != nil {
			return err
		}
		cmd = exec.Command(command, paramList...)
	}
	termChan := make(chan os.Signal)
	sigKilled := false
	go func() {
		<-termChan
		sigKilled = true
		termThenKill(cmd)
		_ = saveStatus(unitdir, WorkStateFailed, "Killed", stdoutSize(unitdir))
		os.Exit(-1)
	}()
	signal.Notify(termChan, syscall.SIGINT, syscall.SIGTERM)
	stdin, err := os.Open(path.Join(unitdir, "stdin"))
	if err != nil {
		return err
	}
	cmd.Stdin = stdin
	stdout, err := os.OpenFile(path.Join(unitdir, "stdout"), os.O_CREATE+os.O_WRONLY+os.O_SYNC, 0600)
	if err != nil {
		return err
	}
	cmd.Stdout = stdout
	cmd.Stderr = stdout
	err = cmd.Start()
	if err != nil {
		return err
	}
	doneChan := make(chan bool)
	go cmdWaiter(cmd, doneChan)
loop:
	for {
		select {
		case <-doneChan:
			break loop
		case <-time.After(250 * time.Millisecond):
			_ = saveStatus(unitdir, WorkStateRunning, fmt.Sprintf("Running: PID %d", cmd.Process.Pid), stdoutSize(unitdir))
		}
	}
	if err != nil {
		if sigKilled {
			time.Sleep(50 * time.Millisecond)
		} else {
			_ = saveStatus(unitdir, WorkStateFailed, fmt.Sprintf("Error: %s", err), stdoutSize(unitdir))
		}
		return err
	}
	if cmd.ProcessState.Success() {
		_ = saveStatus(unitdir, WorkStateSucceeded, cmd.ProcessState.String(), stdoutSize(unitdir))
	} else {
		_ = saveStatus(unitdir, WorkStateFailed, cmd.ProcessState.String(), stdoutSize(unitdir))
	}
	os.Exit(cmd.ProcessState.ExitCode())
	return nil
}

// Start launches a job with given parameters.
func (cw *commandUnit) Start(params string, unitdir string) error {
	_ = saveStatus(unitdir, WorkStatePending, "Launching process", 0)
	var allParams string
	if params == "" {
		allParams = cw.params
	} else if cw.params == "" {
		allParams = params
	} else {
		allParams = strings.Join([]string{cw.params, params}, " ")
	}
	cw.cmd = exec.Command(os.Args[0], "--command-runner",
		fmt.Sprintf("command=%s", cw.command),
		fmt.Sprintf("params=%s", allParams),
		fmt.Sprintf("unitdir=%s", unitdir))
	cw.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
	cw.done = false
	err := cw.cmd.Start()
	if err != nil {
		return err
	}
	doneChan := make(chan bool)
	go func() {
		<-doneChan
		cw.done = true
	}()
	go cmdWaiter(cw.cmd, doneChan)
	return nil
}

// Cancel releases resources associated with a job, including cancelling it if running.
func (cw *commandUnit) Cancel() error {
	if cw.cmd != nil && !cw.done {
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
	Params  string `description:"Command-line parameters"`
}

func (cfg CommandCfg) newWorker() WorkType {
	return &commandUnit{
		command: cfg.Command,
		params:  cfg.Params,
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
		_ = saveStatus(cfg.UnitDir, WorkStateFailed, err.Error(), 0)
		logger.Error("Command runner exited with error: %s\n", err)
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
