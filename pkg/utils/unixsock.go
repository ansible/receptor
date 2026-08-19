//go:build !windows
// +build !windows

package utils

import (
	"net"
	"os"
)

// UnixSocketListen listens on a Unix socket, handling file locking and permissions.
func UnixSocketListen(filename string, permissions os.FileMode) (net.Listener, *FLock, error) {
	lock, err := TryFLock(filename + ".lock")
	if err != nil {
		return nil, nil, MakeUnixSocketError(ErrSocketLockFileNotAcquired, err)
	}
	err = os.RemoveAll(filename)
	if err != nil {
		_ = lock.Unlock()

		return nil, nil, MakeUnixSocketError(ErrSocketFileNotOverwritten, err)
	}
	uli, err := net.Listen("unix", filename) //nolint:noctx // function does not receive a context
	if err != nil {
		_ = lock.Unlock()

		return nil, nil, MakeUnixSocketError(ErrSocketFileListen, err)
	}
	err = os.Chmod(filename, permissions)
	if err != nil {
		_ = uli.Close()
		_ = lock.Unlock()

		return nil, nil, MakeUnixSocketError(ErrSocketFilePermissionsNotSet, err)
	}

	return uli, lock, nil
}
