package netceptor

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"

	"github.com/ansible/receptor/pkg/utils"
)

// GenerateServerTLSConfig returns the server side *tls.Config and the Server and CA Certificates as byte arrays.
func GenerateServerTLSConfig(commonName string) (*tls.Config, []byte, []byte) {
	caKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"My CA Inc."},
			CommonName:   "My Root CA",
		},
		NotBefore:             time.Now().Add(-1 * time.Minute),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}
	caCertDER, _ := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	caCert, _ := x509.ParseCertificate(caCertDER)

	dnsNames := make([]string, 0)
	ipAddresses := make([]net.IP, 0)
	nodeIds := []string{commonName} // This is a custom receptor type that is included in the certificate SAN
	if ip := net.ParseIP(commonName); ip != nil {
		ipAddresses = append(ipAddresses, ip)
	} else {
		dnsNames = append(dnsNames, commonName)
	}
	subjectAlternativeNamesExtension, _ := utils.MakeReceptorSAN(dnsNames, ipAddresses, nodeIds)

	serverKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: commonName,
		},
		NotBefore:       time.Now().Add(-1 * time.Minute),
		NotAfter:        time.Now().Add(24 * time.Hour),
		DNSNames:        dnsNames,
		IPAddresses:     ipAddresses,
		ExtraExtensions: []pkix.Extension{*subjectAlternativeNamesExtension},
	}
	serverCertDER, _ := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey)
	serverKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverKey)})
	serverCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverCertDER})
	caCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertDER})

	serverCert, _ := tls.X509KeyPair(serverCertPEM, serverKeyPEM)
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(caCertPEM)

	return &tls.Config{
		Certificates:             []tls.Certificate{serverCert},
		NextProtos:               []string{"netceptor"},
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
		ClientCAs:                certPool,
	}, serverCertDER, caCertDER
}

func VerifyServerCertificate(rawCerts [][]byte, _ [][]*x509.Certificate) error {
	for i := 0; i < len(rawCerts); i++ {
		_, err := x509.ParseCertificate(rawCerts[i])
		if err != nil {
			continue
		}
	}

	return fmt.Errorf("insecure connection to secure service")
}

func GenerateClientTLSConfig(host string) *tls.Config {
	return &tls.Config{
		// #nosec G402 -- InsecureSkipVerify is set true in test context only; production usage is config-driven.
		InsecureSkipVerify:    true, //NOSONAR
		VerifyPeerCertificate: VerifyServerCertificate,
		NextProtos:            []string{"netceptor"},
		ServerName:            host,
		MinVersion:            tls.VersionTLS12,
	}
}
