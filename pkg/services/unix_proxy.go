package services

import (
	"crypto/tls"
	"github.com/ghjm/sockceptor/pkg/cmdline"
	"github.com/ghjm/sockceptor/pkg/debug"
	"github.com/ghjm/sockceptor/pkg/netceptor"
	"github.com/ghjm/sockceptor/pkg/sockutils"
	"github.com/juju/fslock"
	"net"
	"os"
	"runtime"
)

// UnixProxyServiceInbound listens on a Unix socket and forwards connections over the Receptor network
func UnixProxyServiceInbound(s *netceptor.Netceptor, filename string, permissions os.FileMode,
	node string, rservice string, tlscfg *tls.Config) {
	lock := fslock.New(filename + ".lock")
	err := lock.TryLock()
	if err != nil {
		debug.Printf("Could not acquire lock on socket file: %s\n", err)
		return
	}
	err = os.RemoveAll(filename)
	if err != nil {
		debug.Printf("Could not overwrite socket file: %s\n", err)
		return
	}
	uli, err := net.Listen("unix", filename)
	if err != nil {
		debug.Printf("Could not listen on socket file: %s\n", err)
		return
	}
	err = os.Chmod(filename, permissions)
	if err != nil {
		debug.Printf("Error setting socket file permissions: %s\n", err)
		return
	}
	for {
		tc, err := uli.Accept()
		if err != nil {
			debug.Printf("Error accepting Unix socket connection: %s\n", err)
			return
		}
		qc, err := s.Dial(node, rservice, tlscfg)
		if err != nil {
			debug.Printf("Error connecting on Receptor network: %s\n", err)
			continue
		}
		go sockutils.BridgeConns(tc, qc)
	}
}

// UnixProxyServiceOutbound listens on the Receptor network and forwards the connection via a Unix socket
func UnixProxyServiceOutbound(s *netceptor.Netceptor, service string, tlscfg *tls.Config, filename string) {
	qli, err := s.ListenAndAdvertise(service, tlscfg, map[string]string{
		"type":     "Unix Proxy",
		"filename": filename,
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
		uc, err := net.Dial("unix", filename)
		if err != nil {
			debug.Printf("Error connecting via Unix socket: %s\n", err)
			continue
		}
		go sockutils.BridgeConns(qc, uc)
	}
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
	debug.Printf("Running Unix socket inbound proxy service %s\n", cfg)
	tlscfg, err := netceptor.GetClientTLSConfig(cfg.TLS, cfg.RemoteNode)
	if err != nil {
		return err
	}
	go UnixProxyServiceInbound(netceptor.MainInstance, cfg.Filename, os.FileMode(cfg.Permissions),
		cfg.RemoteNode, cfg.RemoteService, tlscfg)
	return nil
}

// UnixProxyOutboundCfg is the cmdline configuration object for a Unix socket outbound proxy
type UnixProxyOutboundCfg struct {
	Service  string `required:"true" description:"Receptor service name to bind to"`
	Filename string `required:"true" description:"Socket filename, which must already exist"`
	TLS      string `description:"Name of TLS server config for the Receptor connection"`
}

// Run runs the action
func (cfg UnixProxyOutboundCfg) Run() error {
	debug.Printf("Running Unix socket inbound proxy service %s\n", cfg)
	tlscfg, err := netceptor.GetServerTLSConfig(cfg.TLS)
	if err != nil {
		return err
	}
	go UnixProxyServiceOutbound(netceptor.MainInstance, cfg.Service, tlscfg, cfg.Filename)
	return nil
}

func init() {
	if runtime.GOOS != "windows" {
		cmdline.AddConfigType("unix-socket-server",
			"Listen on a Unix socket and forward via Receptor", UnixProxyInboundCfg{}, false, servicesSection)
		cmdline.AddConfigType("unix-socket-client",
			"Listen via Receptor and forward to a Unix socket", UnixProxyOutboundCfg{}, false, servicesSection)
	}
}
