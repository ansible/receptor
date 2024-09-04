package cmd

import (
	"fmt"
	"os"
	"reflect"

	receptorVersion "github.com/ansible/receptor/internal/version"
	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile        string
	version        bool
	receptorConfig *ReceptorConfig
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "receptor",
	Short: "Run a receptor instance.",
	Long: `
	Receptor is an overlay network intended to ease the distribution of work across a large and dispersed collection of workers.
	Receptor nodes establish peer-to-peer connections with each other via existing networks.
	Once connected, the receptor mesh provides datagram (UDP-like) and stream (TCP-like) capabilities to applications, as well as robust unit-of-work handling with resiliency against transient network failures.`,
	Run: func(cmd *cobra.Command, args []string) {
		if version {
			fmt.Println(receptorVersion.Version)
			os.Exit(0)
		}
		var err error
		var certifcatesConfig *CertificatesConfig

		receptorConfig, certifcatesConfig, err = ParseConfigs(cfgFile)
		if err != nil {
			fmt.Printf("unable to decode into struct, %v", err)
			os.Exit(1)
		}

		isEmptyReceptorConfig := isConfigEmpty(reflect.ValueOf(*receptorConfig))

		RunConfigV2(reflect.ValueOf(*certifcatesConfig))
		if isEmptyReceptorConfig {
			fmt.Println("empty receptor config, skipping...")
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

	rootCmd.Flags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/receptor.yaml)")
	rootCmd.Flags().BoolVar(&version, "version", false, "Show the Receptor version")
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

	viper.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("Config file changed:", e.Name)

		var newConfig *ReceptorConfig
		viper.Unmarshal(&newConfig)

		// used because OnConfigChange runs twice for some reason
		// allows to skip empty first config
		isEmpty := isConfigEmpty(reflect.ValueOf(*newConfig))
		if isEmpty {
			return
		}

		SetConfigDefaults(newConfig)

		isEqual := reflect.DeepEqual(*receptorConfig, *newConfig)
		if !isEqual {
			fmt.Println("reloading backends")

			// this will do a reload of all reloadable services
			// TODO: Optimize to only reload services that have config change
			// NOTE: Make sure to account for two things
			// if current config had two services then new config has zero cancel those backends
			// if services has two items in a slice and one of them has changed iterate and reload on changed service
			netceptor.MainInstance.CancelBackends()

			var reloadableServices *ReloadableServices
			viper.Unmarshal(&reloadableServices)
			ReloadServices(reflect.ValueOf(*reloadableServices))
		}
	})
	viper.WatchConfig()

	err := viper.ReadInConfig()
	if err == nil {
		fmt.Fprintln(os.Stdout, "Using config file:", viper.ConfigFileUsed())
	}
}
