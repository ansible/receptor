package mesh

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/backends"
	"github.com/ansible/receptor/pkg/certificates"
	"github.com/ansible/receptor/pkg/netceptor"
)

// setupTest creates and configures Netceptor instances with backends for testing
// If filesystemCerts is provided, uses those certificates; otherwise generates new ones
type filesystemCerts struct {
	serverCert string
	serverKey  string
	clientCAs  string
	rootCAs    string
}

func setupTest(t *testing.T, serverName, clientName string, fsCerts *filesystemCerts, dnsNameCount, nodeIDCount, caCount int) (*netceptor.Netceptor, *netceptor.Netceptor, *tls.Config, *tls.Config, func()) {
	// Create Netceptor instances with fresh background contexts
	serverNode := netceptor.New(context.Background(), serverName)
	clientNode := netceptor.New(context.Background(), clientName)

	// Set up TCP backends to establish network connection
	b1, err := backends.NewTCPListener("localhost:0", nil, serverNode.Logger)
	if err != nil {
		serverNode.Shutdown()
		clientNode.Shutdown()
		t.Fatalf("Error creating TCP listener: %v", err)
	}
	err = serverNode.AddBackend(b1)
	if err != nil {
		serverNode.Shutdown()
		clientNode.Shutdown()
		t.Fatalf("Error adding backend to server: %v", err)
	}

	// Get the actual port that was assigned
	tcpAddr := b1.GetAddr()
	t.Logf("Server listening on: %s", tcpAddr)

	// Set up TCP dialer on client to connect to the listener
	b2, err := backends.NewTCPDialer(tcpAddr, false, nil, clientNode.Logger)
	if err != nil {
		serverNode.Shutdown()
		clientNode.Shutdown()
		t.Fatalf("Error creating TCP dialer: %v", err)
	}
	err = clientNode.AddBackend(b2)
	if err != nil {
		serverNode.Shutdown()
		clientNode.Shutdown()
		t.Fatalf("Error adding backend to client: %v", err)
	}

	// Give time for backends to establish connection and routing to stabilize
	time.Sleep(3 * time.Second)

	var serverCertFile, serverKeyFile, clientCAsFile, rootCAsFile string

	if fsCerts != nil {
		// Use filesystem certificates
		serverCertFile = fsCerts.serverCert
		serverKeyFile = fsCerts.serverKey
		clientCAsFile = fsCerts.clientCAs
		rootCAsFile = fsCerts.rootCAs
	} else {
		// Generate certificates
		tempDir := t.TempDir()
		var err error
		serverCertFile, serverKeyFile, clientCAsFile, err = createReceptorCertificateAndCA(tempDir, dnsNameCount, nodeIDCount, caCount, serverName, clientName)
		if err != nil {
			serverNode.Shutdown()
			clientNode.Shutdown()
			t.Fatalf("Error creating cert and CA: %v", err)
		}
		rootCAsFile = clientCAsFile // Use same CA bundle for RootCAs when generating
	}

	// Create TLS server configuration using Receptor's PrepareTLSServerConfig
	serverConfig := netceptor.TLSServerConfig{
		Name:                   serverName,
		Cert:                   serverCertFile,
		Key:                    serverKeyFile,
		RequireClientCert:      true,
		ClientCAs:              clientCAsFile,
		SkipReceptorNamesCheck: true,
		MinTLS13:               false,
	}

	// Create client TLS config
	clientConfig := netceptor.TLSClientConfig{
		Name:                   clientName,
		Cert:                   serverCertFile, // Reuse server cert for client
		Key:                    serverKeyFile,  // Reuse server key for client
		RootCAs:                rootCAsFile,
		InsecureSkipVerify:     true,
		SkipReceptorNamesCheck: true,
		MinTLS13:               false,
	}

	serverTLSConfig, err := serverConfig.PrepareTLSServerConfig(serverNode)
	if err != nil {
		serverNode.Shutdown()
		clientNode.Shutdown()
		t.Fatalf("Failed to prepare TLS server config: %v", err)
	}

	clientTLSConfig, _, err := clientConfig.PrepareTLSClientConfig(clientNode)
	if err != nil {
		serverNode.Shutdown()
		clientNode.Shutdown()
		t.Fatalf("Failed to prepare TLS client config: %v", err)
	}

	// Return cleanup function
	cleanup := func() {
		serverNode.Shutdown()
		clientNode.Shutdown()
	}

	return serverNode, clientNode, serverTLSConfig, clientTLSConfig, cleanup
}

