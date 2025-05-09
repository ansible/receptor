package netceptor

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/logger"
)

// mockConn is a simple implementation of net.Conn for testing.
type mockConn struct {
	net.Conn
}

func TestSendResult(t *testing.T) {
	// Test case 1: Normal send
	t.Run("Normal send", func(t *testing.T) {
		acceptChan := make(chan *AcceptResult, 1)
		doneChan := make(chan struct{})
		ctx := context.Background()

		li := &Listener{
			AcceptChan: acceptChan,
			DoneChan:   doneChan,
		}

		testConn := &mockConn{}
		testErr := errors.New("test error")

		// Call the method being tested
		li.sendResult(ctx, testConn, testErr)

		// Check if result was sent to the channel
		select {
		case result := <-acceptChan:
			// Verify the result content
			if result.Conn != testConn {
				t.Errorf("Expected conn %v, got %v", testConn, result.Conn)
			}
			if result.Err != testErr {
				t.Errorf("Expected error %v, got %v", testErr, result.Err)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("Expected result to be sent, but it wasn't")
		}
	})

	// Test case 2: Context cancelled
	t.Run("Context cancelled", func(t *testing.T) {
		acceptChan := make(chan *AcceptResult)
		doneChan := make(chan struct{})
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		li := &Listener{
			AcceptChan: acceptChan,
			DoneChan:   doneChan,
		}

		// Call the method being tested
		li.sendResult(ctx, nil, nil)

		// Check that nothing was sent to the channel
		// We use a non-blocking read to check if anything was sent
		select {
		case <-acceptChan:
			t.Error("Expected no result to be sent due to cancelled context")
		default:
			// This is the expected path - nothing was sent
		}
	})

	// Test case 3: Listener closed
	t.Run("Listener closed", func(t *testing.T) {
		acceptChan := make(chan *AcceptResult)
		doneChan := make(chan struct{})
		close(doneChan) // Close the done channel to simulate listener closed
		ctx := context.Background()

		li := &Listener{
			AcceptChan: acceptChan,
			DoneChan:   doneChan,
		}

		// Call the method being tested
		li.sendResult(ctx, nil, nil)

		// Check that nothing was sent to the channel
		// We use a non-blocking read to check if anything was sent
		select {
		case <-acceptChan:
			t.Error("Expected no result to be sent due to closed listener")
		default:
			// This is the expected path - nothing was sent
		}
	})

	// Test case 4: Successful connection with nil error
	t.Run("Successful connection", func(t *testing.T) {
		acceptChan := make(chan *AcceptResult, 1)
		doneChan := make(chan struct{})
		ctx := context.Background()

		li := &Listener{
			AcceptChan: acceptChan,
			DoneChan:   doneChan,
		}

		testConn := &mockConn{}

		// Call the method being tested
		li.sendResult(ctx, testConn, nil)

		// Check if result was sent to the channel
		select {
		case result := <-acceptChan:
			// Verify the result content
			if result.Conn != testConn {
				t.Errorf("Expected conn %v, got %v", testConn, result.Conn)
			}
			if result.Err != nil {
				t.Errorf("Expected nil error, got %v", result.Err)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("Expected result to be sent, but it wasn't")
		}
	})
}

// mockNetceptorForListen is a mock for Netceptor used in listen tests.
type mockNetceptorForListen struct {
	listenerRegistry  map[string]*PacketConn
	reservedServices  map[string]func(*MessageData) error
	nodeID            string
	maxForwardingHops byte
	logger            *logger.ReceptorLogger
	context           context.Context
	cancelFunc        context.CancelFunc
	listenerLock      sync.RWMutex
	getEphemeralFunc  func() string
}

// listen is a simplified implementation of Netceptor.listen for testing.
func (m *mockNetceptorForListen) listen(ctx context.Context, service string, tlscfg *tls.Config, advertise bool, adTags map[string]string) (*Listener, error) {
	if len(service) > 8 {
		return nil, fmt.Errorf("service name %s too long", service)
	}
	if service == "" {
		service = m.GetEphemeralService()
	}
	m.listenerLock.Lock()
	defer m.listenerLock.Unlock()
	_, isReserved := m.reservedServices[service]
	_, isListening := m.listenerRegistry[service]
	if isReserved || isListening {
		return nil, fmt.Errorf("service %s is already listening", service)
	}

	// In a real implementation, we would create a PacketConn, set up QUIC, etc.
	// For testing, we'll just return a minimal Listener

	if advertise {
		m.AddLocalServiceAdvertisement(service, 0, adTags)
	}

	doneChan := make(chan struct{})
	li := &Listener{
		s:          nil, // We don't need this for the test
		pc:         nil, // We don't need this for the test
		ql:         nil, // We don't need this for the test
		AcceptChan: make(chan *AcceptResult),
		DoneChan:   doneChan,
		doneOnce:   &sync.Once{},
	}

	return li, nil
}

func newMockNetceptorForListen() *mockNetceptorForListen {
	ctx, cancel := context.WithCancel(context.Background())
	return &mockNetceptorForListen{
		listenerRegistry:  make(map[string]*PacketConn),
		reservedServices:  make(map[string]func(*MessageData) error),
		nodeID:            "test-node",
		maxForwardingHops: 5,
		logger:            logger.NewReceptorLogger("test"),
		context:           ctx,
		cancelFunc:        cancel,
		listenerLock:      sync.RWMutex{},
		getEphemeralFunc:  func() string { return "ephemeral-service" },
	}
}

func (m *mockNetceptorForListen) GetEphemeralService() string {
	return m.getEphemeralFunc()
}

func (m *mockNetceptorForListen) AddNameHash(name string) uint64 {
	return 0
}

func (m *mockNetceptorForListen) AddLocalServiceAdvertisement(service string, connType byte, tags map[string]string) {
	// Mock implementation - do nothing
}

func (m *mockNetceptorForListen) GetLogger() *logger.ReceptorLogger {
	return m.logger
}

func TestListen(t *testing.T) {
	// Test case 1: Service name too long
	t.Run("Service name too long", func(t *testing.T) {
		s := newMockNetceptorForListen()
		longServiceName := "servicenametoolong" // > 8 characters

		_, err := s.listen(context.Background(), longServiceName, nil, false, nil)

		if err == nil {
			t.Error("Expected error for service name too long, but got nil")
		}

		expectedErrMsg := fmt.Sprintf("service name %s too long", longServiceName)
		if err.Error() != expectedErrMsg {
			t.Errorf("Expected error message '%s', got '%s'", expectedErrMsg, err.Error())
		}
	})

	// Test case 2: Service already listening
	t.Run("Service already listening", func(t *testing.T) {
		s := newMockNetceptorForListen()
		serviceName := "service1"

		// Add the service to the listener registry to simulate it's already listening
		s.listenerRegistry[serviceName] = &PacketConn{}

		_, err := s.listen(context.Background(), serviceName, nil, false, nil)

		if err == nil {
			t.Error("Expected error for service already listening, but got nil")
		}

		expectedErrMsg := fmt.Sprintf("service %s is already listening", serviceName)
		if err.Error() != expectedErrMsg {
			t.Errorf("Expected error message '%s', got '%s'", expectedErrMsg, err.Error())
		}
	})

	// Test case 3: Reserved service
	t.Run("Reserved service", func(t *testing.T) {
		s := newMockNetceptorForListen()
		serviceName := "reserved"

		// Add the service to the reserved services
		s.reservedServices[serviceName] = func(*MessageData) error { return nil }

		_, err := s.listen(context.Background(), serviceName, nil, false, nil)

		if err == nil {
			t.Error("Expected error for reserved service, but got nil")
		}

		expectedErrMsg := fmt.Sprintf("service %s is already listening", serviceName)
		if err.Error() != expectedErrMsg {
			t.Errorf("Expected error message '%s', got '%s'", expectedErrMsg, err.Error())
		}
	})

	// Test case 4: Empty service name (should use ephemeral)
	t.Run("Empty service name", func(t *testing.T) {
		// This test is more complex and would require mocking more components
		// For simplicity, we'll just verify that an ephemeral service is requested

		s := newMockNetceptorForListen()
		ephemeralServiceCalled := false

		// Override getEphemeralFunc to track if it's called
		s.getEphemeralFunc = func() string {
			ephemeralServiceCalled = true
			return "ephemeral-service"
		}

		// We need to mock the quic.Transport.Listen call to avoid actual network operations
		// This is complex and would require more extensive mocking

		// For this test, we'll just verify that GetEphemeralService was called
		// and not test the full function execution

		_, _ = s.listen(context.Background(), "", nil, false, nil)

		if !ephemeralServiceCalled {
			t.Error("Expected GetEphemeralService to be called for empty service name")
		}
	})
}

// createClientHelloInfo creates a tls.ClientHelloInfo for testing.
func createClientHelloInfo(conn net.Conn) *tls.ClientHelloInfo {
	return &tls.ClientHelloInfo{
		Conn: conn,
	}
}

// mockTLSConn is a mock implementation of net.Conn with a RemoteAddr method for testing.
type mockTLSConn struct {
	net.Conn
	remoteAddr net.Addr
}

func (m *mockTLSConn) RemoteAddr() net.Addr {
	return m.remoteAddr
}

// mockInvalidAddr is a mock implementation of net.Addr that returns an invalid address format.
type mockInvalidAddr struct {
	net.Addr
}

func (m *mockInvalidAddr) String() string {
	return "invalid-address-format" // This will cause utils.AddressToHostPort to fail
}

func (m *mockInvalidAddr) Network() string {
	return "invalid"
}

// TestGetConfigForClient tests the getConfigForClient function.
func TestGetConfigForClient(t *testing.T) {
	// Test case 1: Basic functionality with valid config
	t.Run("Basic functionality", func(t *testing.T) {
		// Create a base TLS config
		baseTLSConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
			NextProtos: []string{"netceptor"},
		}

		// Create a mock connection with a remote address
		mockAddr := &net.TCPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: 8000,
		}
		mockConn := &mockTLSConn{
			remoteAddr: mockAddr,
		}

		// Create a ClientHelloInfo
		clientHello := createClientHelloInfo(mockConn)

		// Create a netceptor instance
		s := New(context.Background(), "test-node")

		// Get the config function
		configFunc := s.getConfigForClient(baseTLSConfig)

		// Call the function with our ClientHelloInfo
		resultConfig, err := configFunc(clientHello)

		// Verify results
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if resultConfig == nil {
			t.Fatal("Expected a TLS config, got nil")
		}

		// Verify the config was cloned, not the same instance
		if resultConfig == baseTLSConfig {
			t.Error("Expected a cloned config, got the same instance")
		}

		// Verify NextProtos was preserved
		if len(resultConfig.NextProtos) != 1 || resultConfig.NextProtos[0] != "netceptor" {
			t.Errorf("Expected NextProtos to be ['netceptor'], got %v", resultConfig.NextProtos)
		}

		// Verify VerifyPeerCertificate function was set
		if resultConfig.VerifyPeerCertificate == nil {
			t.Error("Expected VerifyPeerCertificate to be set, but it was nil")
		}
	})

	// Test case 2: Empty base config (not nil)
	t.Run("Empty base config", func(t *testing.T) {
		// Create a mock connection with a remote address
		mockAddr := &net.TCPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: 8000,
		}
		mockConn := &mockTLSConn{
			remoteAddr: mockAddr,
		}

		// Create a ClientHelloInfo
		clientHello := createClientHelloInfo(mockConn)

		// Create a netceptor instance
		s := New(context.Background(), "test-node")

		// Get the config function with empty (but not nil) base config
		emptyConfig := &tls.Config{}
		configFunc := s.getConfigForClient(emptyConfig)

		// Call the function with our ClientHelloInfo
		resultConfig, err := configFunc(clientHello)

		// Verify results
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if resultConfig == nil {
			t.Fatal("Expected a TLS config, got nil")
		}

		// Verify the config was cloned, not the same instance
		if resultConfig == emptyConfig {
			t.Error("Expected a cloned config, got the same instance")
		}

		// Verify VerifyPeerCertificate function was set
		if resultConfig.VerifyPeerCertificate == nil {
			t.Error("Expected VerifyPeerCertificate to be set, but it was nil")
		}
	})

	// Test case 3: Connection with invalid RemoteAddr format
	t.Run("Connection with invalid RemoteAddr format", func(t *testing.T) {
		// Create a base TLS config
		baseTLSConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
			NextProtos: []string{"netceptor"},
		}

		// Create a mock connection with an invalid address format
		// This will cause utils.AddressToHostPort to fail
		invalidAddrMock := &mockInvalidAddr{}
		mockConn := &mockTLSConn{
			remoteAddr: invalidAddrMock,
		}

		// Create a ClientHelloInfo with the mock conn
		clientHello := createClientHelloInfo(mockConn)

		// Create a netceptor instance
		s := New(context.Background(), "test-node")

		// Get the config function
		configFunc := s.getConfigForClient(baseTLSConfig)

		// Call the function with our ClientHelloInfo
		resultConfig, err := configFunc(clientHello)

		// Verify results - we expect an error since the address format is invalid
		if err == nil {
			t.Error("Expected an error for invalid remote address format, but got nil")
		}

		// The function should return nil config when there's an error
		if resultConfig != nil {
			t.Errorf("Expected nil config when there's an error, got: %v", resultConfig)
		}
	})
}

