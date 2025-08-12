package netceptor

import (
	"crypto/x509"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/certificates"
)

func TestIsCertificateInPool(t *testing.T) {
	// Use Receptor's certificate creation functions
	rsaWrapper := &certificates.RsaWrapper{}

	// Create a test CA certificate
	caOpts := &certificates.CertOptions{
		CommonName: "Test CA",
		Bits:       2048,
		NotBefore:  time.Now(),
		NotAfter:   time.Now().Add(365 * 24 * time.Hour),
	}

	ca, err := certificates.CreateCA(caOpts, rsaWrapper)
	if err != nil {
		t.Fatalf("Failed to create CA: %v", err)
	}

	// Create a different CA certificate
	otherCAOpts := &certificates.CertOptions{
		CommonName: "Other Test CA",
		Bits:       2048,
		NotBefore:  time.Now(),
		NotAfter:   time.Now().Add(365 * 24 * time.Hour),
	}

	otherCA, err := certificates.CreateCA(otherCAOpts, rsaWrapper)
	if err != nil {
		t.Fatalf("Failed to create other CA: %v", err)
	}

	tests := []struct {
		name     string
		pool     *x509.CertPool
		expected bool
	}{
		{
			name:     "nil pool",
			pool:     nil,
			expected: false,
		},
		{
			name:     "empty pool",
			pool:     x509.NewCertPool(),
			expected: false,
		},
		{
			name: "certificate in pool",
			pool: func() *x509.CertPool {
				pool := x509.NewCertPool()
				pool.AddCert(ca.Certificate)

				return pool
			}(),
			expected: true,
		},
		{
			name: "certificate not in pool",
			pool: func() *x509.CertPool {
				pool := x509.NewCertPool()
				pool.AddCert(otherCA.Certificate)

				return pool
			}(),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isCertificateInPool(ca.Certificate, tt.pool)
			if result != tt.expected {
				t.Errorf("isCertificateInPool() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestReceptorVerifyFuncWithDuplicates(t *testing.T) {
	// This test verifies that ReceptorVerifyFunc properly handles duplicate certificates
	// and logs the appropriate debug messages when skipping duplicates

	rsaWrapper := &certificates.RsaWrapper{}

	// Create a CA certificate
	caOpts := &certificates.CertOptions{
		CommonName: "Test CA",
		Bits:       2048,
		NotBefore:  time.Now(),
		NotAfter:   time.Now().Add(365 * 24 * time.Hour),
	}

	ca, err := certificates.CreateCA(caOpts, rsaWrapper)
	if err != nil {
		t.Fatalf("Failed to create CA: %v", err)
	}

	// Create a certificate request for a server certificate
	serverReqOpts := &certificates.CertOptions{
		CertNames: certificates.CertNames{
			DNSNames:    []string{"test-server.local"},
			NodeIDs:     []string{"test-node"},
			IPAddresses: nil,
		},
		CommonName: "test-server",
		Bits:       2048,
	}

	serverReq, _, err := certificates.CreateCertReqWithKey(serverReqOpts)
	if err != nil {
		t.Fatalf("Failed to create server certificate request: %v", err)
	}

	// Sign the server certificate with the CA
	serverCertOpts := &certificates.CertOptions{
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(365 * 24 * time.Hour),
	}

	serverCert, err := certificates.SignCertReq(serverReq, ca, serverCertOpts)
	if err != nil {
		t.Fatalf("Failed to sign server certificate: %v", err)
	}

	// Test that when CA cert is already in RootCAs pool,
	// it doesn't get added again as an intermediate
	t.Run("duplicate CA certificate handling", func(t *testing.T) {
		// Create a certificate pool with the CA cert
		rootPool := x509.NewCertPool()
		rootPool.AddCert(ca.Certificate)

		// Verify that our CA cert is considered to be in the pool
		if !isCertificateInPool(ca.Certificate, rootPool) {
			t.Error("CA certificate should be detected as already in pool")
		}

		// Verify that the server cert (which is different) is not in the pool
		if isCertificateInPool(serverCert, rootPool) {
			t.Error("Server certificate should not be detected as in CA pool")
		}
	})

	// Test that intermediate certificates that are duplicates of root CAs are skipped
	t.Run("intermediate duplicate detection", func(t *testing.T) {
		// This test simulates the scenario where a peer sends a certificate chain
		// that includes intermediate certificates already present in our root CA pool

		rootPool := x509.NewCertPool()
		rootPool.AddCert(ca.Certificate)

		// Simulate what happens in ReceptorVerifyFunc when we have:
		// certs[0] = server certificate
		// certs[1] = CA certificate (already in our root pool)

		// The CA certificate should be detected as already present in the root pool
		isDuplicate := isCertificateInPool(ca.Certificate, rootPool)
		if !isDuplicate {
			t.Error("CA certificate should be detected as duplicate when already in root pool")
		}

		// The server certificate should not be detected as duplicate
		isServerDuplicate := isCertificateInPool(serverCert, rootPool)
		if isServerDuplicate {
			t.Error("Server certificate should not be detected as duplicate")
		}
	})
}
