//+build linux

package workceptor

import (
	"encoding/json"
	"fmt"
	"github.com/project-receptor/receptor/pkg/cmdline"
	"os/exec"
)

// pythonUnit implements the WorkUnit interface
type pythonUnit struct {
	commandUnit
	plugin   string
	function string
	config   map[string]interface{}
}

// Start launches a job with given parameters.
func (pw *pythonUnit) Start() error {
	pw.UpdateBasicStatus(WorkStatePending, "Launching Python runner", 0)
	config := make(map[string]interface{})
	for k, v := range pw.config {
		config[k] = v
	}
	config["params"] = pw.Status().Params
	configJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}
	cmd := exec.Command("receptor-python-worker",
		fmt.Sprintf("%s:%s", pw.plugin, pw.function), pw.UnitDir(), string(configJSON))
	return pw.runCommand(cmd)
}

// **************************************************************************
// Command line
// **************************************************************************

// WorkPythonCfg is the cmdline configuration object for a Python worker plugin
type WorkPythonCfg struct {
	WorkType string                 `required:"true" description:"Name for this worker type"`
	Plugin   string                 `required:"true" description:"Python module name of the worker plugin"`
	Function string                 `required:"true" description:"Receptor-exported function to call"`
	Config   map[string]interface{} `description:"Plugin-specific configuration"`
}

// newWorker is a factory to produce worker instances
func (cfg WorkPythonCfg) newWorker() WorkUnit {
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