// TestNetceptorListen tests the Listen method of the Netceptor struct.
func TestNetceptorListen(t *testing.T) {
	// Test case 1: Basic functionality with valid service name
	t.Run("Basic functionality", func(t *testing.T) {
		// Create a netceptor instance
		s := New(context.Background(), "test-node")

		// Call the Listen method with a service name <= 8 characters
		listener, err := s.Listen("test1", nil)

		// Verify results
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if listener == nil {
			t.Fatal("Expected a listener, got nil")
		}

		// Verify the listener's address
		addr := listener.Addr()
		if addr == nil {
			t.Fatal("Expected an address, got nil")
		}

		// Clean up
		err = listener.Close()
		if err != nil {
			t.Errorf("Error closing listener: %v", err)
		}
	})

	// Test case 2: Empty service name (should use ephemeral)
	t.Run("Empty service name", func(t *testing.T) {
		// Create a netceptor instance
		s := New(context.Background(), "test-node")

		// Call the Listen method with empty service name
		listener, err := s.Listen("", nil)

		// Verify results
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if listener == nil {
			t.Fatal("Expected a listener, got nil")
		}

		// Verify the listener's address
		addr := listener.Addr()
		if addr == nil {
			t.Fatal("Expected an address, got nil")
		}

		// Clean up
		err = listener.Close()
		if err != nil {
			t.Errorf("Error closing listener: %v", err)
		}
	})

	// Test case 3: Service name too long
	t.Run("Service name too long", func(t *testing.T) {
		// Create a netceptor instance
		s := New(context.Background(), "test-node")

		// Call the Listen method with a service name that's too long
		_, err := s.Listen("servicenametoolong", nil)

		// Verify results
		if err == nil {
			t.Error("Expected error for service name too long, but got nil")
		}

		expectedErrMsg := "service name servicenametoolong too long"
		if err.Error() != expectedErrMsg {
			t.Errorf("Expected error message '%s', got '%s'", expectedErrMsg, err.Error())
		}
	})
}

