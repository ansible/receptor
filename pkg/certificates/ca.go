//go:build !no_cert_auth
// +build !no_cert_auth

package certificates

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"strings"
	"time"

	"github.com/ansible/receptor/pkg/utils"
)

type Oser interface {
	ReadFile(name string) ([]byte, error)
}

type OsWrapper struct{}

func (ow *OsWrapper) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

type Rsaer interface {
	GenerateKey(random io.Reader, bits int) (*rsa.PrivateKey, error)
}

type RsaWrapper struct{}

func (rw *RsaWrapper) GenerateKey(random io.Reader, bits int) (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(random, bits)
}

// CertNames lists the subjectAltNames that can be assigned to a certificate or request.
type CertNames struct {
	DNSNames    []string
	NodeIDs     []string
	IPAddresses []net.IP
}

// CertOptions are the parameters used to initialize a new certificate or request.
type CertOptions struct {
	CertNames
	CommonName string
	Bits       int
	NotBefore  time.Time
	NotAfter   time.Time
}

// LoadFromPEMFile loads certificate data from a PEM file.
func LoadFromPEMFile(filename string, osWrapper Oser) ([]interface{}, error) {
	content, err := Oser.ReadFile(osWrapper, filename)
	if err != nil {
		return nil, err
	}
	results := make([]interface{}, 0)
	var block *pem.Block
	for len(content) > 0 {
		block, content = pem.Decode(content)
		if block == nil {
			return nil, fmt.Errorf("failed to decode PEM block")
		}
		switch block.Type {
		case "CERTIFICATE":
			var cert *x509.Certificate
			cert, err = x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, err
			}
			results = append(results, cert)
		case "CERTIFICATE REQUEST":
			var req *x509.CertificateRequest
			req, err = x509.ParseCertificateRequest(block.Bytes)
			if err != nil {
				return nil, err
			}
			results = append(results, req)
		case "RSA PRIVATE KEY":
			var key *rsa.PrivateKey
			key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
			if err != nil {
				return nil, err
			}
			results = append(results, key)
		case "PRIVATE KEY":
			key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err != nil {
				return nil, err
			}
			results = append(results, key)
		case "PUBLIC KEY":
			key, err := x509.ParsePKIXPublicKey(block.Bytes)
			if err != nil {
				return nil, err
			}
			results = append(results, key)
		default:
			return nil, fmt.Errorf("unknown block type %s", block.Type)
		}
	}

	return results, nil
}

// SaveToPEMFile saves certificate data to a PEM file.
func SaveToPEMFile(filename string, data []interface{}) error {
	var err error
	var ok bool
	content := make([]string, 0)
	for _, elem := range data {
		var cert *x509.Certificate
		cert, ok = elem.(*x509.Certificate)
		if ok {
			certPEM := new(bytes.Buffer)
			err = pem.Encode(certPEM, &pem.Block{
				Type:  "CERTIFICATE",
				Bytes: cert.Raw,
			})
			if err != nil {
				return err
			}
			content = append(content, certPEM.String())

			continue
		}
		var req *x509.CertificateRequest
		req, ok = elem.(*x509.CertificateRequest)
		if ok {
			reqPEM := new(bytes.Buffer)
			err = pem.Encode(reqPEM, &pem.Block{
				Type:  "CERTIFICATE REQUEST",
				Bytes: req.Raw,
			})
			if err != nil {
				return err
			}
			content = append(content, reqPEM.String())

			continue
		}
		var keyPrivate *rsa.PrivateKey
		keyPrivate, ok = elem.(*rsa.PrivateKey)
		if ok {
			keyPEM := new(bytes.Buffer)
			err = pem.Encode(keyPEM, &pem.Block{
				Type:  "RSA PRIVATE KEY",
				Bytes: x509.MarshalPKCS1PrivateKey(keyPrivate),
			})
			if err != nil {
				return err
			}
			content = append(content, keyPEM.String())

			continue
		}
		var keyPublic *rsa.PublicKey
		keyPublic, ok = elem.(*rsa.PublicKey)
		if ok {
			keyPEM := new(bytes.Buffer)
			keyPublicBytes, err := x509.MarshalPKIXPublicKey(keyPublic)
			if err != nil {
				return err
			}
			err = pem.Encode(keyPEM, &pem.Block{
				Type:  "PUBLIC KEY",
				Bytes: keyPublicBytes,
			})
			if err != nil {
				return err
			}
			content = append(content, keyPEM.String())

			continue
		}
	}

	return os.WriteFile(filename, []byte(strings.Join(content, "\n")), 0o600)
}

// LoadCertificate loads a single certificate from a file.
func LoadCertificate(filename string, osWrapper Oser) (*x509.Certificate, error) {
	data, err := LoadFromPEMFile(filename, osWrapper)
	if err != nil {
		return nil, err
	}
	if len(data) != 1 {
		return nil, fmt.Errorf("certificate file should contain exactly one item")
	}
	cert, ok := data[0].(*x509.Certificate)
	if !ok {
		return nil, fmt.Errorf("certificate file does not contain certificate data")
	}

	return cert, nil
}

// LoadRequest loads a single certificate request from a file.
func LoadRequest(filename string) (*x509.CertificateRequest, error) {
	data, err := LoadFromPEMFile(filename, &OsWrapper{})
	if err != nil {
		return nil, err
	}
	if len(data) != 1 {
		return nil, fmt.Errorf("certificate request file should contain exactly one item")
	}
	req, ok := data[0].(*x509.CertificateRequest)
	if !ok {
		return nil, fmt.Errorf("certificate request file does not contain certificate request data")
	}

	return req, nil
}

