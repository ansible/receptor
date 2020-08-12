package services

import (
	"crypto/tls"
	"fmt"
	"github.com/project-receptor/receptor/pkg/cmdline"
	"github.com/project-receptor/receptor/pkg/logger"
	"github.com/project-receptor/receptor/pkg/netceptor"
	"github.com/project-receptor/receptor/pkg/sockutils"
	"net"
	"os"
	"runtime"
	"syscall"
)

// errLocked is returned when the flock is already held
var errLocked = fmt.Errorf("fslock is already locked")

// tryFLock non-blockingly attempts to acquire a lock on the file
func tryFLock(filename string) (int, error) {
	fd, err := syscall.Open(filename, syscall.O_CREAT|syscall.O_RDONLY|syscall.O_CLOEXEC, 0600)
	if err != nil {
		return 0, err
	}
	err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
	if err == syscall.EWOULDBLOCK {
		err = errLocked
	}
	if err != nil {
		syscall.Close(fd)
		return 0, err
	}
	return fd, nil
}

// UnixProxyServiceInbound listens on a Unix socket and forwards connections over the Receptor network
func UnixProxyServiceInbound(s *netceptor.Netceptor, filename string, permissions os.FileMode,
	node string, rservice string, tlscfg *tls.Config) error {
	lockFd, err := tryFLock(filename + ".lock")
	if err != nil {
		return fmt.Errorf("could not acquire lock on socket file: %s", err)
	}
	err = os.RemoveAll(filename)
	if err != nil {
		return fmt.Errorf("could not overwrite socket file: %s", err)
	}
	uli, err := net.Listen("unix", filename)
	if err != nil {
		return fmt.Errorf("could not listen on socket file: %s", err)
	}
	err = os.Chmod(filename, permissions)
	if err != nil {
		return fmt.Errorf("error setting socket file permissions: %s", err)
	}
	go func() {
		defer syscall.Close(lockFd)
		for {
			uc, err := uli.Accept()
			if err != nil {
				logger.Error("Error accepting Unix socket connection: %s", err)
				return
			}
			go func() {
				qc, err := s.Dial(node, rservice, tlscfg)
				if err != nil {
					logger.Error("Error connecting on Receptor network: %s", err)
					return
				}
				sockutils.BridgeConns(uc, "unix socket service", qc, "receptor connection")
			}()
		}
	}()
	return nil
}

// UnixProxyServiceOutbound listens on the Receptor network and forwards the connection via a Unix socket
func UnixProxyServiceOutbound(s *netceptor.Netceptor, service string, tlscfg *tls.Config, filename string) error {
	qli, err := s.ListenAndAdvertise(service, tlscfg, map[string]string{
		"type":     "Unix Proxy",
		"filename": filename,
	})
	if err != nil {
		return fmt.Errorf("error listening on Receptor network: %s", err)
	}
	go func() {
		for {
			qc, err := qli.Accept()
			if err != nil {
				logger.Error("Error accepting connection on Receptor network: %s\n", err)
				return

			}
			uc, err := net.Dial("unix", filename)
			if err != nil {
				logger.Error("Error connecting via Unix socket: %s\n", err)
				continue
			}
			go sockutils.BridgeConns(qc, "receptor service", uc, "unix socket connection")
		}
	}()
	return nil
}

// UnixProxyInboundCfg is the cmdline configuration object for a Unix socket inbound proxy
type UnixProxyInboundCfg struct {
	Filename      string `required:"true" description:"Socket filename, which will be overwritten"`
	Permissions   int    `description:"Socket file permissions" default:"0600"`
	RemoteNode    string `required:"true" description:"Receptor node to connect to"`
	RemoteService string `required:"true" description:"Receptor service name to connect to"`
	TLS           string `description:"Name of TLS client config for the Receptor connection"`
}

// Run runs the action
func (cfg UnixProxyInboundCfg) Run() error {
	logger.Debug("Running Unix socket inbound proxy service %v\n", cfg)
	tlscfg, err := netceptor.GetClientTLSConfig(cfg.TLS, cfg.RemoteNode)
	if err != nil {
		return err
	}
	return UnixProxyServiceInbound(netceptor.MainInstance, cfg.Filename, os.FileMode(cfg.Permissions),
		cfg.RemoteNode, cfg.RemoteService, tlscfg)
}

// UnixProxyOutboundCfg is the cmdline configuration object for a Unix socket outbound proxy
type UnixProxyOutboundCfg struct {
	Service  string `required:"true" description:"Receptor service name to bind to"`
	Filename string `required:"true" description:"Socket filename, which must already exist"`
	TLS      string `description:"Name of TLS server config for the Receptor connection"`
}

// Run runs the action
func (cfg UnixProxyOutboundCfg) Run() error {
	logger.Debug("Running Unix socket inbound proxy service %s\n", cfg)
	tlscfg, err := netceptor.GetServerTLSConfig(cfg.TLS)
	if err != nil {
		return err
	}
	return UnixProxyServiceOutbound(netceptor.MainInstance, cfg.Service, tlscfg, cfg.Filename)
}

func init() {
	if runtime.GOOS != "windows" {
		cmdline.AddConfigType("unix-socket-server",
			"Listen on a Unix socket and forward via Receptor", UnixProxyInboundCfg{}, false, false, false, servicesSection)
		cmdline.AddConfigType("unix-socket-client",
			"Listen via Receptor and forward to a Unix socket", UnixProxyOutboundCfg{}, false, false, false, servicesSection)
	}
}
