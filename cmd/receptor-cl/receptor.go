package main

import (
	"fmt"
	"os"

	"github.com/ansible/receptor/cmd"
	"github.com/ansible/receptor/pkg/netceptor"
)

func main() {
	var isV2 bool
	newArgs := []string{}
	for _, arg := range os.Args {
		if arg == "--config-v2" {
			isV2 = true

			continue
		}
		newArgs = append(newArgs, arg)
	}

	os.Args = newArgs

	if isV2 {
		fmt.Println("Running v2 cli/config")
		cmd.Execute()
	} else {
		cmd.RunConfigV1()
	}

	for _, arg := range os.Args {
		if arg == "--help" || arg == "-h" {
			os.Exit(0)
		}
	}

	if netceptor.MainInstance.BackendCount() == 0 {
		netceptor.MainInstance.Logger.Warning("Nothing to do - no backends are running.\n")
		fmt.Printf("Run %s --help for command line instructions.\n", os.Args[0])
		os.Exit(1)
	}

	netceptor.MainInstance.Logger.Info("Initialization complete\n")

	<-netceptor.MainInstance.NetceptorDone()
}
