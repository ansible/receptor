package services

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strconv"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/utils"
	"github.com/ghjm/cmdline"
	"github.com/spf13/viper"
)

//go:generate mockgen -package mock_services -source=tcp_proxy.go -destination=mock_services/tcp_proxy.go

type NetcForTCPProxy interface {
	GetLogger() *logger.ReceptorLogger
	Dial(node string, service string, tlscfg *tls.Config) (*netceptor.Conn, error)
	ListenAndAdvertise(service string, tlscfg *tls.Config, tags map[string]string) (*netceptor.Listener, error)
}

// Interface for the net library to generate stubs with mockgen.
type NetLib interface {
	Listen(network string, address string) (net.Listener, error)
	Dial(network string, address string) (net.Conn, error)
}

type NetTCPWrapper struct{}

func (n *NetTCPWrapper) Listen(network string, address string) (net.Listener, error) {
	return net.Listen(network, address)
}

func (n *NetTCPWrapper) Dial(network string, address string) (net.Conn, error) {
	return net.Dial(network, address)
}

// Interface for the tls library to generate stubs with mockgen.
type TLSLib interface {
	NewListener(inner net.Listener, config *tls.Config) net.Listener
	Dial(network string, addr string, config *tls.Config) (*tls.Conn, error)
}

type TLSTCPWrapper struct{}

func (n *TLSTCPWrapper) NewListener(inner net.Listener, config *tls.Config) net.Listener {
	return tls.NewListener(inner, config)
}

func (n *TLSTCPWrapper) Dial(network string, addr string, config *tls.Config) (*tls.Conn, error) {
	return tls.Dial(network, addr, config)
}

// Interface for the Net Listener to generate stubs with mockgen.
type NetListenerTCP interface {
	net.Listener
}

// Interface for the utils package to generate stubs with mockgen.
type UtilsLib interface {
	BridgeConns(c1 io.ReadWriteCloser, c1Name string, c2 io.ReadWriteCloser, c2Name string, logger *logger.ReceptorLogger)
}

type UtilsTCPWrapper struct{}

func (u *UtilsTCPWrapper) BridgeConns(c1 io.ReadWriteCloser, c1Name string, c2 io.ReadWriteCloser, c2Name string, logger *logger.ReceptorLogger) {
	utils.BridgeConns(c1, c1Name, c2, c2Name, logger)
}

// Interface to mock the Connection object returned from Accept.
type TCPConn interface {
	net.Conn
}

// TCPProxyServiceInbound listens on a TCP port and forwards the connection over the Receptor network.
func TCPProxyServiceInbound(s NetcForTCPProxy, host string, port int, tlsServer *tls.Config,
	node string, rservice string, tlsClient *tls.Config, netTCP NetLib, tlsTCP TLSLib, utilsTCP UtilsLib,
) error {
	tli, err := netTCP.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if tlsServer != nil {
		tli = tlsTCP.NewListener(tli, tlsServer)
	}
	if err != nil {
		return fmt.Errorf("error listening on TCP: %s", err)
	}
	go func() {
		for {
			tc, err := tli.Accept()
			if err != nil {
				s.GetLogger().Error("error accepting TCP connection: %s\n", err)

				return
			}
			qc, err := s.Dial(node, rservice, tlsClient)
			if err != nil {
				s.GetLogger().Error("error connecting on Receptor network: %s\n", err)

				continue
			}
			go utilsTCP.BridgeConns(tc, "tcp service", qc, "receptor connection", s.GetLogger())
		}
	}()

	return nil
}

// TCPProxyServiceOutbound listens on the Receptor network and forwards the connection via TCP.
func TCPProxyServiceOutbound(s NetcForTCPProxy, service string, tlsServer *tls.Config,
	address string, tlsClient *tls.Config, netTCP NetLib, tlsTCP TLSLib, utilsTCP UtilsLib,
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
				s.GetLogger().Error("Error accepting connection on Receptor network: %s\n", err)

				return
			}
			var tc net.Conn
			if tlsClient == nil {
				tc, err = netTCP.Dial("tcp", address)
			} else {
				tc, err = tlsTCP.Dial("tcp", address, tlsClient)
			}
			if err != nil {
				s.GetLogger().Error("Error connecting via TCP: %s\n", err)

				continue
			}
			go utilsTCP.BridgeConns(qc, "receptor service", tc, "tcp connection", s.GetLogger())
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
		cfg.RemoteNode, cfg.RemoteService, tlsClientCfg, &NetTCPWrapper{}, &TLSTCPWrapper{}, &UtilsTCPWrapper{})
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

	return TCPProxyServiceOutbound(netceptor.MainInstance, cfg.Service, TLSServerConfig, cfg.Address, tlsClientCfg, &NetTCPWrapper{}, &TLSTCPWrapper{}, &UtilsTCPWrapper{})
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
