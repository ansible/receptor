package services

import (
	"github.org/ghjm/sockceptor/pkg/debug"
	"github.org/ghjm/sockceptor/pkg/netceptor"
	"net"
)

func bridgeHalf(c1 net.Conn, c2 net.Conn) {
	buf := make([]byte, netceptor.MTU)
	for {
		n, err := c1.Read(buf); if err != nil {
			debug.Printf("Connection read error: %s\n", err)
			return
		}
		wn, err := c2.Write(buf[:n]); if err != nil {
			debug.Printf("Connection write error: %s\n", err)
			return
		}
		if wn != n {
			debug.Printf("Not all bytes written\n", err)
			return
		}
	}
}

func bridgeConns(c1 net.Conn, c2 net.Conn) {
	go bridgeHalf(c1, c2)
	go bridgeHalf(c2, c1)
}

func TCPProxyService_Inbound(s *netceptor.Netceptor, host string, port string, node string, rservice string) {
	tli, err := net.Listen("tcp", net.JoinHostPort(host, port)); if err != nil {
		panic(err)
	}
	for {
		tc, err := tli.Accept(); if err != nil {
			panic(err)
		}
		qc, err := s.Dial(node, rservice); if err != nil {
			panic(err)
		}
		bridgeConns(tc, qc)
	}
}

func TCPProxyService_Outbound(s *netceptor.Netceptor, lservice string, host string,
	port string, node string, rservice string) {
	qli, err := s.Listen(lservice); if err != nil {
		panic(err)
	}
	for {
		qc, err := qli.Accept(); if err != nil {
			panic(err)
		}
		tc, err := net.Dial("tcp", net.JoinHostPort(host, port)); if err != nil {
			panic(err)
		}
		bridgeConns(qc, tc)
	}
}

func TCPProxyService(s *netceptor.Netceptor, direction string, lservice string, host string,
	port string, node string, rservice string) {
	if direction == "in" {
		TCPProxyService_Inbound(s, host, port, node, rservice)
	} else {
		TCPProxyService_Outbound(s, lservice, host, port, node, rservice)
	}
}
