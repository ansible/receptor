// +build !no_workceptor

package workceptor

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
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

// commandUnit implements the WorkUnit interface for the Receptor command worker plugin
type commandUnit struct {
	BaseWorkUnit
	command    string
	baseParams string
	done       bool
}

// commandExtraData is the content of the ExtraData JSON field for a command worker
type commandExtraData struct {
	Pid int
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

// cmdWaiter hangs around and waits for the command to be done because apparently you
// can't safely call exec.Cmd.Exited() unless you already know the command has exited.
func cmdWaiter(cmd *exec.Cmd, doneChan chan bool) {
	_ = cmd.Wait()
	doneChan <- true
}

// commandRunner is run in a separate process, to monitor the subprocess and report back metadata
func commandRunner(command string, params string, unitdir string) error {
	status := StatusFileData{}
	status.ExtraData = &commandExtraData{}
	statusFilename := path.Join(unitdir, "status")
	err := status.UpdateBasicStatus(statusFilename, WorkStatePending, "Not started yet", 0)
	if err != nil {
		logger.Error("Error updating status file %s: %s", statusFilename, err)
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
		err = status.UpdateBasicStatus(statusFilename, WorkStateFailed, "Killed", stdoutSize(unitdir))
		if err != nil {
			logger.Error("Error updating status file %s: %s", statusFilename, err)
		}
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
			err = status.UpdateBasicStatus(statusFilename, WorkStateRunning, fmt.Sprintf("Running: PID %d", cmd.Process.Pid), stdoutSize(unitdir))
			if err != nil {
				logger.Error("Error updating status file %s: %s", statusFilename, err)
			}

		}
	}
	if err != nil {
		if sigKilled {
			time.Sleep(50 * time.Millisecond)
		} else {
			err = status.UpdateBasicStatus(statusFilename, WorkStateFailed, fmt.Sprintf("Error: %s", err), stdoutSize(unitdir))
			if err != nil {
				logger.Error("Error updating status file %s: %s", statusFilename, err)
			}
		}
		return err
	}
	if cmd.ProcessState.Success() {
		err = status.UpdateBasicStatus(statusFilename, WorkStateSucceeded, cmd.ProcessState.String(), stdoutSize(unitdir))
		if err != nil {
			logger.Error("Error updating status file %s: %s", statusFilename, err)
		}
	} else {
		err = status.UpdateBasicStatus(statusFilename, WorkStateFailed, cmd.ProcessState.String(), stdoutSize(unitdir))
		if err != nil {
			logger.Error("Error updating status file %s: %s", statusFilename, err)
		}
	}
	os.Exit(cmd.ProcessState.ExitCode())
	return nil
}

// Init initializes the work unit data
func (cw *commandUnit) Init(w *Workceptor, ident string, workType string, params string) {
	cw.BaseWorkUnit.Init(w, ident, workType, params)
	cw.status.ExtraData = &commandExtraData{}
	var allParams string
	if params == "" {
		allParams = cw.baseParams
	} else if cw.baseParams == "" {
		allParams = params
	} else {
		allParams = strings.Join([]string{cw.baseParams, params}, " ")
	}
	cw.UpdateFullStatus(func(status *StatusFileData) {
		status.Params = allParams
	})
}

// Status returns a copy of the status currently loaded in memory
func (cw *commandUnit) Status() *StatusFileData {
	cw.statusLock.RLock()
	defer cw.statusLock.RUnlock()
	status := cw.getStatus()
	ed, ok := cw.status.ExtraData.(*commandExtraData)
	if ok {
		edCopy := *ed
		status.ExtraData = &edCopy
	}
	return status
}

// monitorLocalStatus watches a unit dir and keeps the workUnit up to date with status changes
func (cw *commandUnit) monitorLocalStatus() {
	statusFile := path.Join(cw.UnitDir(), "status")
	watcher, err := fsnotify.NewWatcher()
	if err == nil {
		err = watcher.Add(statusFile)
		if err == nil {
			defer func() {
				_ = watcher.Close()
			}()
		} else {
			_ = watcher.Close()
			watcher = nil
		}
	} else {
		watcher = nil
	}
	fi, err := os.Stat(statusFile)
	if err != nil {
		fi = nil
	}
	var watcherEvents chan fsnotify.Event
	if watcher == nil {
		watcherEvents = make(chan fsnotify.Event)
	} else {
		watcherEvents = watcher.Events
	}
	for {
		select {
		case event := <-watcherEvents:
			if event.Op&fsnotify.Write == fsnotify.Write {
				err = cw.Load()
				if err != nil {
					logger.Error("Error reading %s: %s", statusFile, err)
				}
			}
		case <-time.After(time.Second):
			newFi, err := os.Stat(statusFile)
			if err == nil {
				if fi == nil || fi.ModTime() != newFi.ModTime() {
					fi = newFi
					err = cw.Load()
					if err != nil {
						logger.Error("Error reading %s: %s", statusFile, err)
					}
				}
			}
		}
		complete := IsComplete(cw.Status().State)
		if complete {
			break
		}
	}
}

