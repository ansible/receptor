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
var udpServices stringList
var tunServices stringList

func main() {
	flag.StringVar(&nodeId, "node-id", "", "local node ID")
	flag.BoolVar(&debug.Enable, "debug", false, "show debug output")
	flag.Var(&peers, "peer", "host:port  to connect outbound to")
	flag.Var(&listeners, "listen", "host:port to listen on for peer connections")
	flag.Var(&udpServices, "udp", "{in|out}:lservice:host:port:node:rservice")
	flag.Var(&tunServices, "tun", "{dial|listen}:lservice:tun_interface:node:rservice")
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
		go services.UdpProxyService(s, params[0], params[1], params[2], params[3], params[4], params[5])
	}
	for _, tunService := range tunServices {
		debug.Printf("Running tun service %s\n", tunService)
		params := strings.Split(tunService, ":")
		if len(params) != 5 { panic("Invalid parameters for tun service") }
		go services.TunProxyService(s, params[0], params[1], params[2], params[3], params[4])
	}
	debug.Printf("Initialization complete\n")
	// Main goroutine sleeps forever
	select{}
}
