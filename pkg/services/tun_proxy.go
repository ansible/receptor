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
	node string, rservice string, ifaddress string, destaddress string, route string) {

	cfg := water.Config{
		DeviceType: water.TUN,
	}
	cfg.Name = tunInterface
	iface, err := water.New(water.Config{DeviceType: water.TUN})
	if err != nil {
		logger.Error("Error opening tun device: %s\n", err)
		return
	}

	if ifaddress != "" {
		link, err := netlink.LinkByName(iface.Name())
		if err != nil {
			logger.Error("Error accessing link for tun device: %s\n", err)
			return
		}
		localaddr := net.ParseIP(ifaddress)
		if localaddr == nil {
			logger.Debug("Invalid IP address: %s\n", ifaddress)
			return
		}
		destaddr := net.ParseIP(destaddress)
		if destaddr == nil {
			logger.Debug("Invalid IP address: %s\n", ifaddress)
			return
		}
		addr := &netlink.Addr{
			IPNet: netlink.NewIPNet(localaddr),
			Peer:  netlink.NewIPNet(destaddr),
		}
		err = netlink.AddrAdd(link, addr)
		if err != nil {
			logger.Error("Error adding IP address to link: %s\n", err)
			return
		}
		err = netlink.LinkSetUp(link)
		if err != nil {
			logger.Error("Error setting link up: %s\n", err)
			return
		}
		if route != "" {
			ipnet, err := netlink.ParseIPNet(route)
			if err != nil {
				logger.Error("Error parsing route address: %s\n", err)
				return
			}
			err = netlink.RouteAdd(&netlink.Route{
				LinkIndex: link.Attrs().Index,
				Scope:     netlink.SCOPE_UNIVERSE,
				Dst:       ipnet,
				Gw:        destaddr,
			})
			if err != nil {
				logger.Error("Error adding route to interface: %s\n", err)
				return
			}
		}
	}

	logger.Debug("Connecting to remote netceptor node %s service %s\n", node, rservice)
	nconn, err := s.ListenPacketAndAdvertise(lservice, map[string]string{
		"type":          "Tunnel Proxy",
		"interface":     tunInterface,
		"remotenode":    node,
		"remoteservice": rservice,
		"ifaddress":     ifaddress,
		"route":         route,
	})

	if err != nil {
		logger.Error("Error listening on Receptor network\n")
		return
	}
	raddr := s.NewAddr(node, rservice)
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
	go TunProxyService(netceptor.MainInstance, cfg.Interface, cfg.Service, cfg.RemoteNode, cfg.RemoteService,
		cfg.IfAddress, cfg.DestAddress, cfg.Route)
	return nil
}

func init() {
	cmdline.AddConfigType("ip-tunnel", "Run an IP tunnel using a tun interface", TunProxyCfg{}, false, false, false, servicesSection)
}