// TestQuicCryptoBufferExceeded tests CRYPTO_BUFFER_EXCEEDED scenarios with large TLS configurations
// This test demonstrates various customer errors when TLS configurations exceed QUIC buffer limits.
func TestQuicCryptoBufferExceeded(t *testing.T) {

	tests := []struct {
		name         string
		dnsNameCount int
		nodeIDCount  int
		caCount      int
		serverName   string
		clientName   string
	}{
		{
			name:         "large server certificate with small CA",
			dnsNameCount: 380,
			nodeIDCount:  190,
			caCount:      1,
			serverName:   "node1",
			clientName:   "node2",
		},
		{
			name:         "small server certificate with large CA",
			dnsNameCount: 1,
			nodeIDCount:  1,
			caCount:      5, // Test with slightly larger CAs to find exact threshold
			serverName:   "node1",
			clientName:   "node2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const serviceName = "test"

			// Set up test infrastructure using generated certificates
			serverNode, clientNode, serverTLSConfig, clientTLSConfig, cleanup := setupTest(t, tt.serverName, tt.clientName, nil, tt.dnsNameCount, tt.nodeIDCount, tt.caCount)
			defer cleanup()

			// Log combined TLS config sizes for analysis (what actually gets sent during handshake)
			logCombinedTLSConfigSizes(t, "TLS Handshake", serverTLSConfig, clientTLSConfig)

			// ListenAndAdvertise with TLS config
			listener, err := serverNode.ListenAndAdvertise(serviceName, serverTLSConfig, map[string]string{
				"type": "crypto-buffer-test",
			})
			if err != nil {
				t.Fatalf("ListenAndAdvertise failed: %v", err)
			}
			defer listener.Close()

			t.Logf("ListenAndAdvertise succeeded")

			// Give time for service advertisement to propagate through the mesh
			time.Sleep(2 * time.Second)

			// Now attempt to dial the service with TLS config
			// This should trigger CRYPTO_BUFFER_EXCEEDED when QUIC tries to send large cert data
			t.Logf("Client attempting to dial service with TLS config...")
			conn, err := clientNode.Dial(tt.serverName, serviceName, clientTLSConfig)
			if err != nil {
				// Fail the test to show the CRYPTO_BUFFER_EXCEEDED error
				t.Fatalf("CRYPTO_BUFFER_EXCEEDED error: %v", err)
			}

			if conn != nil {
				defer conn.Close()
				t.Logf("Connection succeeded - attempting to write data to trigger full TLS handshake")

				// Try to write data through the connection - this should trigger CRYPTO_BUFFER_EXCEEDED
				_, writeErr := conn.Write([]byte("test data to trigger TLS handshake"))
				if writeErr != nil {
					// Fail the test to show the CRYPTO_BUFFER_EXCEEDED error
					t.Fatalf("CRYPTO_BUFFER_EXCEEDED error during write: %v", writeErr)
				}

				// If we reach here, fail because CRYPTO_BUFFER_EXCEEDED should have occurred
				t.Fatalf("Unexpected success - CRYPTO_BUFFER_EXCEEDED should have occurred with large TLS config")
			}
		})
	}
}

