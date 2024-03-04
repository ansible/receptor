package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile                  string
	logLevel                 string
	maxidleconnectiontimeout string
	nodeId                   string
	dataDir                  string
	firewallRules            string
)

var rootCmd = &cobra.Command{
	Use:   "receptor",
	Short: "Run a Receptor instance",
	Long:  `Receptor is a distributed messaging system.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("root called")
	},
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

	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "error", "Set specific log level output")
	rootCmd.PersistentFlags().Bool("local-only", false, "Run a self-contained node with no backends")
	rootCmd.PersistentFlags().Bool("version", false, "Show the Receptor version")
	rootCmd.PersistentFlags().Bool("trace", false, "Enables packet tracing output")
	rootCmd.PersistentFlags().Bool("bash-completion", false, "Generate a completion script for the bash shell")
	rootCmd.PersistentFlags().String("node", "", "Node configuration of this instance (required)")

	rootCmd.Flags().StringVar(&maxidleconnectiontimeout, "maxidleconnectiontimeout", "", "Max duration with no traffic before a backend connection is timed out and refreshed.")
	rootCmd.Flags().StringVar(&nodeId, "node-id", "", "Node ID. Defaults to local hostname.")
	rootCmd.Flags().StringVar(&dataDir, "datadir", "", "Directory in which to store node data")
	rootCmd.Flags().StringVar(&firewallRules, "firewallrules", "", "Firewall Rules (see documentation for syntax)")

	rootCmd.MarkFlagRequired("node")

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
