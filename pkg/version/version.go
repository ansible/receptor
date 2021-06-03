package version

import (
	"fmt"
	"github.com/ghjm/cmdline"
)

// Version is receptor app version
var Version string

// cmdlineCfg is a cmdline-compatible struct for a --version command
type cmdlineCfg struct{}

// Run runs the action
func (cfg cmdlineCfg) Run() error {
	if Version == "" {
		fmt.Printf("Version unknown\n")
	} else {
		fmt.Printf("%s\n", Version)
	}
	return nil
}

func init() {
	cmdline.RegisterConfigTypeForApp("receptor-version",
		"version", "Show the Receptor version", cmdlineCfg{}, cmdline.Exclusive)
}
