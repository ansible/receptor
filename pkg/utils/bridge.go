package utils

import (
	"errors"
	"io"
	"net"

	"github.com/ansible/receptor/pkg/logger"
)

// NormalBufferSize is the size of buffers used by various processes when copying data between sockets.
const NormalBufferSize = 65536

// BridgeConns bridges two connections, like netcat.
func BridgeConns(connection1 io.ReadWriteCloser, connection1Name string, connection2 io.ReadWriteCloser, connection2Name string, logger *logger.ReceptorLogger) {
	doneChan := make(chan bool)
	go bridgeHalf(connection1, connection1Name, connection2, connection2Name, doneChan, logger)
	go bridgeHalf(connection2, connection2Name, connection1, connection1Name, doneChan, logger)
	<-doneChan
	<-doneChan
}

// BridgeHalf bridges the read side of sourceConnection to the write side of destinationConnection.
func bridgeHalf(sourceConnection io.ReadWriteCloser, sourceConnectionName string, destinationConnection io.ReadWriteCloser, destinationConnectionName string, done chan bool, logger *logger.ReceptorLogger) {
	logger.Trace("    Bridging %s to %s\n", sourceConnectionName, destinationConnectionName)
	defer func() {
		done <- true
	}()
	buf := make([]byte, NormalBufferSize)
	shouldClose := false
	for {
		n, err := sourceConnection.Read(buf)
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
				logger.Error("Connection read error: %s\n", err)
			}
			shouldClose = true
		}
		if n > 0 {
			logger.Trace("    Copied %d bytes from %s to %s\n", n, sourceConnectionName, destinationConnectionName)
			wn, err := destinationConnection.Write(buf[:n])
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
			logger.Trace("    Stopping bridge %s to %s\n", sourceConnectionName, destinationConnectionName)
			if err := destinationConnection.Close(); err != nil {
				logger.Error("Error closing %s: %s\n", destinationConnectionName, err)
			}

			return
		}
	}
}
