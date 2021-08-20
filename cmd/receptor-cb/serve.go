package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ansible/receptor/pkg"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// TODO: Shameless copy from signal pkg. Remove when switching to go 1.16.
type signalCtx struct {
	context.Context

	cancel  context.CancelFunc
	signals []os.Signal
	ch      chan os.Signal
}

// TODO: Shameless copy from signal pkg. Remove when switching to go 1.16.
func (c *signalCtx) stop() {
	c.cancel()
	signal.Stop(c.ch)
}

// TODO: Shameless copy from signal pkg. Remove when switching to go 1.16.
func notifyContext(parent context.Context, signals ...os.Signal) (ctx context.Context, stop context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	c := &signalCtx{
		Context: ctx,
		cancel:  cancel,
		signals: signals,
	}
	c.ch = make(chan os.Signal, 1)
	signal.Notify(c.ch, c.signals...)
	if ctx.Err() == nil {
		go func() {
			select {
			case <-c.ch:
				c.cancel()
			case <-c.Done():
			}
		}()
	}

	return c, c.stop
}

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

			// TODO replace with signal.NotifyContext when switching to go 1.16.
			ctx, cancel := notifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
			defer cancel()

			return cfg.Serve(ctx)
		},
	}
	cmd.Flags().StringVarP(&cfgPath, "configuration-file", "f", "", "Path to the configuration file containing node definition")
	cmd.MarkFlagRequired("configuration-file")
	cmd.MarkFlagFilename("configuration-file")

	return cmd
}
