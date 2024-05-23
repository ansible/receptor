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
	cmd.Execute()

	if netceptor.MainInstance.BackendCount() == 0 {
		netceptor.MainInstance.Logger.Warning("Nothing to do - no backends are running.\n")
		fmt.Printf("Run %s --help for command line instructions.\n", os.Args[0])
		os.Exit(1)
	}

	netceptor.MainInstance.Logger.Info("Initialization complete\n")

	<-netceptor.MainInstance.NetceptorDone()
}
