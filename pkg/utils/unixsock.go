package utils

import (
	"fmt"
	"net"
	"os"
	"runtime"
)

// UnixSocketListen listens on a Unix socket, handling file locking and permissions.
func UnixSocketListen(filename string, permissions os.FileMode) (net.Listener, *FLock, error) {
	return unixSocketListen(filename, permissions, runtime.GOOS)
}

func unixSocketListen(filename string, permissions os.FileMode, operatingSystem string) (net.Listener, *FLock, error) {
	if operatingSystem == "windows" {
		return nil, nil, fmt.Errorf("unix sockets not available on Windows")
	}
	lock, err := TryFLock(filename + ".lock")
	if err != nil {
		return nil, nil, fmt.Errorf("could not acquire lock on socket file: %s", err)
	}
	err = os.RemoveAll(filename)
	if err != nil {
		_ = lock.Unlock()

		return nil, nil, fmt.Errorf("could not overwrite socket file: %s", err)
	}
	uli, err := net.Listen("unix", filename)
	if err != nil {
		_ = lock.Unlock()

		return nil, nil, fmt.Errorf("could not listen on socket file: %s", err)
	}
	err = os.Chmod(filename, permissions)
	if err != nil {
		_ = uli.Close()
		_ = lock.Unlock()

		return nil, nil, fmt.Errorf("error setting socket file permissions: %s", err)
	}

	return uli, lock, nil
}
