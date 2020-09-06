// +build !windows

package services

import (
	"crypto/tls"
	"github.com/creack/pty"
	"github.com/project-receptor/receptor/pkg/cmdline"
	"github.com/project-receptor/receptor/pkg/logger"
	"github.com/project-receptor/receptor/pkg/netceptor"
	"github.com/project-receptor/receptor/pkg/utils"
	"net"
	"os/exec"
	"strings"
)

func runCommand(qc net.Conn, command string) error {
	args := strings.Split(command, " ")
	cmd := exec.Command(args[0], args[1:]...)
	tty, err := pty.Start(cmd)
	if err != nil {
		return err
	}
	utils.BridgeConns(tty, "external command", qc, "command service")
	return nil
}

// CommandService listens on the Receptor network and runs a local command
func CommandService(s *netceptor.Netceptor, service string, tlscfg *tls.Config, command string) {
	qli, err := s.ListenAndAdvertise(service, tlscfg, map[string]string{
		"type": "Command Service",
	})
	if err != nil {
		logger.Error("Error listening on Receptor network: %s\n", err)
		return
	}
	for {
		qc, err := qli.Accept()
		if err != nil {
			logger.Error("Error accepting connection on Receptor network: %s\n", err)
			return
		}
		go func() {
			err := runCommand(qc, command)
			if err != nil {
				logger.Error("Error running command: %s\n", err)
			}
			_ = qc.Close()
		}()
	}
}

// CommandSvcCfg is the cmdline configuration object for a command service
type CommandSvcCfg struct {
	Service string `required:"true" description:"Receptor service name to bind to"`
	Command string `required:"true" description:"Command to execute on a connection"`
	TLS     string `description:"Name of TLS server config"`
}

// Run runs the action
func (cfg CommandSvcCfg) Run() error {
	logger.Info("Running command service %s\n", cfg)
	tlscfg, err := netceptor.MainInstance.GetServerTLSConfig(cfg.TLS)
	if err != nil {
		return err
	}
	go CommandService(netceptor.MainInstance, cfg.Service, tlscfg, cfg.Command)
	return nil
}

func init() {
	cmdline.AddConfigType("command-service",
		"Run an interactive command via a Receptor service", CommandSvcCfg{}, false, false, false, false, servicesSection)
}
