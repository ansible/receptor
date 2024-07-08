package cmd

import (
	"fmt"
	"os"
	"reflect"

	receptorVersion "github.com/ansible/receptor/internal/version"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	version bool
	latest  bool
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "receptor",
	Short: "Run a receptor instance.",
	Long: `
	Receptor is an overlay network intended to ease the distribution of work across a large and dispersed collection of workers. Receptor nodes establish peer-to-peer connections with each other via existing networks. Once connected, the receptor mesh provides datagram (UDP-like) and stream (TCP-like) capabilities to applications, as well as robust unit-of-work handling with resiliency against transient network failures.`,
	Run: func(cmd *cobra.Command, args []string) {
		if version {
			fmt.Println(receptorVersion.Version)
			os.Exit(0)
		}
		receptorConfig, certifcatesConfig, err := ParseConfigs(cfgFile)
		if err != nil {
			fmt.Printf("unable to decode into struct, %v", err)
			os.Exit(1)
		}

		isEmptyReceptorConfig := isConfigEmpty(reflect.ValueOf(*receptorConfig))

		RunConfigV2(reflect.ValueOf(*certifcatesConfig))
		if isEmptyReceptorConfig {
			os.Exit(0)
		}

		SetConfigDefaults(receptorConfig)
		RunConfigV2(reflect.ValueOf(*receptorConfig))
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.Flags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.receptor.yaml)")
	rootCmd.Flags().BoolVar(&version, "version", false, "Show the Receptor version")
	rootCmd.Flags().BoolVar(&latest, "latest", false, "Use the latest config and cli")
}

var FindMe = true

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName("receptor")
	}

	viper.AutomaticEnv()

	err := viper.ReadInConfig()
	if err == nil {
		fmt.Fprintln(os.Stdout, "Using config file:", viper.ConfigFileUsed())
	}
}
