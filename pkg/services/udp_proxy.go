package services

import (
	"fmt"
	"github.com/project-receptor/receptor/pkg/cmdline"
	"github.com/project-receptor/receptor/pkg/logger"
	"github.com/project-receptor/receptor/pkg/netceptor"
	"net"
)

// UDPProxyServiceInbound listens on a UDP port and forwards packets to a remote Receptor service
func UDPProxyServiceInbound(s *netceptor.Netceptor, host string, port int, node string, service string) {
	connMap := make(map[string]*netceptor.PacketConn)
	buffer := make([]byte, netceptor.MTU)

	addrStr := fmt.Sprintf("%s:%d", host, port)
	udpAddr, err := net.ResolveUDPAddr("udp", addrStr)
	if err != nil {
		logger.Debug("Could not resolve address %s\n", addrStr)
		return
	}

	uc, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		logger.Error("Error listening on UDP: %s\n", err)
		return
	}

	ncAddr := s.NewAddr(node, service)

	for {
		n, addr, err := uc.ReadFrom(buffer)
		raddrStr := addr.String()
		pc, ok := connMap[raddrStr]
		if !ok {
			pc, err = s.ListenPacket("")
			if err != nil {
				logger.Error("Error listening on Receptor network: %s\n", err)
				return
			}
			logger.Debug("Received new UDP connection from %s\n", raddrStr)
			connMap[raddrStr] = pc
			go runNetceptorToUDPInbound(pc, uc, addr, s.NewAddr(node, service))
		}
		wn, err := pc.WriteTo(buffer[:n], ncAddr)
		if err != nil {
			logger.Error("Error sending packet on Receptor network: %s\n", err)
			continue
		}
		if wn != n {
			logger.Debug("Not all bytes written on Receptor network\n")
			continue
		}
	}
}

func runNetceptorToUDPInbound(pc *netceptor.PacketConn, uc *net.UDPConn, udpAddr net.Addr, expectedAddr netceptor.Addr) {
	buf := make([]byte, netceptor.MTU)
	for {
		n, addr, err := pc.ReadFrom(buf)
		if err != nil {
			logger.Error("Error reading from Receptor network: %s\n", err)
			continue
		}
		if addr != expectedAddr {
			logger.Debug("Received packet from unexpected source %s\n", addr)
			continue
		}
		wn, err := uc.WriteTo(buf[:n], udpAddr)
		if err != nil {
			logger.Error("Error sending packet via UDP: %s\n", err)
			continue
		}
		if wn != n {
			logger.Debug("Not all bytes written via UDP\n")
			continue
		}
	}
}

// UDPProxyServiceOutbound listens on the Receptor network and forwards packets via UDP
func UDPProxyServiceOutbound(s *netceptor.Netceptor, service string, address string) {
	connMap := make(map[string]*net.UDPConn)
	buffer := make([]byte, netceptor.MTU)

	udpAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		logger.Debug("Could not resolve UDP address %s\n", address)
		return
	}

	pc, err := s.ListenPacketAndAdvertise(service, map[string]string{
		"type":    "UDP Proxy",
		"address": address,
	})
	if err != nil {
		logger.Error("Error listening on UDP: %s\n", err)
		return
	}
	for {
		n, addr, err := pc.ReadFrom(buffer)
		if err != nil {
			logger.Error("Error reading from Receptor network: %s\n", err)
			return
		}
		raddrStr := addr.String()
		uc, ok := connMap[raddrStr]
		if !ok {
			uc, err = net.DialUDP("udp", nil, udpAddr)
			if err != nil {
				logger.Error("Error connecting via UDP: %s\n", err)
				return
			}
			logger.Debug("Opened new UDP connection to %s\n", raddrStr)
			connMap[raddrStr] = uc
			go runUDPToNetceptorOutbound(uc, pc, addr)
		}
		wn, err := uc.Write(buffer[:n])
		if err != nil {
			logger.Error("Error writing to UDP: %s\n", err)
			continue
		}
		if wn != n {
			logger.Debug("Not all bytes written to UDP\n")
			continue
		}
	}
}

func runUDPToNetceptorOutbound(uc *net.UDPConn, pc *netceptor.PacketConn, addr net.Addr) {
	buf := make([]byte, netceptor.MTU)
	for {
		n, err := uc.Read(buf)
		if err != nil {
			logger.Error("Error reading from UDP: %s\n", err)
			return
		}
		wn, err := pc.WriteTo(buf[:n], addr)
		if err != nil {
			logger.Error("Error writing to the Receptor network: %s\n", err)
			continue
		}
		if wn != n {
			logger.Debug("Not all bytes written to the Netceptor network\n")
			continue
		}
	}
}

// UDPProxyInboundCfg is the cmdline configuration object for a UDP inbound proxy
type UDPProxyInboundCfg struct {
	Port          int    `required:"true" description:"Local UDP port to bind to"`
	BindAddr      string `description:"Address to bind UDP listener to" default:"0.0.0.0"`
	RemoteNode    string `required:"true" description:"Receptor node to connect to"`
	RemoteService string `required:"true" description:"Receptor service name to connect to"`
}

// Run runs the action
func (cfg UDPProxyInboundCfg) Run() error {
	logger.Debug("Running UDP inbound proxy service %v\n", cfg)
	go UDPProxyServiceInbound(netceptor.MainInstance, cfg.BindAddr, cfg.Port, cfg.RemoteNode, cfg.RemoteService)
	return nil
}

// UDPProxyOutboundCfg is the cmdline configuration object for a UDP outbound proxy
type UDPProxyOutboundCfg struct {
	Service string `required:"true" description:"Receptor service name to bind to"`
	Address string `required:"true" description:"Address for outbound UDP connection"`
}

// Run runs the action
func (cfg UDPProxyOutboundCfg) Run() error {
	logger.Debug("Running UDP outbound proxy service %s\n", cfg)
	go UDPProxyServiceOutbound(netceptor.MainInstance, cfg.Service, cfg.Address)
	return nil
}

func init() {
	cmdline.AddConfigType("udp-server",
		"Listen for UDP and forward via Receptor", UDPProxyInboundCfg{}, false, false, false, servicesSection)
	cmdline.AddConfigType("udp-client",
		"Listen on a Receptor service and forward via UDP", UDPProxyOutboundCfg{}, false, false, false, servicesSection)
}
