package services

import (
	"crypto/tls"
	"github.com/ghjm/sockceptor/pkg/cmdline"
	"github.com/ghjm/sockceptor/pkg/debug"
	"github.com/ghjm/sockceptor/pkg/netceptor"
	"github.com/ghjm/sockceptor/pkg/sockutils"
	"net"
	"strconv"
)

// TCPProxyServiceInbound listens on a TCP port and forwards the connection over the Receptor network
func TCPProxyServiceInbound(s *netceptor.Netceptor, host string, port int, tlsServer *tls.Config,
	node string, rservice string, tlsClient *tls.Config) {
	tli, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if tlsServer != nil {
		tli = tls.NewListener(tli, tlsServer)
	}
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
		qc, err := s.Dial(node, rservice, tlsClient)
		if err != nil {
			debug.Printf("Error connecting on Receptor network: %s\n", err)
			continue
		}
		go sockutils.BridgeConns(tc, qc)
	}
}

// TCPProxyServiceOutbound listens on the Receptor network and forwards the connection via TCP
func TCPProxyServiceOutbound(s *netceptor.Netceptor, service string, tlsServer *tls.Config,
	address string, tlsClient *tls.Config) {
	qli, err := s.ListenAndAdvertise(service, tlsServer, map[string]string{
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
		var tc net.Conn
		if tlsClient == nil {
			tc, err = net.Dial("tcp", address)
		} else {
			tc, err = tls.Dial("tcp", address, tlsClient)
		}
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
	TLSServer     string `description:"Name of TLS server config for the TCP listener"`
	TLSClient     string `description:"Name of TLS client config for the Receptor connection"`
}

// Run runs the action
func (cfg TCPProxyInboundCfg) Run() error {
	debug.Printf("Running TCP inbound proxy service %s\n", cfg)
	tlsClientCfg, err := netceptor.GetClientTLSConfig(cfg.TLSClient, cfg.RemoteNode)
	if err != nil {
		return err
	}
	tlsServerCfg, err := netceptor.GetServerTLSConfig(cfg.TLSServer)
	if err != nil {
		return err
	}
	go TCPProxyServiceInbound(netceptor.MainInstance, cfg.BindAddr, cfg.Port, tlsServerCfg,
		cfg.RemoteNode, cfg.RemoteService, tlsClientCfg)
	return nil
}

// TCPProxyOutboundCfg is the cmdline configuration object for a TCP outbound proxy
type TCPProxyOutboundCfg struct {
	Service   string `required:"true" description:"Receptor service name to bind to"`
	Address   string `required:"true" description:"Address for outbound TCP connection"`
	TLSServer string `description:"Name of TLS server config for the Receptor service"`
	TLSClient string `description:"Name of TLS client config for the TCP connection"`
}

// Run runs the action
func (cfg TCPProxyOutboundCfg) Run() error {
	debug.Printf("Running TCP inbound proxy service %s\n", cfg)
	tlsServerCfg, err := netceptor.GetServerTLSConfig(cfg.TLSServer)
	if err != nil {
		return err
	}
	host, _, err := net.SplitHostPort(cfg.Address)
	if err != nil {
		return err
	}
	tlsClientCfg, err := netceptor.GetClientTLSConfig(cfg.TLSClient, host)
	if err != nil {
		return err
	}
	go TCPProxyServiceOutbound(netceptor.MainInstance, cfg.Service, tlsServerCfg, cfg.Address, tlsClientCfg)
	return nil
}

func init() {
	cmdline.AddConfigType("tcp-server",
		"Listen for TCP and forward via Receptor", TCPProxyInboundCfg{}, false, servicesSection)
	cmdline.AddConfigType("tcp-client",
		"Listen on a Receptor service and forward via TCP", TCPProxyOutboundCfg{}, false, servicesSection)
}
