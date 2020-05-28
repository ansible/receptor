package netceptor

import "fmt"

type Addr struct {
	node 	string
	service string
}

// Generates a Receptor network address from a node ID and service name.
func NewAddr(node string, service string) Addr {
	return Addr{
		node:    node,
		service: service,
	}
}

func(a Addr) Network() string {
	return "netceptor"
}

func(a Addr) String() string {
	return fmt.Sprintf("%s:%s", a.node, a.service)
}

