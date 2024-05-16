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
func RunConfig(config Config) {
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
