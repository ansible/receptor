package main

import (
	"flag"
	"fmt"
	"github.org/ghjm/sockceptor/pkg/debug"
	"github.org/ghjm/sockceptor/pkg/netceptor"
	"net"
	"os"
	"strings"
)

type stringList []string

func (i *stringList) String() string {
	return strings.Join(*i, ", ")
}

func (i *stringList) Set(value string) error {
	*i = append(*i, value)
	return nil
}

var nodeId string
var peers stringList
var listeners stringList
var udpServices stringList

func runUdpService(s *netceptor.Netceptor, direction string, lservice string, host string,
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

func main() {
	flag.StringVar(&nodeId, "node-id", "", "local node ID")
	flag.BoolVar(&debug.Enable, "debug", false, "show debug output")
	flag.Var(&peers, "peer", "host:port  to connect outbound to")
	flag.Var(&listeners, "listen", "host:port to listen on for peer connections")
	flag.Var(&udpServices, "udp", "{in|out}:lservice:host:port:node:rservice")
	flag.Parse()
	if nodeId == "" {
		println("Must specify a node ID")
		os.Exit(1)
	}

	s := netceptor.New(nodeId)
	for _, listener := range listeners {
		debug.Printf("Running listener %s\n", listener)
		li, err := netceptor.NewUdpListener(listener); if err != nil {
			fmt.Printf("Error listening on %s: %s\n", listener, err)
			return
		} else {
			s.RunBackend(li, func(err error) {
				fmt.Printf("Error in listener backend: %s\n", err)
			})
		}
	}
	for _, peer := range peers {
		debug.Printf("Running peer connection %s\n", peer)
		li, err := netceptor.NewUdpDialer(peer); if err != nil {
			fmt.Printf("Error creating peer %s: %s\n", peer, err)
			return
		} else {
			s.RunBackend(li, func(err error) {
				fmt.Printf("Error in peer connection backend: %s\n", err)
			})
		}
	}
	for _, udpService := range udpServices {
		debug.Printf("Running UDP service %s\n", udpService)
		params := strings.Split(udpService, ":")
		if len(params) != 6 { panic("Invalid parameters for udp service") }
		go runUdpService(s, params[0], params[1], params[2], params[3], params[4], params[5])
	}
	debug.Printf("Initialization complete\n")
	// Main goroutine sleeps forever
	select{}
}
