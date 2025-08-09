package mesh

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/backends"
	"github.com/ansible/receptor/pkg/netceptor"
)

// TestQuicCryptoBufferExceededLargeCert tests CRYPTO_BUFFER_EXCEEDED with large server certificates
// This test demonstrates the customer error when individual server certificates exceed QUIC buffer limits
func TestQuicCryptoBufferExceededLargeCert(t *testing.T) {
	t.Run("Large server certificate triggers CRYPTO_BUFFER_EXCEEDED", func(t *testing.T) {
		const maxBufferSize = 16384 // QUIC crypto stream buffer limit

		// Create large TLS configuration with oversized certificate
		largeTLSConfig := createLargeTLSConfig()

		// Analyze the certificate size
		if len(largeTLSConfig.Certificates) > 0 {
			certSize := len(largeTLSConfig.Certificates[0].Certificate[0])
			t.Logf("Large certificate size: %d bytes", certSize)
		}

		// Check client CA pool size
		if largeTLSConfig.ClientCAs != nil {
			subjects := largeTLSConfig.ClientCAs.Subjects()
			t.Logf("Number of CA certificates: %d", len(subjects))
			totalSize := 0
			for _, subject := range subjects {
				totalSize += len(subject)
			}
			t.Logf("Total CA subjects size: %d bytes", totalSize)
		}

		// Create Netceptor instance
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		n := netceptor.New(ctx, "test-node")
		defer n.Shutdown()

		// ListenAndAdvertise with large TLS config
		listener, err := n.ListenAndAdvertise("testlrg", largeTLSConfig, map[string]string{
			"type": "large-tls-test",
		})

		if err != nil {
			t.Errorf("ListenAndAdvertise failed with large TLS config: %v", err)
			return
		}
		defer listener.Close()

		t.Logf("ListenAndAdvertise succeeded - now setting up Receptor network connection")
		t.Logf("Large certificate size: %d bytes exceeds buffer limit %d bytes", len(largeTLSConfig.Certificates[0].Certificate[0]), maxBufferSize)

		// Create second Netceptor instance for client
		client := netceptor.New(ctx, "test-client")
		defer client.Shutdown()

		// Set up TCP backends to establish network connection
		b1, err := backends.NewTCPListener("localhost:0", nil, n.Logger)
		if err != nil {
			t.Errorf("Error creating TCP listener: %v", err)
			return
		}
		err = n.AddBackend(b1)
		if err != nil {
			t.Errorf("Error adding backend to server: %v", err)
			return
		}

		// Get the actual port that was assigned
		tcpAddr := b1.GetAddr()
		t.Logf("Server listening on: %s", tcpAddr)

		// Set up TCP dialer on client to connect to the listener
		b2, err := backends.NewTCPDialer(tcpAddr, false, nil, client.Logger)
		if err != nil {
			t.Errorf("Error creating TCP dialer: %v", err)
			return
		}
		err = client.AddBackend(b2)
		if err != nil {
			t.Errorf("Error adding backend to client: %v", err)
			return
		}

		// Give time for backends to establish connection
		time.Sleep(2 * time.Second)

		// Now attempt to dial the service with large TLS config
		// This should trigger CRYPTO_BUFFER_EXCEEDED when QUIC tries to send large cert data
		t.Logf("Client attempting to dial service with large TLS config...")
		conn, err := client.Dial("test-node", "testlrg", largeTLSConfig)
		if err != nil {
			t.Errorf("Dial failed with large TLS config: %v", err)
			return
		}

		if conn != nil {
			defer conn.Close()
			t.Logf("Connection succeeded - attempting to write data to trigger full TLS handshake")

			// Try to write data through the connection - this should trigger CRYPTO_BUFFER_EXCEEDED
			_, writeErr := conn.Write([]byte("test data to trigger TLS handshake"))
			if writeErr != nil {
				t.Errorf("Write failed during TLS handshake: %v", writeErr)
				return
			}

			t.Errorf("Write succeeded - CRYPTO_BUFFER_EXCEEDED should have occurred with cert size %d > %d",
				len(largeTLSConfig.Certificates[0].Certificate[0]), maxBufferSize)
		}
	})
}

