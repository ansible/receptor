package netceptor_test

import (
	"crypto/tls"
	"crypto/x509"
	"testing"

	"github.com/ansible/receptor/pkg/netceptor"
)

func TestGenerateServerTLSConfig(t *testing.T) {
	// Call the function to generate a server TLS config
	config := netceptor.GenerateServerTLSConfig()

	// Verify the config has expected properties
	if config == nil {
		t.Fatal("Expected non-nil TLS config")
	}

	// Check certificates
	if len(config.Certificates) != 1 {
		t.Errorf("Expected 1 certificate, got %d", len(config.Certificates))
	}

	// Check NextProtos
	if len(config.NextProtos) != 1 || config.NextProtos[0] != "netceptor" {
		t.Errorf("Expected NextProtos to be ['netceptor'], got %v", config.NextProtos)
	}

	// Check MinVersion
	if config.MinVersion != tls.VersionTLS12 {
		t.Errorf("Expected MinVersion to be TLS 1.2, got %d", config.MinVersion)
	}

	// Check PreferServerCipherSuites
	if !config.PreferServerCipherSuites {
		t.Error("Expected PreferServerCipherSuites to be true")
	}

	// Verify the certificate has the expected common name
	cert, err := x509.ParseCertificate(config.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	expectedCN := "netceptor-insecure-common-name"
	if cert.Subject.CommonName != expectedCN {
		t.Errorf("Expected certificate CommonName to be %s, got %s", expectedCN, cert.Subject.CommonName)
	}
}

func TestVerifyServerCertificate(t *testing.T) {
	// Generate a server TLS config to get a valid certificate
	config := netceptor.GenerateServerTLSConfig()
	rawCert := config.Certificates[0].Certificate[0]

	// Test cases
	testCases := []struct {
		name      string
		rawCerts  [][]byte
		expectErr bool
	}{
		{
			name:      "Valid certificate with correct common name",
			rawCerts:  [][]byte{rawCert},
			expectErr: false,
		},
		{
			name:      "No certificates",
			rawCerts:  [][]byte{},
			expectErr: true,
		},
		{
			name:      "Invalid certificate data",
			rawCerts:  [][]byte{{1, 2, 3, 4}},
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := netceptor.VerifyServerCertificate(tc.rawCerts, nil)
			
			if tc.expectErr && err == nil {
				t.Error("Expected error but got nil")
			}
			
			if !tc.expectErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestGenerateClientTLSConfig(t *testing.T) {
	testHost := "test-host"
	
	// Call the function to generate a client TLS config
	config := netceptor.GenerateClientTLSConfig(testHost)

	// Verify the config has expected properties
	if config == nil {
		t.Fatal("Expected non-nil TLS config")
	}

	// Check InsecureSkipVerify
	if !config.InsecureSkipVerify {
		t.Error("Expected InsecureSkipVerify to be true")
	}

	// Check VerifyPeerCertificate is set
	if config.VerifyPeerCertificate == nil {
		t.Error("Expected VerifyPeerCertificate to be set")
	}

	// Check NextProtos
	if len(config.NextProtos) != 1 || config.NextProtos[0] != "netceptor" {
		t.Errorf("Expected NextProtos to be ['netceptor'], got %v", config.NextProtos)
	}

	// Check ServerName
	if config.ServerName != testHost {
		t.Errorf("Expected ServerName to be %s, got %s", testHost, config.ServerName)
	}

	// Check MinVersion
	if config.MinVersion != tls.VersionTLS12 {
		t.Errorf("Expected MinVersion to be TLS 1.2, got %d", config.MinVersion)
	}
}
