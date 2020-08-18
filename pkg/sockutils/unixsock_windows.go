//+build windows

package sockutils

import (
	"fmt"
	"net"
	"os"
)

// UnixSocketListen is not available on Windows
func UnixSocketListen(filename string, permissions os.FileMode) (net.Listener, int, error) {
	return nil, -1, fmt.Errorf("Unix sockets not available on Windows")
}
