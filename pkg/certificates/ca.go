package certificates

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"github.com/project-receptor/receptor/pkg/utils"
	"math/big"
	"net"
	"time"
)

// CertOptions are the parameters used to initialize a new CA.
type CertOptions struct {
	CommonName  string
	Bits        int
	NotBefore   time.Time
	NotAfter    time.Time
	DNSNames    []string
	NodeIDs     []string
	IPAddresses []net.IP
}

// CreateCA initializes a new CA from given parameters.
func CreateCA(opts CertOptions) (string, string, error) {
	if opts.CommonName == "" {
		return "", "", fmt.Errorf("must provide CommonName")
	}
	if opts.Bits == 0 {
		opts.Bits = 2048
	}
	if opts.NotBefore.IsZero() {
		opts.NotBefore = time.Now()
	}
	if opts.NotAfter.IsZero() {
		opts.NotAfter = time.Now().AddDate(10, 0, 0)
	}
	if opts.DNSNames != nil || opts.NodeIDs != nil || opts.IPAddresses != nil {
		return "", "", fmt.Errorf("CA certificate cannot have DNSNames, NodeIDs or IPAddresses")
	}

	ca := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().Unix()),
		Subject: pkix.Name{
			CommonName: opts.CommonName,
		},
		NotBefore:             opts.NotBefore,
		NotAfter:              opts.NotAfter,
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caPrivKey, err := rsa.GenerateKey(rand.Reader, opts.Bits)
	if err != nil {
		return "", "", err
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return "", "", err
	}

	caPEM := new(bytes.Buffer)
	err = pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})
	if err != nil {
		return "", "", err
	}

	caPrivKeyPEM := new(bytes.Buffer)
	err = pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
	})
	if err != nil {
		return "", "", err
	}

	return caPEM.String(), caPrivKeyPEM.String(), nil
}

// CreateCertReq creates a new (unsigned) Receptor certificate from given parameters.
func CreateCertReq(opts CertOptions) (string, string, error) {
	if opts.CommonName == "" {
		return "", "", fmt.Errorf("must provide CommonName")
	}
	if opts.Bits == 0 {
		opts.Bits = 2048
	}
	if opts.NotBefore.IsZero() {
		opts.NotBefore = time.Now()
	}
	if opts.NotAfter.IsZero() {
		opts.NotAfter = time.Now().AddDate(1, 0, 0)
	}

	san, err := utils.MakeReceptorSAN(opts.DNSNames, opts.IPAddresses, opts.NodeIDs)
	if err != nil {
		return "", "", err
	}
	req := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName: opts.CommonName,
		},
		ExtraExtensions: []pkix.Extension{*san},
	}

	reqPrivKey, err := rsa.GenerateKey(rand.Reader, opts.Bits)
	if err != nil {
		return "", "", err
	}

	reqBytes, err := x509.CreateCertificateRequest(rand.Reader, req, reqPrivKey)
	if err != nil {
		return "", "", err
	}

	reqPEM := new(bytes.Buffer)
	err = pem.Encode(reqPEM, &pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: reqBytes,
	})
	if err != nil {
		return "", "", err
	}

	reqPrivKeyPEM := new(bytes.Buffer)
	err = pem.Encode(reqPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(reqPrivKey),
	})
	if err != nil {
		return "", "", err
	}

	return reqPEM.String(), reqPrivKeyPEM.String(), nil
}
