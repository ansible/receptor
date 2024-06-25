//go:build linux && !no_ip_router && linux && !no_services
// +build linux,!no_ip_router,linux,!no_services

package services

import (
	"bytes"
	"fmt"
	"math"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/utils"
	"github.com/ghjm/cmdline"
	"github.com/songgao/water"
	"github.com/spf13/viper"
	"github.com/vishvananda/netlink"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const adTypeIPRouter = "IP Router"

type ipRoute struct {
	dest *net.IPNet
	via  string
}

// IPRouterService is an IP router service.
type IPRouterService struct {
	nc              *netceptor.Netceptor
	networkName     string
	tunIfName       string
	localNet        *net.IPNet
	advertiseRoutes []*net.IPNet
	linkIP          net.IP
	destIP          net.IP
	tunIf           *water.Interface
	link            netlink.Link
	nConn           netceptor.PacketConner
	knownRoutes     []ipRoute
	knownRoutesLock *sync.RWMutex
}

// NewIPRouter creates a new IP router service.
func NewIPRouter(nc *netceptor.Netceptor, networkName string, tunInterface string,
	localNet string, routes string,
) (*IPRouterService, error) {
	ipr := &IPRouterService{
		nc:              nc,
		networkName:     networkName,
		tunIfName:       tunInterface,
		knownRoutes:     make([]ipRoute, 0),
		knownRoutesLock: &sync.RWMutex{},
	}
	var err error
	_, ipr.localNet, err = net.ParseCIDR(localNet)
	if err != nil {
		return nil, fmt.Errorf("could not parse %s as a CIDR address", localNet)
	}
	ones, bits := ipr.localNet.Mask.Size()
	if ones != 30 {
		return nil, fmt.Errorf("local network %s must be a /30 CIDR", localNet)
	}
	if bits != 32 {
		return nil, fmt.Errorf("local network %s must be IPv4", localNet)
	}
	ipr.advertiseRoutes = make([]*net.IPNet, 0)
	if routes != "" {
		routeList := strings.Split(routes, ",")
		for i := range routeList {
			_, ipNet, err := net.ParseCIDR(routeList[i])
			if err != nil {
				return nil, fmt.Errorf("could not parse %s as a CIDR address", routeList[i])
			}
			ipr.advertiseRoutes = append(ipr.advertiseRoutes, ipNet)
		}
	}
	err = ipr.run()
	if err != nil {
		return nil, err
	}

	return ipr, nil
}

func (ipr *IPRouterService) updateKnownRoutes() {
	newRoutes := make([]ipRoute, 0)
	status := ipr.nc.Status()
	for i := range status.Advertisements {
		ad := status.Advertisements[i]
		adType, ok := ad.Tags["type"]
		if !ok || adType != adTypeIPRouter {
			continue
		}
		network, ok := ad.Tags["network"]
		if !ok || network != ipr.networkName {
			continue
		}
		_, ok = status.RoutingTable[ad.NodeID]
		if !ok {
			continue
		}
		for key, value := range ad.Tags {
			if strings.HasPrefix(key, "route") {
				_, ipNet, err := net.ParseCIDR(value)
				if err == nil {
					newRoute := ipRoute{
						dest: ipNet,
						via:  ad.NodeID,
					}
					newRoutes = append(newRoutes, newRoute)
				}
			}
		}
	}
	ipr.knownRoutesLock.Lock()
	ipr.knownRoutes = newRoutes
	ipr.knownRoutesLock.Unlock()
}

func (ipr *IPRouterService) reconcileRoutingTable() {
	ipr.knownRoutesLock.RLock()
	defer ipr.knownRoutesLock.RUnlock()
	routes, err := netlink.RouteList(ipr.link, netlink.FAMILY_ALL)
	if err != nil {
		ipr.nc.Logger.Error("error retrieving kernel routes list: %s", err)

		return
	}

	fmt.Printf("=========\n")
	fmt.Printf("Receptor Routes:\n")
	for i := range ipr.knownRoutes {
		fmt.Printf("   dest %s via %s\n", ipr.knownRoutes[i].dest.String(), ipr.knownRoutes[i].via)
	}
	fmt.Printf("Kernel Routes:\n")
	for i := range routes {
		fmt.Printf("   dest %s\n", routes[i].Dst.String())
	}

	for i := range ipr.knownRoutes {
		kr := ipr.knownRoutes[i]
		found := false
		for j := range routes {
			route := routes[j]
			if kr.dest.IP.Equal(route.Dst.IP) && bytes.Equal(kr.dest.Mask, route.Dst.Mask) {
				found = true

				break
			}
		}
		if !found {
			ipr.nc.Logger.Debug("Adding route to %s", kr.dest.String())
			err := ipr.addRoute(kr.dest)
			if err != nil {
				ipr.nc.Logger.Error("error adding kernel route to %s: %s", kr.dest.String(), err)
			}
		}
	}
	_, ipv6LinkLocal, _ := net.ParseCIDR("fe80::/10")
	for i := range routes {
		route := routes[i]
		if ipr.localNet.Contains(route.Dst.IP) {
			continue
		}
		if ipv6LinkLocal.Contains(route.Dst.IP) {
			continue
		}
		found := false
		for j := range ipr.knownRoutes {
			kr := ipr.knownRoutes[j]
			if kr.dest.IP.Equal(route.Dst.IP) && bytes.Equal(kr.dest.Mask, route.Dst.Mask) {
				found = true

				break
			}
		}
		if !found {
			ipr.nc.Logger.Debug("Removing route to %s", route.Dst.String())
			err := netlink.RouteDel(&route)
			if err != nil {
				ipr.nc.Logger.Error("error deleting kernel route to %s: %s", route.Dst.String(), err)
			}
		}
	}
}

func (ipr *IPRouterService) runAdvertisingWatcher() {
	for {
		ipr.updateKnownRoutes()
		ipr.reconcileRoutingTable()
		select {
		case <-ipr.nc.Context().Done():
			return
		case <-time.After(10 * time.Second):
		}
	}
}

func (ipr *IPRouterService) runTunToNetceptor() {
	ipr.nc.Logger.Debug("Running tunnel-to-Receptor forwarder\n")
	buf := make([]byte, utils.NormalBufferSize)
	for {
		if ipr.nc.Context().Err() != nil {
			return
		}
		n, err := ipr.tunIf.Read(buf)
		if err != nil {
			ipr.nc.Logger.Error("Error reading from tun device: %s\n", err)

			continue
		}
		packet := buf[:n]

		// Get the destination address from the received packet
		ipVersion := int(packet[0] >> 4)
		var destIP net.IP
		switch ipVersion {
		case 4:
			header, err := ipv4.ParseHeader(packet)
			if err != nil {
				ipr.nc.Logger.Debug("Malformed ipv4 packet received: %s", err)
			}
			destIP = header.Dst
		case 6:
			header, err := ipv6.ParseHeader(packet)
			if err != nil {
				ipr.nc.Logger.Debug("Malformed ipv6 packet received: %s", err)
			}
			destIP = header.Dst
		default:
			ipr.nc.Logger.Debug("Packet received with unknown version %d", ipVersion)

			continue
		}

		// Find the lowest cost receptor node that can accept this packet
		remoteNode := ""
		remoteCost := math.MaxFloat64
		ipr.knownRoutesLock.RLock()
		for i := range ipr.knownRoutes {
			route := ipr.knownRoutes[i]
			cost, err := ipr.nc.PathCost(route.via)
			if err != nil {
				continue
			}
			if cost < remoteCost && route.dest.Contains(destIP) {
				remoteCost = cost
				remoteNode = route.via
			}
		}
		ipr.knownRoutesLock.RUnlock()
		if remoteNode == "" {
			continue
		}

		// Send the packet via Receptor
		remoteAddr := ipr.nc.NewAddr(remoteNode, ipr.networkName)
		ipr.nc.Logger.Trace("    Forwarding data length %d to %s via %s\n", n, destIP, remoteAddr.String())
		wn, err := ipr.nConn.WriteTo(packet, remoteAddr)
		if err != nil || wn != n {
			ipr.nc.Logger.Error("Error writing to Receptor network: %s\n", err)
		}
	}
}

func (ipr *IPRouterService) runNetceptorToTun() {
	ipr.nc.Logger.Debug("Running netceptor to tunnel forwarder\n")
	buf := make([]byte, utils.NormalBufferSize)
	for {
		if ipr.nc.Context().Err() != nil {
			return
		}
		n, addr, err := ipr.nConn.ReadFrom(buf)
		if err != nil {
			ipr.nc.Logger.Error("Error reading from Receptor: %s\n", err)

			continue
		}
		ipr.nc.Logger.Trace("    Forwarding data length %d from %s to %s\n", n,
			addr.String(), ipr.tunIf.Name())
		wn, err := ipr.tunIf.Write(buf[:n])
		if err != nil || wn != n {
			ipr.nc.Logger.Error("Error writing to tun device: %s\n", err)
		}
	}
}

func (ipr *IPRouterService) addRoute(route *net.IPNet) error {
	err := netlink.RouteAdd(&netlink.Route{
		LinkIndex: ipr.link.Attrs().Index,
		Scope:     netlink.SCOPE_UNIVERSE,
		Dst:       route,
		Gw:        ipr.destIP,
	})
	if err != nil {
		return fmt.Errorf("error adding route to interface: %s", err)
	}

	return nil
}

// Run runs the IP router.
func (ipr *IPRouterService) run() error {
	cfg := water.Config{
		DeviceType: water.TUN,
	}
	cfg.Name = ipr.tunIfName
	var err error
	ipr.tunIf, err = water.New(water.Config{DeviceType: water.TUN})
	if err != nil {
		return fmt.Errorf("error opening tun device: %s", err)
	}
	ipr.link, err = netlink.LinkByName(ipr.tunIf.Name())
	if err != nil {
		return fmt.Errorf("error accessing link for tun device: %s", err)
	}
	baseIP := ipr.localNet.IP.To4()
	ipr.linkIP = make([]byte, 4)
	copy(ipr.linkIP, baseIP)
	ipr.linkIP[3]++
	ipr.destIP = make([]byte, 4)
	copy(ipr.destIP, ipr.linkIP)
	ipr.destIP[3]++
	if !ipr.localNet.Contains(ipr.linkIP) || !ipr.localNet.Contains(ipr.destIP) {
		return fmt.Errorf("error calculating link and remote addresses")
	}
	addr := &netlink.Addr{
		IPNet: netlink.NewIPNet(ipr.linkIP),
		Peer:  netlink.NewIPNet(ipr.destIP),
	}
	err = netlink.AddrAdd(ipr.link, addr)
	if err != nil {
		return fmt.Errorf("error adding IP address to link: %s", err)
	}
	err = netlink.LinkSetUp(ipr.link)
	if err != nil {
		return fmt.Errorf("error setting link up: %s", err)
	}
	advertisement := map[string]string{
		"type":        adTypeIPRouter,
		"network":     ipr.networkName,
		"route_local": ipr.localNet.String(),
	}
	for i := range ipr.advertiseRoutes {
		advertisement[fmt.Sprintf("route_%d", i)] = ipr.advertiseRoutes[i].String()
	}
	ipr.nConn, err = ipr.nc.ListenPacketAndAdvertise(ipr.networkName, advertisement)
	if err != nil {
		return fmt.Errorf("error listening for service %s: %s", ipr.networkName, err)
	}
	go ipr.runAdvertisingWatcher()
	go ipr.runTunToNetceptor()
	go ipr.runNetceptorToTun()

	return nil
}

// ipRouterCfg is the cmdline configuration object for an IP router.
type IpRouterCfg struct {
	NetworkName string `required:"true" description:"Name of this network and service."`
	Interface   string `description:"Name of the local tun interface"`
	LocalNet    string `required:"true" description:"Local /30 CIDR address"`
	Routes      string `description:"Comma separated list of CIDR subnets to advertise"`
}

// Run runs the action.
func (cfg IpRouterCfg) Run() error {
	netceptor.MainInstance.Logger.Debug("Running tun router service %s\n", cfg)
	_, err := NewIPRouter(netceptor.MainInstance, cfg.NetworkName, cfg.Interface, cfg.LocalNet, cfg.Routes)
	if err != nil {
		return err
	}

	return nil
}

func init() {
	version := viper.GetInt("version")
	if version > 1 {
		return
	}
	cmdline.RegisterConfigTypeForApp("receptor-ip-router",
		"ip-router", "Run an IP router using a tun interface", IpRouterCfg{}, cmdline.Section(servicesSection))
}
