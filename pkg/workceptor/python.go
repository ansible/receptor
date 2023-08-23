//go:build !no_workceptor
// +build !no_workceptor

package workceptor

import (
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/ghjm/cmdline"
)

// pythonUnit implements the WorkUnit interface.
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
	config["params"] = pw.Status().ExtraData.(*commandExtraData).Params
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

// workPythonCfg is the cmdline configuration object for a Python worker plugin.
type workPythonCfg struct {
	WorkType string                 `required:"true" description:"Name for this worker type"`
	Plugin   string                 `required:"true" description:"Python module name of the worker plugin"`
	Function string                 `required:"true" description:"Receptor-exported function to call"`
	Config   map[string]interface{} `description:"Plugin-specific configuration"`
}

// NewWorker is a factory to produce worker instances.
func (cfg workPythonCfg) NewWorker(w *Workceptor, unitID string, workType string) WorkUnit {
	cw := &pythonUnit{
		commandUnit: commandUnit{
			BaseWorkUnit: BaseWorkUnit{
				status: &StatusFileData{
					ExtraData: &commandExtraData{},
				},
			},
		},
		plugin:   cfg.Plugin,
		function: cfg.Function,
		config:   cfg.Config,
	}
	cw.BaseWorkUnit.Init(w, unitID, workType, FileSystem{})

	return cw
}

// Run runs the action.
func (cfg workPythonCfg) Run() error {
	err := MainInstance.RegisterWorker(cfg.WorkType, cfg.NewWorker, false)

	return err
}

func init() {
	cmdline.RegisterConfigTypeForApp("receptor-workers",
		"work-python", "Run a worker using a Python plugin", workPythonCfg{}, cmdline.Section(workersSection))
}
