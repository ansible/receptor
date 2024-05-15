/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type Initer interface {
	Init() error
}

type Preparer interface {
	Prepare() error
}

type Runer interface {
	Run() error
}

// type ReceptorCommand interface {
// 	Init() error
// 	// Prepare()
// 	// Run() error
// }

// var receptorCommands = map[string]interface{}{
// 	"node":           types.NodeCfg{},
// 	"log-level":      logger.LoglevelCfg{},
// 	"control-server": controlsvc.CmdlineConfigUnix{},
// 	"tcp-peer":       backends.TCPDialerCfg{},
// }

var (
	cfgFile                  string
	logLevel                 string
	maxidleconnectiontimeout string
	// nodeId                   string
	localOnly bool
	version   bool
	trace     bool
	// dataDir                  string
	// firewallRules            string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "receptor",
	Short: "Run a receptor instance.",
	Long: `
	Receptor is an overlay network intended to ease the distribution of work across a large and dispersed collection of workers. Receptor nodes establish peer-to-peer connections with each other via existing networks. Once connected, the receptor mesh provides datagram (UDP-like) and stream (TCP-like) capabilities to applications, as well as robust unit-of-work handling with resiliency against transient network failures.`,
	Run: func(cmd *cobra.Command, args []string) {
		// cmd.Help()
	},
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.Flags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.receptor.yaml)")

	rootCmd.Flags().StringVar(&logLevel, "log-level", "error", "Set specific log level output")
	rootCmd.Flags().BoolVar(&localOnly, "local-only", false, "Run a self-contained node with no backends")
	rootCmd.Flags().BoolVar(&version, "version", false, "Show the Receptor version")
	rootCmd.Flags().BoolVar(&trace, "trace", false, "Enables packet tracing output")
	// rootCmd.PersistentFlags().String("node", "", "Node configuration of this instance (required)")

	rootCmd.Flags().StringVar(&maxidleconnectiontimeout, "maxidleconnectiontimeout", "", "Max duration with no traffic before a backend connection is timed out and refreshed.")
	// rootCmd.Flags().StringVar(&nodeId, "node-id", "", "Node ID. Defaults to local hostname.")
	// rootCmd.Flags().StringVar(&dataDir, "datadir", "", "Directory in which to store node data")
	// rootCmd.Flags().StringVar(&firewallRules, "firewallrules", "", "Firewall Rules (see documentation for syntax)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

var FindMe = true

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".receptor" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".receptor")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}

	config, err := ParseConfig()
	if err != nil {
		fmt.Printf("unable to decode into struct, %v", err)
		os.Exit(1)
	}
	RunConfig(*config)

	// pp.Printf("%+V\n", config)
}