// TestQuicCryptoBufferExceededReceptorTLSConfig tests CRYPTO_BUFFER_EXCEEDED using Receptor's TLS config functions
// This test uses PrepareTLSServerConfig/PrepareTLSClientConfig instead of manual certificate creation
func TestQuicCryptoBufferExceededReceptorTLSConfig(t *testing.T) {
	t.Run("Receptor TLS config with large certificates", func(t *testing.T) {
		// Create temporary directory for test certificates
		tempDir := t.TempDir()

		// Create large certificate and key files similar to createLargeTLSConfig
		serverCertFile := filepath.Join(tempDir, "server.crt")
		serverKeyFile := filepath.Join(tempDir, "server.key")
		clientCAsFile := filepath.Join(tempDir, "client-cas.crt")

		// Create server certificate with threshold size that triggers CRYPTO_BUFFER_EXCEEDED
		serverCertTemplate := x509.Certificate{
			SerialNumber: big.NewInt(1),
			Subject: pkix.Name{
				Organization:       []string{"Large Server Cert"},
				Country:            []string{"US"},
				Province:           []string{"CA"},
				Locality:           []string{"SF"},
				StreetAddress:      []string{"123 Main St"},
				PostalCode:         []string{"94102"},
				OrganizationalUnit: []string{"IT"},
			},
			NotBefore:   time.Now(),
			NotAfter:    time.Now().Add(365 * 24 * time.Hour),
			KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
			ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
			IsCA:        true,
			IPAddresses: nil,
			DNSNames:    []string{"localhost"},
			// Add email addresses to reach threshold that triggers CRYPTO_BUFFER_EXCEEDED
			EmailAddresses: func() []string {
				emails := make([]string, 111) // 111 emails triggers CRYPTO_BUFFER_EXCEEDED
				for i := range emails {
					emails[i] = fmt.Sprintf("very-long-email-address-to-increase-certificate-size-%d@extremely-long-domain-name-to-exceed-quic-crypto-buffer-limits.example.com", i)
				}
				return emails
			}(),
		}

		// Generate private key
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatalf("Failed to generate private key: %v", err)
		}

		// Create certificate
		certDER, err := x509.CreateCertificate(rand.Reader, &serverCertTemplate, &serverCertTemplate, &privateKey.PublicKey, privateKey)
		if err != nil {
			t.Fatalf("Failed to create certificate: %v", err)
		}

		// Convert to PEM
		certPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: certDER,
		})

		privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
		if err != nil {
			t.Fatalf("Failed to marshal private key: %v", err)
		}
		keyPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: privateKeyBytes,
		})

		// Write certificate and key files
		err = os.WriteFile(serverCertFile, certPEM, 0644)
		if err != nil {
			t.Fatalf("Failed to write server certificate: %v", err)
		}
		err = os.WriteFile(serverKeyFile, keyPEM, 0600)
		if err != nil {
			t.Fatalf("Failed to write server key: %v", err)
		}

		// Create large CA bundle
		largeTLSConfig := createLargeTLSConfig()
		largeCert := largeTLSConfig.Certificates[0]
		largeCertPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: largeCert.Certificate[0],
		})

		// Create client CAs file with large certificates
		var clientCAsPEM []byte
		for i := 0; i < 50; i++ {
			clientCAsPEM = append(clientCAsPEM, largeCertPEM...)
		}
		err = os.WriteFile(clientCAsFile, clientCAsPEM, 0644)
		if err != nil {
			t.Fatalf("Failed to write client CAs: %v", err)
		}

		t.Logf("Created test certificates:")
		t.Logf("  Server cert: %s (%d bytes)", serverCertFile, len(certPEM))
		t.Logf("  Server key: %s (%d bytes)", serverKeyFile, len(keyPEM))
		t.Logf("  Client CAs: %s (%d bytes)", clientCAsFile, len(clientCAsPEM))

		// Create Netceptor instance
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		n := netceptor.New(ctx, "test-node-tls")
		defer n.Shutdown()

		// Create TLS server configuration using Receptor's PrepareTLSServerConfig
		serverConfig := netceptor.TLSServerConfig{
			Name:                   "test-server",
			Cert:                   serverCertFile,
			Key:                    serverKeyFile,
			RequireClientCert:      true,
			ClientCAs:              clientCAsFile,
			SkipReceptorNamesCheck: true,
			MinTLS13:               false,
		}

		serverTLSConfig, err := serverConfig.PrepareTLSServerConfig(n)
		if err != nil {
			t.Fatalf("Failed to prepare TLS server config: %v", err)
		}

		// Log server TLS config details
		serverCertSize := len(serverTLSConfig.Certificates[0].Certificate[0])
		t.Logf("Server TLS config prepared:")
		t.Logf("  Server certificate size: %d bytes", serverCertSize)
		if serverTLSConfig.ClientCAs != nil {
			subjects := serverTLSConfig.ClientCAs.Subjects()
			t.Logf("  Client CAs count: %d", len(subjects))
		}

		// This should trigger CRYPTO_BUFFER_EXCEEDED when used in connection
		const maxBufferSize = 16384
		if serverCertSize > maxBufferSize {
			t.Logf("Certificate size %d exceeds QUIC buffer limit %d - will trigger CRYPTO_BUFFER_EXCEEDED",
				serverCertSize, maxBufferSize)
		}

		// Now test ListenAndAdvertise with this TLS configuration
		listener, err := n.ListenAndAdvertise("testtls", serverTLSConfig, map[string]string{
			"type": "receptor-tls-test",
		})

		if err != nil {
			t.Errorf("ListenAndAdvertise failed with Receptor TLS config: %v", err)
			return
		}
		defer listener.Close()

		t.Logf("ListenAndAdvertise succeeded with Receptor TLS config")

		// Create client Netceptor
		client := netceptor.New(ctx, "test-client-tls")
		defer client.Shutdown()

		// Set up TCP backends for network connection
		b1, err := backends.NewTCPListener("localhost:0", nil, n.Logger)
		if err != nil {
			t.Errorf("Error creating TCP listener: %v", err)
			return
		}
		err = n.AddBackend(b1)
		if err != nil {
			t.Errorf("Error adding backend to server: %v", err)
			return
		}

		tcpAddr := b1.GetAddr()
		t.Logf("Server listening on: %s", tcpAddr)

		b2, err := backends.NewTCPDialer(tcpAddr, false, nil, client.Logger)
		if err != nil {
			t.Errorf("Error creating TCP dialer: %v", err)
			return
		}
		err = client.AddBackend(b2)
		if err != nil {
			t.Errorf("Error adding backend to client: %v", err)
			return
		}

		// Wait for backend connection
		time.Sleep(2 * time.Second)

		// Create client TLS config
		clientConfig := netceptor.TLSClientConfig{
			Name:                   "test-client",
			Cert:                   serverCertFile,
			Key:                    serverKeyFile,
			RootCAs:                clientCAsFile,
			InsecureSkipVerify:     true,
			SkipReceptorNamesCheck: true,
			MinTLS13:               false,
		}

		clientTLSConfig, _, err := clientConfig.PrepareTLSClientConfig(client)
		if err != nil {
			t.Errorf("Failed to prepare TLS client config: %v", err)
			return
		}

		// Log client TLS config details
		if len(clientTLSConfig.Certificates) > 0 {
			clientCertSize := len(clientTLSConfig.Certificates[0].Certificate[0])
			t.Logf("Client certificate size: %d bytes", clientCertSize)
		}
		if clientTLSConfig.RootCAs != nil {
			subjects := clientTLSConfig.RootCAs.Subjects()
			t.Logf("Client root CAs count: %d", len(subjects))
		}

		// Attempt to dial - this should trigger CRYPTO_BUFFER_EXCEEDED and FAIL the test
		t.Logf("Client attempting to dial with Receptor TLS config (large certificates)...")
		conn, err := client.Dial("test-node-tls", "testtls", clientTLSConfig)
		if err != nil {
			t.Errorf("Dial failed with Receptor TLS config: %v", err)
			return
		} else {
			defer conn.Close()
			t.Logf("Connection succeeded - attempting write to trigger full handshake")

			_, writeErr := conn.Write([]byte("test data"))
			if writeErr != nil {
				t.Errorf("Write failed during handshake: %v", writeErr)
				return
			} else {
				t.Errorf("Write succeeded - CRYPTO_BUFFER_EXCEEDED should have occurred with certificate size %d bytes",
					len(serverTLSConfig.Certificates[0].Certificate[0]))
				return
			}
		}
	})
}

