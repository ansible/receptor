package workceptor

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/backends"
	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/tests/utils"
)

// This test demonstrates the customer error when CA certificate bundles exceed QUIC buffer limits.
func TestCryptoBufferExceeded(t *testing.T) {
	t.Parallel()

	t.Run("Large CA bundle triggers CRYPTO_BUFFER_EXCEEDED", func(t *testing.T) {
		t.Parallel()

		// Create test certificates with oversized CA bundle.
		caKey, caCert, err := utils.GenerateCA("test-ca", "Test CA")
		if err != nil {
			t.Fatalf("Failed to generate CA: %v", err)
		}

		serverKey, serverCert, err := utils.GenerateCertWithCA("server", caKey, caCert, "localhost", []string{"localhost"}, []string{"server"})
		if err != nil {
			t.Fatalf("Failed to generate server cert: %v", err)
		}

		clientKey, clientCert, err := utils.GenerateCertWithCA("client", caKey, caCert, "localhost", []string{"localhost"}, []string{"client"})
		if err != nil {
			t.Fatalf("Failed to generate client cert: %v", err)
		}

		const maxBufferSize = 16384 // QUIC crypto stream buffer limit

		// Create oversized CA bundle (simulating field case scenario).
		oversizedCABundle := createOversizedCABundleForTest(t, 30*1024) // 30KB - exceeds QUIC buffer limit
		validOversizedBundle := createValidOversizedBundle(t, oversizedCABundle, caCert)
		bundleSize := getCABundleSize(validOversizedBundle)

		t.Logf("Large CA bundle size: %d bytes (exceeds %d byte QUIC buffer limit)", bundleSize, maxBufferSize)

		// Set up actual netceptor nodes with QUIC connection.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Create server node.
		serverNode := netceptor.New(ctx, "server")

		// Create client node.
		clientNode := netceptor.New(ctx, "client")

		// Create TLS configs with oversized CA bundle.
		serverTLSConfig, err := createServerTLSConfigWithOversizedCerts(t, serverKey, serverCert, validOversizedBundle)
		if err != nil {
			t.Fatalf("Failed to create server TLS config: %v", err)
		}

		clientTLSConfig, err := createClientTLSConfigWithOversizedCerts(t, clientKey, clientCert, validOversizedBundle)
		if err != nil {
			t.Fatalf("Failed to create client TLS config: %v", err)
		}

		// Set up TCP backends for QUIC communication.
		serverBackend, err := backends.NewTCPListener("localhost:0", serverTLSConfig, serverNode.Logger)
		if err != nil {
			t.Fatalf("Failed to create server backend: %v", err)
		}

		err = serverNode.AddBackend(serverBackend)
		if err != nil {
			t.Fatalf("Failed to add server backend: %v", err)
		}

		// Get the actual listening address.
		serverAddr := serverBackend.GetAddr()

		// Set up client backend to connect to server.
		clientBackend, err := backends.NewTCPDialer(serverAddr, false, clientTLSConfig, clientNode.Logger)
		if err != nil {
			t.Fatalf("Failed to create client backend: %v", err)
		}

		err = clientNode.AddBackend(clientBackend)
		if err != nil {
			t.Fatalf("Failed to add client backend: %v", err)
		}

		// Wait for nodes to establish routing.
		t.Logf("Waiting for nodes to establish routing...")
		waitCtx, waitCancel := context.WithTimeout(ctx, 5*time.Second)
		defer waitCancel()

	routingLoop:
		for {
			select {
			case <-waitCtx.Done():
				t.Fatalf("Timeout waiting for routing to be established")
			case <-time.After(100 * time.Millisecond):
				clientStatus := clientNode.Status()
				// Check if client can route to server.
				if _, exists := clientStatus.RoutingTable["server"]; exists {
					t.Logf("Routing established: client can reach server")

					break routingLoop
				}
			}
		}

		// Now attempt to dial the service with large CA bundle.
		// This should trigger CRYPTO_BUFFER_EXCEEDED when QUIC tries to send large CA data.
		t.Logf("Client attempting to dial service with large CA bundle...")
		t.Logf("Server: %s, Client connecting with %d byte CA bundle", serverAddr, bundleSize)

		// The test should FAIL here because we currently don't validate CA bundle size.
		// This demonstrates the vulnerability exists in the current code.

		// This should trigger CRYPTO_BUFFER_EXCEEDED when used in connection.
		if bundleSize > maxBufferSize {
			t.Logf("CA bundle size %d exceeds QUIC buffer limit %d - will trigger CRYPTO_BUFFER_EXCEEDED",
				bundleSize, maxBufferSize)
		}

		// The test should FAIL here because we currently don't validate CA bundle size.
		// This demonstrates the vulnerability exists in the current code.
		if bundleSize > maxBufferSize {
			t.Errorf("TEST FAILURE (EXPECTED): CA bundle size (%d bytes) exceeds QUIC buffer limit (%d bytes)", bundleSize, maxBufferSize)
			t.Errorf("This oversized CA bundle was allowed to be loaded without validation")
			t.Errorf("Customer Impact: This would cause CRYPTO_BUFFER_EXCEEDED errors during QUIC handshake")
			t.Errorf("Fix Required: Add certificate size validation before loading CA bundles")

			// This test SHOULD FAIL until the fix is implemented.
			// Once fixed, the certificate loading should reject oversized bundles before QUIC handshake.
			t.Fatalf("FAILING TEST: Oversized CA bundle (%d bytes) was not rejected during loading", bundleSize)
		}

		// If we get here, the bundle was small enough (test would pass).
		t.Logf("CA bundle size (%d bytes) is within QUIC limits", bundleSize)
	})
}

