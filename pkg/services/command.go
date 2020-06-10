package services

import (
	"crypto/tls"
	"github.com/ghjm/sockceptor/pkg/cmdline"
	"github.com/ghjm/sockceptor/pkg/debug"
	"github.com/ghjm/sockceptor/pkg/netceptor"
	"io"
	"net"
	"os/exec"
	"strings"
	"sync"
)

func copyReadToWrite(writer io.WriteCloser, reader io.Reader, wg *sync.WaitGroup) {
	defer wg.Done()
	_, err := io.Copy(writer, reader)
	if err != nil {
		debug.Printf("Error in pipe: %s\n", err)
	}
	err = writer.Close()
	if err != nil {
		debug.Printf("Error in pipe: %s\n", err)
	}
}

func runCommand(qc net.Conn, command string) error {
	args := strings.Split(command, " ")
	cmd := exec.Command(args[0], args[1:]...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	err = cmd.Start()
	if err != nil {
		return err
	}
	wg := &sync.WaitGroup{}
	wg.Add(3)
	go copyReadToWrite(stdin, qc, wg)
	go copyReadToWrite(qc, stdout, wg)
	go copyReadToWrite(qc, stderr, wg)
	wg.Wait()
	return nil
}

// CommandService listens on the Receptor network and runs a local command
func CommandService(s *netceptor.Netceptor, service string, tlscfg *tls.Config, command string) {
	qli, err := s.ListenAndAdvertise(service, tlscfg, map[string]string{
		"type": "Command Service",
	})
	if err != nil {
		debug.Printf("Error listening on Receptor network: %s\n", err)
		return
	}
	for {
		qc, err := qli.Accept()
		if err != nil {
			debug.Printf("Error accepting connection on Receptor network: %s\n", err)
			return
		}
		go func() {
			err := runCommand(qc, command)
			if err != nil {
				debug.Printf("Error running command: %s\n", err)
			}
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
	debug.Printf("Running command service %s\n", cfg)
	tlscfg, err := netceptor.GetServerTLSConfig(cfg.TLS)
	if err != nil {
		return err
	}
	go CommandService(netceptor.MainInstance, cfg.Service, tlscfg, cfg.Command)
	return nil
}

func init() {
	cmdline.AddConfigType("command-service",
		"Run an interactive command via a Receptor service", CommandSvcCfg{}, false, servicesSection)
}
