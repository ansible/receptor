//+build linux

package services

import (
	"github.com/songgao/water"
	"github.com/ghjm/sockceptor/pkg/debug"
	"github.com/ghjm/sockceptor/pkg/netceptor"
)

func runTunToNetceptor(tunif *water.Interface, nconn *netceptor.PacketConn, remoteAddr netceptor.Addr) {
	debug.Printf("Running tunnel to netceptor forwarder\n")
	buf := make([]byte, netceptor.MTU)
	for {
		n, err := tunif.Read(buf); if err != nil {
			debug.Printf("Error reading from tun device: %s\n", err)
			continue
		}
		// debug.Printf("Forwarding packet of length %d from tun to netceptor\n", n)
		wn, err := nconn.WriteTo(buf[:n], remoteAddr); if err != nil || wn != n {
			debug.Printf("Error writing to netceptor: %s\n", err)
		}
	}
}

func runNetceptorToTun(nconn *netceptor.PacketConn, tunif *water.Interface, remoteAddr netceptor.Addr) {
	debug.Printf("Running netceptor to tunnel forwarder\n")
	buf := make([]byte, netceptor.MTU)
	for {
		n, addr, err := nconn.ReadFrom(buf); if err != nil {
			debug.Printf("Error reading from netceptor: %s\n", err)
			continue
		}
		if addr != remoteAddr {
			debug.Printf("Data received from unexpected source: %s\n", addr)
			continue
		}
		// debug.Printf("Forwarding packet of length %d from netceptor to tun\n", n)
		wn, err := tunif.Write(buf[:n]); if err != nil || wn != n {
			debug.Printf("Error writing to tun device: %s\n", err)
		}
	}
}

// TunProxyService runs the Receptor to tun interface proxy.
func TunProxyService(s *netceptor.Netceptor, tunInterface string, lservice string,
	node string, rservice string) {

	cfg := water.Config{
		DeviceType: water.TUN,
	}
	cfg.Name = tunInterface
	iface, err := water.New(water.Config{DeviceType: water.TUN}); if err != nil {
		debug.Printf("Error opening tun device: %s\n", err)
		return
	}

	debug.Printf("Connecting to remote netceptor node %s service %s\n", node, rservice)
	nconn, err := s.ListenPacket(lservice); if err != nil {
		debug.Printf("Error listening on Netceptor network\n")
		return
	}
	raddr := netceptor.NewAddr(node, rservice)
	go runTunToNetceptor(iface, nconn, raddr)
	go runNetceptorToTun(nconn, iface, raddr)
}
