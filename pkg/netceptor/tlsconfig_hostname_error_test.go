package netceptor

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
)

type certTestType int

const (
	dnsName certTestType = iota
	ipAddress
)

func TestTLSConfigCertCount(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		testName        string
		testType        certTestType
		hostnameToMatch string
		itemCount       int
		expectedError   string
	}{
		{
			testName:        "Zero DNS names",
			testType:        dnsName,
			hostnameToMatch: "MyHostname",
			itemCount:       0,
			expectedError:   "x509: certificate is not valid for any names, but wanted to match MyHostname",
		},
		{
			testName:        "Less than 100 DNS names",
			testType:        dnsName,
			hostnameToMatch: "MyHostname",
			itemCount:       3,
			expectedError:   "x509: certificate is valid for server-0, server-1, server-2, not MyHostname",
		},
		{
			testName:        "More than 100 DNS names",
			testType:        dnsName,
			hostnameToMatch: "MyHostname",
			itemCount:       105,
			expectedError:   "x509: certificate is valid for 105 names, but none matched MyHostname",
		},
		{
			testName:        "Exactly 100 DNS names",
			testType:        dnsName,
			hostnameToMatch: "MyHostname",
			itemCount:       100,
			expectedError:   "x509: certificate is valid for 100 names, but none matched MyHostname",
		},
		{
			testName:        "Zero IP SANs",
			testType:        ipAddress,
			hostnameToMatch: "127.0.0.1",
			itemCount:       0,
			expectedError:   "x509: cannot validate certificate for 127.0.0.1 because it doesn't contain any IP SANs",
		},
		{
			testName:        "Less than 100 IP SANs",
			testType:        ipAddress,
			hostnameToMatch: "127.0.0.1",
			itemCount:       3,
			expectedError:   "x509: certificate is valid for 1.2.3.4, 1.2.3.4, 1.2.3.4, not 127.0.0.1",
		},
		{
			testName:        "More than 100 IP SANs",
			testType:        ipAddress,
			hostnameToMatch: "127.0.0.1",
			itemCount:       105,
			expectedError:   "x509: certificate is valid for 105 IP SANs, but none matched 127.0.0.1",
		},
		{
			testName:        "Exactly 100 IP SANs",
			testType:        ipAddress,
			hostnameToMatch: "127.0.0.1",
			itemCount:       100,
			expectedError:   "x509: certificate is valid for 100 IP SANs, but none matched 127.0.0.1",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()

			cert := &x509.Certificate{}

			switch tt.testType {
			case dnsName:
				cert.DNSNames = make([]string, tt.itemCount)
				for n := range tt.itemCount {
					cert.DNSNames[n] = fmt.Sprintf("server-%d", n)
				}
			case ipAddress:
				cert.IPAddresses = make([]net.IP, tt.itemCount)
				for n := range tt.itemCount {
					cert.IPAddresses[n] = net.IPv4(1, 2, 3, 4)
				}
			default:
				t.Errorf("invalid test type: %d", tt.testType)
			}

			hostnameErr := x509.HostnameError{
				Host:        tt.hostnameToMatch,
				Certificate: cert,
			}

			err := handleHostnameError(hostnameErr)

			errMsg := err.Error()
			if !strings.Contains(errMsg, tt.expectedError) {
				t.Fatalf("Error message should contain %q, got %q", tt.expectedError, errMsg)
			}
		})
	}
}

func TestHostnameErrorHandlerCalled(t *testing.T) {
	hostnameToMatch := "MyHostname"
	expectedError := "x509: certificate is not valid for any names, but wanted to match MyHostname"

	caCertFilename, certFilename, _, teardownFunc := useUtilsSetupSuiteWithGenerateWithCA(t, hostnameToMatch)
	defer teardownFunc(t)

	// Read CA certificate.
	caBytes, err := os.ReadFile(caCertFilename)
	if err != nil {
		t.Fatalf("failed to read CA certificate file: %v", err)
	}
	rootCA := x509.NewCertPool()
	rootCA.AppendCertsFromPEM(caBytes)

	// Read regular certificate.
	certBytes, err := os.ReadFile(certFilename)
	if err != nil {
		t.Fatalf("failed to read certificate file: %v", err)
	}
	block, rest := pem.Decode(certBytes)
	if block == nil {
		t.Fatal("failed to decode PEM certificate")
	}
	if len(rest) != 0 {
		t.Fatalf("unexpected remaining bytes after PEM decode: %d bytes remaining", len(rest))
	}

	cfg := &tls.Config{
		RootCAs:            rootCA,
		InsecureSkipVerify: true,
	}

	MainInstance = New(context.Background(), "testnode")
	verifyFunc := ReceptorVerifyFunc(cfg, nil, hostnameToMatch, ExpectedHostnameTypeDNS, VerifyServer, MainInstance.Logger)

	rawCerts := [][]byte{block.Bytes}
	err = verifyFunc(rawCerts, nil)

	if err == nil {
		t.Fatalf("Expected an error but received none")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, expectedError) {
		t.Fatalf("Error message should contain %q, got %q", expectedError, errMsg)
	}
}
