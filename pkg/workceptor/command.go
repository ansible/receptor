//go:build !no_workceptor
// +build !no_workceptor

package workceptor

import (
	"flag"
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
)

// commandUnit implements the WorkUnit interface for the Receptor command worker plugin.
type commandUnit struct {
	BaseWorkUnit
	command            string
	baseParams         string
	allowRuntimeParams bool
	done               bool
}

// commandExtraData is the content of the ExtraData JSON field for a command worker.
type commandExtraData struct {
	Pid    int
	Params string
}

func termThenKill(cmd *exec.Cmd, doneChan chan bool) {
	if cmd.Process == nil {
		return
	}
	_ = cmd.Process.Signal(os.Interrupt)
	select {
	case <-doneChan:
		return
	case <-time.After(10 * time.Second):
		MainInstance.nc.Logger.Warning("timed out waiting for pid %d to terminate with SIGINT", cmd.Process.Pid)
	}
	if cmd.Process != nil {
		MainInstance.nc.Logger.Info("sending SIGKILL to pid %d", cmd.Process.Pid)
		_ = cmd.Process.Kill()
	}
}

// cmdWaiter hangs around and waits for the command to be done because apparently you
// can't safely call exec.Cmd.Exited() unless you already know the command has exited.
func cmdWaiter(cmd *exec.Cmd, doneChan chan bool) {
	_ = cmd.Wait()
	doneChan <- true
}

