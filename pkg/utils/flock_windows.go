// +build windows

package utils

import (
	"fmt"
	"time"
)

// ErrLocked is returned when the flock is already held
var ErrLocked = fmt.Errorf("fslock is already locked")

// FLock represents a Unix file lock, but is not usable on Windows
type FLock struct {
}

// TryFLock is not implemented on Windows
func TryFLock(filename string, retries int, delay time.Duration) (*FLock, error) {
	return nil, fmt.Errorf("file locks not implemented on Windows")
}

// Unlock is not implemented on Windows
func (lock *FLock) Unlock() error {
	return fmt.Errorf("file locks not implemented on Windows")
}
