//go:generate mockgen -source=command.go -destination=mock_services/command.go
//go:build !windows
// +build !windows

package services

import (
	"crypto/tls"
	"errors"
	"net"
	"os/exec"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/creack/pty"
	"github.com/ghjm/cmdline"
	"github.com/google/shlex"
	"github.com/spf13/viper"
)

type NetCForCommandService interface {
	GetLogger() *logger.ReceptorLogger
	ListenAndAdvertise(service string, tlscfg *tls.Config, tags map[string]string) (*netceptor.Listener, error)
}

func runCommand(qc net.Conn, command string, logger *logger.ReceptorLogger, utilsLib UtilsLib) error {
	// Note: shlex.Split does not return error for the empty string
	args, err := shlex.Split(command)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		return errors.New("shell command is empty")
	}
	cmd := exec.Command(args[0], args[1:]...)
	tty, err := pty.Start(cmd)
	if err != nil {
		return err
	}
	utilsLib.BridgeConns(tty, "external command", qc, "command service", logger)

	return nil
}

// CommandService listens on the Receptor network and runs a local command.
func CommandService(s NetCForCommandService, service string, tlscfg *tls.Config, command string, utilsLib UtilsLib) {
	if command == "" {
		s.GetLogger().Error("initializing command service: command not provided\n")

		return
	}

	qli, err := s.ListenAndAdvertise(service, tlscfg, map[string]string{
		"type": "Command Service",
	})
	if err != nil {
		s.GetLogger().Error("listening on Receptor network: %s\n", err)

		return
	}
	for {
		qc, err := qli.Accept()
		if err != nil {
			s.GetLogger().Error("accepting connection on Receptor network: %s\n", err)

			return
		}
		go func() {
			err := runCommand(qc, command, s.GetLogger(), utilsLib)
			if err != nil {
				s.GetLogger().Error("running command: %s\n", err)
			}
			_ = qc.Close()
		}()
	}
}

// commandSvcCfg is the cmdline configuration object for a command service.
type CommandSvcCfg struct {
	Service string `required:"true" description:"Receptor service name to bind to"`
	Command string `required:"true" description:"Command to execute on a connection"`
	TLS     string `description:"Name of TLS server config"`
}

// Run runs the action.
func (cfg CommandSvcCfg) Run() error {
	netceptor.MainInstance.Logger.Info("Running command service %s\n", cfg)
	tlscfg, err := netceptor.MainInstance.GetServerTLSConfig(cfg.TLS)
	if err != nil {
		return err
	}

	go CommandService(netceptor.MainInstance, cfg.Service, tlscfg, cfg.Command, &UtilsTCPWrapper{})

	return nil
}

func init() {
	version := viper.GetInt("version")
	if version > 1 {
		return
	}
	cmdline.RegisterConfigTypeForApp("receptor-command-service",
		"command-service", "Run an interactive command via a Receptor service", CommandSvcCfg{}, cmdline.Section(servicesSection))
}
