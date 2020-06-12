package sockutils

import (
	"github.com/ghjm/sockceptor/pkg/debug"
	"github.com/ghjm/sockceptor/pkg/netceptor"
	"io"
	"strings"
)

// BridgeConns bridges two connections, like netcat.
func BridgeConns(c1 io.ReadWriteCloser, c1Name string, c2 io.ReadWriteCloser, c2Name string) {
	doneChan := make(chan bool)
	go bridgeHalf(c1, c1Name, c2, c2Name, doneChan)
	go bridgeHalf(c2, c2Name, c1, c1Name, doneChan)
	<-doneChan
}

// BridgeHalf bridges the read side of c1 to the write side of c2.
func bridgeHalf(c1 io.ReadWriteCloser, c1Name string, c2 io.ReadWriteCloser, c2Name string, done chan bool) {
	debug.Tracef("    Bridging %s to %s\n", c1Name, c2Name)
	defer func() {
		done <- true
	}()
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
		if n > 0 {
			debug.Tracef("    Copied %d bytes from %s to %s\n", n, c1Name, c2Name)
			wn, err := c2.Write(buf[:n])
			if err != nil {
				debug.Printf("Connection write error: %s\n", err)
				shouldClose = true
			}
			if wn != n {
				debug.Printf("Not all bytes written\n")
				shouldClose = true
			}
		}
		if shouldClose {
			debug.Tracef("    Stopping bridge %s to %s\n", c1Name, c2Name)
			_ = c2.Close()
			return
		}
	}
}
