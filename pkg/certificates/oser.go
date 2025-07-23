package certificates

import (
	"io/fs"
	"os"
)

// Oser is the function calls interfaces for mocking os.
type Oser interface {
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm fs.FileMode) error
}

// OsWrapper is the Wrapper structure for Oser.
type OsWrapper struct{}

// ReadFile for Oser defaults to os library call.
func (ow *OsWrapper) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

// WriteFile for Oser defaults to os library call.
func (ow *OsWrapper) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(name, data, perm)
}
