package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "viper",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
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

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.viper.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".viper" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".viper")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

// Cmds in here
// --help: Show this help

// --config <filename>: Load additional config options from a YAML file

// --bash-completion: Generate a completion script for the bash shell
// 	  Run ". <(receptor --bash-completion)" to activate now

// --node: Node configuration of this instance (required)
//    id=<string>: Node ID. Defaults to local hostname.
//    datadir=<string>: Directory in which to store node data
//    firewallrules=<JSON list of JSON dict of JSON data to JSON data>: Firewall Rules (see documentation for syntax)
//    maxidleconnectiontimeout=<string>: Max duration with no traffic before a backend connection is timed out and refreshed.

// --local-only: Run a self-contained node with no backends

// --version: Show the Receptor version

// --log-level: Set specific log level output
//    level=<string>: Log level: Error, Warning, Info or Debug (default: error)

// --trace: Enables packet tracing output

// --control-service: Run a control service
//    service=<string>: Receptor service name to listen on (default: control)
//    filename=<string>: Filename of local Unix socket to bind to the service
//    permissions=<int>: Socket file permissions (default: 0600)
//    tls=<string>: Name of TLS server config for the Receptor listener
//    tcplisten=<string>: Local TCP port or host:port to bind to the control service
//    tcptls=<string>: Name of TLS server config for the TCP listener
