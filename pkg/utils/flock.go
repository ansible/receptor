// +build !windows

package utils

import (
	"errors"
	"fmt"
	"syscall"
)

// ErrLocked is returned when the flock is already held
var ErrLocked = fmt.Errorf("fslock is already locked")

// FLock represents a file lock
type FLock struct {
	fd int
}

// TryFLock non-blockingly attempts to acquire a lock on the file
func TryFLock(filename string) (*FLock, error) {
	fd, err := syscall.Open(filename, syscall.O_CREAT|syscall.O_RDONLY|syscall.O_CLOEXEC, 0o600)
	if err != nil {
		return nil, err
	}
	err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		err = ErrLocked
	}
	if err != nil {
		_ = syscall.Close(fd)
		return nil, err
	}
	return &FLock{fd: fd}, nil
}

// Unlock unlocks the file lock
func (lock *FLock) Unlock() error {
	return syscall.Close(lock.fd)
}
