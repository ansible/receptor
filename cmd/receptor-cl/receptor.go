package main

import (
	"fmt"
	"os"

	"github.com/ansible/receptor/cmd"
	_ "github.com/ansible/receptor/internal/version"
	_ "github.com/ansible/receptor/pkg/backends"
	_ "github.com/ansible/receptor/pkg/certificates"
	"github.com/ansible/receptor/pkg/netceptor"
	_ "github.com/ansible/receptor/pkg/services"
)

func main() {
	var latest bool
	for _, arg := range os.Args {
		if arg == "--latest" {
			latest = true
		}
	}

	if !latest {
		fmt.Println("Running older cli/config")
		cmd.RunConfigV1()
	} else {
		fmt.Println("Running latest cli/config")
		cmd.Execute()
	}

	if netceptor.MainInstance.BackendCount() == 0 {
		netceptor.MainInstance.Logger.Warning("Nothing to do - no backends are running.\n")
		fmt.Printf("Run %s --help for command line instructions.\n", os.Args[0])
		os.Exit(1)
	}

	netceptor.MainInstance.Logger.Info("Initialization complete\n")

	<-netceptor.MainInstance.NetceptorDone()
}
