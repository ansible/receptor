//+build linux

package services

import (
	"fmt"
	"github.com/project-receptor/receptor/pkg/cmdline"
	"github.com/project-receptor/receptor/pkg/logger"
	"github.com/project-receptor/receptor/pkg/netceptor"
	"github.com/songgao/water"
	"github.com/vishvananda/netlink"
	"net"
)

func runTunToNetceptor(tunif *water.Interface, nconn *netceptor.PacketConn, remoteAddr netceptor.Addr) {
	logger.Debug("Running tunnel-to-Receptor forwarder\n")
	buf := make([]byte, netceptor.MTU)
	for {
		n, err := tunif.Read(buf)
		if err != nil {
			logger.Error("Error reading from tun device: %s\n", err)
			continue
		}
		logger.Trace("    Forwarding data length %d from %s to %s\n", n,
			tunif.Name(), remoteAddr.String())
		wn, err := nconn.WriteTo(buf[:n], remoteAddr)
		if err != nil || wn != n {
			logger.Error("Error writing to Receptor network: %s\n", err)
		}
	}
}

func runNetceptorToTun(nconn *netceptor.PacketConn, tunif *water.Interface, remoteAddr netceptor.Addr) {
	logger.Debug("Running netceptor to tunnel forwarder\n")
	buf := make([]byte, netceptor.MTU)
	for {
		n, addr, err := nconn.ReadFrom(buf)
		if err != nil {
			logger.Error("Error reading from Receptor: %s\n", err)
			continue
		}
		if addr != remoteAddr {
			logger.Debug("Data received from unexpected source: %s\n", addr)
			continue
		}
		logger.Trace("    Forwarding data length %d from %s to %s\n", n,
			addr.String(), tunif.Name())
		wn, err := tunif.Write(buf[:n])
		if err != nil || wn != n {
			logger.Error("Error writing to tun device: %s\n", err)
		}
	}
}

// TunProxyService runs the Receptor to tun interface proxy.
func TunProxyService(s *netceptor.Netceptor, tunInterface string, lservice string,
	node string, rservice string, ifaddress string, destaddress string, route string) error {
	cfg := water.Config{
		DeviceType: water.TUN,
	}
	cfg.Name = tunInterface
	iface, err := water.New(water.Config{DeviceType: water.TUN})
	if err != nil {
		return fmt.Errorf("error opening tun device: %s", err)
	}
	if ifaddress != "" {
		link, err := netlink.LinkByName(iface.Name())
		if err != nil {
			return fmt.Errorf("error accessing link for tun device: %s", err)
		}
		localaddr := net.ParseIP(ifaddress)
		if localaddr == nil {
			return fmt.Errorf("invalid IP address: %s", ifaddress)
		}
		destaddr := net.ParseIP(destaddress)
		if destaddr == nil {
			return fmt.Errorf("invalid IP address: %s", ifaddress)
		}
		addr := &netlink.Addr{
			IPNet: netlink.NewIPNet(localaddr),
			Peer:  netlink.NewIPNet(destaddr),
		}
		err = netlink.AddrAdd(link, addr)
		if err != nil {
			return fmt.Errorf("error adding IP address to link: %s", err)
		}
		err = netlink.LinkSetUp(link)
		if err != nil {
			return fmt.Errorf("error setting link up: %s", err)
		}
		if route != "" {
			ipnet, err := netlink.ParseIPNet(route)
			if err != nil {
				return fmt.Errorf("error parsing route address: %s", err)
			}
			err = netlink.RouteAdd(&netlink.Route{
				LinkIndex: link.Attrs().Index,
				Scope:     netlink.SCOPE_UNIVERSE,
				Dst:       ipnet,
				Gw:        destaddr,
			})
			if err != nil {
				return fmt.Errorf("error adding route to interface: %s", err)
			}
		}
	}

	nconn, err := s.ListenPacketAndAdvertise(lservice, map[string]string{
		"type":          "Tunnel Proxy",
		"interface":     tunInterface,
		"remotenode":    node,
		"remoteservice": rservice,
		"ifaddress":     ifaddress,
		"route":         route,
	})

	if err != nil {
		return fmt.Errorf("error listening for service %s: %s", lservice, err)
	}
	raddr := s.NewAddr(node, rservice)
	go runTunToNetceptor(iface, nconn, raddr)
	go runNetceptorToTun(nconn, iface, raddr)
	return nil
}

// TunProxyCfg is the cmdline configuration object for a tun proxy
type TunProxyCfg struct {
	Service       string `required:"true" description:"Local Receptor service name to bind to"`
	RemoteNode    string `required:"true" description:"Receptor node to connect to"`
	RemoteService string `required:"true" description:"Receptor service name to connect to"`
	Interface     string `description:"Name of the tun interface"`
	IfAddress     string `description:"IP address to assign to the created interface"`
	DestAddress   string `description:"IP address of the point-to-point destination"`
	Route         string `description:"CIDR subnet to route over the created interface"`
}

// Prepare verifies we are ready to run
func (cfg TunProxyCfg) Prepare() error {
	if (cfg.IfAddress == "") != (cfg.DestAddress == "") {
		return fmt.Errorf("ifaddress and destaddress must both be specified, or neither")
	}
	if cfg.Route != "" && cfg.IfAddress == "" {
		return fmt.Errorf("when supplying a route, an IP address must also be given")
	}
	return nil
}

// Run runs the action
func (cfg TunProxyCfg) Run() error {
	logger.Debug("Running tun proxy service %s\n", cfg)
	return TunProxyService(netceptor.MainInstance, cfg.Interface, cfg.Service, cfg.RemoteNode, cfg.RemoteService,
		cfg.IfAddress, cfg.DestAddress, cfg.Route)
}

func init() {
	cmdline.AddConfigType("ip-tunnel", "Run an IP tunnel using a tun interface", TunProxyCfg{}, false, false, false, servicesSection)
}