// TestQuicCryptoBufferExceededFilesystem tests CRYPTO_BUFFER_EXCEEDED with real filesystem certificates
// This test uses certificates from the filesystem via environment variables, skips if not set
func TestQuicCryptoBufferExceededFilesystem(t *testing.T) {
	// Check for required environment variables
	serverCertFile := os.Getenv("RECEPTOR_TEST_SERVER_CERT")
	serverKeyFile := os.Getenv("RECEPTOR_TEST_SERVER_KEY")
	clientCAsFile := os.Getenv("RECEPTOR_TEST_CLIENT_CAS")
	rootCAsFile := os.Getenv("RECEPTOR_TEST_ROOT_CAS")

	if serverCertFile == "" || serverKeyFile == "" || clientCAsFile == "" || rootCAsFile == "" {
		t.Skip("Skipping filesystem test - required environment variables not set (RECEPTOR_TEST_SERVER_CERT, RECEPTOR_TEST_SERVER_KEY, RECEPTOR_TEST_CLIENT_CAS, RECEPTOR_TEST_ROOT_CAS)")
	}

	t.Logf("Using filesystem certificate files:")
	t.Logf("  Server cert: %s", serverCertFile)
	t.Logf("  Server key: %s", serverKeyFile)
	t.Logf("  Client CAs: %s", clientCAsFile)
	t.Logf("  Root CAs: %s", rootCAsFile)

	// Check if files exist
	if _, err := os.Stat(serverCertFile); os.IsNotExist(err) {
		t.Skipf("Skipping filesystem test - server cert file not found: %s", serverCertFile)
	}
	if _, err := os.Stat(serverKeyFile); os.IsNotExist(err) {
		t.Skipf("Skipping filesystem test - server key file not found: %s", serverKeyFile)
	}
	if _, err := os.Stat(clientCAsFile); os.IsNotExist(err) {
		t.Skipf("Skipping filesystem test - client CAs file not found: %s", clientCAsFile)
	}
	if _, err := os.Stat(rootCAsFile); os.IsNotExist(err) {
		t.Skipf("Skipping filesystem test - root CAs file not found: %s", rootCAsFile)
	}

	const serviceName = "testfs"
	serverName := "fsnode1"
	clientName := "fsnode2"

	// Set up test infrastructure using filesystem certificates
	fsCerts := &filesystemCerts{
		serverCert: serverCertFile,
		serverKey:  serverKeyFile,
		clientCAs:  clientCAsFile,
		rootCAs:    rootCAsFile,
	}

	serverNode, clientNode, serverTLSConfig, clientTLSConfig, cleanup := setupTest(t, serverName, clientName, fsCerts, 0, 0, 0)
	defer cleanup()

	// Log filesystem certificate details
	if len(serverTLSConfig.Certificates) > 0 {
		serverCertSize := len(serverTLSConfig.Certificates[0].Certificate[0])
		t.Logf("Filesystem server certificate size: %d bytes", serverCertSize)
	}

	if serverTLSConfig.ClientCAs != nil {
		subjects := serverTLSConfig.ClientCAs.Subjects()
		t.Logf("Filesystem client CAs count: %d", len(subjects))
	}

	if clientTLSConfig.RootCAs != nil {
		subjects := clientTLSConfig.RootCAs.Subjects()
		t.Logf("Filesystem root CAs count: %d", len(subjects))
	}

	// Log combined TLS config sizes for analysis
	logCombinedTLSConfigSizes(t, "Filesystem TLS Handshake", serverTLSConfig, clientTLSConfig)

	// ListenAndAdvertise with TLS config
	listener, err := serverNode.ListenAndAdvertise(serviceName, serverTLSConfig, map[string]string{
		"type": "filesystem-crypto-buffer-test",
	})
	if err != nil {
		t.Fatalf("ListenAndAdvertise failed: %v", err)
	}
	defer listener.Close()

	t.Logf("ListenAndAdvertise succeeded")

	// Give time for service advertisement to propagate through the mesh
	time.Sleep(2 * time.Second)

	// Attempt to dial the service with filesystem TLS config
	t.Logf("Client attempting to dial service with filesystem TLS config...")
	conn, err := clientNode.Dial(serverName, serviceName, clientTLSConfig)
	if err != nil {
		// Fail the test to show the CRYPTO_BUFFER_EXCEEDED error
		t.Fatalf("CRYPTO_BUFFER_EXCEEDED error: %v", err)
	}

	if conn != nil {
		defer conn.Close()
		t.Logf("Connection succeeded - attempting to write data to trigger full TLS handshake")

		// Try to write data through the connection - this should trigger CRYPTO_BUFFER_EXCEEDED
		_, writeErr := conn.Write([]byte("test data to trigger TLS handshake"))
		if writeErr != nil {
			// Fail the test to show the CRYPTO_BUFFER_EXCEEDED error
			t.Fatalf("CRYPTO_BUFFER_EXCEEDED error during write: %v", writeErr)
		}

		// If we reach here, fail because CRYPTO_BUFFER_EXCEEDED should have occurred
		t.Fatalf("Unexpected success - CRYPTO_BUFFER_EXCEEDED should have occurred with large filesystem TLS config")
	}
}

