package cmd

import (
	"fmt"
	"os"
	"reflect"

	"github.com/ansible/receptor/pkg/backends"
	"github.com/ansible/receptor/pkg/controlsvc"
	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/types"
	"github.com/ansible/receptor/pkg/workceptor"
	"github.com/ghjm/cmdline"
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

type Config struct {
	Node             types.NodeCfg
	Trace            logger.TraceCfg
	LocalOnly        backends.NullBackendCfg          `mapstructure:"local-only"`
	LogLevel         logger.LoglevelCfg               `mapstructure:"log-level"`
	ControlServices  []controlsvc.CmdlineConfigUnix   `mapstructure:"control-services"`
	TCPPeers         []backends.TCPDialerCfg          `mapstructure:"tcp-peers"`
	UDPPeers         []backends.UDPDialerCfg          `mapstructure:"udp-peers"`
	WSPeers          []backends.WebsocketDialerCfg    `mapstructure:"ws-peers"`
	TCPListeners     []backends.TCPListenerCfg        `mapstructure:"tcp-listeners"`
	UDPListeners     []backends.UDPListenerCfg        `mapstructure:"udp-listeners"`
	WSListeners      []backends.WebsocketListenerCfg  `mapstructure:"ws-listeners"`
	TLSClients       []netceptor.TLSClientConfig      `mapstructure:"tls-clients"`
	TLSServer        []netceptor.TLSServerConfig      `mapstructure:"tls-clients"`
	WorkCommands     []workceptor.CommandWorkerCfg    `mapstructure:"work-commands"`
	WorkKubernetes   []workceptor.KubeWorkerCfg       `mapstructure:"work-kubernetes"`
	WorkSigning      workceptor.SigningKeyPrivateCfg  `mapstructure:"work-signing"`
	WorkVerification workceptor.VerifyingKeyPublicCfg `mapstructure:"work-verification"`
	// need certs
}

// ParseConfig returns a config struct that has unmarshaled a yaml config file
func ParseConfig(configFile string) (*Config, error) {
	if configFile == "" {
		fmt.Fprintln(os.Stderr, "Could not locate config file (default is $HOME/.receptor.yaml)")
		os.Exit(1)
	}
	var config Config
	err := viper.Unmarshal(&config)
	if err != nil {
		return nil, err

	}
	return &config, nil
}

// RunConfig spins up receptor based on the provided yaml config
func RunConfigV2(config Config) {
	v := reflect.ValueOf(config)
	phases := []string{"Init", "Prepare", "Run"}

	for _, phase := range phases {
		for i := 0; i < v.NumField(); i++ {
			if reflect.Value.IsZero(v.Field(i)) {
				continue
			}

			switch v.Field(i).Kind() {
			case reflect.Slice:
				for j := 0; j < v.Field(i).Len(); j++ {
					RunPhases(phase, v.Field(i).Index(j))
				}
			default:
				RunPhases(phase, v.Field(i))
			}

		}
	}
}

// RunPhases runs the appropriate function (Init, Prepare, Run) on a command
func RunPhases(phase string, v reflect.Value) {
	cmd := v.Interface()
	// may add logging for the phases when they are invoked
	if phase == "Init" {
		switch c := cmd.(type) {
		case Initer:
			c.Init()
		}
	}
	if phase == "Prepare" {
		switch c := cmd.(type) {
		case Preparer:
			c.Prepare()
		}
	}
	if phase == "Run" {
		switch c := cmd.(type) {
		case Runer:
			c.Run()
		}
	}
}

func RunConfigV1() {

	cl := cmdline.NewCmdline()
	cl.AddConfigType("node", "Specifies the node configuration of this instance", types.NodeCfg{}, cmdline.Required, cmdline.Singleton)
	cl.AddConfigType("local-only", "Runs a self-contained node with no backend", backends.NullBackendCfg{}, cmdline.Singleton)

	// Add registered config types from imported modules
	for _, appName := range []string{
		"receptor-version",
		"receptor-logging",
		"receptor-tls",
		"receptor-certificates",
		"receptor-control-service",
		"receptor-command-service",
		"receptor-ip-router",
		"receptor-proxies",
		"receptor-backends",
		"receptor-workers",
	} {
		cl.AddRegisteredConfigTypes(appName)
	}

	osArgs := os.Args[1:]

	err := cl.ParseAndRun(osArgs, []string{"Init", "Prepare", "Run"}, cmdline.ShowHelpIfNoArgs)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}
	if cl.WhatRan() != "" {
		// We ran an exclusive command, so we aren't starting any back-ends
		os.Exit(0)
	}

	configPath := ""
	for i, arg := range osArgs {
		if arg == "--config" || arg == "-c" {
			if len(osArgs) > i+1 {
				configPath = osArgs[i+1]
			}

			break
		}
	}

	// only allow reloading if a configuration file was provided. If ReloadCL is
	// not set, then the control service reload command will fail
	if configPath != "" {
		// create closure with the passed in args to be ran during a reload
		reloadParseAndRun := func(toRun []string) error {
			return cl.ParseAndRun(osArgs, toRun)
		}
		err = controlsvc.InitReload(configPath, reloadParseAndRun)
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			os.Exit(1)
		}
	}
}