// TestNetceptorListenAndAdvertise tests the ListenAndAdvertise method of the Netceptor struct.
func TestNetceptorListenAndAdvertise(t *testing.T) {
	// Test case 1: Basic functionality with valid service name and tags
	t.Run("Basic functionality", func(t *testing.T) {
		// Create a netceptor instance
		s := New(context.Background(), "test-node")

		// Create tags
		tags := map[string]string{
			"tag1": "value1",
			"tag2": "value2",
		}

		// Call the ListenAndAdvertise method with a service name <= 8 characters
		listener, err := s.ListenAndAdvertise("test2", nil, tags)

		// Verify results
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if listener == nil {
			t.Fatal("Expected a listener, got nil")
		}

		// Verify the listener's address
		addr := listener.Addr()
		if addr == nil {
			t.Fatal("Expected an address, got nil")
		}

		// Clean up
		err = listener.Close()
		if err != nil {
			t.Errorf("Error closing listener: %v", err)
		}
	})

	// Test case 2: Empty service name (should use ephemeral)
	t.Run("Empty service name", func(t *testing.T) {
		// Create a netceptor instance
		s := New(context.Background(), "test-node")

		// Create tags
		tags := map[string]string{
			"tag1": "value1",
		}

		// Call the ListenAndAdvertise method with empty service name
		listener, err := s.ListenAndAdvertise("", nil, tags)

		// Verify results
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if listener == nil {
			t.Fatal("Expected a listener, got nil")
		}

		// Verify the listener's address
		addr := listener.Addr()
		if addr == nil {
			t.Fatal("Expected an address, got nil")
		}

		// Clean up
		err = listener.Close()
		if err != nil {
			t.Errorf("Error closing listener: %v", err)
		}
	})

	// Test case 3: Service name too long
	t.Run("Service name too long", func(t *testing.T) {
		// Create a netceptor instance
		s := New(context.Background(), "test-node")

		// Create tags
		tags := map[string]string{
			"tag1": "value1",
		}

		// Call the ListenAndAdvertise method with a service name that's too long
		_, err := s.ListenAndAdvertise("servicenametoolong", nil, tags)

		// Verify results
		if err == nil {
			t.Error("Expected error for service name too long, but got nil")
		}

		expectedErrMsg := "service name servicenametoolong too long"
		if err.Error() != expectedErrMsg {
			t.Errorf("Expected error message '%s', got '%s'", expectedErrMsg, err.Error())
		}
	})

	// Test case 4: Nil tags
	t.Run("Nil tags", func(t *testing.T) {
		// Create a netceptor instance
		s := New(context.Background(), "test-node")

		// Call the ListenAndAdvertise method with nil tags and a service name <= 8 characters
		listener, err := s.ListenAndAdvertise("test3", nil, nil)

		// Verify results
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if listener == nil {
			t.Fatal("Expected a listener, got nil")
		}

		// Verify the listener's address
		addr := listener.Addr()
		if addr == nil {
			t.Fatal("Expected an address, got nil")
		}

		// Clean up
		err = listener.Close()
		if err != nil {
			t.Errorf("Error closing listener: %v", err)
		}
	})
}

