package utils

import (
	"github.com/project-receptor/receptor/pkg/logger"
	"io"
	"strings"
)

// NormalBufferSize is the size of buffers used by various processes when copying data between sockets
const NormalBufferSize = 65536

// BridgeConns bridges two connections, like netcat.
func BridgeConns(c1 io.ReadWriteCloser, c1Name string, c2 io.ReadWriteCloser, c2Name string) {
	doneChan := make(chan bool)
	go bridgeHalf(c1, c1Name, c2, c2Name, doneChan)
	go bridgeHalf(c2, c2Name, c1, c1Name, doneChan)
	<-doneChan
}

// BridgeHalf bridges the read side of c1 to the write side of c2.
func bridgeHalf(c1 io.ReadWriteCloser, c1Name string, c2 io.ReadWriteCloser, c2Name string, done chan bool) {
	logger.Trace("    Bridging %s to %s\n", c1Name, c2Name)
	defer func() {
		done <- true
	}()
	buf := make([]byte, NormalBufferSize)
	shouldClose := false
	for {
		n, err := c1.Read(buf)
		if err != nil {
			if err.Error() != "EOF" && !strings.Contains(err.Error(), "use of closed network connection") {
				logger.Error("Connection read error: %s\n", err)
			}
			shouldClose = true
		}
		if n > 0 {
			logger.Trace("    Copied %d bytes from %s to %s\n", n, c1Name, c2Name)
			wn, err := c2.Write(buf[:n])
			if err != nil {
				logger.Error("Connection write error: %s\n", err)
				shouldClose = true
			}
			if wn != n {
				logger.Error("Not all bytes written\n")
				shouldClose = true
			}
		}
		if shouldClose {
			logger.Trace("    Stopping bridge %s to %s\n", c1Name, c2Name)
			_ = c2.Close()
			return
		}
	}
}
