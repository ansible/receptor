package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ansible/receptor/internal/version"
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

func Execute() {
	var cfgPath string
	var cfgStr string
	cmd := &cobra.Command{
		Use:   "receptor",
		Short: "Receptor is a network mesh overlayer",
		Long: "Receptor is a flexible multi-service relayer with remote " +
			"execution and orchestration capabilities linking controllers with " +
			"executors across a mesh of nodes.",
		Version: version.Version,
		RunE: func(*cobra.Command, []string) error {
			var cfg pkg.Receptor

			v := viper.New()
			v.AutomaticEnv()

			switch {
			case cfgPath == "" && cfgStr == "":
				return fmt.Errorf("specify the node configuration eith with -c or -f")
			case cfgPath != "" && cfgStr != "":
				return fmt.Errorf("set only one of -c and -f")
			case cfgStr != "":
				buf := bytes.NewBuffer([]byte(cfgStr))
				if err := v.ReadConfig(buf); err != nil {
					return fmt.Errorf("given config ist invalid: %w", err)
				}
			case cfgPath == "-":
				if err := v.ReadConfig(os.Stdin); err != nil {
					return fmt.Errorf("could not read serve config file from stdin: %w", err)
				}
			default:
				v.SetConfigFile(cfgPath)
				if err := v.ReadInConfig(); err != nil {
					return fmt.Errorf("could not read serve config file: %w", err)
				}
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
	cmd.Flags().StringVarP(&cfgPath, "configuration-file", "f", "", "Path to the configuration file containing node definition. - for stdin")
	cmd.Flags().StringVarP(&cfgStr, "configuration", "c", "", "Configuration of the node directly as string")
	cmd.MarkFlagRequired("configuration-file")
	cmd.MarkFlagFilename("configuration-file")

	cmd.AddCommand(tlsCmd())

	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func main() {
	Execute()
}