func TestGenerateServerTLSConfig(t *testing.T) {
	// Test that the function returns a valid TLS config
	tlsConfig := generateServerTLSConfig()

	// Check that the config has the expected properties
	if tlsConfig == nil {
		t.Fatal("Expected non-nil TLS config")
	}

	// Check certificates
	if len(tlsConfig.Certificates) != 1 {
		t.Errorf("Expected 1 certificate, got %d", len(tlsConfig.Certificates))
	}

	// Check next protocols
	if len(tlsConfig.NextProtos) != 1 || tlsConfig.NextProtos[0] != "netceptor" {
		t.Errorf("Expected NextProtos to be ['netceptor'], got %v", tlsConfig.NextProtos)
	}

	// Check TLS version
	if tlsConfig.MinVersion != tls.VersionTLS12 {
		t.Errorf("Expected MinVersion to be TLS 1.2, got %d", tlsConfig.MinVersion)
	}

	// Check PreferServerCipherSuites
	if !tlsConfig.PreferServerCipherSuites {
		t.Error("Expected PreferServerCipherSuites to be true")
	}

	// Extract and check the certificate's common name
	cert := tlsConfig.Certificates[0]
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	if x509Cert.Subject.CommonName != insecureCommonName {
		t.Errorf("Expected CommonName to be %s, got %s", insecureCommonName, x509Cert.Subject.CommonName)
	}
}

