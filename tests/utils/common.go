package utils

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/ansible/receptor/pkg/certificates"
)

// GenerateCA generates a CA certificate and key.
func GenerateCA(name, commonName string) (string, string, error) {
	dir, err := os.MkdirTemp("", "")
	if err != nil {
		return "", "", err
	}

	keyPath := filepath.Join(dir, name+".key")
	crtPath := filepath.Join(dir, name+".crt")

	// Create our certificate and private key
	CA, err := certificates.CreateCA(&certificates.CertOptions{CommonName: commonName, Bits: 2048}, &certificates.RsaWrapper{})
	if err != nil {
		return "", "", err
	}
	err = certificates.SaveToPEMFile(crtPath, []interface{}{CA.Certificate}, &certificates.OsWrapper{})
	if err != nil {
		return "", "", err
	}
	err = certificates.SaveToPEMFile(keyPath, []interface{}{CA.PrivateKey}, &certificates.OsWrapper{})
	if err != nil {
		return "", "", err
	}

	return keyPath, crtPath, nil
}

// GenerateCert generates a private and public key for testing in the directory specified.
func GenerateCert(name, commonName string, dnsNames, nodeIDs []string) (string, string, error) {
	dir, err := os.MkdirTemp("", "")
	if err != nil {
		return "", "", err
	}
	keyPath := filepath.Join(dir, name+".key")
	crtPath := filepath.Join(dir, name+".crt")

	// Create a temporary CA to sign this certificate
	CA, err := certificates.CreateCA(&certificates.CertOptions{CommonName: "temp ca", Bits: 2048}, &certificates.RsaWrapper{})
	if err != nil {
		return "", "", err
	}
	// Create certificate request
	req, key, err := certificates.CreateCertReqWithKey(&certificates.CertOptions{
		CommonName: commonName,
		Bits:       2048,
		CertNames: certificates.CertNames{
			DNSNames: dnsNames,
			NodeIDs:  nodeIDs,
		},
	})
	if err != nil {
		return "", "", err
	}
	// Sign the certificate
	cert, err := certificates.SignCertReq(req, CA, &certificates.CertOptions{})
	if err != nil {
		return "", "", err
	}
	// Save cert and key to files
	err = certificates.SaveToPEMFile(crtPath, []interface{}{cert}, &certificates.OsWrapper{})
	if err != nil {
		return "", "", err
	}
	err = certificates.SaveToPEMFile(keyPath, []interface{}{key}, &certificates.OsWrapper{})
	if err != nil {
		return "", "", err
	}

	return keyPath, crtPath, nil
}

// GenerateCertWithCA generates a private and public key for testing in the directory
// specified using the ca specified.
func GenerateCertWithCA(name, caKeyPath, caCrtPath, commonName string, dnsNames, nodeIDs []string) (string, string, error) {
	dir, err := os.MkdirTemp("", "")
	if err != nil {
		return "", "", err
	}
	CA := &certificates.CA{}
	CA.Certificate, err = certificates.LoadCertificate(caCrtPath, &certificates.OsWrapper{})
	if err != nil {
		return "", "", err
	}
	CA.PrivateKey, err = certificates.LoadPrivateKey(caKeyPath, &certificates.OsWrapper{})
	if err != nil {
		return "", "", err
	}
	keyPath := filepath.Join(dir, name+".key")
	crtPath := filepath.Join(dir, name+".crt")
	// Create certificate request
	req, key, err := certificates.CreateCertReqWithKey(&certificates.CertOptions{
		CommonName: commonName,
		Bits:       2048,
		CertNames: certificates.CertNames{
			DNSNames: dnsNames,
			NodeIDs:  nodeIDs,
		},
	})
	if err != nil {
		return "", "", err
	}
	// Sign the certificate
	cert, err := certificates.SignCertReq(req, CA, &certificates.CertOptions{})
	if err != nil {
		return "", "", err
	}
	// Save cert and key to files
	err = certificates.SaveToPEMFile(crtPath, []interface{}{cert}, &certificates.OsWrapper{})
	if err != nil {
		return "", "", err
	}
	err = certificates.SaveToPEMFile(keyPath, []interface{}{key}, &certificates.OsWrapper{})
	if err != nil {
		return "", "", err
	}

	return keyPath, crtPath, nil
}

func GenerateRSAPair() (string, string, error) {
	dir, err := os.MkdirTemp("", "")
	if err != nil {
		return "", "", err
	}
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}
	publicKey := &privateKey.PublicKey

	privateKeyPath := filepath.Join(dir, "private.pem")
	err = certificates.SaveToPEMFile(privateKeyPath, []interface{}{privateKey}, &certificates.OsWrapper{})
	if err != nil {
		return "", "", err
	}

	publicKeyPath := filepath.Join(dir, "public.pem")
	err = certificates.SaveToPEMFile(publicKeyPath, []interface{}{publicKey}, &certificates.OsWrapper{})
	if err != nil {
		return "", "", err
	}

	return privateKeyPath, publicKeyPath, nil
}

// CheckUntilTimeout Polls the check function until the context expires, in
// which case it returns false.
func CheckUntilTimeout(ctx context.Context, interval time.Duration, check func() bool) bool {
	for ready := check(); !ready; ready = check() {
		if ctx.Err() != nil {
			return false
		}
		time.Sleep(interval)
	}

	return true
}

// CheckUntilTimeoutWithErr does the same as CheckUntilTimeout but requires the
// check function returns (bool, error), and will return an error immediately
// if the check function returns an error.
func CheckUntilTimeoutWithErr(ctx context.Context, interval time.Duration, check func() (bool, error)) (bool, error) {
	for ready, err := check(); !ready; ready, err = check() {
		if err != nil {
			return false, err
		}
		if ctx.Err() != nil {
			return false, nil //nolint:nilerr // Make this nice later.
		}
		time.Sleep(interval)
	}

	return true, nil
}

func GetFreeTCPPort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()

	return l.Addr().(*net.TCPAddr).Port, nil
}
