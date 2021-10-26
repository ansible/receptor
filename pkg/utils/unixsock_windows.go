//go:build windows
// +build windows

package utils

import (
	"fmt"
	"net"
	"os"
)

// UnixSocketListen is not available on Windows
func UnixSocketListen(filename string, permissions os.FileMode) (net.Listener, *FLock, error) {
	return nil, nil, fmt.Errorf("Unix sockets not available on Windows")
}
