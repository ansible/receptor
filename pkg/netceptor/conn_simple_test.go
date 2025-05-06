package netceptor

import (
	"context"
	"crypto/x509"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestServerTLSConfig tests the GenerateServerTLSConfig function.
func TestServerTLSConfig(t *testing.T) {
	// Call the function
	config := GenerateServerTLSConfig()

	// Verify the result
	assert.NotNil(t, config)
	assert.Equal(t, []string{"netceptor"}, config.NextProtos)
	assert.Len(t, config.Certificates, 1)

	// Verify the certificate
	cert, err := x509.ParseCertificate(config.Certificates[0].Certificate[0])
	assert.NoError(t, err)
	assert.Equal(t, "netceptor-insecure-common-name", cert.Subject.CommonName)
}

// TestServerCertVerification tests the VerifyServerCertificate function.
func TestServerCertVerification(t *testing.T) {
	// Generate a server TLS config to get a valid certificate
	config := GenerateServerTLSConfig()
	rawCert := config.Certificates[0].Certificate[0]

	// Test with a valid certificate
	err := VerifyServerCertificate([][]byte{rawCert}, nil)
	assert.NoError(t, err)

	// Test with no certificates
	err = VerifyServerCertificate([][]byte{}, nil)
	assert.Error(t, err)

	// Test with invalid certificate data
	err = VerifyServerCertificate([][]byte{{1, 2, 3, 4}}, nil)
	assert.Error(t, err)
}

// TestClientTLSConfig tests the GenerateClientTLSConfig function.
func TestClientTLSConfig(t *testing.T) {
	// Call the function
	host := "test-host"
	config := GenerateClientTLSConfig(host)

	// Verify the result
	assert.NotNil(t, config)
	assert.True(t, config.InsecureSkipVerify)
	assert.NotNil(t, config.VerifyPeerCertificate)
	assert.Equal(t, []string{"netceptor"}, config.NextProtos)
	assert.Equal(t, host, config.ServerName)
}

// Skip the tracer test since it's difficult to test without mocking
// the quic.ConnectionID type

// TestNetceptorListen tests basic functionality of the Listen method
func TestNetceptorListen(t *testing.T) {
	// Skip this test in CI environments or when network operations are not possible
	if os.Getenv("CI") != "" {
		t.Skip("Skipping test in CI environment")
	}

	// Create a Netceptor instance
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := New(ctx, "test-node")

	// Generate a random service name
	serviceName := randString(8)

	// Call Listen
	listener, err := s.Listen(serviceName, nil)
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	// Verify the listener
	assert.NotNil(t, listener)

	// Clean up
	err = listener.Close()
	assert.NoError(t, err)
}

// TestNetceptorListenServiceTooLong tests that Listen returns an error for service names that are too long
func TestNetceptorListenServiceTooLong(t *testing.T) {
	// Create a Netceptor instance
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := New(ctx, "test-node")

	// Call Listen with a service name that's too long
	listener, err := s.Listen("service-name-too-long", nil)

	// Verify the result
	assert.Error(t, err)
	assert.Nil(t, listener)
	assert.Contains(t, err.Error(), "service name service-name-too-long too long")
}

// Helper function to generate a random string
func randString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		time.Sleep(1 * time.Nanosecond) // Ensure uniqueness
	}

	return string(result)
}

// TestNetceptorListenAndAdvertise tests basic functionality of the ListenAndAdvertise method
func TestNetceptorListenAndAdvertise(t *testing.T) {
	// Skip this test in CI environments or when network operations are not possible
	if os.Getenv("CI") != "" {
		t.Skip("Skipping test in CI environment")
	}

	// Create a Netceptor instance
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := New(ctx, "test-node")

	// Generate a random service name
	serviceName := randString(8)

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

// TestNetceptorDialInvalidService tests that Dial returns an error for invalid services
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

// TestNetceptorDialContextCanceled tests that DialContext returns an error when the context is canceled
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