// TestQuicCryptoBufferExceededLargeCABundle tests CRYPTO_BUFFER_EXCEEDED with only large CA bundle
// This test uses a small server certificate but an extremely large CA bundle to trigger the error
func TestQuicCryptoBufferExceededLargeCABundle(t *testing.T) {
	t.Run("Large CA bundle with small server certificate", func(t *testing.T) {
		// Create temporary directory for test certificates
		tempDir := t.TempDir()

		// Create certificate and key files
		serverCertFile := filepath.Join(tempDir, "server.crt")
		serverKeyFile := filepath.Join(tempDir, "server.key")
		clientCAsFile := filepath.Join(tempDir, "client-cas.crt")

		// Create MINIMAL server certificate (very small)
		minimalistCertTemplate := x509.Certificate{
			SerialNumber: big.NewInt(1),
			Subject: pkix.Name{
				Organization: []string{"Minimal Cert"},
				Country:      []string{"US"},
			},
			NotBefore:   time.Now(),
			NotAfter:    time.Now().Add(365 * 24 * time.Hour),
			KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
			ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
			IsCA:        true,
			DNSNames:    []string{"localhost"},
			// Minimal fields - just one email
			EmailAddresses: []string{"test@example.com"},
		}

		// Generate minimal private key
		minimalPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatalf("Failed to generate minimal private key: %v", err)
		}

		// Create minimal certificate
		minimalCertDER, err := x509.CreateCertificate(rand.Reader, &minimalistCertTemplate, &minimalistCertTemplate, &minimalPrivateKey.PublicKey, minimalPrivateKey)
		if err != nil {
			t.Fatalf("Failed to create minimal certificate: %v", err)
		}

		// Convert to PEM
		minimalCertPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: minimalCertDER,
		})

		minimalPrivateKeyBytes, err := x509.MarshalPKCS8PrivateKey(minimalPrivateKey)
		if err != nil {
			t.Fatalf("Failed to marshal minimal private key: %v", err)
		}
		minimalKeyPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: minimalPrivateKeyBytes,
		})

		// Write MINIMAL server certificate and key
		err = os.WriteFile(serverCertFile, minimalCertPEM, 0644)
		if err != nil {
			t.Fatalf("Failed to write minimal server certificate: %v", err)
		}
		err = os.WriteFile(serverKeyFile, minimalKeyPEM, 0600)
		if err != nil {
			t.Fatalf("Failed to write minimal server key: %v", err)
		}

		// Create EXTREMELY LARGE CA bundle with many different large certificates
		// This simulates the field case where CA bundle causes CRYPTO_BUFFER_EXCEEDED
		var massiveCABundle []byte

		// Create several different large CA certificates to add to the bundle
		for caIndex := 0; caIndex < 10; caIndex++ {
			// Create a unique large CA certificate for each iteration
			largeCACertTemplate := x509.Certificate{
				SerialNumber: big.NewInt(int64(100 + caIndex)),
				Subject: pkix.Name{
					Organization:  []string{fmt.Sprintf("Large CA Organization %d", caIndex)},
					Country:       []string{"US"},
					Province:      []string{"CA"},
					Locality:      []string{"SF"},
					StreetAddress: []string{fmt.Sprintf("123 CA Street %d", caIndex)},
					PostalCode:    []string{"94102"},
					// Large organizational units for each CA
					OrganizationalUnit: []string{
						fmt.Sprintf("Large CA OU %d: %s", caIndex, strings.Repeat("Large OU Data ", 50)),
						fmt.Sprintf("Another Large CA OU %d: %s", caIndex, strings.Repeat("More OU Data ", 50)),
					},
				},
				NotBefore:   time.Now(),
				NotAfter:    time.Now().Add(365 * 24 * time.Hour),
				KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
				ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
				IsCA:        true,
				IPAddresses: nil,
				DNSNames:    []string{fmt.Sprintf("ca%d.example.com", caIndex)},
				// Large number of email addresses for each CA certificate
				EmailAddresses: func() []string {
					emails := make([]string, 100) // 100 emails per CA cert
					for i := range emails {
						emails[i] = fmt.Sprintf("ca%d-very-long-email-address-to-increase-certificate-size-%d@extremely-long-domain-name-to-exceed-quic-crypto-buffer-limits-for-ca-bundle.example.com", caIndex, i)
					}
					return emails
				}(),
			}

			// Generate private key for this CA
			caPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
			if err != nil {
				t.Fatalf("Failed to generate CA private key %d: %v", caIndex, err)
			}

			// Create CA certificate
			caCertDER, err := x509.CreateCertificate(rand.Reader, &largeCACertTemplate, &largeCACertTemplate, &caPrivateKey.PublicKey, caPrivateKey)
			if err != nil {
				t.Fatalf("Failed to create CA certificate %d: %v", caIndex, err)
			}

			// Convert to PEM and add to bundle
			caCertPEM := pem.EncodeToMemory(&pem.Block{
				Type:  "CERTIFICATE",
				Bytes: caCertDER,
			})

			// Add this CA certificate to the bundle multiple times to increase size
			for repeat := 0; repeat < 20; repeat++ { // 20 copies of each CA cert
				massiveCABundle = append(massiveCABundle, caCertPEM...)
			}
		}

		// Write the MASSIVE CA bundle file
		err = os.WriteFile(clientCAsFile, massiveCABundle, 0644)
		if err != nil {
			t.Fatalf("Failed to write massive CA bundle: %v", err)
		}

		t.Logf("Created field-case test certificates:")
		t.Logf("  MINIMAL Server cert: %s (%d bytes)", serverCertFile, len(minimalCertPEM))
		t.Logf("  MINIMAL Server key: %s (%d bytes)", serverKeyFile, len(minimalKeyPEM))
		t.Logf("  MASSIVE CA bundle: %s (%d bytes)", clientCAsFile, len(massiveCABundle))

		// Create Netceptor instance
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		n := netceptor.New(ctx, "test-node-ca")
		defer n.Shutdown()

		// Create TLS server configuration with minimal server cert but massive CA bundle
		serverConfig := netceptor.TLSServerConfig{
			Name:                   "test-server-ca",
			Cert:                   serverCertFile,
			Key:                    serverKeyFile,
			RequireClientCert:      true,
			ClientCAs:              clientCAsFile, // This is the massive CA bundle that should trigger error
			SkipReceptorNamesCheck: true,
			MinTLS13:               false,
		}

		serverTLSConfig, err := serverConfig.PrepareTLSServerConfig(n)
		if err != nil {
			t.Fatalf("Failed to prepare TLS server config with massive CA bundle: %v", err)
		}

		// Log the configuration details
		serverCertSize := len(serverTLSConfig.Certificates[0].Certificate[0])
		t.Logf("Server TLS config prepared:")
		t.Logf("  MINIMAL server certificate size: %d bytes", serverCertSize)
		if serverTLSConfig.ClientCAs != nil {
			subjects := serverTLSConfig.ClientCAs.Subjects()
			t.Logf("  MASSIVE CA bundle - CA count: %d", len(subjects))
			t.Logf("  MASSIVE CA bundle - file size: %d bytes", len(massiveCABundle))
		}

		const maxBufferSize = 16384
		t.Logf("CA bundle size %d bytes should exceed QUIC buffer limit %d bytes during TLS handshake",
			len(massiveCABundle), maxBufferSize)

		// Test ListenAndAdvertise with massive CA bundle
		listener, err := n.ListenAndAdvertise("testca", serverTLSConfig, map[string]string{
			"type": "massive-ca-test",
		})

		if err != nil {
			t.Errorf("ListenAndAdvertise failed with massive CA bundle: %v", err)
			return
		}
		defer listener.Close()

		t.Logf("ListenAndAdvertise succeeded with massive CA bundle")

		// Create client Netceptor
		client := netceptor.New(ctx, "test-client-ca")
		defer client.Shutdown()

		// Set up TCP backends
		b1, err := backends.NewTCPListener("localhost:0", nil, n.Logger)
		if err != nil {
			t.Errorf("Error creating TCP listener: %v", err)
			return
		}
		err = n.AddBackend(b1)
		if err != nil {
			t.Errorf("Error adding backend to server: %v", err)
			return
		}

		tcpAddr := b1.GetAddr()
		t.Logf("Server listening on: %s", tcpAddr)

		b2, err := backends.NewTCPDialer(tcpAddr, false, nil, client.Logger)
		if err != nil {
			t.Errorf("Error creating TCP dialer: %v", err)
			return
		}
		err = client.AddBackend(b2)
		if err != nil {
			t.Errorf("Error adding backend to client: %v", err)
			return
		}

		// Wait for backend connection
		time.Sleep(2 * time.Second)

		// Create client TLS config that will also use the massive CA bundle
		clientConfig := netceptor.TLSClientConfig{
			Name:                   "test-client-ca",
			Cert:                   serverCertFile, // Reuse minimal cert for client
			Key:                    serverKeyFile,  // Reuse minimal key for client
			RootCAs:                clientCAsFile,  // Use the same massive CA bundle
			InsecureSkipVerify:     true,
			SkipReceptorNamesCheck: true,
			MinTLS13:               false,
		}

		clientTLSConfig, _, err := clientConfig.PrepareTLSClientConfig(client)
		if err != nil {
			t.Errorf("Failed to prepare TLS client config: %v", err)
			return
		}

		// Log client config details
		if len(clientTLSConfig.Certificates) > 0 {
			clientCertSize := len(clientTLSConfig.Certificates[0].Certificate[0])
			t.Logf("Client certificate size: %d bytes", clientCertSize)
		}
		if clientTLSConfig.RootCAs != nil {
			subjects := clientTLSConfig.RootCAs.Subjects()
			t.Logf("Client root CAs count: %d", len(subjects))
		}

		// Attempt to dial - this should trigger CRYPTO_BUFFER_EXCEEDED due to massive CA bundle
		t.Logf("Client attempting to dial with massive CA bundle (field case reproduction)...")
		conn, err := client.Dial("test-node-ca", "testca", clientTLSConfig)
		if err != nil {
			t.Errorf("Dial failed with massive CA bundle: %v", err)
			return
		} else {
			defer conn.Close()
			t.Logf("Connection succeeded - attempting write to trigger full TLS handshake")

			_, writeErr := conn.Write([]byte("test data with massive CA bundle"))
			if writeErr != nil {
				t.Errorf("Write failed during TLS handshake with massive CA bundle: %v", writeErr)
				return
			} else {
				t.Errorf("Write succeeded - CRYPTO_BUFFER_EXCEEDED should have occurred with massive CA bundle (%d bytes)",
					len(massiveCABundle))
				return
			}
		}
	})
}