// logCombinedTLSConfigSizes logs detailed size information for both server and client TLS configurations
// This shows the actual total size that gets transmitted during the TLS handshake
func logCombinedTLSConfigSizes(t *testing.T, name string, serverConfig *tls.Config, clientConfig *tls.Config) {
	t.Logf("=== %s Combined TLS Config Sizes ===", name)

	if serverConfig == nil && clientConfig == nil {
		t.Logf("  Both TLS Configs: nil")
		return
	}

	totalHandshakeSize := 0

	// Server Certificate information (sent to client during handshake)
	if serverConfig != nil && len(serverConfig.Certificates) > 0 {
		t.Logf("  Server Certificates (%d) [sent to client]:", len(serverConfig.Certificates))
		serverCertSize := 0
		for i, cert := range serverConfig.Certificates {
			certSize := 0
			if len(cert.Certificate) > 0 {
				for _, certDER := range cert.Certificate {
					certSize += len(certDER)
				}
			}
			serverCertSize += certSize
			t.Logf("    Total for Server Cert[%d]: %d bytes", i, certSize)
		}
		totalHandshakeSize += serverCertSize
	} else {
		t.Logf("  Server Certificates: 0")
	}

	// Client Certificate information (sent to server during handshake, if required)
	if clientConfig != nil && len(clientConfig.Certificates) > 0 {
		t.Logf("  Client Certificates (%d) [sent to server]:", len(clientConfig.Certificates))
		clientCertSize := 0
		for i, cert := range clientConfig.Certificates {
			certSize := 0
			if len(cert.Certificate) > 0 {
				for _, certDER := range cert.Certificate {
					certSize += len(certDER)
				}
			}
			clientCertSize += certSize
			t.Logf("    Total for Client Cert[%d]: %d bytes", i, certSize)
		}
		totalHandshakeSize += clientCertSize
	} else {
		t.Logf("  Client Certificates: 0")
	}

	// Server's Client CAs (sent to client to indicate acceptable client cert authorities)
	if serverConfig != nil && serverConfig.ClientCAs != nil {
		subjects := serverConfig.ClientCAs.Subjects()
		caSize := 0
		t.Logf("  Server's ClientCAs (%d subjects) [sent to client]:", len(subjects))

		if len(subjects) <= 5 {
			// Show detailed breakdown for small numbers of CAs
			for i, subject := range subjects {
				subjectSize := len(subject)
				caSize += subjectSize
				t.Logf("    ClientCA[%d] subject: %d bytes", i, subjectSize)
			}
		} else {
			// Show summary for large numbers of CAs
			minSize, maxSize := 0, 0
			for i, subject := range subjects {
				subjectSize := len(subject)
				caSize += subjectSize
				if i == 0 || subjectSize < minSize {
					minSize = subjectSize
				}
				if subjectSize > maxSize {
					maxSize = subjectSize
				}
			}
			avgSize := caSize / len(subjects)
			t.Logf("    ClientCA subject sizes: min=%d, max=%d, avg=%d bytes", minSize, maxSize, avgSize)
		}

		totalHandshakeSize += caSize
		t.Logf("    Total Server ClientCAs size: %d bytes", caSize)
	} else {
		t.Logf("  Server's ClientCAs: nil")
	}

	// Client's Root CAs (used for validation, not sent over wire but affects handshake)
	if clientConfig != nil && clientConfig.RootCAs != nil {
		subjects := clientConfig.RootCAs.Subjects()
		rootCASize := 0
		t.Logf("  Client's RootCAs (%d subjects) [for validation, affects handshake size]:", len(subjects))

		if len(subjects) <= 5 {
			// Show detailed breakdown for small numbers of CAs
			for i, subject := range subjects {
				subjectSize := len(subject)
				rootCASize += subjectSize
				t.Logf("    RootCA[%d] subject: %d bytes", i, subjectSize)
			}
		} else {
			// Show summary for large numbers of CAs
			minSize, maxSize := 0, 0
			for i, subject := range subjects {
				subjectSize := len(subject)
				rootCASize += subjectSize
				if i == 0 || subjectSize < minSize {
					minSize = subjectSize
				}
				if subjectSize > maxSize {
					maxSize = subjectSize
				}
			}
			avgSize := rootCASize / len(subjects)
			t.Logf("    RootCA subject sizes: min=%d, max=%d, avg=%d bytes", minSize, maxSize, avgSize)
		}

		// Note: RootCAs are not sent over the wire, but they affect handshake processing
		t.Logf("    Total Client RootCAs size: %d bytes (local validation only)", rootCASize)
	} else {
		t.Logf("  Client's RootCAs: nil")
	}

	// Server name and other config
	if serverConfig != nil && serverConfig.ServerName != "" {
		serverNameSize := len(serverConfig.ServerName)
		totalHandshakeSize += serverNameSize
		t.Logf("  Server Name: %q (%d bytes)", serverConfig.ServerName, serverNameSize)
	}
	if clientConfig != nil && clientConfig.ServerName != "" {
		serverNameSize := len(clientConfig.ServerName)
		totalHandshakeSize += serverNameSize
		t.Logf("  Client Server Name: %q (%d bytes)", clientConfig.ServerName, serverNameSize)
	}

	// Summary
	t.Logf("  TOTAL HANDSHAKE SIZE (transmitted): %d bytes", totalHandshakeSize)
	const maxBufferSize = 16384
	t.Logf("  QUIC BUFFER LIMIT: %d bytes", maxBufferSize)
}

