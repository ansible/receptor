package sockutils

import (
	"github.com/ghjm/sockceptor/pkg/debug"
	"github.com/ghjm/sockceptor/pkg/netceptor"
	"net"
	"strings"
	"sync"
)

// BridgeConns bridges two connections, like netcat.
func BridgeConns(c1 net.Conn, c2 net.Conn) {
	wg := &sync.WaitGroup{}
	wg.Add(2)
	go bridgeHalf(c1, c2, wg)
	go bridgeHalf(c2, c1, wg)
	wg.Wait()
}

// BridgeHalf bridges the read side of c1 to the write side of c2.
func bridgeHalf(c1 net.Conn, c2 net.Conn, wg *sync.WaitGroup) {
	defer wg.Done()
	buf := make([]byte, netceptor.MTU)
	shouldClose := false
	for {
		n, err := c1.Read(buf)
		if err != nil {
			if err.Error() != "EOF" && !strings.Contains(err.Error(), "use of closed network connection") {
				debug.Printf("Connection read error: %s\n", err)
			}
			shouldClose = true
		}
		debug.Tracef("    Forwarding data length %d from %s to %s\n", n,
			c1.RemoteAddr().String(), c2.RemoteAddr().String())
		wn, err := c2.Write(buf[:n])
		if err != nil {
			debug.Printf("Connection write error: %s\n", err)
			shouldClose = true
		}
		if wn != n {
			debug.Printf("Not all bytes written\n", err)
			shouldClose = true
		}
		if shouldClose {
			_ = c2.Close()
			return
		}
	}
}
