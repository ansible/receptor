//+build linux

package workceptor

import (
	"fmt"
	"github.com/ghjm/sockceptor/pkg/cmdline"
	"github.com/ghjm/sockceptor/pkg/debug"
	"io"
	"os"
	"os/exec"
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
	unitdir string
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
	err = saveState(unitdir, WorkStateRunning, fmt.Sprintf("Running: PID %d", cmd.Process.Pid))
	if err != nil {
		return err
	}
	err = cmd.Wait()
	if err != nil {
		return err
	}
	if cmd.ProcessState.Success() {
		err = saveState(unitdir, WorkStateSucceeded, cmd.ProcessState.String())
	} else {
		err = saveState(unitdir, WorkStateFailed, cmd.ProcessState.String())
	}
	return err
}

// cmdWaiter hangs around and waits for the command to be done because apparently you
// can't safely call exec.Cmd.Exited() unless you already know the command has exited.
func cmdWaiter(cw *commandUnit) {
	_ = cw.cmd.Wait()
	*cw.done = true
}

// Start launches a job with given parameters.
func (cw *commandUnit) Start(params string, unitdir string) error {
	cw.unitdir = unitdir
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

// Release releases resources associated with a job, including cancelling it if running.
func (cw *commandUnit) Release() error {
	if cw.cmd != nil && !*cw.done {
		err := cw.cmd.Process.Kill()
		if err != nil {
			return err
		}
	}
	return nil
}

// Results returns the results of the job.
func (cw *commandUnit) Results() (results chan []byte, err error) {
	resultChan := make(chan []byte)
	go func() {
		for cw.unitdir == "" {
			time.Sleep(250 * time.Millisecond)
		}
		stdoutFilename := path.Join(cw.unitdir, "stdout")
		var stdout *os.File
		var filePos int64
		buf := make([]byte, 1024)
		for {
			if stdout == nil {
				stdout, err = os.Open(stdoutFilename)
				if err != nil {
					time.Sleep(250 * time.Millisecond)
					continue
				}
			}
			newPos, err := stdout.Seek(filePos, 0)
			if newPos != filePos {
				debug.Printf("Seek error processing stdout\n")
				return
			}
			n, err := stdout.Read(buf)
			if n > 0 {
				filePos += int64(n)
				resultChan <- buf[:n]
			}
			if err == io.EOF {
				if *cw.done {
					close(resultChan)
					break
				}
				time.Sleep(50 * time.Millisecond)
				continue
			} else if err != nil {
				debug.Printf("Error reading stdout: %s\n", err)
				return
			}
		}
	}()
	return resultChan, nil
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
	return commandRunner(cfg.Command, cfg.Params, cfg.UnitDir)
}

func init() {
	cmdline.AddConfigType("work-command", "Run a worker using an external command", CommandCfg{}, false, false, false, workersSection)
	cmdline.AddConfigType("command-runner", "Wrapper around a process invocation", CommandRunnerCfg{}, false, true, true, nil)
}
