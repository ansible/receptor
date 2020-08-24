//+build linux

package workceptor

import (
	"encoding/json"
	"fmt"
	"github.com/project-receptor/receptor/pkg/cmdline"
	"os"
	"os/exec"
	"syscall"
)

// pythonUnit implements the WorkUnit interface
type pythonUnit struct {
	plugin   string
	function string
	config   map[string]interface{}
	cmd      *exec.Cmd
	done     bool
}

// Start launches a job with given parameters.
func (pw *pythonUnit) Start(params string, unitdir string) error {
	_ = saveStatus(unitdir, WorkStatePending, "Launching process", 0)
	config := make(map[string]interface{})
	for k, v := range pw.config {
		config[k] = v
	}
	config["params"] = params
	configJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}
	pw.cmd = exec.Command("receptor-python-worker",
		fmt.Sprintf("%s:%s", pw.plugin, pw.function), unitdir, string(configJSON))
	pw.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
	pw.done = false
	err = pw.cmd.Start()
	if err != nil {
		return err
	}
	doneChan := make(chan bool)
	go func() {
		<-doneChan
		pw.done = true
	}()
	go cmdWaiter(pw.cmd, doneChan)
	return nil
}

// Cancel releases resources associated with a job, including cancelling it if running.
func (pw *pythonUnit) Cancel() error {
	if pw.cmd != nil && !pw.done {
		err := pw.cmd.Process.Signal(os.Interrupt)
		if err != nil {
			return err
		}
	}
	return nil
}

// **************************************************************************
// Command line
// **************************************************************************

// WorkPythonCfg is the cmdline configuration object for a Python worker plugin
type WorkPythonCfg struct {
	WorkType string                 `required:"true" description:"Name for this worker type"`
	Plugin   string                 `required:"true" description:"Python module name of the worker plugin"`
	Function string                 `required:"true" description:"Receptor-exported function within the module"`
	Config   map[string]interface{} `description:"Plugin-specific configuration settings"`
}

// newWorker is a factory to produce worker instances
func (cfg WorkPythonCfg) newWorker() WorkType {
	return &pythonUnit{
		plugin:   cfg.Plugin,
		function: cfg.Function,
		config:   cfg.Config,
	}
}

// Run runs the action
func (cfg WorkPythonCfg) Run() error {
	err := MainInstance.RegisterWorker(cfg.WorkType, cfg.newWorker)
	return err
}

func init() {
	cmdline.AddConfigType("work-python", "Run a worker using a Python plugin", WorkPythonCfg{}, false, false, false, workersSection)
}
