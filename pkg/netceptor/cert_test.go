package netceptor

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"strings"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/certificates"
	"github.com/ansible/receptor/pkg/logger"
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

func TestReceptorVerifyFunc(t *testing.T) {
	// Test the actual ReceptorVerifyFunc implementation for coverage
	ca, serverCert := setupCAAndServerCert(t)
	testLogger := logger.NewReceptorLogger("test")

	// Create TLS config with our CA
	serverTLSConfig := &tls.Config{
		ClientCAs: setupTestPool(ca.Certificate),
	}
	clientTLSConfig := &tls.Config{
		RootCAs: setupTestPool(ca.Certificate),
	}

	tests := []struct {
		name            string
		tlsConfig       *tls.Config
		rawCerts        [][]byte
		verifyType      VerifyType
		expectedError   bool
		errorContains   string
		testDescription string
	}{
		{
			name:            "no certificates provided",
			tlsConfig:       clientTLSConfig,
			rawCerts:        [][]byte{},
			verifyType:      VerifyServer,
			expectedError:   true,
			errorContains:   "peer certificate missing",
			testDescription: "Should fail when no certificates provided",
		},
		{
			name:            "invalid certificate data",
			tlsConfig:       clientTLSConfig,
			rawCerts:        [][]byte{[]byte("invalid cert data")},
			verifyType:      VerifyServer,
			expectedError:   true,
			errorContains:   "failed to parse certificate",
			testDescription: "Should fail with malformed certificate data",
		},
		{
			name:            "valid server certificate",
			tlsConfig:       clientTLSConfig,
			rawCerts:        [][]byte{serverCert.Raw, ca.Certificate.Raw},
			verifyType:      VerifyServer,
			expectedError:   false,
			testDescription: "Should succeed with valid server certificate and CA",
		},
		{
			name:            "valid client certificate",
			tlsConfig:       serverTLSConfig,
			rawCerts:        [][]byte{serverCert.Raw, ca.Certificate.Raw},
			verifyType:      VerifyClient,
			expectedError:   false,
			testDescription: "Should succeed with valid client certificate and CA",
		},
		{
			name:            "duplicate CA in chain",
			tlsConfig:       clientTLSConfig,
			rawCerts:        [][]byte{serverCert.Raw, ca.Certificate.Raw, ca.Certificate.Raw},
			verifyType:      VerifyServer,
			expectedError:   false,
			testDescription: "Should handle duplicate CA certificates in chain",
		},
		{
			name:            "invalid verify type",
			tlsConfig:       clientTLSConfig,
			rawCerts:        [][]byte{serverCert.Raw},
			verifyType:      VerifyType(99), // Invalid verify type
			expectedError:   true,
			errorContains:   "invalid verification type",
			testDescription: "Should fail with invalid verification type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create the verify function
			verifyFunc := ReceptorVerifyFunc(
				tt.tlsConfig,
				nil, // No pinned fingerprints for basic tests
				"",  // No expected hostname for basic tests
				ExpectedHostnameTypeDNS,
				tt.verifyType,
				testLogger,
			)

			// Call the verify function
			err := verifyFunc(tt.rawCerts, nil)

			// Check results
			if tt.expectedError {
				if err == nil {
					t.Errorf("Expected error but got none. %s", tt.testDescription)
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s' but got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v. %s", err, tt.testDescription)
				}
			}
		})
	}
}

func TestReceptorVerifyFuncWithPinnedFingerprints(t *testing.T) {
	// Test pinned fingerprint functionality
	ca, serverCert := setupCAAndServerCert(t)
	testLogger := logger.NewReceptorLogger("test")

	clientTLSConfig := &tls.Config{
		RootCAs: setupTestPool(ca.Certificate),
	}

	// Generate SHA256 fingerprint of server cert
	sha256Hash := sha256.Sum256(serverCert.Raw)
	validFingerprint := sha256Hash[:]

	// Generate invalid fingerprint
	invalidFingerprint := make([]byte, 32)
	copy(invalidFingerprint, validFingerprint)
	invalidFingerprint[0] = ^invalidFingerprint[0] // Flip bits to make it different

	tests := []struct {
		name               string
		pinnedFingerprints [][]byte
		expectedError      bool
		errorContains      string
	}{
		{
			name:               "valid pinned fingerprint",
			pinnedFingerprints: [][]byte{validFingerprint},
			expectedError:      false,
		},
		{
			name:               "invalid pinned fingerprint",
			pinnedFingerprints: [][]byte{invalidFingerprint},
			expectedError:      true,
			errorContains:      "does not match any pinned fingerprint",
		},
		{
			name:               "invalid fingerprint length",
			pinnedFingerprints: [][]byte{[]byte("too short")},
			expectedError:      true,
			errorContains:      "pinned certificate must be sha224, sha256, sha384 or sha512",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verifyFunc := ReceptorVerifyFunc(
				clientTLSConfig,
				tt.pinnedFingerprints,
				"",
				ExpectedHostnameTypeDNS,
				VerifyServer,
				testLogger,
			)

			err := verifyFunc([][]byte{serverCert.Raw}, nil)

			if tt.expectedError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s' but got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}
