// +build !no_workceptor

package workceptor

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/ghjm/cmdline"
	"github.com/google/shlex"
	"github.com/project-receptor/receptor/pkg/logger"
)

// commandUnit implements the WorkUnit interface for the Receptor command worker plugin
type commandUnit struct {
	BaseWorkUnit
	command            string
	baseParams         string
	allowRuntimeParams bool
	done               bool
}

// commandExtraData is the content of the ExtraData JSON field for a command worker
type commandExtraData struct {
	Pid    int
	Params string
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
	signal.Notify(termChan, syscall.SIGINT, syscall.SIGTERM)
	stdin, err := os.Open(path.Join(unitdir, "stdin"))
	if err != nil {
		return err
	}
	cmd.Stdin = stdin
	stdout, err := os.OpenFile(path.Join(unitdir, "stdout"), os.O_CREATE+os.O_WRONLY+os.O_SYNC, 0o600)
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
		case <-termChan:
			termThenKill(cmd)
			err = status.UpdateBasicStatus(statusFilename, WorkStateFailed, "Killed", stdoutSize(unitdir))
			if err != nil {
				logger.Error("Error updating status file %s: %s", statusFilename, err)
			}
			os.Exit(-1)
		case <-time.After(250 * time.Millisecond):
			err = status.UpdateBasicStatus(statusFilename, WorkStateRunning, fmt.Sprintf("Running: PID %d", cmd.Process.Pid), stdoutSize(unitdir))
			if err != nil {
				logger.Error("Error updating status file %s: %s", statusFilename, err)
			}

		}
	}
	if err != nil {
		err = status.UpdateBasicStatus(statusFilename, WorkStateFailed, fmt.Sprintf("Error: %s", err), stdoutSize(unitdir))
		if err != nil {
			logger.Error("Error updating status file %s: %s", statusFilename, err)
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

func combineParams(baseParams string, userParams string) string {
	var allParams string
	switch {
	case userParams == "":
		allParams = baseParams
	case baseParams == "":
		allParams = userParams
	default:
		allParams = strings.Join([]string{baseParams, userParams}, " ")
	}
	return allParams
}

// SetFromParams sets the in-memory state from parameters
func (cw *commandUnit) SetFromParams(params map[string]string) error {
	cmdParams, ok := params["params"]
	if !ok {
		cmdParams = ""
	}
	if cmdParams != "" && !cw.allowRuntimeParams {
		return fmt.Errorf("extra params provided but not allowed")
	}
	cw.status.ExtraData.(*commandExtraData).Params = combineParams(cw.baseParams, cmdParams)
	return nil
}

// Status returns a copy of the status currently loaded in memory
func (cw *commandUnit) Status() *StatusFileData {
	return cw.UnredactedStatus()
}

// UnredactedStatus returns a copy of the status currently loaded in memory, including secrets
func (cw *commandUnit) UnredactedStatus() *StatusFileData {
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

// runCommand actually runs the exec.Cmd.  This is in a separate function so the Python worker can call it.
func (cw *commandUnit) runCommand(cmd *exec.Cmd) error {
	cmdSetDetach(cmd)
	cw.done = false
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
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
	level := logger.GetLogLevel()
	levelName, _ := logger.LogLevelToName(level)
	cw.UpdateBasicStatus(WorkStatePending, "Launching command runner", 0)
	cmd := exec.Command(os.Args[0], "--log-level", levelName, "--command-runner",
		fmt.Sprintf("command=%s", cw.command),
		fmt.Sprintf("params=%s", cw.Status().ExtraData.(*commandExtraData).Params),
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
	cw.cancel()
	status := cw.Status()
	ced, ok := status.ExtraData.(*commandExtraData)
	if !ok || ced.Pid <= 0 {
		return nil
	}
	proc, err := os.FindProcess(ced.Pid)
	if err != nil {
		return err
	}
	defer func() {
		if err := proc.Release(); err != nil {
			panic(err)
		}
	}()
	err = proc.Signal(os.Interrupt)
	switch {
	case errors.Is(err, os.ErrProcessDone):
		return nil
	case err != nil:
		return err
	}

	if _, err := proc.Wait(); err != nil {
		panic(err)
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

// commandCfg is the cmdline configuration object for a worker that runs a command
type commandCfg struct {
	WorkType           string `required:"true" description:"Name for this worker type"`
	Command            string `required:"true" description:"Command to run to process units of work"`
	Params             string `description:"Command-line parameters"`
	AllowRuntimeParams bool   `description:"Allow users to add more parameters" default:"false"`
}

func (cfg commandCfg) newWorker(w *Workceptor, unitID string, workType string) WorkUnit {
	cw := &commandUnit{
		BaseWorkUnit: BaseWorkUnit{
			status: StatusFileData{
				ExtraData: &commandExtraData{},
			},
		},
		command:            cfg.Command,
		baseParams:         cfg.Params,
		allowRuntimeParams: cfg.AllowRuntimeParams,
	}
	cw.BaseWorkUnit.Init(w, unitID, workType)
	return cw
}

// Run runs the action
func (cfg commandCfg) Run() error {
	err := MainInstance.RegisterWorker(cfg.WorkType, cfg.newWorker)
	return err
}

// commandRunnerCfg is a hidden command line option for a command runner process
type commandRunnerCfg struct {
	Command string `required:"true"`
	Params  string `required:"true"`
	UnitDir string `required:"true"`
}

// Run runs the action
func (cfg commandRunnerCfg) Run() error {
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
	cmdline.RegisterConfigTypeForApp("receptor-workers",
		"work-command", "Run a worker using an external command", commandCfg{}, cmdline.Section(workersSection))
	cmdline.RegisterConfigTypeForApp("receptor-workers",
		"command-runner", "Wrapper around a process invocation", commandRunnerCfg{}, cmdline.Hidden)
}
