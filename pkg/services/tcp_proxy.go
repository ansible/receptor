package services

import (
	"crypto/tls"
	"fmt"
	"net"
	"strconv"

	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/utils"
	"github.com/ghjm/cmdline"
	"github.com/spf13/viper"
)

// TCPProxyServiceInbound listens on a TCP port and forwards the connection over the Receptor network.
func TCPProxyServiceInbound(s *netceptor.Netceptor, host string, port int, tlsServer *tls.Config,
	node string, rservice string, tlsClient *tls.Config,
) error {
	tli, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if tlsServer != nil {
		tli = tls.NewListener(tli, tlsServer)
	}
	if err != nil {
		return fmt.Errorf("error listening on TCP: %s", err)
	}
	go func() {
		for {
			tc, err := tli.Accept()
			if err != nil {
				s.Logger.Error("Error accepting TCP connection: %s\n", err)

				return
			}
			qc, err := s.Dial(node, rservice, tlsClient)
			if err != nil {
				s.Logger.Error("Error connecting on Receptor network: %s\n", err)

				continue
			}
			go utils.BridgeConns(tc, "tcp service", qc, "receptor connection", s.Logger)
		}
	}()

	return nil
}

// TCPProxyServiceOutbound listens on the Receptor network and forwards the connection via TCP.
func TCPProxyServiceOutbound(s *netceptor.Netceptor, service string, tlsServer *tls.Config,
	address string, tlsClient *tls.Config,
) error {
	qli, err := s.ListenAndAdvertise(service, tlsServer, map[string]string{
		"type":    "TCP Proxy",
		"address": address,
	})
	if err != nil {
		return fmt.Errorf("error listening on Receptor network: %s", err)
	}
	go func() {
		for {
			qc, err := qli.Accept()
			if err != nil {
				s.Logger.Error("Error accepting connection on Receptor network: %s\n", err)

				return
			}
			var tc net.Conn
			if tlsClient == nil {
				tc, err = net.Dial("tcp", address)
			} else {
				tc, err = tls.Dial("tcp", address, tlsClient)
			}
			if err != nil {
				s.Logger.Error("Error connecting via TCP: %s\n", err)

				continue
			}
			go utils.BridgeConns(qc, "receptor service", tc, "tcp connection", s.Logger)
		}
	}()

	return nil
}

// tcpProxyInboundCfg is the cmdline configuration object for a TCP inbound proxy.
type TCPProxyInboundCfg struct {
	Port          int    `required:"true" description:"Local TCP port to bind to"`
	BindAddr      string `description:"Address to bind TCP listener to" default:"0.0.0.0"`
	RemoteNode    string `required:"true" description:"Receptor node to connect to"`
	RemoteService string `required:"true" description:"Receptor service name to connect to"`
	TLSServer     string `description:"Name of TLS server config for the TCP listener"`
	TLSClient     string `description:"Name of TLS client config for the Receptor connection"`
}

// Run runs the action.
func (cfg TCPProxyInboundCfg) Run() error {
	netceptor.MainInstance.Logger.Debug("Running TCP inbound proxy service %v\n", cfg)
	tlsClientCfg, err := netceptor.MainInstance.GetClientTLSConfig(cfg.TLSClient, cfg.RemoteNode, netceptor.ExpectedHostnameTypeReceptor)
	if err != nil {
		return err
	}
	TLSServerConfig, err := netceptor.MainInstance.GetServerTLSConfig(cfg.TLSServer)
	if err != nil {
		return err
	}

	return TCPProxyServiceInbound(netceptor.MainInstance, cfg.BindAddr, cfg.Port, TLSServerConfig,
		cfg.RemoteNode, cfg.RemoteService, tlsClientCfg)
}

// tcpProxyOutboundCfg is the cmdline configuration object for a TCP outbound proxy.
type TCPProxyOutboundCfg struct {
	Service   string `required:"true" description:"Receptor service name to bind to"`
	Address   string `required:"true" description:"Address for outbound TCP connection"`
	TLSServer string `description:"Name of TLS server config for the Receptor service"`
	TLSClient string `description:"Name of TLS client config for the TCP connection"`
}

// Run runs the action.
func (cfg TCPProxyOutboundCfg) Run() error {
	netceptor.MainInstance.Logger.Debug("Running TCP inbound proxy service %s\n", cfg)
	TLSServerConfig, err := netceptor.MainInstance.GetServerTLSConfig(cfg.TLSServer)
	if err != nil {
		return err
	}
	host, _, err := net.SplitHostPort(cfg.Address)
	if err != nil {
		return err
	}
	tlsClientCfg, err := netceptor.MainInstance.GetClientTLSConfig(cfg.TLSClient, host, netceptor.ExpectedHostnameTypeDNS)
	if err != nil {
		return err
	}

	return TCPProxyServiceOutbound(netceptor.MainInstance, cfg.Service, TLSServerConfig, cfg.Address, tlsClientCfg)
}

func init() {
	version := viper.GetInt("version")
	if version > 1 {
		return
	}
	cmdline.RegisterConfigTypeForApp("receptor-proxies",
		"tcp-server", "Listen for TCP and forward via Receptor", TCPProxyInboundCfg{}, cmdline.Section(servicesSection))
	cmdline.RegisterConfigTypeForApp("receptor-proxies",
		"tcp-client", "Listen on a Receptor service and forward via TCP", TCPProxyOutboundCfg{}, cmdline.Section(servicesSection))
}
