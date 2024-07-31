package version

import (
	"fmt"

	"github.com/ghjm/cmdline"
	"github.com/spf13/viper"
)

// Version is receptor app version.
var Version string

// cmdlineCfg is a cmdline-compatible struct for a --version command.
type cmdlineCfg struct{}

// Run runs the action.
func (cfg cmdlineCfg) Run() error {
	if Version == "" {
		fmt.Printf("Version unknown\n")
	} else {
		fmt.Printf("%s\n", Version)
	}

	return nil
}

func init() {
	version := viper.GetInt("version")
	if version > 1 {
		return
	}
	cmdline.RegisterConfigTypeForApp("receptor-version",
		"version", "Displays the Receptor version.", cmdlineCfg{}, cmdline.Exclusive)
}
