package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/ansible/receptor/pkg"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func serveCmd() *cobra.Command {
	var cfgPath string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run receptor",
		RunE: func(*cobra.Command, []string) error {
			var cfg pkg.Receptor

			v := viper.New()
			v.SetConfigFile(cfgPath)
			v.AutomaticEnv()
			if err := v.ReadInConfig(); err != nil {
				return fmt.Errorf("could not read serve config file: %w", err)
			}
			if err := v.UnmarshalExact(&cfg); err != nil {
				return fmt.Errorf("could not unmarshal serve config file: %w", err)
			}

			ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
			defer cancel()

			return cfg.Serve(ctx)
		},
	}
	cmd.Flags().StringVarP(&cfgPath, "configuration-file", "f", "", "Path to the configuration file containing node definition")
	cmd.MarkFlagRequired("configuration-file")
	cmd.MarkFlagFilename("configuration-file")

	return cmd
}
