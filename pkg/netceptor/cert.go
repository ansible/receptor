package netceptor

import (
	"bytes"
	"crypto/x509"
)

// isCertificateInPool checks if a certificate is already present in the given certificate pool.
// Since x509.CertPool doesn't expose its contents directly in modern Go versions,
// we use a verification approach specifically for self-signed certificates (CAs).
func isCertificateInPool(cert *x509.Certificate, pool *x509.CertPool) bool {
	if pool == nil {
		return false
	}

	// Only check if this is a CA certificate that could be in the pool
	// For non-CA certificates, we don't want to skip adding them as intermediates
	if !cert.IsCA {
		return false
	}

	// For CA certificates, check if it's self-signed (root CA)
	// and can be verified directly against the pool
	if !cert.IsCA || !bytes.Equal(cert.RawIssuer, cert.RawSubject) {
		return false
	}

	// Try to verify the self-signed certificate against the existing pool
	// If the certificate is already in the pool, verification should succeed
	// with an empty intermediates pool
	opts := x509.VerifyOptions{
		Roots:         pool,
		Intermediates: x509.NewCertPool(),
	}

	_, err := cert.Verify(opts)

	// If verification succeeds, the certificate is likely already in the pool
	return err == nil
}
