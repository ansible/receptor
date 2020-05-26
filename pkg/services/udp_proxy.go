package services

import (
	"fmt"
	"github.org/ghjm/sockceptor/pkg/debug"
	"github.org/ghjm/sockceptor/pkg/netceptor"
	"net"
)

func UdpProxyService(s *netceptor.Netceptor, direction string, lservice string, host string,
	port string, node string, rservice string) {

	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%s", host, port))
	if err != nil { panic(err) }

	connRegistry := make(map[string]*netceptor.Conn)

	runNetceptorToUDP := func(rc *netceptor.Conn, uc *net.UDPConn, addr net.Addr) {
		for {
			md := <- rc.RecvChan
			s := "nil"
			if addr != nil {
				s = addr.String()
			}
			debug.Printf("Forwarding Netceptor message from node %s service %s to UDP addr %s\n",
				rc.RemoteNode(), rc.RemoteService(), s)
			if addr == nil {
				_, err = uc.Write(md.Data)
			} else {
				_, err = uc.WriteTo(md.Data, addr)
			}
			if err != nil { panic(err) }
		}
	}

	runUdpToNetceptor := func(s *netceptor.Netceptor, uc *net.UDPConn) {
		for {
			buffer := make([]byte, 1<<16)
			n, addr, err := uc.ReadFrom(buffer)
			if err != nil {
				panic(err)
			}
			crkey := addr.String()+"-"+uc.LocalAddr().String()
			rc, ok := connRegistry[crkey]
			if !ok {
				rc, _ = s.Dial(node, rservice)
				connRegistry[crkey] = rc
				go runNetceptorToUDP(rc, uc, addr)
			}
			debug.Printf("Forwarding UDP message from %s to Netceptor node %s service %s\n",
				addr.String(), rc.RemoteNode(), rc.RemoteService())
			_ = rc.Send(buffer[:n])
		}
	}

	if direction == "in" {
		udpConn, err := net.ListenUDP("udp", udpAddr)
		if err != nil { panic(err) }
		go runUdpToNetceptor(s, udpConn)
	} else {
		li, err := s.Listen(lservice)
		if err != nil { panic(err) }
		go func(li *netceptor.Listener) {
			for {
				rc := li.Accept()
				udpConn, err := net.DialUDP("udp", nil, udpAddr)
				if err != nil { panic(err) }
				connRegistry[udpConn.RemoteAddr().String()+"-"+udpConn.LocalAddr().String()] = rc
				go runNetceptorToUDP(rc, udpConn, nil)
				go runUdpToNetceptor(s, udpConn)
			}
		}(li)
	}
}

