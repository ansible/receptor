package cmd

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	currentConfig = struct {
		LogLevel    string
		FeatureFlag bool
	}{}
	mu sync.RWMutex
)

var rootCmd = &cobra.Command{
	Use:   "spike",
	Short: "Watches and reloads config changes",
	Run: func(cmd *cobra.Command, args []string) {
		loadConfig()

		viper.OnConfigChange(func(e fsnotify.Event) {
			fmt.Printf("Config file changed: %s\n", e.Name)
			handleConfigChange()
		})
		viper.WatchConfig()

		// Keep the application alive
		select {}
	},
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func loadConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config: %v", err)
	}
	handleConfigChange()

	viper.OnConfigChange(func(e fsnotify.Event) {
		fmt.Printf("Config file change detected: %s — waiting briefly...\n", e.Name)
		handleConfigChange()
	})

	viper.WatchConfig()
}

func handleConfigChange() {
	mu.Lock()
	defer mu.Unlock()

	// this is a workaround for a viper bug where values are set to ''
	// before getting changed to the new value. :(
	time.Sleep(1 * time.Second)

	err := viper.ReadInConfig() // Make sure you read the latest config
	if err != nil {
		log.Printf("Config reload failed: %v\n", err)

		return
	}

	newLogLevel := viper.GetString("logLevel")
	newFeatureFlag := viper.GetBool("featureFlag")

	if newLogLevel == "" || !viper.IsSet("logLevel") {
		log.Printf("Log level not set in config")

		return
	}

	if !viper.IsSet("FeatureFlag") {
		log.Printf("Feature Flag is not in config")

		return
	}

	if newLogLevel != currentConfig.LogLevel {
		log.Printf("Log level changed from '%s' to '%s'\n", currentConfig.LogLevel, newLogLevel)
		currentConfig.LogLevel = newLogLevel
	}

	if newFeatureFlag != currentConfig.FeatureFlag {
		log.Printf("Feature flag changed from '%v' to '%v'\n", currentConfig.FeatureFlag, newFeatureFlag)
		currentConfig.FeatureFlag = newFeatureFlag
	}
}
