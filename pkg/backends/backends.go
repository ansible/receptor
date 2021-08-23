//go:build !no_backends
// +build !no_backends

package backends

import (
	"errors"
	"fmt"

	"github.com/ansible/receptor/pkg/netceptor"
)

// ErrInvalidCost indicates an invalid path cost.
var ErrInvalidCost = errors.New("cost is smaller or equal 0")

func validateDialCost(rawCost *float64) (float64, error) {
	cost := 1.0
	if rawCost != nil {
		cost = *rawCost
		if cost <= 0.0 {
			return 0, fmt.Errorf("connection cost: %w", ErrInvalidCost)
		}
	}

	return cost, nil
}

func validateListenerCost(rawCost *float64, rawNodeCost map[string]float64) (float64, map[string]float64, error) {
	cost := 1.0
	if rawCost != nil {
		cost = *rawCost
		if cost <= 0.0 {
			return 0, nil, fmt.Errorf("connection cost: %w", ErrInvalidCost)
		}
	}
	for node, cost := range rawNodeCost {
		if cost <= 0.0 {
			return 0, nil, fmt.Errorf("node cost for %s: %w", node, ErrInvalidCost)
		}
	}

	return cost, rawNodeCost, nil
}

// Dial to other instances.
type Dial struct {
	// WS dial.
	WS []WSDial `mapstructure:"ws"`
	// UDP dial.
	UDP []UDPDial `mapstructure:"udp"`
	// TCP dial.
	TCP []TCPDial `mapstructure:"tcp"`
}

// Listen for connections of other instances.
type Listen struct {
	// WS listener.
	WS []WSListen `mapstructure:"ws"`
	// UDP listener.
	UDP []UDPListen `mapstructure:"udp"`
	// TCP listener.
	TCP []TCPListen `mapstructure:"tcp"`
}

// Backends is a set of backends used by a receptor instance.
type Backends struct {
	// Dial to other instances.
	Dial Dial `mapstructure:"dial"`
	// Listen for connections of other instances.
	Listen Listen `mapstructure:"listen"`
}

// Setup attaches the defined backends to the given netceptor.
func (b Backends) Setup(nc *netceptor.Netceptor) error {
	for _, c := range b.Listen.UDP {
		if err := c.setup(nc); err != nil {
			return fmt.Errorf("could not setup udp connection from connection config: %w", err)
		}
	}
	for _, l := range b.Dial.UDP {
		if err := l.setup(nc); err != nil {
			return fmt.Errorf("could not setup ws listen from listener config: %w", err)
		}
	}

	for _, c := range b.Listen.TCP {
		if err := c.setup(nc); err != nil {
			return fmt.Errorf("could not setup tcp connection from connection config: %w", err)
		}
	}
	for _, l := range b.Dial.TCP {
		if err := l.setup(nc); err != nil {
			return fmt.Errorf("could not setup tcp listen from listener config: %w", err)
		}
	}

	for _, c := range b.Listen.WS {
		if err := c.setup(nc); err != nil {
			return fmt.Errorf("could not setup ws connection from connection config: %w", err)
		}
	}
	for _, l := range b.Dial.WS {
		if err := l.setup(nc); err != nil {
			return fmt.Errorf("could not setup ws listen from listener config: %w", err)
		}
	}

	return nil
}
