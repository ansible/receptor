package utils_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ansible/receptor/pkg/utils"
)

func TestMakeUnixSocketError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		err            error
		underlyingErr  error
		expectedSubstr string
	}{
		{
			name:           "Socket lock file not acquired",
			err:            utils.ErrSocketLockFileNotAcquired,
			underlyingErr:  errors.New("permission denied"),
			expectedSubstr: "could not acquire lock on socket file: permission denied",
		},
		{
			name:           "Socket file not overwritten",
			err:            utils.ErrSocketFileNotOverwritten,
			underlyingErr:  errors.New("file exists"),
			expectedSubstr: "could not overwrite socket file: file exists",
		},
		{
			name:           "Socket file listen error",
			err:            utils.ErrSocketFileListen,
			underlyingErr:  errors.New("address already in use"),
			expectedSubstr: "could not listen on socket file: address already in use",
		},
		{
			name:           "Socket file permissions not set",
			err:            utils.ErrSocketFilePermissionsNotSet,
			underlyingErr:  errors.New("chmod failed"),
			expectedSubstr: "error setting socket file permissions: chmod failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := utils.MakeUnixSocketError(tt.err, tt.underlyingErr)
			if result == nil {
				t.Error("Expected an error but got nil")
			}
			if !strings.Contains(result.Error(), tt.expectedSubstr) {
				t.Errorf("Expected error to contain %q, got %q", tt.expectedSubstr, result.Error())
			}
		})
	}
}

func TestMakeWindowsSocketError(t *testing.T) {
	t.Parallel()

	err := utils.MakeWindowsSocketError()
	if err == nil {
		t.Error("Expected an error but got nil")
	}
	if err != utils.ErrWindowsNotSupported {
		t.Errorf("Expected ErrWindowsNotSupported, got %v", err)
	}
	expectedMsg := "unix sockets not available on Windows"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestUnixSocketErrorConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		err         error
		expectedMsg string
	}{
		{
			name:        "ErrSocketLockFileNotAcquired",
			err:         utils.ErrSocketLockFileNotAcquired,
			expectedMsg: "could not acquire lock on socket file",
		},
		{
			name:        "ErrSocketFileNotOverwritten",
			err:         utils.ErrSocketFileNotOverwritten,
			expectedMsg: "could not overwrite socket file",
		},
		{
			name:        "ErrSocketFileListen",
			err:         utils.ErrSocketFileListen,
			expectedMsg: "could not listen on socket file",
		},
		{
			name:        "ErrSocketFilePermissionsNotSet",
			err:         utils.ErrSocketFilePermissionsNotSet,
			expectedMsg: "error setting socket file permissions",
		},
		{
			name:        "ErrWindowsNotSupported",
			err:         utils.ErrWindowsNotSupported,
			expectedMsg: "unix sockets not available on Windows",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.err == nil {
				t.Error("Error constant is nil")
			}
			if tt.err.Error() != tt.expectedMsg {
				t.Errorf("Expected error message %q, got %q", tt.expectedMsg, tt.err.Error())
			}
		})
	}
}