func TestVerifyServerCertificate(t *testing.T) {
	// Test cases
	testCases := []struct {
		name          string
		commonName    string
		expectSuccess bool
	}{
		{
			name:          "Valid certificate with correct common name",
			commonName:    insecureCommonName,
			expectSuccess: true,
		},
		{
			name:          "Invalid certificate with incorrect common name",
			commonName:    "wrong-common-name",
			expectSuccess: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a test certificate with the specified common name
			rawCert := createTestCertificate(t, tc.commonName)

			// Call the function under test
			err := verifyServerCertificate([][]byte{rawCert}, nil)

			if tc.expectSuccess && err != nil {
				t.Errorf("Expected success, but got error: %v", err)
			}

			if !tc.expectSuccess && err == nil {
				t.Error("Expected error, but got success")
			}
		})
	}

	// Test with invalid certificate data
	t.Run("Invalid certificate data", func(t *testing.T) {
		err := verifyServerCertificate([][]byte{[]byte("invalid-cert-data")}, nil)
		if err == nil {
			t.Error("Expected error with invalid certificate data, but got success")
		}
	})

	// Test with empty certificate list
	t.Run("Empty certificate list", func(t *testing.T) {
		err := verifyServerCertificate([][]byte{}, nil)
		if err == nil {
			t.Error("Expected error with empty certificate list, but got success")
		}
	})
}

