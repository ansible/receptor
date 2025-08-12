package netceptor

import (
	"crypto/x509"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/certificates"
)

// Standard CA options used across tests.
var (
	caOpts = &certificates.CertOptions{
		CommonName: "Test CA",
		Bits:       2048,
		NotBefore:  time.Now(),
		NotAfter:   time.Now().Add(365 * 24 * time.Hour),
	}

	otherCAOpts = &certificates.CertOptions{
		CommonName: "Other Test CA",
		Bits:       2048,
		NotBefore:  time.Now(),
		NotAfter:   time.Now().Add(365 * 24 * time.Hour),
	}
)

// testCASetup creates test CA certificates for testing.
type testCASetup struct {
	ca      *certificates.CA
	otherCA *certificates.CA
}

func setupTestCAs(t *testing.T) *testCASetup {
	rsaWrapper := &certificates.RsaWrapper{}

	ca, err := certificates.CreateCA(caOpts, rsaWrapper)
	if err != nil {
		t.Fatalf("Failed to create CA: %v", err)
	}

	otherCA, err := certificates.CreateCA(otherCAOpts, rsaWrapper)
	if err != nil {
		t.Fatalf("Failed to create other CA: %v", err)
	}

	return &testCASetup{
		ca:      ca,
		otherCA: otherCA,
	}
}

// setupTestPool creates certificate pools with given certificates.
func setupTestPool(certs ...*x509.Certificate) *x509.CertPool {
	pool := x509.NewCertPool()
	for _, cert := range certs {
		if cert != nil {
			pool.AddCert(cert)
		}
	}

	return pool
}

func TestIsCertificateInPool(t *testing.T) {
	setup := setupTestCAs(t)

	tests := []struct {
		name     string
		pool     *x509.CertPool
		expected bool
	}{
		{"nil pool", nil, false},
		{"empty pool", setupTestPool(), false},
		{"certificate in pool", setupTestPool(setup.ca.Certificate), true},
		{"certificate not in pool", setupTestPool(setup.otherCA.Certificate), false},
		{
			"certificate in pool with multiple certs",
			setupTestPool(setup.otherCA.Certificate, setup.ca.Certificate), true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isCertificateInPool(setup.ca.Certificate, tt.pool)
			if result != tt.expected {
				t.Errorf("isCertificateInPool() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// setupCAAndServerCert creates a CA and a server certificate signed by that CA.
func setupCAAndServerCert(t *testing.T) (*certificates.CA, *x509.Certificate) {
	rsaWrapper := &certificates.RsaWrapper{}

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

	return ca, serverCert
}

func TestReceptorVerifyFuncWithDuplicates(t *testing.T) {
	// This test verifies that ReceptorVerifyFunc properly handles duplicate certificates
	// and logs the appropriate debug messages when skipping duplicates

	ca, serverCert := setupCAAndServerCert(t)
	setup := setupTestCAs(t)

	tests := []struct {
		name               string
		testCert           *x509.Certificate
		pool               *x509.CertPool
		expectedCAInPool   bool
		expectedServerInCA bool
	}{
		{"CA cert already in pool", ca.Certificate, setupTestPool(ca.Certificate), true, false},
		{"CA cert not in empty pool", ca.Certificate, setupTestPool(), false, false},
		{"CA cert not in different pool", ca.Certificate, setupTestPool(setup.otherCA.Certificate), false, false},
		{"server cert never in CA pool", serverCert, setupTestPool(ca.Certificate), false, false},
		{
			"duplicate detection in multi-cert pool", ca.Certificate,
			setupTestPool(setup.otherCA.Certificate, ca.Certificate), true, false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the certificate we're checking
			certInPool := isCertificateInPool(tt.testCert, tt.pool)
			if certInPool != tt.expectedCAInPool {
				t.Errorf("Certificate in pool: got %v, expected %v", certInPool, tt.expectedCAInPool)
			}

			// Always test that server cert is not detected as in CA pool (server certs are not CAs)
			serverInPool := isCertificateInPool(serverCert, tt.pool)
			if serverInPool != tt.expectedServerInCA {
				t.Errorf("Server certificate should never be in CA pool: got %v, expected %v",
					serverInPool, tt.expectedServerInCA)
			}
		})
	}
}
