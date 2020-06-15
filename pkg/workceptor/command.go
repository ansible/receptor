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
	"time"
)

// commandUnit implements the WorkUnit interface
type commandUnit struct {
	command    string
	cmd        *exec.Cmd
	done       *bool
	unitdir    string
	statusChan chan *StatusInfo
}

// cmdWaiter hangs around and waits for the command to be done because apparently you
// can't safely call exec.Cmd.Exited() unless you already know the command has exited.
func cmdWaiter(cw *commandUnit) {
	_ = cw.cmd.Wait()
	*cw.done = true
	var state int
	if cw.cmd.ProcessState.Success() {
		state = WorkStateSucceeded
	} else {
		state = WorkStateFailed
	}
	cw.statusChan <- &StatusInfo{
		State:  state,
		Detail: cw.cmd.ProcessState.String(),
	}
}

// Start launches a job with given parameters.  It returns an identifier string and an error.
func (cw *commandUnit) Start(params string, unitdir string, statusChan chan *StatusInfo) error {
	cw.unitdir = unitdir
	cw.statusChan = statusChan
	var cmd *exec.Cmd
	if params == "" {
		cmd = exec.Command(cw.command)
	} else {
		cmd = exec.Command(cw.command, strings.Split(params, " ")...)
	}
	stdin, err := os.Open(path.Join(cw.unitdir, "stdin"))
	if err != nil {
		return err
	}
	cmd.Stdin = stdin
	stdout, err := os.OpenFile(path.Join(cw.unitdir, "stdout"), os.O_CREATE+os.O_WRONLY, 0700)
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
	cw.statusChan <- &StatusInfo{
		State:  WorkStateRunning,
		Detail: fmt.Sprintf("Running: PID %d", cw.cmd.Process.Pid),
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
		defer close(resultChan)
		var stdout *os.File
		var filePos int64
		buf := make([]byte, 1024)
		for {
			if cw.cmd == nil {
				// Command has not started running yet
				time.Sleep(1 * time.Second)
				continue
			}
			if stdout == nil {
				stdout, err = os.Open(path.Join(cw.unitdir, "stdout"))
				filePos = 0
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
					return
				}
				// TODO: something more intelligent using fsnotify
				time.Sleep(250 * time.Millisecond)
			} else if err != nil {
				debug.Printf("Error reading from stdout: %s\n", err)
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

func init() {
	cmdline.AddConfigType("work-command", "Run a worker using an external command", CommandCfg{}, false, workersSection)
}
