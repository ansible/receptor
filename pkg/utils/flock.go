// +build !windows

package utils

import (
	"fmt"
	"syscall"
	"time"
)

// ErrLocked is returned when the flock is already held
var ErrLocked = fmt.Errorf("fslock is already locked")

// FLock represents a file lock
type FLock struct {
	fd int
}

// TryFLock non-blockingly attempts to acquire a lock on the file
func TryFLock(filename string, retries int, delay time.Duration) (*FLock, error) {
	remainingRetries := retries
	var fd int
	var err error
	for {
		fd, err = syscall.Open(filename, syscall.O_CREAT|syscall.O_RDONLY|syscall.O_CLOEXEC, 0600)
		if err == nil {
			err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
			if err == syscall.EWOULDBLOCK {
				err = ErrLocked
			}
			if err == nil {
				break
			} else {
				_ = syscall.Close(fd)
			}
		}
		if remainingRetries <= 0 {
			return nil, err
		}
		remainingRetries--
		time.Sleep(delay)
	}
	return &FLock{fd: fd}, nil
}

// Unlock unlocks the file lock
func (lock *FLock) Unlock() error {
	return syscall.Close(lock.fd)
}
