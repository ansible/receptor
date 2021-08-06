package main

import "github.com/spf13/cobra"

func tlsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tls",
		Short: "Helper to manage certificates for TLS",
	}
	cmd.AddCommand(initCaCmd())
	cmd.AddCommand(makeReqCmd())
	cmd.AddCommand(signReqCmd())

	return cmd
}
