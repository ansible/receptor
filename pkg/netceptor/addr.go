package netceptor

import "fmt"

// Addr represents an endpoint address on the Netceptor network
type Addr struct {
	node    string
	service string
}

// NewAddr generates a Receptor network address from a node ID and service name
func NewAddr(node string, service string) Addr {
	return Addr{
		node:    node,
		service: service,
	}
}

// Network returns the network name, which is always just "netceptor"
func (a Addr) Network() string {
	return "netceptor"
}

// String formats this address as a string
func (a Addr) String() string {
	return fmt.Sprintf("%s:%s", a.node, a.service)
}
