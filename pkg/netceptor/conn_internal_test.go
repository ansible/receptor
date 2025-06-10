package netceptor

import (
	"context"
	"crypto/x509"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateServerTLSConfig(t *testing.T) {
	// Call the function
	config := generateServerTLSConfig()

	// Verify the result
	assert.NotNil(t, config)
	assert.Equal(t, []string{"netceptor"}, config.NextProtos)
	assert.Len(t, config.Certificates, 1)

	// Verify the certificate
	cert, err := x509.ParseCertificate(config.Certificates[0].Certificate[0])
	assert.NoError(t, err)
	assert.Equal(t, "netceptor-insecure-common-name", cert.Subject.CommonName)
}

func TestVerifyServerCertificate(t *testing.T) {
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
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGenerateClientTLSConfig(t *testing.T) {
	// Call the function
	host := "test-host"
	config := generateClientTLSConfig(host)

	// Verify the result
	assert.NotNil(t, config)
	assert.True(t, config.InsecureSkipVerify)
	assert.NotNil(t, config.VerifyPeerCertificate)
	assert.Equal(t, []string{"netceptor"}, config.NextProtos)
	assert.Equal(t, host, config.ServerName)
}
func TestNetceptorListen(t *testing.T) {
	// Skip this test in CI environments or when network operations are not possible
	if os.Getenv("CI") != "" {
		t.Skip("Skipping test in CI environment")
	}

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
				assert.Error(t, err)
				assert.Nil(t, listener)
				if tt.expectedErrorSubstr != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorSubstr)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, listener)

				if tt.needsCleanup && listener != nil {
					err = listener.Close()
					assert.NoError(t, err)
				}
			}
		})
	}
}

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
	assert.NotNil(t, listener)

	// Clean up
	err = listener.Close()
	assert.NoError(t, err)
}

func TestNetceptorDialInvalidService(t *testing.T) {
	// Create a Netceptor instance
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := New(ctx, "test-node")

	// Call Dial with a non-existent node and service
	conn, err := s.Dial("non-existent-node", "non-existent-service", nil)

	// Verify the result - we expect an error because the node doesn't exist
	assert.Error(t, err)
	assert.Nil(t, conn)
}

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
	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Contains(t, err.Error(), "context canceled")
}