func createReceptorCertificateAndCA(tempDir string, dnsNameCount, nodeIDCount, caCount int, serverName, clientName string) (string, string, string, error) {
	osWrapper := &certificates.OsWrapper{}
	serverCertFile := filepath.Join(tempDir, "server.crt")
	serverKeyFile := filepath.Join(tempDir, "server.key")
	clientCAsFile := filepath.Join(tempDir, "client-cas.crt")

	dnsNames := make([]string, dnsNameCount)
	for i := range dnsNames {
		dnsNames[i] = fmt.Sprintf("dns-name-%d.example.com", i)
	}
	dnsNames = append(dnsNames, "localhost")

	nodeIDs := make([]string, nodeIDCount)
	for i := range nodeIDs {
		nodeIDs[i] = fmt.Sprintf("node-id-%d", i)
	}
	// Add the actual server and client node names to the certificate
	nodeIDs = append(nodeIDs, serverName, clientName)

	// Receptor Certificate
	certOpts := &certificates.CertOptions{
		CommonName: "Receptor Certificate",
		Bits:       2048,
		NotBefore:  time.Now(),
		NotAfter:   time.Now().Add(365 * 24 * time.Hour),
		CertNames: certificates.CertNames{
			DNSNames:    dnsNames,
			NodeIDs:     nodeIDs,
			IPAddresses: nil,
		},
	}

	// Create certificate request
	req, reqKey, err := certificates.CreateCertReqWithKey(certOpts)
	if err != nil {
		return "", "", "", err
	}

	// Sign the certificate
	signOpts := &certificates.CertOptions{
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(365 * 24 * time.Hour),
	}

	// Create CA for signing the server certificate
	caOpts := &certificates.CertOptions{
		CommonName: "Large Test CA Organization",
		Bits:       2048,
		NotBefore:  time.Now(),
		NotAfter:   time.Now().Add(10 * 365 * 24 * time.Hour),
	}

	rsaWrapper := &certificates.RsaWrapper{}
	ca, err := certificates.CreateCA(caOpts, rsaWrapper)
	if err != nil {
		return "", "", "", err
	}

	cert, err := certificates.SignCertReq(req, ca, signOpts)
	if err != nil {
		return "", "", "", err
	}

	var allCAs []interface{}
	allCAs = append(allCAs, cert)

	// Add the base CA first
	allCAs = append(allCAs, ca.Certificate)

	// For large CA tests, generate or load large CA bundle from file
	if caCount > 1 {
		// Generate large CAs and save to file for reuse
		largeCAbundlePath, err := getOrCreateLargeCABundle(tempDir, caCount)
		if err != nil {
			return "", "", "", fmt.Errorf("failed to create large CA bundle: %v", err)
		}

		// Read the large CA bundle and parse certificates
		caBundleData, err := os.ReadFile(largeCAbundlePath)
		if err != nil {
			return "", "", "", fmt.Errorf("failed to read CA bundle: %v", err)
		}

		// Parse PEM blocks from the bundle
		block, rest := pem.Decode(caBundleData)
		for block != nil {
			if block.Type == "CERTIFICATE" {
				cert, err := x509.ParseCertificate(block.Bytes)
				if err == nil {
					allCAs = append(allCAs, cert.Raw)
				}
			}
			block, rest = pem.Decode(rest)
		}

		// Copy the large CA bundle to our client CAs file
		clientCAsFile = largeCAbundlePath
	} else {
		// For smaller CA counts, use the simple duplication approach
		for i := 0; i < caCount; i++ {
			allCAs = append(allCAs, ca.Certificate)
		}

		// Save CA bundle using the standard approach
		err = certificates.SaveToPEMFile(clientCAsFile, allCAs, osWrapper)
		if err != nil {
			return "", "", "", err
		}
	}

	// Save server certificate
	err = certificates.SaveToPEMFile(serverCertFile, []interface{}{cert}, osWrapper)
	if err != nil {
		return "", "", "", err
	}

	// Save server key
	err = certificates.SaveToPEMFile(serverKeyFile, []interface{}{reqKey}, osWrapper)
	if err != nil {
		return "", "", "", err
	}

	return serverCertFile, serverKeyFile, clientCAsFile, nil
}

