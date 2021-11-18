package controlsvc

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
	"gopkg.in/yaml.v2"
)

type (
	reloadCommandType struct{}
	reloadCommand     struct{}
)

var (
	configPath       = ""
	mu               sync.Mutex
	cfgPrevious      = make(map[string]struct{})
	cfgNext          = make(map[string]struct{})
	backendModified  = false
	loglevelModified = false
	loglevelPresent  = false
)

var reloadParseAndRun = func(toRun []string) error {
	return fmt.Errorf("no configuration file was provided")
}

var reloadableActions = map[string]*bool{
	"tcp-peer":     &backendModified,
	"tcp-listener": &backendModified,
	"ws-peer":      &backendModified,
	"ws-listener":  &backendModified,
	"udp-peer":     &backendModified,
	"udp-listener": &backendModified,
	"local-only":   &backendModified,
	"log-level":    &loglevelModified,
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

func parseConfig(filename string, cfgMap map[string]struct{}) error {
	data, err := ioutil.ReadFile(filename)
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
		cfgMap[cfg] = struct{}{}
	}

	return nil
}

func checkReload() error {
	// Determine which items from the old config have been added or modified
	for cfg := range cfgNext {
		action := getActionKeyword(cfg)
		_, isReloadable := reloadableActions[action]
		_, inPrevious := cfgPrevious[cfg]
		if !isReloadable && !inPrevious {
			return fmt.Errorf("a non-reloadable config action '%s' was added or modified. Must restart receptor for these changes to take effect", action)
		}
		if isReloadable && !inPrevious {
			*reloadableActions[action] = true
		}
	}

	// Determine which items from the old config are no longer present, or have been modified
	for cfg := range cfgPrevious {
		action := getActionKeyword(cfg)
		_, isReloadable := reloadableActions[action]
		_, inNext := cfgNext[cfg]
		if !isReloadable && !inNext {
			return fmt.Errorf("a non-reloadable config action '%s' was removed or modified. Must restart receptor for changes to take effect", action)
		}
		if isReloadable && !inNext {
			*reloadableActions[action] = true
		}
	}

	// check if log-level is defined
	for cfg := range cfgNext {
		loglevelPresent = getActionKeyword(cfg) == "log-level"
		if loglevelPresent {
			break
		}
	}

	return nil
}

func resetAfterReload() {
	cfgNext = make(map[string]struct{})
	backendModified = false
	loglevelModified = false
	loglevelPresent = false
}

// InitReload initializes objects required before reload commands are issued.
func InitReload(cPath string, fParseAndRun func([]string) error) error {
	configPath = cPath
	reloadParseAndRun = fParseAndRun

	return parseConfig(configPath, cfgPrevious)
}

func (t *reloadCommandType) InitFromString(params string) (ControlCommand, error) {
	c := &reloadCommand{}

	return c, nil
}

func (t *reloadCommandType) InitFromJSON(config map[string]interface{}) (ControlCommand, error) {
	c := &reloadCommand{}

	return c, nil
}

func handleError(err error, errorcode int) (map[string]interface{}, error) {
	cfr := make(map[string]interface{})
	cfr["Success"] = false
	cfr["Error"] = fmt.Sprintf("%s ERRORCODE %d", err.Error(), errorcode)
	logger.Warning("Reload not successful: %s", err.Error())

	return cfr, nil
}

func (c *reloadCommand) ControlFunc(ctx context.Context, nc *netceptor.Netceptor, cfo ControlFuncOperations) (map[string]interface{}, error) {
	// grab a mutex, so that only one goroutine can call reload at a time
	mu.Lock()
	defer mu.Unlock()

	logger.Debug("Reloading")
	defer resetAfterReload()

	cfr := make(map[string]interface{})
	cfr["Success"] = true

	// do a quick check to catch any yaml errors before canceling backends
	err := reloadParseAndRun([]string{"PreReload"})
	if err != nil {
		return handleError(err, 4)
	}

	err = parseConfig(configPath, cfgNext)
	if err != nil {
		return handleError(err, 4)
	}

	// check if non-reloadable items have been added or modified
	err = checkReload()
	if err != nil {
		return handleError(err, 3)
	}

	if !loglevelModified && !backendModified {
		logger.Debug("Nothing to reload")

		return cfr, nil
	}

	toRun := []string{}
	// if backend has been modified,  reload the backend
	if backendModified {
		nc.CancelBackends()
		toRun = append(toRun, "ReloadBackend")
	}

	// if log-level has been modified reload only the logger 
	if loglevelPresent && loglevelModified {
		toRun = append(toRun, "ReloadLogger")
	}

	// if log level has been removed after relaod, reset the logger
	if !loglevelPresent {
		logger.InitLogger()
	}

	// reloadParseAndRun is a ParseAndRun closure, set in receptor.go/main()
	err = reloadParseAndRun(toRun)
	if err != nil {
		return handleError(err, 4)
	}

	// set old config to new config, only if successful
	cfgPrevious = make(map[string]struct{})
	for cfg := range cfgNext {
		cfgPrevious[cfg] = struct{}{}
	}

	return cfr, nil
}
