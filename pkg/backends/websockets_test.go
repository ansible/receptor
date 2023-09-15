package backends_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/backends"
	"github.com/ansible/receptor/pkg/logger"
)

func setupTlsCfg(t *testing.T) tls.Certificate {
	// Create a server TLS certificate for "localhost"
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "localhost",
		},
		DNSNames:  []string{"localhost"},
		NotBefore: time.Now().Add(-1 * time.Minute),
		NotAfter:  time.Now().Add(24 * time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	return tlsCert
}

func TestNewWebsocketDialer(t *testing.T) {

	NewWebsocketDialerTestCases := []struct {
		name             string
		address          string
		redial           bool
		tlscfg           *tls.Config
		extraHeader      string
		logger           *logger.ReceptorLogger
		expectedErr      string
		failedTestString string
	}{
		{
			name:    "NewWebsocketDialer wss Success ",
			address: "wss://demo.piesocket.com/v3/channel_123?api_key=VCXCEuvhGcBDP7XhiJJUDvR1e1D3eiVjgZ9VRiaV&notify_self",
			redial:  false,
			tlscfg: &tls.Config{
				Certificates:             []tls.Certificate{setupTlsCfg(t)},
				MinVersion:               tls.VersionTLS12,
				PreferServerCipherSuites: true,
			},
			extraHeader:      "",
			logger:           logger.NewReceptorLogger("websockets_test.go>TestNewWebsocketDialer"),
			expectedErr:      "",
			failedTestString: "Expected no error, but got: %v",
		},
		{
			name:    "NewWebsocketDialer non-wss Success ",
			address: "demo.piesocket.com/v3/channel_123?api_key=VCXCEuvhGcBDP7XhiJJUDvR1e1D3eiVjgZ9VRiaV&notify_self",
			redial:  false,
			tlscfg: &tls.Config{
				Certificates:             []tls.Certificate{setupTlsCfg(t)},
				MinVersion:               tls.VersionTLS12,
				PreferServerCipherSuites: true,
			},
			extraHeader:      "",
			logger:           logger.NewReceptorLogger("websockets_test.go>TestNewWebsocketDialer"),
			expectedErr:      "",
			failedTestString: "Expected no error, but got: %v",
		},
	}

	for _, testCase := range NewWebsocketDialerTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := backends.NewWebsocketDialer(testCase.address, testCase.tlscfg, testCase.extraHeader, testCase.redial, testCase.logger)
			if testCase.expectedErr == "" && err != nil {
				t.Errorf(testCase.failedTestString, err)
			}
			if testCase.expectedErr != "" && err != nil && err.Error() != testCase.expectedErr {
				t.Errorf(testCase.failedTestString, err)
			}
			if testCase.expectedErr != "" && err == nil {
				t.Errorf(testCase.failedTestString, err)
			}
		})
	}
}