// commandRunner is run in a separate process, to monitor the subprocess and report back metadata.
func commandRunner(command string, params string, unitdir string) error {
	status := StatusFileData{}
	status.ExtraData = &commandExtraData{}
	statusFilename := path.Join(unitdir, "status")
	err := status.UpdateBasicStatus(statusFilename, WorkStatePending, "Not started yet", 0)
	if err != nil {
		MainInstance.nc.Logger.Error("Error updating status file %s: %s", statusFilename, err)
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
	termChan := make(chan os.Signal, 1)
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
	doneChan := make(chan bool, 1)
	go cmdWaiter(cmd, doneChan)
	writeStatusFailures := 0
loop:
	for {
		select {
		case <-doneChan:
			break loop
		case <-termChan:
			termThenKill(cmd, doneChan)
			err = status.UpdateBasicStatus(statusFilename, WorkStateFailed, "Killed", stdoutSize(unitdir))
			if err != nil {
				MainInstance.nc.Logger.Error("Error updating status file %s: %s", statusFilename, err)
			}
			os.Exit(-1)
		case <-time.After(250 * time.Millisecond):
			err = status.UpdateBasicStatus(statusFilename, WorkStateRunning, fmt.Sprintf("Running: PID %d", cmd.Process.Pid), stdoutSize(unitdir))
			if err != nil {
				MainInstance.nc.Logger.Error("Error updating status file %s: %s", statusFilename, err)
				writeStatusFailures++
				if writeStatusFailures > 3 {
					MainInstance.nc.Logger.Error("Exceeded retries for updating status file %s: %s", statusFilename, err)
					os.Exit(-1)
				}
			} else {
				writeStatusFailures = 0
			}
		}
	}
	if err != nil {
		err = status.UpdateBasicStatus(statusFilename, WorkStateFailed, fmt.Sprintf("Error: %s", err), stdoutSize(unitdir))
		if err != nil {
			MainInstance.nc.Logger.Error("Error updating status file %s: %s", statusFilename, err)
		}

		return err
	}
	if cmd.ProcessState.Success() {
		err = status.UpdateBasicStatus(statusFilename, WorkStateSucceeded, cmd.ProcessState.String(), stdoutSize(unitdir))
		if err != nil {
			MainInstance.nc.Logger.Error("Error updating status file %s: %s", statusFilename, err)
		}
	} else {
		err = status.UpdateBasicStatus(statusFilename, WorkStateFailed, cmd.ProcessState.String(), stdoutSize(unitdir))
		if err != nil {
			MainInstance.nc.Logger.Error("Error updating status file %s: %s", statusFilename, err)
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

// SetFromParams sets the in-memory state from parameters.
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

// Status returns a copy of the status currently loaded in memory.
func (cw *commandUnit) Status() *StatusFileData {
	return cw.UnredactedStatus()
}

// UnredactedStatus returns a copy of the status currently loaded in memory, including secrets.
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
	if err := cmd.Start(); err != nil {
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
	level := cw.w.nc.Logger.GetLogLevel()
	levelName, _ := cw.w.nc.Logger.LogLevelToName(level)
	cw.UpdateBasicStatus(WorkStatePending, "Launching command runner", 0)

	// TODO: This is another place where we rely on a pre-built binary for testing.
	// Consider invoking the commandRunner directly?
	var receptorBin string
	if flag.Lookup("test.v") == nil {
		receptorBin = os.Args[0]
	} else {
		receptorBin = "receptor"
	}

	cmd := exec.Command(receptorBin, "--node", "id=worker",
		"--log-level", levelName,
		"--command-runner",
		fmt.Sprintf("command=%s", cw.command),
		fmt.Sprintf("params=%s", cw.Status().ExtraData.(*commandExtraData).Params),
		fmt.Sprintf("unitdir=%s", cw.UnitDir()))

	return cw.runCommand(cmd)
}

// Restart resumes monitoring a job after a Receptor restart.
func (cw *commandUnit) Restart() error {
	if err := cw.Load(); err != nil {
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
	defer proc.Release()
	err = proc.Signal(os.Interrupt)
	if err != nil {
		if strings.Contains(err.Error(), "already finished") {
			return nil
		}

		return err
	}

	proc.Wait()

	cw.UpdateBasicStatus(WorkStateCanceled, "Canceled", -1)

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

// CommandWorkerCfg is the cmdline configuration object for a worker that runs a command.
type CommandWorkerCfg struct {
	WorkType           string `required:"true" description:"Name for this worker type"`
	Command            string `required:"true" description:"Command to run to process units of work"`
	Params             string `description:"Command-line parameters"`
	AllowRuntimeParams bool   `description:"Allow users to add more parameters" default:"false"`
	VerifySignature    bool   `description:"Verify a signed work submission" default:"false"`
}

func (cfg CommandWorkerCfg) NewWorker(w *Workceptor, unitID string, workType string) WorkUnit {
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

func (cfg CommandWorkerCfg) GetWorkType() string {
	return cfg.WorkType
}

func (cfg CommandWorkerCfg) GetVerifySignature() bool {
	return cfg.VerifySignature
}

// Run runs the action.
func (cfg CommandWorkerCfg) Run() error {
	if cfg.VerifySignature && MainInstance.VerifyingKey == "" {
		return fmt.Errorf("VerifySignature for work command '%s' is true, but the work verification public key is not specified", cfg.WorkType)
	}
	err := MainInstance.RegisterWorker(cfg.WorkType, cfg.NewWorker, cfg.VerifySignature)

	return err
}

// commandRunnerCfg is a hidden command line option for a command runner process.
type commandRunnerCfg struct {
	Command string `required:"true"`
	Params  string `required:"true"`
	UnitDir string `required:"true"`
}

// Run runs the action.
func (cfg commandRunnerCfg) Run() error {
	err := commandRunner(cfg.Command, cfg.Params, cfg.UnitDir)
	if err != nil {
		statusFilename := path.Join(cfg.UnitDir, "status")
		err = (&StatusFileData{}).UpdateBasicStatus(statusFilename, WorkStateFailed, err.Error(), stdoutSize(cfg.UnitDir))
		if err != nil {
			MainInstance.nc.Logger.Error("Error updating status file %s: %s", statusFilename, err)
		}
		MainInstance.nc.Logger.Error("Command runner exited with error: %s\n", err)
		os.Exit(-1)
	}
	os.Exit(0)

	return nil
}

type SigningKeyPrivateCfg struct {
	PrivateKey      string `description:"Private key to sign work submissions" barevalue:"yes" default:""`
	TokenExpiration string `description:"Expiration of the signed json web token, e.g. 3h or 3h30m" default:""`
}

type VerifyingKeyPublicCfg struct {
	PublicKey string `description:"Public key to verify signed work submissions" barevalue:"yes" default:""`
}

func filenameExists(filename string) error {
	if _, err := os.Stat(filename); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s does not exist", filename)
		}

		return err
	}

	return nil
}

func (cfg SigningKeyPrivateCfg) Prepare() error {
	duration, err := cfg.PrepareSigningKeyPrivateCfg()
	if err != nil {
		return fmt.Errorf(err.Error())
	}

	MainInstance.SigningExpiration = *duration
	MainInstance.SigningKey = cfg.PrivateKey

	return nil
}

func (cfg SigningKeyPrivateCfg) PrepareSigningKeyPrivateCfg() (*time.Duration, error) {
	err := filenameExists(cfg.PrivateKey)
	if err != nil {
		return nil, err
	}

	if cfg.TokenExpiration != "" {
		duration, err := time.ParseDuration(cfg.TokenExpiration)
		if err != nil {
			return nil, fmt.Errorf("failed to parse TokenExpiration -- valid examples include '1.5h', '30m', '30m10s'")
		}

		return &duration, nil
	}

	return nil, nil
}

func (cfg VerifyingKeyPublicCfg) Prepare() error {
	err := filenameExists(cfg.PublicKey)
	if err != nil {
		return err
	}
	MainInstance.VerifyingKey = cfg.PublicKey

	return nil
}

func (cfg VerifyingKeyPublicCfg) PrepareVerifyingKeyPublicCfg() error {
	err := filenameExists(cfg.PublicKey)
	if err != nil {
		return err
	}

	return nil
}

func init() {
	cmdline.RegisterConfigTypeForApp("receptor-workers",
		"work-signing", "Private key to sign work submissions", SigningKeyPrivateCfg{}, cmdline.Singleton, cmdline.Section(workersSection))
	cmdline.RegisterConfigTypeForApp("receptor-workers",
		"work-verification", "Public key to verify work submissions", VerifyingKeyPublicCfg{}, cmdline.Singleton, cmdline.Section(workersSection))
	cmdline.RegisterConfigTypeForApp("receptor-workers",
		"work-command", "Run a worker using an external command", CommandWorkerCfg{}, cmdline.Section(workersSection))
	cmdline.RegisterConfigTypeForApp("receptor-workers",
		"command-runner", "Wrapper around a process invocation", commandRunnerCfg{}, cmdline.Hidden)
}
