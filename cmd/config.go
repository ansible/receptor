package cmd

import (
	"fmt"
	"os"
	"reflect"

	"github.com/ansible/receptor/pkg/backends"
	"github.com/ansible/receptor/pkg/certificates"
	"github.com/ansible/receptor/pkg/controlsvc"
	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/services"
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

type Reloader interface {
	Reload() error
}

type ReceptorConfig struct {
	// Used pointer structs to apply defaults to config
	Node              *types.NodeCfg
	Trace             logger.TraceCfg
	LogLevel          *logger.LoglevelCfg              `mapstructure:"log-level"`
	ControlServices   []*controlsvc.CmdlineConfigUnix  `mapstructure:"control-services"`
	TLSClients        []netceptor.TLSClientConfig      `mapstructure:"tls-clients"`
	TLSServer         []netceptor.TLSServerConfig      `mapstructure:"tls-servers"`
	WorkCommands      []workceptor.CommandWorkerCfg    `mapstructure:"work-commands"`
	WorkKubernetes    []*workceptor.KubeWorkerCfg      `mapstructure:"work-kubernetes"`
	WorkSigning       workceptor.SigningKeyPrivateCfg  `mapstructure:"work-signing"`
	WorkVerification  workceptor.VerifyingKeyPublicCfg `mapstructure:"work-verification"`
	IPRouters         []services.IPRouterCfg
	TCPClients        []services.TCPProxyOutboundCfg  `mapstructure:"tcp-clients"`
	TCPServers        []services.TCPProxyInboundCfg   `mapstructure:"tcp-servers"`
	UDPClients        []services.TCPProxyInboundCfg   `mapstructure:"udp-clients"`
	UDPServers        []services.TCPProxyInboundCfg   `mapstructure:"udp-servers"`
	UnixSocketClients []services.UnixProxyOutboundCfg `mapstructure:"unix-socket-clients"`
	UnixSocketServers []services.UnixProxyInboundCfg  `mapstructure:"unix-socket-servers"`
}

type CertificatesConfig struct {
	InitCA  certificates.InitCAConfig    `mapstructure:"cert-init"`
	MakeReq []certificates.MakeReqConfig `mapstructure:"cert-makereqs"`
	SignReq []certificates.SignReqConfig `mapstructure:"cert-signreqs"`
}

type BackendConfig struct {
	TCPListeners []*backends.TCPListenerCfg       `mapstructure:"tcp-listeners"`
	UDPListeners []*backends.UDPListenerCfg       `mapstructure:"udp-listeners"`
	WSListeners  []*backends.WebsocketListenerCfg `mapstructure:"ws-listeners"`
	TCPPeers     []*backends.TCPDialerCfg         `mapstructure:"tcp-peers"`
	UDPPeers     []*backends.UDPDialerCfg         `mapstructure:"udp-peers"`
	WSPeers      []*backends.WebsocketDialerCfg   `mapstructure:"ws-peers"`
	LocalOnly    backends.NullBackendCfg          `mapstructure:"local-only"`
}

func PrintPhaseErrorMessage(configName string, phase string, err error) {
	fmt.Printf("ERROR: %s for %s on %s phase\n", err, configName, phase)
}

func ParseReceptorConfig(configFile string) (*ReceptorConfig, error) {
	var receptorConfig ReceptorConfig
	err := viper.Unmarshal(&receptorConfig)
	if err != nil {
		return nil, err
	}

	return &receptorConfig, nil
}

func ParseCertificatesConfig(configFile string) (*CertificatesConfig, error) {
	var certifcatesConfig CertificatesConfig
	err := viper.Unmarshal(&certifcatesConfig)
	if err != nil {
		return nil, err
	}

	return &certifcatesConfig, nil
}

func ParseBackendConfig(configFile string) (*BackendConfig, error) {
	var backendConfig BackendConfig
	err := viper.Unmarshal(&backendConfig)
	if err != nil {
		return nil, err
	}

	return &backendConfig, nil
}

func isConfigEmpty(v reflect.Value) bool {
	isEmpty := true
	for i := 0; i < v.NumField(); i++ {
		if reflect.Value.IsZero(v.Field(i)) {
			continue
		}
		isEmpty = false
	}

	return isEmpty
}

func RunConfigV2(v reflect.Value) {
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

// RunPhases runs the appropriate function (Init, Prepare, Run) on a command.
func RunPhases(phase string, v reflect.Value) {
	cmd := v.Interface()
	var err error

	if phase == "Init" {
		switch c := cmd.(type) {
		case Initer:
			err = c.Init()
			if err != nil {
				PrintPhaseErrorMessage(v.Type().Name(), phase, err)
			}
		default:
		}
	}
	if phase == "Prepare" {
		switch c := cmd.(type) {
		case Preparer:
			err = c.Prepare()
			if err != nil {
				PrintPhaseErrorMessage(v.Type().Name(), phase, err)
			}
		default:
		}
	}
	if phase == "Run" {
		switch c := cmd.(type) {
		case Runer:
			err = c.Run()
			if err != nil {
				PrintPhaseErrorMessage(v.Type().Name(), phase, err)
			}
		default:
		}
	}
}

// ReloadServices iterates through key/values calling reload on applicable services.
func ReloadServices(v reflect.Value) {
	for i := 0; i < v.NumField(); i++ {
		// if the services is not initialised, skip
		if reflect.Value.IsZero(v.Field(i)) {
			continue
		}

		var err error
		switch v.Field(i).Kind() {
		case reflect.Slice:
			// iterate over all the type fields
			for j := 0; j < v.Field(i).Len(); j++ {
				serviceItem := v.Field(i).Index(j).Interface()
				switch c := serviceItem.(type) {
				// check to see if the selected type field satisfies reload
				// call reload on cfg object
				case Reloader:
					err = c.Reload()
					if err != nil {
						PrintPhaseErrorMessage(v.Type().Name(), "reload", err)
					}
				// if cfg object does not satisfy, do nothing
				default:
				}
			}
		// runs for non slice fields
		default:
			switch c := v.Field(i).Interface().(type) {
			case Reloader:
				err = c.Reload()
				if err != nil {
					PrintPhaseErrorMessage(v.Type().Name(), "reload", err)
				}
			default:
			}
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
