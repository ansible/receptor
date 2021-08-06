// +build linux,!no_ip_router
// +build linux,!no_services

package services

import (
	"fmt"

	"github.com/project-receptor/receptor/pkg/netceptor"
)

// Services defines a set of receptor services.
type Services struct {
	// Run commands.
	Command []Command `mapstructure:"command"`
	// Route IP.
	IPRouter []IPRouter `mapstructure:"ip-router"`
	// Proxy sockets.
	Proxies *Proxies `mapstructure:"proxies"`
}

// Services defines a set of receptor services that proxy sockets.
type Proxies struct {
	// Expose TCP ports.
	TCPIn []TCPInProxy `mapstructure:"tls-in"`
	// Export TCP ports.
	TCPOut []TCPOutProxy `mapstructure:"tls-out"`
	// Expose UDP ports.
	UDPIn []UDPInProxy `mapstructure:"udp-in"`
	// Export udp sockets.
	UDPOut []UDPOutProxy `mapstructure:"udp-out"`
	// Expose unix sockets.
	UnixIn []UnixInProxy `mapstructure:"unix-in"`
	// Export unix sockets.
	UnixOut []UnixOutProxy `mapstructure:"unix-out"`
}

func (p Proxies) setup(nc *netceptor.Netceptor) error {
	for _, c := range p.UnixIn {
		if err := c.setup(nc); err != nil {
			return fmt.Errorf("could not setup unix inbound proxy connection from proxies config: %w", err)
		}
	}

	for _, c := range p.UnixOut {
		if err := c.setup(nc); err != nil {
			return fmt.Errorf("could not setup unix outbound proxy connection from proxies config: %w", err)
		}
	}

	for _, c := range p.UDPIn {
		if err := c.setup(nc); err != nil {
			return fmt.Errorf("could not setup udp inbound proxy connection from proxies config: %w", err)
		}
	}

	for _, c := range p.UDPOut {
		if err := c.setup(nc); err != nil {
			return fmt.Errorf("could not setup udp outbound proxy connection from proxies config: %w", err)
		}
	}

	for _, c := range p.TCPIn {
		if err := c.setup(nc); err != nil {
			return fmt.Errorf("could not setup tcp inbound proxy connection from proxies config: %w", err)
		}
	}

	for _, c := range p.TCPOut {
		if err := c.setup(nc); err != nil {
			return fmt.Errorf("could not setup tcp outbound proxy connection from proxies config: %w", err)
		}
	}

	return nil
}

// Setup attaches all the defined services to the given netceptor.
func (s *Services) Setup(nc *netceptor.Netceptor) error {
	for _, s := range s.Command {
		if err := s.setup(nc); err != nil {
			return fmt.Errorf("could not setup control service from service config: %w", err)
		}
	}

	for _, r := range s.IPRouter {
		if err := r.setup(nc); err != nil {
			return fmt.Errorf("could not setup ip router from service config: %w", err)
		}
	}

	if s.Proxies != nil {
		if err := s.Proxies.setup(nc); err != nil {
			return fmt.Errorf("could not setup proxies from service config: %w", err)
		}
	}

	return nil
}
