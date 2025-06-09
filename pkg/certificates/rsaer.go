package certificates

import (
	"crypto/rsa"
	"io"
)

// Rsaer is the function calls interface for mocking rsa.
type Rsaer interface {
	GenerateKey(random io.Reader, bits int) (*rsa.PrivateKey, error)
}

// RsaWrapper is the Wrapper structure for Rsaer.
type RsaWrapper struct{}

// GenerateKey for RsaWrapper defaults to rsa library call.
func (rw *RsaWrapper) GenerateKey(random io.Reader, bits int) (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(random, bits)
}
