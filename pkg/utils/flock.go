//go:build !windows
// +build !windows

package utils

import (
	"fmt"
	"syscall"
)

// ErrLocked is returned when the flock is already held.
var ErrLocked = fmt.Errorf("fslock is already locked")

// FLock represents a file lock.
type FLock struct {
	fd int
}

type Syscaller interface {
	Close(fd int) (err error)
	Flock(fd int, how int) (err error)
	Open(path string, mode int, perm uint32) (fd int, err error)
}

type SyscallImpl struct{}

func (si SyscallImpl) Close(fd int) (err error) {
	return syscall.Close(fd)
}

func (si SyscallImpl) Flock(fd int, how int) (err error) {
	return syscall.Flock(fd, how)
}

func (si SyscallImpl) Open(path string, mode int, perm uint32) (fd int, err error) {
	return syscall.Open(path, mode, perm)
}

// TryFLock non-blockingly attempts to acquire a lock on the file.
func TryFLock(s Syscaller, filename string) (*FLock, error) {
	fd, err := s.Open(filename, syscall.O_CREAT|syscall.O_RDONLY|syscall.O_CLOEXEC, 0o600)
	if err != nil {
		return nil, err
	}
	err = s.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
	if err == syscall.EWOULDBLOCK {
		err = ErrLocked
	}
	if err != nil {
		_ = s.Close(fd)

		return nil, err
	}

	return &FLock{fd: fd}, nil
}

// Unlock unlocks the file lock.
func (lock *FLock) Unlock(s Syscaller) error {
	return s.Close(lock.fd)
}
