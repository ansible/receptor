package controlsvc

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
	"gopkg.in/yaml.v2"
)

type (
	reloadCommandType struct{}
	reloadCommand     struct{}
)

var configPath = ""

var reloadParseAndRun = func(toRun []string) error {
	return fmt.Errorf("no configuration file was provided, reload function not set")
}

var cfgNotReloadable = make(map[string]bool)

var reloadableActions = []string{
	"tcp-peer",
	"tcp-listener",
	"ws-peer",
	"ws-listener",
	"udp-peer",
	"udp-listener",
	"local-only",
}

func isReloadable(cfg string) bool {
	// checks if top-level keys (e.g. tcp-peer) are in the list of reloadable
	// actions
	for _, a := range reloadableActions {
		if strings.HasPrefix(cfg, a) {
			return true
		}
	}

	return false
}

func getActionKeyword(cfg string) string {
	// extracts top-level key from the full configuration item
	cfgSplit := strings.Split(cfg, ":")
	var action string
	if len(cfgSplit) == 0 {
		action = cfg
	} else {
		action = cfgSplit[0]
	}

	return action
}

func parseConfigForReload(filename string, checkReload bool) error {
	// cfgNotReloadable is a map, each key being the full configuration item
	// e.g. "work-command: worktype: echosleep command: bash params:..."
	// Initially all values of map are set to false,
	// e.g. cfgNotReloadable["work-command: worktype: echosleep..."] = false
	//
	// Upon reload, the config is reparsed and each item is checked.
	// if item not in cfgNotReloadable, return error
	// if item is in cfgNotReloadable, set it to true
	// e.g. cfgNotReloadable["work-command: worktype: echosleep..."] = true
	//
	// Finally, cfgAbsent() will loop through the map and check for any remaining
	// items that are still false. This means the original item is missing from
	// the config, and an error will be thrown
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	m := make([]interface{}, 0)
	err = yaml.Unmarshal(data, &m)
	if err != nil {
		return err
	}
	for i := range m {
		cfgBytes, err := yaml.Marshal(&m[i])
		if err != nil {
			return err
		}
		cfg := string(cfgBytes)
		if !isReloadable(cfg) {
			if checkReload {
				if _, ok := cfgNotReloadable[cfg]; !ok {
					action := getActionKeyword(cfg)

					return fmt.Errorf("a non-reloadable config action '%s' was modified or added. Must restart receptor for these changes to take effect", action)
				}
				cfgNotReloadable[cfg] = true
			} else {
				cfgNotReloadable[cfg] = false
			}
		}
	}

	return nil
}

func cfgAbsent() error {
	// checks to see if any item in cfgNotReloadable has a value of false,
	// if so, that means an unreloadable item has been removed from the config
	defer func() {
		for k := range cfgNotReloadable {
			cfgNotReloadable[k] = false
		}
	}()

	for cfg, v := range cfgNotReloadable {
		if !v {
			action := getActionKeyword(cfg)

			return fmt.Errorf("a non-reloadable config action '%s' was removed. Must restart receptor for changes to take effect", action)
		}
	}

	return nil
}

// InitReload initializes objects required before reload commands are issued.
func InitReload(cPath string, fParseAndRun func([]string) error) error {
	configPath = cPath
	reloadParseAndRun = fParseAndRun

	return parseConfigForReload(configPath, false)
}

func checkReload() error {
	return parseConfigForReload(configPath, true)
}

func (t *reloadCommandType) InitFromString(_ string) (ControlCommand, error) {
	c := &reloadCommand{}

	return c, nil
}

func (t *reloadCommandType) InitFromJSON(_ map[string]interface{}) (ControlCommand, error) {
	c := &reloadCommand{}

	return c, nil
}

func handleError(err error, errorcode int, logger *logger.ReceptorLogger) (map[string]interface{}, error) {
	cfr := make(map[string]interface{})
	cfr["Success"] = false
	cfr["Error"] = fmt.Sprintf("%s ERRORCODE %d", err.Error(), errorcode)
	logger.Warning("Reload not successful: %s", err.Error())

	return cfr, nil
}

func (c *reloadCommand) ControlFunc(_ context.Context, nc *netceptor.Netceptor, _ ControlFuncOperations) (map[string]interface{}, error) {
	// Reload command stops all backends, and re-runs the ParseAndRun() on the
	// initial config file
	nc.Logger.Debug("Reloading")

	// Do a quick check to catch any yaml errors before canceling backends
	err := reloadParseAndRun([]string{"PreReload"})
	if err != nil {
		return handleError(err, 4, nc.Logger)
	}

	// check if non-reloadable items have been added or modified
	err = checkReload()
	if err != nil {
		return handleError(err, 3, nc.Logger)
	}

	// check if non-reloadable items have been removed
	err = cfgAbsent()
	if err != nil {
		return handleError(err, 3, nc.Logger)
	}

	nc.CancelBackends()
	// reloadParseAndRun is a ParseAndRun closure, set in receptor.go/main()
	err = reloadParseAndRun([]string{"PreReload", "Reload"})
	if err != nil {
		return handleError(err, 4, nc.Logger)
	}

	cfr := make(map[string]interface{})
	cfr["Success"] = true

	return cfr, nil
}
