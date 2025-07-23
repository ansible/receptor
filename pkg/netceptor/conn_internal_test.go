package netceptor

import (
	"context"
	"crypto/x509"
	"strings"
	"testing"
)

// TestServerTLSConfig tests the GenerateServerTLSConfig function.
func TestServerTLSConfig(t *testing.T) {
	// Call the function
	config := generateServerTLSConfig()

	// Verify the result
	if config == nil {
		t.Fatal("Expected config to be non-nil")
	}
	if len(config.NextProtos) != 1 || config.NextProtos[0] != "netceptor" {
		t.Errorf("Expected NextProtos to be ['netceptor'], got %v", config.NextProtos)
	}
	if len(config.Certificates) != 1 {
		t.Errorf("Expected 1 certificate, got %d", len(config.Certificates))
	}

	// Verify the certificate
	cert, err := x509.ParseCertificate(config.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}
	if cert.Subject.CommonName != "netceptor-insecure-common-name" {
		t.Errorf("Expected CommonName to be 'netceptor-insecure-common-name', got '%s'", cert.Subject.CommonName)
	}
}

// TestServerCertVerification tests the VerifyServerCertificate function.
func TestServerCertVerification(t *testing.T) {
	// Generate a server TLS config to get a valid certificate
	config := generateServerTLSConfig()
	rawCert := config.Certificates[0].Certificate[0]

	tests := []struct {
		name        string
		rawCerts    [][]byte
		expectError bool
	}{
		{
			name:        "Valid certificate",
			rawCerts:    [][]byte{rawCert},
			expectError: false,
		},
		{
			name:        "No certificates",
			rawCerts:    [][]byte{},
			expectError: true,
		},
		{
			name:        "Invalid certificate data",
			rawCerts:    [][]byte{{1, 2, 3, 4}},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := verifyServerCertificate(tt.rawCerts, nil)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for test case '%s', but got nil", tt.name)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for test case '%s', but got: %v", tt.name, err)
				}
			}
		})
	}
}

// TestClientTLSConfig tests the GenerateClientTLSConfig function.
func TestClientTLSConfig(t *testing.T) {
	// Call the function
	host := "test-host"
	config := generateClientTLSConfig(host)

	// Verify the result
	if config == nil {
		t.Fatal("Expected config to be non-nil")
	}
	if !config.InsecureSkipVerify {
		t.Error("Expected InsecureSkipVerify to be true")
	}
	if config.VerifyPeerCertificate == nil {
		t.Error("Expected VerifyPeerCertificate to be non-nil")
	}
	if len(config.NextProtos) != 1 || config.NextProtos[0] != "netceptor" {
		t.Errorf("Expected NextProtos to be ['netceptor'], got %v", config.NextProtos)
	}
	if config.ServerName != host {
		t.Errorf("Expected ServerName to be '%s', got '%s'", host, config.ServerName)
	}
}

// TestNetceptorListen tests the Listen method functionality.
func TestNetceptorListen(t *testing.T) {
	tests := []struct {
		name                string
		serviceName         string
		expectError         bool
		expectedErrorSubstr string
		needsCleanup        bool
	}{
		{
			name:         "Valid service name",
			serviceName:  "abcd", // 4 characters, within the 8-character limit
			expectError:  false,
			needsCleanup: true,
		},
		{
			name:                "Service name too long",
			serviceName:         "service-name-too-long", // 22 characters, exceeds 8-character limit
			expectError:         true,
			expectedErrorSubstr: "service name service-name-too-long too long",
			needsCleanup:        false,
		},
		{
			name:         "Empty service name gets ephemeral",
			serviceName:  "", // Empty service name should get an ephemeral service
			expectError:  false,
			needsCleanup: true,
		},
		{
			name:         "Maximum length service name",
			serviceName:  "abcd1234", // 8 characters, maximum allowed length
			expectError:  false,
			needsCleanup: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a Netceptor instance for each test case
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s := New(ctx, "test-node")

			// Call Listen
			listener, err := s.Listen(tt.serviceName, nil)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for test case '%s', but got nil", tt.name)
				}
				if listener != nil {
					t.Errorf("Expected listener to be nil for test case '%s', but got non-nil", tt.name)
				}
				if tt.expectedErrorSubstr != "" && err != nil {
					if !strings.Contains(err.Error(), tt.expectedErrorSubstr) {
						t.Errorf("Expected error to contain '%s', but got '%s'", tt.expectedErrorSubstr, err.Error())
					}
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for test case '%s', but got: %v", tt.name, err)
				}
				if listener == nil {
					t.Errorf("Expected listener to be non-nil for test case '%s'", tt.name)
				}

				if tt.needsCleanup && listener != nil {
					err = listener.Close()
					if err != nil {
						t.Errorf("Failed to close listener for test case '%s': %v", tt.name, err)
					}
				}
			}
		})
	}
}

// TestNetceptorListenAndAdvertise tests basic functionality of the ListenAndAdvertise method.
func TestNetceptorListenAndAdvertise(t *testing.T) {
	// Create a Netceptor instance
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := New(ctx, "test-node")

	serviceName := "test-svc"

	// Call ListenAndAdvertise
	tags := map[string]string{"tag1": "value1"}
	listener, err := s.ListenAndAdvertise(serviceName, nil, tags)
	if err != nil {
		t.Fatalf("Failed to listen and advertise: %v", err)
	}

	// Verify the listener
	if listener == nil {
		t.Fatal("Expected listener to be non-nil")
	}

	// Clean up
	err = listener.Close()
	if err != nil {
		t.Errorf("Failed to close listener: %v", err)
	}
}

// TestNetceptorDialInvalidService tests that Dial returns an error for invalid services.
func TestNetceptorDialInvalidService(t *testing.T) {
	// Create a Netceptor instance
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := New(ctx, "test-node")

	// Call Dial with a non-existent node and service
	conn, err := s.Dial("non-existent-node", "non-existent-service", nil)

	// Verify the result - we expect an error because the node doesn't exist
	if err == nil {
		t.Error("Expected error when dialing non-existent service, but got nil")
	}
	if conn != nil {
		t.Error("Expected conn to be nil when dialing non-existent service, but got non-nil")
	}
}

// TestNetceptorDialContextCanceled tests that DialContext returns an error when the context is canceled.
func TestNetceptorDialContextCanceled(t *testing.T) {
	// Create a Netceptor instance
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := New(ctx, "test-node")

	// Create a canceled context
	canceledCtx, cancelFunc := context.WithCancel(context.Background())
	cancelFunc()

	// Call DialContext with the canceled context
	conn, err := s.DialContext(canceledCtx, "non-existent-node", "non-existent-service", nil)

	// Verify the result - we expect a context canceled error
	if err == nil {
		t.Error("Expected error when dialing with canceled context, but got nil")
	}
	if conn != nil {
		t.Error("Expected conn to be nil when dialing with canceled context, but got non-nil")
	}
	if err != nil && !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("Expected error to contain 'context canceled', but got '%s'", err.Error())
	}
}