// runCommand actually runs the exec.Cmd.  This is in a separate function so the Python worker can call it.
func (cw *commandUnit) runCommand(cmd *exec.Cmd) error {
	cmdSetDetach(cmd)
	cw.done = false
	err := cmd.Start()
	if err != nil {
		cw.UpdateBasicStatus(WorkStateFailed, fmt.Sprintf("Failed to start command runner: %s", err), 0)
		return err
	}
	cw.UpdateFullStatus(func(status *StatusFileData) {
		if status.ExtraData == nil {
			status.ExtraData = &commandExtraData{}
		}
		status.ExtraData.(*commandExtraData).Pid = cmd.Process.Pid
	})
	doneChan := make(chan bool)
	go func() {
		<-doneChan
		cw.done = true
		cw.UpdateFullStatus(func(status *StatusFileData) {
			status.ExtraData = nil
		})
	}()
	go cmdWaiter(cmd, doneChan)
	go cw.monitorLocalStatus()
	return nil
}

// Start launches a job with given parameters.
func (cw *commandUnit) Start() error {
	cw.UpdateBasicStatus(WorkStatePending, "Launching command runner", 0)
	cmd := exec.Command(os.Args[0], "--command-runner",
		fmt.Sprintf("command=%s", cw.command),
		fmt.Sprintf("params=%s", cw.Status().Params),
		fmt.Sprintf("unitdir=%s", cw.UnitDir()))
	return cw.runCommand(cmd)
}

// Restart resumes monitoring a job after a Receptor restart
func (cw *commandUnit) Restart() error {
	err := cw.Load()
	if err != nil {
		return err
	}
	state := cw.Status().State
	if IsComplete(state) {
		// Job already complete - no need to restart monitoring
		return nil
	}
	if state == WorkStatePending {
		// Job never started - mark it failed
		cw.UpdateBasicStatus(WorkStateFailed, "Pending at restart", stdoutSize(cw.UnitDir()))
	}
	go cw.monitorLocalStatus()
	return nil
}

// Cancel stops a running job.
func (cw *commandUnit) Cancel() error {
	status := cw.Status()
	ced, ok := status.ExtraData.(*commandExtraData)
	if !ok || ced.Pid <= 0 {
		return nil
	}
	proc, err := os.FindProcess(ced.Pid)
	if err != nil {
		return nil
	}
	defer proc.Release()
	err = proc.Signal(os.Interrupt)
	if err != nil {
		if strings.Contains(err.Error(), "already finished") {
			return nil
		}
		return err
	}
	return nil
}

// Release releases resources associated with a job.  Implies Cancel.
func (cw *commandUnit) Release(force bool) error {
	err := cw.Cancel()
	if err != nil && !force {
		return err
	}
	return cw.BaseWorkUnit.Release(force)
}

// **************************************************************************
// Command line
// **************************************************************************

// CommandCfg is the cmdline configuration object for a worker that runs a command
type CommandCfg struct {
	WorkType string `required:"true" description:"Name for this worker type"`
	Command  string `required:"true" description:"Command to run to process units of work"`
	Params   string `description:"Command-line parameters"`
}

func (cfg CommandCfg) newWorker() WorkUnit {
	return &commandUnit{
		command:    cfg.Command,
		baseParams: cfg.Params,
	}
}

// Run runs the action
func (cfg CommandCfg) Run() error {
	err := MainInstance.RegisterWorker(cfg.WorkType, cfg.newWorker)
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
		statusFilename := path.Join(cfg.UnitDir, "status")
		err = (&StatusFileData{}).UpdateBasicStatus(statusFilename, WorkStateFailed, err.Error(), stdoutSize(cfg.UnitDir))
		if err != nil {
			logger.Error("Error updating status file %s: %s", statusFilename, err)
		}
		logger.Error("Command runner exited with error: %s\n", err)
		os.Exit(-1)
	} else {
		os.Exit(0)
	}
	return nil
}

func init() {
	cmdline.AddConfigType("work-command", "Run a worker using an external command", CommandCfg{}, false, false, false, false, workersSection)
	cmdline.AddConfigType("command-runner", "Wrapper around a process invocation", CommandRunnerCfg{}, false, false, true, true, nil)
}
