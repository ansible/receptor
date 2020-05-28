package services

import (
	"fmt"
	"github.org/ghjm/sockceptor/pkg/debug"
	"github.org/ghjm/sockceptor/pkg/netceptor"
	"net"
)

type udpProxy struct {
	s               *netceptor.Netceptor
	lservice   		string
	node       		string
	rservice   		string
	udpAddr         *net.UDPAddr
}

func (up *udpProxy) runUDPToNetceptor_Inbound() {

	connMap := make(map[string]*netceptor.PacketConn)
	buffer := make([]byte, netceptor.MTU)

	uc, err := net.ListenUDP("udp", up.udpAddr)
	if err != nil { panic(err) }

	ncAddr := netceptor.NewAddr(up.node, up.rservice)

	for {
		n, addr, err := uc.ReadFrom(buffer)
		raddrStr := addr.String()
		pc, ok := connMap[raddrStr]
		if !ok {
			pc, err = up.s.ListenPacket("")
			if err != nil { panic(err) }
			debug.Printf("Received new UDP connection from %s\n", raddrStr)
			connMap[raddrStr] = pc
			go up.runNetceptorToUDP_Inbound(pc, uc, addr)
		}
		debug.Printf("Forwarding UDP packet length %d from %s to %s\n", n, raddrStr, ncAddr)
		wn, err := pc.WriteTo(buffer[:n], ncAddr)
		if err != nil { panic(err) }
		if wn != n { panic("not all bytes written") }
	}
}

func (up *udpProxy) runNetceptorToUDP_Inbound(pc *netceptor.PacketConn, uc *net.UDPConn, udpAddr net.Addr) {
	buf := make([]byte, netceptor.MTU)
	expectedAddr := netceptor.NewAddr(up.node, up.rservice)
	for {
		n, addr, err := pc.ReadFrom(buf)
		if err != nil { panic(err) }
		if addr != expectedAddr { panic("received packet from unexpected address") }
		debug.Printf("Forwarding UDP packet length %d from %s to %s\n", n, pc.LocalAddr(), udpAddr)
		wn, err := uc.WriteTo(buf[:n], udpAddr)
		if err != nil { panic(err) }
		if wn != n { panic("not all bytes written") }
	}
}

func (up *udpProxy) runNetceptorToUDP_Outbound() {
	connMap := make(map[string]*net.UDPConn)
	buffer := make([]byte, netceptor.MTU)
	pc, err := up.s.ListenPacket(up.lservice)
	if err != nil { panic(err) }
	for {
		n, addr, err := pc.ReadFrom(buffer)
		if err != nil { panic(err) }
		raddrStr := addr.String()
		uc, ok := connMap[raddrStr]
		if !ok {
			uc, err = net.DialUDP("udp", nil, up.udpAddr)
			if err != nil { panic(err) }
			debug.Printf("Opened new UDP connection to %s\n", raddrStr)
			connMap[raddrStr] = uc
			go up.runUDPToNetceptor_Outbound(uc, pc, addr)
		}
		debug.Printf("Forwarding UDP packet length %d from %s to %s\n", n, addr, uc.LocalAddr())
		wn, err := uc.Write(buffer[:n])
		if err != nil { panic(err) }
		if wn != n { panic("not all bytes written") }
	}
}

func (up *udpProxy) runUDPToNetceptor_Outbound(uc *net.UDPConn, pc *netceptor.PacketConn, addr net.Addr) {
	buf := make([]byte, netceptor.MTU)
	for {
		n, err := uc.Read(buf)
		if err != nil { panic(err) }
		debug.Printf("Forwarding UDP packet length %d from %s to %s\n", n, uc.LocalAddr(), addr)
		wn, err := pc.WriteTo(buf[:n], addr)
		if err != nil { panic(err) }
		if wn != n { panic("not all bytes written") }
	}
}

func UdpProxyService(s *netceptor.Netceptor, direction string, lservice string, host string,
	port string, node string, rservice string) {

	up := &udpProxy{
		s:			s,
		lservice:   lservice,
		node:       node,
		rservice:   rservice,
	}

	ua, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%s", host, port))
	if err != nil { panic(err) }
	up.udpAddr = ua

	if direction == "in" {
		go up.runUDPToNetceptor_Inbound()
	} else {
		go up.runNetceptorToUDP_Outbound()
	}
}