// createLargeTLSConfig creates a TLS config with large client CA certificates
// This simulates the scenario that triggers CRYPTO_BUFFER_EXCEEDED errors
func createLargeTLSConfig() *tls.Config {
	// Create a large certificate with many extensions and large fields
	// This will cause the TLS handshake to exceed crypto buffer limits

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Large Test CA Organization"},
			Country:       []string{"US"},
			Province:      []string{"Test State"},
			Locality:      []string{"Test City"},
			StreetAddress: []string{"123 Test Street"},
			PostalCode:    []string{"12345"},
			// Add fewer organizational units to find threshold
			OrganizationalUnit: []string{
				strings.Repeat("OU ", 10), // Much smaller
			},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IsCA:        true,
		IPAddresses: nil,
		DNSNames:    []string{"localhost"},
		// Add email addresses to find threshold
		EmailAddresses: func() []string {
			emails := make([]string, 110) // 110 emails is threshold, 109 emails returns a different TLS error
			for i := range emails {
				emails[i] = fmt.Sprintf("very-long-email-address-to-increase-certificate-size-%d@extremely-long-domain-name-to-exceed-quic-crypto-buffer-limits.example.com", i)
			}
			return emails
		}(),
	}

	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048) // Reduce key size from 4096 to 2048
	if err != nil {
		panic(err)
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		panic(err)
	}

	// Parse certificate
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		panic(err)
	}

	// Create certificate pool with minimal certificates to find threshold
	certPool := x509.NewCertPool()
	for i := 0; i < 5; i++ { // Reduce to just 5
		certPool.AddCert(cert)
	}

	// Create TLS certificate pair
	tlsCert := tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  privateKey,
	}

	// Return TLS config with large client CA pool
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		ClientCAs:    certPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ServerName:   "localhost",
	}
}
