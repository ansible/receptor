package services

import (
	"github.com/ghjm/sockceptor/pkg/cmdline"
	"github.com/ghjm/sockceptor/pkg/debug"
	"github.com/ghjm/sockceptor/pkg/netceptor"
	"github.com/ghjm/sockceptor/pkg/sockutils"
	"net"
	"strconv"
)

// TCPProxyServiceInbound listens on a TCP port and forwards the connection over the Receptor network
func TCPProxyServiceInbound(s *netceptor.Netceptor, host string, port int, node string, rservice string) {
	tli, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		debug.Printf("Error listening on TCP: %s\n", err)
		return
	}
	for {
		tc, err := tli.Accept()
		if err != nil {
			debug.Printf("Error accepting TCP connection: %s\n", err)
			return
		}
		qc, err := s.Dial(node, rservice)
		if err != nil {
			debug.Printf("Error connecting on Receptor network: %s\n", err)
			continue
		}
		go sockutils.BridgeConns(tc, qc)
	}
}

// TCPProxyServiceOutbound listens on the Receptor network and forwards the connection via TCP
func TCPProxyServiceOutbound(s *netceptor.Netceptor, service string, address string) {
	qli, err := s.ListenAndAdvertise(service, map[string]string{
		"type":    "TCP Proxy",
		"address": address,
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
		tc, err := net.Dial("tcp", address)
		if err != nil {
			debug.Printf("Error connecting via TCP: %s\n", err)
			continue
		}
		go sockutils.BridgeConns(qc, tc)
	}
}

// TCPProxyInboundCfg is the cmdline configuration object for a TCP inbound proxy
type TCPProxyInboundCfg struct {
	Port          int    `required:"true" description:"Local TCP port to bind to"`
	BindAddr      string `description:"Address to bind TCP listener to" default:"0.0.0.0"`
	RemoteNode    string `required:"true" description:"Receptor node to connect to"`
	RemoteService string `required:"true" description:"Receptor service name to connect to"`
}

// Run runs the action
func (cfg TCPProxyInboundCfg) Run() error {
	debug.Printf("Running TCP inbound proxy service %s\n", cfg)
	go TCPProxyServiceInbound(netceptor.MainInstance, cfg.BindAddr, cfg.Port, cfg.RemoteNode, cfg.RemoteService)
	return nil
}

// TCPProxyOutboundCfg is the cmdline configuration object for a TCP outbound proxy
type TCPProxyOutboundCfg struct {
	Service string `required:"true" description:"Receptor service name to bind to"`
	Address string `required:"true" description:"Address for outbound TCP connection"`
}

// Run runs the action
func (cfg TCPProxyOutboundCfg) Run() error {
	debug.Printf("Running TCP inbound proxy service %s\n", cfg)
	go TCPProxyServiceOutbound(netceptor.MainInstance, cfg.Service, cfg.Address)
	return nil
}

func init() {
	cmdline.AddConfigType("tcp-server",
		"Listen for TCP and forward via Receptor", TCPProxyInboundCfg{}, false, servicesSection)
	cmdline.AddConfigType("tcp-client",
		"Listen on a Receptor service and forward via TCP", TCPProxyOutboundCfg{}, false, servicesSection)
}
