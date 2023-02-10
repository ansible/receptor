//go:build !windows && !no_command_service && !windows && !no_services
// +build !windows,!no_command_service,!windows,!no_services

package services

import (
	"crypto/tls"
	"net"
	"os/exec"

	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/utils"
	"github.com/creack/pty"
	"github.com/ghjm/cmdline"
	"github.com/google/shlex"
)

func runCommand(qc net.Conn, command string) error {
	args, err := shlex.Split(command)
	if err != nil {
		return err
	}
	cmd := exec.Command(args[0], args[1:]...)
	tty, err := pty.Start(cmd)
	if err != nil {
		return err
	}
	utils.BridgeConns(tty, "external command", qc, "command service")

	return nil
}

// CommandService listens on the Receptor network and runs a local command.
func CommandService(s *netceptor.Netceptor, service string, tlscfg *tls.Config, command string) {
	qli, err := s.ListenAndAdvertise(service, tlscfg, map[string]string{
		"type": "Command Service",
	})
	if err != nil {
		s.Logger.Error("Error listening on Receptor network: %s\n", err)

		return
	}
	for {
		qc, err := qli.Accept()
		if err != nil {
			s.Logger.Error("Error accepting connection on Receptor network: %s\n", err)

			return
		}
		go func() {
			err := runCommand(qc, command)
			if err != nil {
				s.Logger.Error("Error running command: %s\n", err)
			}
			_ = qc.Close()
		}()
	}
}

// commandSvcCfg is the cmdline configuration object for a command service.
type commandSvcCfg struct {
	Service string `required:"true" description:"Receptor service name to bind to"`
	Command string `required:"true" description:"Command to execute on a connection"`
	TLS     string `description:"Name of TLS server config"`
}

// Run runs the action.
func (cfg commandSvcCfg) Run() error {
	netceptor.MainInstance.Logger.Info("Running command service %s\n", cfg)
	tlscfg, err := netceptor.MainInstance.GetServerTLSConfig(cfg.TLS)
	if err != nil {
		return err
	}
	go CommandService(netceptor.MainInstance, cfg.Service, tlscfg, cfg.Command)

	return nil
}

func init() {
	cmdline.RegisterConfigTypeForApp("receptor-command-service",
		"command-service", "Run an interactive command via a Receptor service", commandSvcCfg{}, cmdline.Section(servicesSection))
}
