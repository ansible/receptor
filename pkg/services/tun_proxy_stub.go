//+build !linux

package services

import (
	"github.org/ghjm/sockceptor/pkg/debug"
	"github.org/ghjm/sockceptor/pkg/netceptor"
)

// TunProxyService runs the Receptor to tun interface proxy.
func TunProxyService(s *netceptor.Netceptor, tunInterface string, lservice string,
	node string, rservice string) {
	debug.Printf("Tun proxy only supported on Linux")
}
