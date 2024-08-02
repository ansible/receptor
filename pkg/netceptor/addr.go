package netceptor

import "fmt"

// Addr represents an endpoint address on the Netceptor network.
type Addr struct {
	NetworkStr string
	Node       string
	Service    string
}

// Network returns the network name.
func (a Addr) Network() string {
	return a.NetworkStr
}

// String formats this address as a string.
func (a Addr) String() string {
	return fmt.Sprintf("%s:%s", a.Node, a.Service)
}