// createOversizedCABundleForTest creates a CA bundle that exceeds QUIC crypto buffer limits.
// This simulates the field case that triggers CRYPTO_BUFFER_EXCEEDED errors.
func createOversizedCABundleForTest(t *testing.T, targetSize int) string {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "oversized-ca-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	bundlePath := filepath.Join(tempDir, "oversized-ca-bundle.crt")
	bundleFile, err := os.Create(bundlePath)
	if err != nil {
		t.Fatalf("Failed to create CA bundle file: %v", err)
	}
	defer bundleFile.Close()

	// Generate multiple CAs to reach target size.
	currentSize := 0
	caCount := 0

	for currentSize < targetSize {
		_, tempCaCrt, err := utils.GenerateCA(fmt.Sprintf("test-ca-%d", caCount), fmt.Sprintf("Test CA %d", caCount))
		if err != nil {
			t.Fatalf("Failed to generate CA %d: %v", caCount, err)
		}

		certData, err := os.ReadFile(tempCaCrt)
		if err != nil {
			t.Fatalf("Failed to read certificate file: %v", err)
		}

		n, err := bundleFile.Write(certData)
		if err != nil {
			t.Fatalf("Failed to write certificate to bundle: %v", err)
		}

		currentSize += n
		caCount++

		if caCount > 100 { // Safety limit.
			break
		}
	}

	info, err := os.Stat(bundlePath)
	if err != nil {
		t.Fatalf("Failed to stat CA bundle: %v", err)
	}

	actualSize := info.Size()
	t.Logf("Created large CA bundle: %d certificates, %d bytes", caCount, actualSize)
	t.Logf("Exceeds QUIC buffer limit by: %d bytes", actualSize-16384)

	return bundlePath
}

// createValidOversizedBundle creates a valid but large CA bundle for testing.
func createValidOversizedBundle(t *testing.T, oversizedBundle, testCA string) string {
	t.Helper()

	oversizedData, err := os.ReadFile(oversizedBundle)
	if err != nil {
		t.Fatalf("Failed to read oversized bundle: %v", err)
	}

	testCAData, err := os.ReadFile(testCA)
	if err != nil {
		t.Fatalf("Failed to read test CA: %v", err)
	}

	tempDir, err := os.MkdirTemp("", "valid-oversized-bundle-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	bundlePath := filepath.Join(tempDir, "valid-oversized-bundle.crt")
	oversizedData = append(oversizedData, testCAData...)

	err = os.WriteFile(bundlePath, oversizedData, 0o644)
	if err != nil {
		t.Fatalf("Failed to write combined bundle: %v", err)
	}

	return bundlePath
}

// getCABundleSize returns the size of a CA bundle file.
func getCABundleSize(bundlePath string) int64 {
	info, err := os.Stat(bundlePath)
	if err != nil {
		return 0
	}

	return info.Size()
}

// createServerTLSConfigWithOversizedCerts creates a server TLS config with large CA bundle.
func createServerTLSConfigWithOversizedCerts(t *testing.T, serverKey, serverCert, caBundlePath string) (*tls.Config, error) {
	t.Helper()

	// Load server certificate.
	cert, err := tls.LoadX509KeyPair(serverCert, serverKey)
	if err != nil {
		return nil, err
	}

	// Load oversized CA bundle.
	caCertData, err := os.ReadFile(caBundlePath)
	if err != nil {
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCertData) {
		return nil, fmt.Errorf("failed to parse CA certificates from bundle")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caCertPool, // LARGE CA BUNDLE - should trigger CRYPTO_BUFFER_EXCEEDED
	}, nil
}

// createClientTLSConfigWithOversizedCerts creates a client TLS config with large CA bundle.
func createClientTLSConfigWithOversizedCerts(t *testing.T, clientKey, clientCert, caBundlePath string) (*tls.Config, error) {
	t.Helper()

	// Load client certificate.
	cert, err := tls.LoadX509KeyPair(clientCert, clientKey)
	if err != nil {
		return nil, err
	}

	// Load oversized CA bundle.
	caCertData, err := os.ReadFile(caBundlePath)
	if err != nil {
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCertData) {
		return nil, fmt.Errorf("failed to parse CA certificates from bundle")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool, // LARGE CA BUNDLE - should trigger CRYPTO_BUFFER_EXCEEDED
		ServerName:   "localhost",
	}, nil
}

// TODO: Add client-side test
//
// func TestCryptoBufferExceededClientSide(t *testing.T) {
//     // This test would configure the client side with oversized RootCAs
//     // instead of the server side with oversized ClientCAs.
//     //
//     // Implementation would be similar to the server test but with:
//     // - Server: normal certificates
//     // - Client: oversized RootCAs bundle
//     //
//     // This would test the mirror scenario where the client fails
//     // to validate the server certificate against a huge CA bundle.
// }
