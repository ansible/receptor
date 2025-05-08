//go:build windows
// +build windows

package utils

import (
	"net"
	"os"
)

// UnixSocketListen is not available on Windows
func UnixSocketListen(filename string, permissions os.FileMode) (net.Listener, *FLock, error) {
	return nil, nil, MakeWindowsSocketError()
}
