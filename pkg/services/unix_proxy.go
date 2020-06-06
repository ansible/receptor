//+build linux

package services

import (
	"github.com/ghjm/sockceptor/pkg/cmdline"
	"github.com/ghjm/sockceptor/pkg/debug"
	"github.com/ghjm/sockceptor/pkg/netceptor"
	"net"
	"os"
)

// UnixProxyServiceInbound listens on a Unix socket and forwards connections over the Receptor network
func UnixProxyServiceInbound(s *netceptor.Netceptor, filename string, node string, rservice string) {
	err := os.RemoveAll(filename)
	if err != nil {
		debug.Printf("Could not overwrite socket file: %s\n", err)
		return
	}
	uli, err := net.Listen("unix", filename)
	if err != nil {
		debug.Printf("Could not listen on socket file: %s\n", err)
		return
	}
	for {
		tc, err := uli.Accept()
		if err != nil {
			debug.Printf("Error accepting Unix socket connection: %s\n", err)
			return
		}
		qc, err := s.Dial(node, rservice)
		if err != nil {
			debug.Printf("Error connecting on Receptor network: %s\n", err)
			continue
		}
		bridgeConns(tc, qc)
	}
}

// UnixProxyServiceOutbound listens on the Receptor network and forwards the connection via a Unix socket
func UnixProxyServiceOutbound(s *netceptor.Netceptor, service string, filename string) {
	qli, err := s.ListenAndAdvertise(service, map[string]string{
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
		bridgeConns(qc, uc)
	}
}

// UnixProxyInboundCfg is the cmdline configuration object for a Unix socket inbound proxy
type UnixProxyInboundCfg struct {
	Filename      string `required:"true" description:"Filename of the socket file, which will be overwritten"`
	RemoteNode    string `required:"true" description:"Receptor node to connect to"`
	RemoteService string `required:"true" description:"Receptor service name to connect to"`
}

// Run runs the action
func (cfg UnixProxyInboundCfg) Run() error {
	debug.Printf("Running Unix socket inbound proxy service %s\n", cfg)
	go UnixProxyServiceInbound(netceptor.MainInstance, cfg.Filename, cfg.RemoteNode, cfg.RemoteService)
	return nil
}

// UnixProxyOutboundCfg is the cmdline configuration object for a Unix socket outbound proxy
type UnixProxyOutboundCfg struct {
	Service  string `required:"true" description:"Receptor service name to bind to"`
	Filename string `required:"true" description:"Filename of the socket file, which must already exist"`
}

// Run runs the action
func (cfg UnixProxyOutboundCfg) Run() error {
	debug.Printf("Running Unix socket inbound proxy service %s\n", cfg)
	go UnixProxyServiceOutbound(netceptor.MainInstance, cfg.Service, cfg.Filename)
	return nil
}

func init() {
	cmdline.AddConfigType("unix-inbound-proxy",
		"Listen on a Unix socket and forward via Receptor", UnixProxyInboundCfg{}, false, servicesSection)
	cmdline.AddConfigType("unix-outbound-proxy",
		"Listen on a Receptor service and forward via a Unix socket", UnixProxyOutboundCfg{}, false, servicesSection)
}
