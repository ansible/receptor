//+build !windows

package sockutils

import (
	"fmt"
	"net"
	"os"
	"syscall"
)

// errLocked is returned when the flock is already held
var errLocked = fmt.Errorf("fslock is already locked")

// tryFLock non-blockingly attempts to acquire a lock on the file
func tryFLock(filename string) (int, error) {
	fd, err := syscall.Open(filename, syscall.O_CREAT|syscall.O_RDONLY|syscall.O_CLOEXEC, 0600)
	if err != nil {
		return 0, err
	}
	err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
	if err == syscall.EWOULDBLOCK {
		err = errLocked
	}
	if err != nil {
		_ = syscall.Close(fd)
		return 0, err
	}
	return fd, nil
}

// UnixSocketListen listens on a Unix socket, handling file locking and permissions
func UnixSocketListen(filename string, permissions os.FileMode) (net.Listener, int, error) {
	lockFd, err := tryFLock(filename + ".lock")
	if err != nil {
		return nil, -1, fmt.Errorf("could not acquire lock on socket file: %s", err)
	}
	err = os.RemoveAll(filename)
	if err != nil {
		_ = syscall.Close(lockFd)
		return nil, -1, fmt.Errorf("could not overwrite socket file: %s", err)
	}
	uli, err := net.Listen("unix", filename)
	if err != nil {
		_ = syscall.Close(lockFd)
		return nil, -1, fmt.Errorf("could not listen on socket file: %s", err)
	}
	err = os.Chmod(filename, permissions)
	if err != nil {
		_ = uli.Close()
		_ = syscall.Close(lockFd)
		return nil, -1, fmt.Errorf("error setting socket file permissions: %s", err)
	}
	return uli, lockFd, nil
}