// LoadPrivateKey loads a single RSA private key from a file.
func LoadPrivateKey(filename string, osWrapper Oser) (*rsa.PrivateKey, error) {
	data, err := LoadFromPEMFile(filename, osWrapper)
	if err != nil {
		return nil, err
	}
	if len(data) != 1 {
		return nil, fmt.Errorf("private key file should contain exactly one item")
	}
	key, ok := data[0].(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key file does not contain private key data")
	}

	return key, nil
}

// LoadPublicKey loads a single RSA public key from a file.
func LoadPublicKey(filename string) (*rsa.PublicKey, error) {
	data, err := LoadFromPEMFile(filename, &OsWrapper{})
	if err != nil {
		return nil, err
	}
	if len(data) != 1 {
		return nil, fmt.Errorf("public key file should contain exactly one item")
	}
	key, ok := data[0].(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key file does not contain public key data")
	}

	return key, nil
}

// CA contains internal data for a certificate authority.
type CA struct {
	Certificate *x509.Certificate
	PrivateKey  *rsa.PrivateKey
}

// CreateCA initializes a new CertKeyPair from given parameters.
func CreateCA(opts *CertOptions, rsaWrapper Rsaer) (*CA, error) {
	if opts.CommonName == "" {
		return nil, fmt.Errorf("must provide CommonName")
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
		return nil, fmt.Errorf("CertKeyPair certificate cannot have DNSNames, NodeIDs or IPAddresses")
	}

	var err error
	ca := &CA{}
	ca.PrivateKey, err = rsaWrapper.GenerateKey(rand.Reader, opts.Bits)
	if err != nil {
		return nil, err
	}

	caTemplate := &x509.Certificate{
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

	caBytes, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &ca.PrivateKey.PublicKey, ca.PrivateKey)
	if err != nil {
		return nil, err
	}
	ca.Certificate, err = x509.ParseCertificate(caBytes)
	if err != nil {
		return nil, err
	}

	return ca, nil
}

// CreateCertReqWithKey creates a new x.509 certificate request with a newly generated private key.
func CreateCertReqWithKey(opts *CertOptions) (*x509.CertificateRequest, *rsa.PrivateKey, error) {
	key, err := rsa.GenerateKey(rand.Reader, opts.Bits)
	if err != nil {
		return nil, nil, err
	}
	req, err := CreateCertReq(opts, key)
	if err != nil {
		return nil, nil, err
	}

	return req, key, nil
}

// CreateCertReq creates a new x.509 certificate request for an existing private key.
func CreateCertReq(opts *CertOptions, privateKey *rsa.PrivateKey) (*x509.CertificateRequest, error) {
	if opts.CommonName == "" {
		return nil, fmt.Errorf("must provide CommonName")
	}
	if opts.Bits == 0 {
		opts.Bits = 2048
	}

	var err error
	var san *pkix.Extension
	san, err = utils.MakeReceptorSAN(opts.DNSNames, opts.IPAddresses, opts.NodeIDs)
	if err != nil {
		return nil, err
	}
	reqTemplate := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName: opts.CommonName,
		},
		ExtraExtensions: []pkix.Extension{*san},
	}

	var reqBytes []byte
	reqBytes, err = x509.CreateCertificateRequest(rand.Reader, reqTemplate, privateKey)
	if err != nil {
		return nil, err
	}

	var req *x509.CertificateRequest
	req, err = x509.ParseCertificateRequest(reqBytes)
	if err != nil {
		return nil, err
	}

	return req, nil
}

// GetReqNames returns the names coded into a certificate request, including Receptor node IDs.
func GetReqNames(request *x509.CertificateRequest) (*CertNames, error) {
	nodeIDs, err := utils.ReceptorNames(request.Extensions)
	if err != nil {
		return nil, err
	}
	cn := &CertNames{
		DNSNames:    request.DNSNames,
		NodeIDs:     nodeIDs,
		IPAddresses: request.IPAddresses,
	}

	return cn, nil
}

// SignCertReq signs a certificate request using a CA key.
func SignCertReq(req *x509.CertificateRequest, ca *CA, opts *CertOptions) (*x509.Certificate, error) {
	if opts.NotBefore.IsZero() {
		opts.NotBefore = time.Now()
	}
	if opts.NotAfter.IsZero() {
		opts.NotAfter = time.Now().AddDate(1, 0, 0)
	}
	certTemplate := &x509.Certificate{
		SerialNumber:       big.NewInt(time.Now().Unix()),
		Signature:          req.Signature,
		SignatureAlgorithm: req.SignatureAlgorithm,
		PublicKey:          req.PublicKey,
		PublicKeyAlgorithm: req.PublicKeyAlgorithm,
		Issuer:             ca.Certificate.Subject,
		Subject:            req.Subject,
		NotBefore:          opts.NotBefore,
		NotAfter:           opts.NotAfter,
		IsCA:               false,
		KeyUsage:           x509.KeyUsageDigitalSignature,
		ExtKeyUsage:        []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	}
	found := false
	for _, ext := range req.Extensions {
		if ext.Id.Equal(utils.OIDSubjectAltName) {
			certTemplate.ExtraExtensions = []pkix.Extension{ext}
			found = true

			break
		}
	}
	if !found {
		certTemplate.DNSNames = req.DNSNames
		certTemplate.IPAddresses = req.IPAddresses
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, certTemplate, ca.Certificate, req.PublicKey, ca.PrivateKey)
	if err != nil {
		return nil, err
	}
	var cert *x509.Certificate
	cert, err = x509.ParseCertificate(certBytes)
	if err != nil {
		return nil, err
	}

	return cert, nil
}
