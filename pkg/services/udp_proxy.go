package services

import (
	"fmt"
	"github.org/ghjm/sockceptor/pkg/debug"
	"github.org/ghjm/sockceptor/pkg/netceptor"
	"net"
)

type udpProxy struct {
	s        *netceptor.Netceptor
	lservice string
	node     string
	rservice string
	udpAddr  *net.UDPAddr
}

func (up *udpProxy) runUDPToNetceptorInbound() {

	connMap := make(map[string]*netceptor.PacketConn)
	buffer := make([]byte, netceptor.MTU)

	uc, err := net.ListenUDP("udp", up.udpAddr); if err != nil {
		debug.Printf("Error listening on UDP: %s\n", err)
		return
	}

	ncAddr := netceptor.NewAddr(up.node, up.rservice)

	for {
		n, addr, err := uc.ReadFrom(buffer)
		raddrStr := addr.String()
		pc, ok := connMap[raddrStr]
		if !ok {
			pc, err = up.s.ListenPacket(""); if err != nil {
				debug.Printf("Error listening on Netceptor: %s\n", err)
				return
			}
			debug.Printf("Received new UDP connection from %s\n", raddrStr)
			connMap[raddrStr] = pc
			go up.runNetceptorToUDPInbound(pc, uc, addr)
		}
		debug.Tracef("Forwarding UDP packet length %d from %s to %s\n", n, raddrStr, ncAddr)
		wn, err := pc.WriteTo(buffer[:n], ncAddr); if err != nil {
			debug.Printf("Error sending packet on Netceptor network: %s\n", err)
			continue
		}
		if wn != n {
			debug.Printf("Not all bytes written on Netceptor network\n")
			continue
		}
	}
}

func (up *udpProxy) runNetceptorToUDPInbound(pc *netceptor.PacketConn, uc *net.UDPConn, udpAddr net.Addr) {
	buf := make([]byte, netceptor.MTU)
	expectedAddr := netceptor.NewAddr(up.node, up.rservice)
	for {
		n, addr, err := pc.ReadFrom(buf); if err != nil {
			debug.Printf("Error reading from Netceptor network: %s\n", err)
			continue
		}
		if addr != expectedAddr {
			debug.Printf("Received packet from unexpected source %s\n", addr)
			continue
		}
		debug.Tracef("Forwarding UDP packet length %d from %s to %s\n", n, pc.LocalAddr(), udpAddr)
		wn, err := uc.WriteTo(buf[:n], udpAddr); if err != nil {
			debug.Printf("Error sending packet via UDP: %s\n", err)
			continue
		}
		if wn != n {
			debug.Printf("Not all bytes written via UDP\n")
			continue
		}
	}
}

func (up *udpProxy) runNetceptorToUDPOutbound() {
	connMap := make(map[string]*net.UDPConn)
	buffer := make([]byte, netceptor.MTU)
	pc, err := up.s.ListenPacket(up.lservice); if err != nil {
		debug.Printf("Error listening on UDP: %s\n", err)
		return
	}
	for {
		n, addr, err := pc.ReadFrom(buffer); if err != nil {
			debug.Printf("Error reading from Netceptor network: %s\n", err)
			return
		}
		raddrStr := addr.String()
		uc, ok := connMap[raddrStr]
		if !ok {
			uc, err = net.DialUDP("udp", nil, up.udpAddr); if err != nil {
				debug.Printf("Error connecting via UDP: %s\n", err)
				return
			}
			debug.Printf("Opened new UDP connection to %s\n", raddrStr)
			connMap[raddrStr] = uc
			go up.runUDPToNetceptorOutbound(uc, pc, addr)
		}
		debug.Printf("Forwarding UDP packet length %d from %s to %s\n", n, addr, uc.LocalAddr())
		wn, err := uc.Write(buffer[:n]); if err != nil {
			debug.Printf("Error writing to UDP: %s\n", err)
			continue
		}
		if wn != n {
			debug.Printf("Not all bytes written to UDP\n")
			continue
		}
	}
}

func (up *udpProxy) runUDPToNetceptorOutbound(uc *net.UDPConn, pc *netceptor.PacketConn, addr net.Addr) {
	buf := make([]byte, netceptor.MTU)
	for {
		n, err := uc.Read(buf); if err != nil {
			debug.Printf("Error reading from UDP: %s\n", err)
			return
		}
		debug.Printf("Forwarding UDP packet length %d from %s to %s\n", n, uc.LocalAddr(), addr)
		wn, err := pc.WriteTo(buf[:n], addr); if err != nil {
			debug.Printf("Error writing to the Netceptor network: %s\n", err)
			continue
		}
		if wn != n {
			debug.Printf("Not all bytes written to the Netceptor network\n")
			continue
		}
	}
}

// UDPProxyService runs the UDP-to-Receptor proxying service.
func UDPProxyService(s *netceptor.Netceptor, direction string, lservice string, host string,
	port string, node string, rservice string) {

	up := &udpProxy{
		s:			s,
		lservice:   lservice,
		node:       node,
		rservice:   rservice,
	}

	ua, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%s", host, port)); if err != nil {
		debug.Printf("Could not resolve UDP address %s:%s\n", host, port)
		return
	}
	up.udpAddr = ua

	if direction == "in" {
		go up.runUDPToNetceptorInbound()
	} else {
		go up.runNetceptorToUDPOutbound()
	}
}

