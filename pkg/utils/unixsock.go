//go:build !windows
// +build !windows

package utils

import (
	"fmt"
	"net"
	"os"
)

// UnixSocketListen listens on a Unix socket, handling file locking and permissions.
func UnixSocketListen(filename string, permissions os.FileMode) (net.Listener, *FLock, error) {
	lock, err := TryFLock(filename + ".lock")
	if err != nil {
		return nil, nil, ErrSocketLockFileNotAcquired
	}
	err = os.RemoveAll(filename)
	if err != nil {
		_ = lock.Unlock()

		return nil, nil, fmt.Errorf("%s: %s", ErrSocketFileNotOverwritten, err)
	}
	uli, err := net.Listen("unix", filename)
	if err != nil {
		_ = lock.Unlock()

		return nil, nil, fmt.Errorf("%s: %s", ErrSocketFileListen, err)
	}
	err = os.Chmod(filename, permissions)
	if err != nil {
		_ = uli.Close()
		_ = lock.Unlock()

		return nil, nil, fmt.Errorf("%s: %s", ErrSocketFilePermissionsNotSet, err)
	}

	return uli, lock, nil
}
