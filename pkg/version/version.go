package version

import (
	"fmt"
)

// Version is receptor app version
var Version string

// CmdlineCfg is a cmdline-compatible struct for a --version command
type CmdlineCfg struct{}

// Run runs the action
func (cfg CmdlineCfg) Run() error {
	if Version == "" {
		fmt.Printf("Version unknown\n")
	} else {
		fmt.Printf("%s\n", Version)
	}
	return nil
}