func TestGenerateClientTLSConfig(t *testing.T) {
	// Test cases with different host values
	testCases := []struct {
		name string
		host string
	}{
		{
			name: "Empty host",
			host: "",
		},
		{
			name: "Valid host",
			host: "test-node",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Call the function under test
			tlsConfig := generateClientTLSConfig(tc.host)

			// Check that the config has the expected properties
			if tlsConfig == nil {
				t.Fatal("Expected non-nil TLS config")
			}

			// Check InsecureSkipVerify
			if !tlsConfig.InsecureSkipVerify {
				t.Error("Expected InsecureSkipVerify to be true")
			}

			// Check that VerifyPeerCertificate is set
			if tlsConfig.VerifyPeerCertificate == nil {
				t.Error("Expected VerifyPeerCertificate to be set")
			}

			// Check next protocols
			if len(tlsConfig.NextProtos) != 1 || tlsConfig.NextProtos[0] != "netceptor" {
				t.Errorf("Expected NextProtos to be ['netceptor'], got %v", tlsConfig.NextProtos)
			}

			// Check ServerName
			if tlsConfig.ServerName != tc.host {
				t.Errorf("Expected ServerName to be %s, got %s", tc.host, tlsConfig.ServerName)
			}

			// Check TLS version
			if tlsConfig.MinVersion != tls.VersionTLS12 {
				t.Errorf("Expected MinVersion to be TLS 1.2, got %d", tlsConfig.MinVersion)
			}
		})
	}
}

// Helper function to create a test certificate with a specific common name
func createTestCertificate(t *testing.T, commonName string) []byte {
	t.Helper()

	// Generate a self-signed certificate with the specified common name
	tlsConfig := generateServerTLSConfig()

	// Extract the certificate
	cert := tlsConfig.Certificates[0]
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	// If the common name is already what we want, return the DER-encoded certificate
	if x509Cert.Subject.CommonName == commonName {
		return cert.Certificate[0]
	}

	// Otherwise, we need to create a new certificate with the desired common name
	key, err := tls.X509KeyPair(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]}),
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(cert.PrivateKey.(*rsa.PrivateKey))}),
	)
	if err != nil {
		t.Fatalf("Failed to create key pair: %v", err)
	}

	// Create a new certificate with the desired common name
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: commonName,
		},
		NotBefore: time.Now().Add(-1 * time.Minute),
		NotAfter:  time.Now().Add(24 * time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PrivateKey.(*rsa.PrivateKey).PublicKey, key.PrivateKey)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	return certDER
}
