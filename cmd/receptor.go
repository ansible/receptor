package main

import (
	"flag"
	"fmt"
	"github.org/ghjm/sockceptor/pkg/debug"
	"github.org/ghjm/sockceptor/pkg/netceptor"
	"github.org/ghjm/sockceptor/pkg/services"
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
var tcpServices stringList
var udpServices stringList
var tunServices stringList

func main() {
	flag.StringVar(&nodeId, "node-id", "", "local node ID")
	flag.BoolVar(&debug.Enable, "debug", false, "show debug output")
	flag.BoolVar(&debug.Trace, "trace", false, "show full packet traces")
	flag.Var(&peers, "peer", "host:port  to connect outbound to")
	flag.Var(&listeners, "listen", "host:port to listen on for peer connections")
	flag.Var(&tcpServices, "tcp", "{in|out}:lservice:host:port:node:rservice")
	flag.Var(&udpServices, "udp", "{in|out}:lservice:host:port:node:rservice")
	flag.Var(&tunServices, "tun", "tun_interface:lservice:node:rservice")
	flag.Parse()
	if nodeId == "" {
		println("Must specify a node ID")
		os.Exit(1)
	}

	s := netceptor.New(nodeId)

	for _, listener := range listeners {
		debug.Printf("Running listener %s\n", listener)
		li, err := netceptor.NewUDPListener(listener); if err != nil {
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
		li, err := netceptor.NewUDPDialer(peer); if err != nil {
			fmt.Printf("Error creating peer %s: %s\n", peer, err)
			return
		} else {
			s.RunBackend(li, func(err error) {
				fmt.Printf("Error in peer connection backend: %s\n", err)
			})
		}
	}

	for _, tcpService := range tcpServices {
		debug.Printf("Running TCP service %s\n", tcpService)
		params := strings.Split(tcpService, ":")
		if len(params) != 6 { panic("Invalid parameters for TCP service") }
		go services.TCPProxyService(s, params[0], params[1], params[2], params[3], params[4], params[5])
	}

	for _, udpService := range udpServices {
		debug.Printf("Running UDP service %s\n", udpService)
		params := strings.Split(udpService, ":")
		if len(params) != 6 { panic("Invalid parameters for UDP service") }
		go services.UDPProxyService(s, params[0], params[1], params[2], params[3], params[4], params[5])
	}

	for _, tunService := range tunServices {
		debug.Printf("Running tun service %s\n", tunService)
		params := strings.Split(tunService, ":")
		if len(params) != 4 { panic("Invalid parameters for tun service") }
		go services.TunProxyService(s, params[0], params[1], params[2], params[3])
	}

	debug.Printf("Initialization complete\n")
	// Main goroutine sleeps forever
	select{}
}