// generateLargeCABundle creates a bundle of large CA certificates using Receptor functions
// This is designed to create certificates that will trigger CRYPTO_BUFFER_EXCEEDED errors
func generateLargeCABundle(filePath string, caCount int) error {
	rsaWrapper := &certificates.RsaWrapper{}
	osWrapper := &certificates.OsWrapper{}
	var allCAs []interface{}

	// Create a shared private key for speed
	baseCA, err := certificates.CreateCA(&certificates.CertOptions{
		CommonName: "Large CA Base for Testing",
		Bits:       2048,
		NotBefore:  time.Now(),
		NotAfter:   time.Now().Add(10 * 365 * 24 * time.Hour),
	}, rsaWrapper)
	if err != nil {
		return fmt.Errorf("failed to create base CA: %v", err)
	}

	for i := 0; i < caCount; i++ {
		// Create a CA certificate with an extremely long CommonName to maximize certificate size
		// CA certificates can't have DNSNames/NodeIDs, so we make the CommonName extremely long
		// Use repeated long strings to create massive certificates
		paddingString := "Very-Long-Padding-String-To-Increase-Certificate-Size-For-Testing-QUIC-Crypto-Buffer-Limits-And-Trigger-Exceeded-Errors-"
		minimalPadding := "Very-Long-Padding-String-To-Increase-Certificate-Size-For-"
		longCommonName := fmt.Sprintf("Massive-CA-Organization-%d-%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s-Certificate-Authority-Department-Security-Division-Unit-%d",
			i, paddingString, paddingString, paddingString, paddingString, paddingString,
			paddingString, paddingString, paddingString, paddingString, paddingString,
			paddingString, paddingString, paddingString, paddingString, paddingString,
			paddingString, paddingString, paddingString, paddingString, paddingString,
			paddingString, paddingString, paddingString, paddingString, minimalPadding, i)

		// Create CA options with extremely long CommonName and larger key size to maximize certificate size
		largeCAOpts := &certificates.CertOptions{
			CommonName: longCommonName,
			Bits:       4096, // Use larger key size for bigger certificates
			NotBefore:  time.Now(),
			NotAfter:   time.Now().Add(10 * 365 * 24 * time.Hour),
		}

		// Create the large CA certificate
		largeCA, err := certificates.CreateCA(largeCAOpts, rsaWrapper)
		if err != nil {
			// If creation fails, reuse the base CA to maintain functionality
			fmt.Printf("Warning: Failed to create large CA %d, reusing base CA: %v\n", i, err)
			allCAs = append(allCAs, baseCA.Certificate)
		} else {
			allCAs = append(allCAs, largeCA.Certificate)
		}
	}

	// Save all CAs to the bundle file using Receptor's SaveToPEMFile
	err = certificates.SaveToPEMFile(filePath, allCAs, osWrapper)
	if err != nil {
		return fmt.Errorf("failed to save CA bundle: %v", err)
	}

	// Get file size for reporting
	stat, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to stat CA bundle file: %v", err)
	}

	fmt.Printf("Generated large CA bundle with %d CAs (%d bytes) using Receptor functions at %s\n", caCount, stat.Size(), filePath)
	return nil
}

// getOrCreateLargeCABundle returns the path to a large CA bundle file, creating it if it doesn't exist
func getOrCreateLargeCABundle(tempDir string, caCount int) (string, error) {
	caBundlePath := filepath.Join(tempDir, fmt.Sprintf("large-ca-bundle-%d.pem", caCount))

	// Check if file already exists and is recent (within 1 hour)
	if stat, err := os.Stat(caBundlePath); err == nil {
		if time.Since(stat.ModTime()) < 1*time.Hour {
			fmt.Printf("Using existing large CA bundle: %s (%d bytes)\n", caBundlePath, stat.Size())
			return caBundlePath, nil
		}
	}

	// Generate new CA bundle
	err := generateLargeCABundle(caBundlePath, caCount)
	if err != nil {
		return "", err
	}

	return caBundlePath, nil
}
