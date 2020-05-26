package services

import (
	"github.com/songgao/water"
	"github.org/ghjm/sockceptor/pkg/debug"
	"github.org/ghjm/sockceptor/pkg/netceptor"
)

func runTunToNetceptor(tunif *water.Interface, nconn *netceptor.Conn) {
	debug.Printf("Running tunnel to netceptor forwarder\n")
	buf := make([]byte, 4096)
	for {
		n, err := tunif.Read(buf); if err != nil {
			debug.Printf("Error reading from tun device: %s\n", err)
			break
		}
		debug.Printf("Forwarding packet of length %d from tun to netceptor\n", n)
		err = nconn.Send(buf[:n]); if err != nil {
			debug.Printf("Error writing to netceptor: %s\n", err)
			break
		}
	}
}

func runNetceptorToTun(nconn *netceptor.Conn, tunif *water.Interface) {
	debug.Printf("Running netceptor to tunnel forwarder\n")
	for {
		buf, err := nconn.Recv(); if err != nil {
			debug.Printf("Error reading from netceptor: %s\n", err)
			break
		}
		debug.Printf("Forwarding packet of length %d from netceptor to tun\n", len(buf))
		n, err := tunif.Write(buf); if err != nil || n != len(buf){
			debug.Printf("Error writing to tun device: %s\n", err)
			break
		}
	}
}

func TunProxyService(s *netceptor.Netceptor, direction string, tunInterface string, lservice string,
	node string, rservice string) {

	cfg := water.Config{
		DeviceType: water.TUN,
	}
	cfg.Name = tunInterface
	iface, err := water.New(water.Config{DeviceType: water.TUN}); if err != nil {
		panic(err)
	}

	if direction == "dial" {
		debug.Printf("Connecting to remote netceptor node %s service %s\n", node, rservice)
		nconn, err := s.Dial(node, rservice); if err != nil { panic(err) }
		go runTunToNetceptor(iface, nconn)
		go runNetceptorToTun(nconn, iface)
	} else {
		debug.Printf("Listening for netceptor service %s\n", lservice)
		li, err := s.Listen(lservice)
		if err != nil { panic(err) }
		go func(li *netceptor.Listener) {
			for {
				nconn := li.Accept()
				debug.Printf("Accepted connection from node %s service %s\n", nconn.RemoteNode(),
					nconn.RemoteService())
				go runTunToNetceptor(iface, nconn)
				go runNetceptorToTun(nconn, iface)
			}
		}(li)
	}
}


