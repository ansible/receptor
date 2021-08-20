package main

import (
	"fmt"
	"os"

	"github.com/ansible/receptor/pkg/version"
	"github.com/spf13/cobra"
)

func Execute() {
	cmd := &cobra.Command{
		Use:   "receptor",
		Short: "Receptor is a network mesh overlayer",
		Long: "Receptor is a flexible multi-service relayer with remote " +
			"execution and orchestration capabilities linking controllers with " +
			"executors across a mesh of nodes.",
		Version: version.Version,
	}

	cmd.AddCommand(tlsCmd())
	cmd.AddCommand(serveCmd())

	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func main() {
	Execute()
}
