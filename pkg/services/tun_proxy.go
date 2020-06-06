//+build linux

package services

import (
	"fmt"
	"github.com/ghjm/sockceptor/pkg/cmdline"
	"github.com/ghjm/sockceptor/pkg/debug"
	"github.com/ghjm/sockceptor/pkg/netceptor"
	"github.com/songgao/water"
	"github.com/vishvananda/netlink"
)

func runTunToNetceptor(tunif *water.Interface, nconn *netceptor.PacketConn, remoteAddr netceptor.Addr) {
	debug.Printf("Running tunnel-to-Receptor forwarder\n")
	buf := make([]byte, netceptor.MTU)
	for {
		n, err := tunif.Read(buf)
		if err != nil {
			debug.Printf("Error reading from tun device: %s\n", err)
			continue
		}
		debug.Tracef("    Forwarding data length %d from %s to %s\n", n,
			tunif.Name(), remoteAddr.String())
		wn, err := nconn.WriteTo(buf[:n], remoteAddr)
		if err != nil || wn != n {
			debug.Printf("Error writing to Receptor network: %s\n", err)
		}
	}
}

func runNetceptorToTun(nconn *netceptor.PacketConn, tunif *water.Interface, remoteAddr netceptor.Addr) {
	debug.Printf("Running netceptor to tunnel forwarder\n")
	buf := make([]byte, netceptor.MTU)
	for {
		n, addr, err := nconn.ReadFrom(buf)
		if err != nil {
			debug.Printf("Error reading from Receptor: %s\n", err)
			continue
		}
		if addr != remoteAddr {
			debug.Printf("Data received from unexpected source: %s\n", addr)
			continue
		}
		debug.Tracef("    Forwarding data length %d from %s to %s\n", n,
			addr.String(), tunif.Name())
		wn, err := tunif.Write(buf[:n])
		if err != nil || wn != n {
			debug.Printf("Error writing to tun device: %s\n", err)
		}
	}
}

// TunProxyService runs the Receptor to tun interface proxy.
func TunProxyService(s *netceptor.Netceptor, tunInterface string, lservice string,
	node string, rservice string, ifaddress string, route string) {

	cfg := water.Config{
		DeviceType: water.TUN,
	}
	cfg.Name = tunInterface
	iface, err := water.New(water.Config{DeviceType: water.TUN})
	if err != nil {
		debug.Printf("Error opening tun device: %s\n", err)
		return
	}

	if ifaddress != "" {
		link, err := netlink.LinkByName(iface.Name())
		if err != nil {
			debug.Printf("Error accessing link for tun device: %s\n", err)
			return
		}
		ip, err := netlink.ParseAddr(ifaddress)
		if err != nil {
			debug.Printf("Error parsing IP address: %s\n", err)
			return
		}
		err = netlink.AddrAdd(link, ip)
		if err != nil {
			debug.Printf("Error adding IP address to link: %s\n", err)
			return
		}
		err = netlink.LinkSetUp(link)
		if err != nil {
			debug.Printf("Error setting link up: %s\n", err)
			return
		}
		if route != "" {
			ipnet, err := netlink.ParseIPNet(route)
			if err != nil {
				debug.Printf("Error parsing route address: %s\n", err)
				return
			}
			err = netlink.RouteAdd(&netlink.Route{
				LinkIndex: link.Attrs().Index,
				Scope:     netlink.SCOPE_UNIVERSE,
				Dst:       ipnet,
			})
			if err != nil {
				debug.Printf("Error adding route to interface: %s\n", err)
				return
			}
		}
	}

	debug.Printf("Connecting to remote netceptor node %s service %s\n", node, rservice)
	nconn, err := s.ListenPacketAndAdvertise(lservice, map[string]string{
		"type":          "Tunnel Proxy",
		"interface":     tunInterface,
		"remotenode":    node,
		"remoteservice": rservice,
		"ifaddress":     ifaddress,
		"route":         route,
	})

	if err != nil {
		debug.Printf("Error listening on Receptor network\n")
		return
	}
	raddr := netceptor.NewAddr(node, rservice)
	go runTunToNetceptor(iface, nconn, raddr)
	go runNetceptorToTun(nconn, iface, raddr)
}

// TunProxyCfg is the cmdline configuration object for a tun proxy
type TunProxyCfg struct {
	Service       string `required:"true" description:"Local Receptor service name to bind to"`
	RemoteNode    string `required:"true" description:"Receptor node to connect to"`
	RemoteService string `required:"true" description:"Receptor service name to connect to"`
	Interface     string `description:"Name of the tun interface"`
	IfAddress     string `description:"IP address to assign to the created interface"`
	Route         string `description:"CIDR subnet to route over the created interface"`
}

// Prepare verifies we are ready to run
func (cfg TunProxyCfg) Prepare() error {
	if cfg.Route != "" && cfg.IfAddress == "" {
		return fmt.Errorf("when supplying a route, an IP address must also be given")
	}
	return nil
}

// Run runs the action
func (cfg TunProxyCfg) Run() error {
	debug.Printf("Running tun proxy service %s\n", cfg)
	go TunProxyService(netceptor.MainInstance, cfg.Interface, cfg.Service, cfg.RemoteNode, cfg.RemoteService,
		cfg.IfAddress, cfg.Route)
	return nil
}

func init() {
	cmdline.AddConfigType("tun-proxy", "Run a proxy service using a tun interface", TunProxyCfg{}, false)
}
