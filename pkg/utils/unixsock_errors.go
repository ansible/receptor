package utils

import (
	"errors"
	"fmt"
)

var (
	ErrSocketLockFileNotAcquired   = errors.New("could not acquire lock on socket file")
	ErrSocketFileNotOverwritten    = errors.New("could not overwrite socket file")
	ErrSocketFileListen            = errors.New("could not listen on socket file")
	ErrSocketFilePermissionsNotSet = errors.New("error setting socket file permissions")
	ErrWindowsNotSupported         = errors.New("unix sockets not available on Windows")
)

func MakeUnixSocketError(err, underlyingErr error) error {
	return fmt.Errorf("%s: %s", err, underlyingErr)
}

func MakeWindowsSocketError() error {
	return ErrWindowsNotSupported
}
