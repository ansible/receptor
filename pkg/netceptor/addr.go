package netceptor

import "fmt"

// Addr represents an endpoint address on the Netceptor network.
type Addr struct {
	network string
	node       string
	service    string
}

// Network returns the network name.
func (a Addr) Network() string {
	return a.network
}

// String formats this address as a string.
func (a Addr) String() string {
	return fmt.Sprintf("%s:%s", a.node, a.service)
}

// SetNetwork sets the network variable.
func (a *Addr) SetNetwork(network string) {
	a.network = network
}

// SetNetwork sets the node variable.
func (a *Addr) SetNode(node string) {
	a.node = node
}

// SetNetwork sets the service variable.
func (a *Addr) SetService(service string) {
	a.service = service
}
