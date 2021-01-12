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
	"github.com/project-receptor/receptor/pkg/utils"
	"io/ioutil"
	"math/big"
	"net"
	"strings"
	"time"
)

// CertNames lists the subjectAltNames that can be assigned to a certificate or request
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
func LoadFromPEMFile(filename string) ([]interface{}, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	results := make([]interface{}, 0)
	var block *pem.Block
	for len(content) > 0 {
		block, content = pem.Decode(content)
		if block.Type == "CERTIFICATE" {
			var cert *x509.Certificate
			cert, err = x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, err
			}
			results = append(results, cert)
		} else if block.Type == "CERTIFICATE REQUEST" {
			var req *x509.CertificateRequest
			req, err = x509.ParseCertificateRequest(block.Bytes)
			if err != nil {
				return nil, err
			}
			results = append(results, req)
		} else if block.Type == "RSA PRIVATE KEY" {
			var key *rsa.PrivateKey
			key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
			if err != nil {
				return nil, err
			}
			results = append(results, key)
		} else {
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
		var key *rsa.PrivateKey
		key, ok = elem.(*rsa.PrivateKey)
		if ok {
			keyPEM := new(bytes.Buffer)
			err = pem.Encode(keyPEM, &pem.Block{
				Type:  "RSA PRIVATE KEY",
				Bytes: x509.MarshalPKCS1PrivateKey(key),
			})
			if err != nil {
				return err
			}
			content = append(content, keyPEM.String())
			continue
		}
	}
	return ioutil.WriteFile(filename, []byte(strings.Join(content, "\n")), 0600)
}

// CertKeyPair contains a certificate and private key.
type CertKeyPair struct {
	Certificate *x509.Certificate
	PrivateKey  *rsa.PrivateKey
}

// SaveToFile saves the certificate and private key to a PEM file.
func (ckp *CertKeyPair) SaveToFile(filename string) error {
	data := []interface{}{ckp.Certificate, ckp.PrivateKey}
	return SaveToPEMFile(filename, data)
}

// LoadFromFile loads the certificate and private key from a PEM file.
func (ckp *CertKeyPair) LoadFromFile(filename string) error {
	data, err := LoadFromPEMFile(filename)
	if err != nil {
		return err
	}
	var newCert *x509.Certificate
	var newKey *rsa.PrivateKey
	var ok bool
	for _, elem := range data {
		var cert *x509.Certificate
		cert, ok = elem.(*x509.Certificate)
		if ok {
			if newCert != nil {
				return fmt.Errorf("PEM file should only contain one certificate")
			}
			newCert = cert
		}
		var key *rsa.PrivateKey
		key, ok = elem.(*rsa.PrivateKey)
		if ok {
			if newKey != nil {
				return fmt.Errorf("PEM file should only contain one private key")
			}
			newKey = key
		}
	}
	if newCert == nil {
		return fmt.Errorf("PEM file did not include a certificate")
	}
	if newKey == nil {
		return fmt.Errorf("PEM file did not include a private key")
	}
	ckp.Certificate = newCert
	ckp.PrivateKey = newKey
	return nil
}

// ReqKeyPair contains a certificate request and private key.
type ReqKeyPair struct {
	Request    *x509.CertificateRequest
	PrivateKey *rsa.PrivateKey
}

// SaveToFile saves the certificate request and private key to a PEM file.
func (rkp *ReqKeyPair) SaveToFile(filename string) error {
	data := []interface{}{rkp.Request, rkp.PrivateKey}
	return SaveToPEMFile(filename, data)
}

// LoadFromFile loads the certificate and private key from a PEM file.
func (rkp *ReqKeyPair) LoadFromFile(filename string) error {
	data, err := LoadFromPEMFile(filename)
	if err != nil {
		return err
	}
	var newReq *x509.CertificateRequest
	var newKey *rsa.PrivateKey
	var ok bool
	for _, elem := range data {
		var req *x509.CertificateRequest
		req, ok = elem.(*x509.CertificateRequest)
		if ok {
			if newReq != nil {
				return fmt.Errorf("PEM file should only contain one certificate request")
			}
			newReq = req
		}
		var key *rsa.PrivateKey
		key, ok = elem.(*rsa.PrivateKey)
		if ok {
			if newKey != nil {
				return fmt.Errorf("PEM file should only contain one private key")
			}
			newKey = key
		}
	}
	if newReq == nil {
		return fmt.Errorf("PEM file did not include a certificate request")
	}
	if newKey == nil {
		return fmt.Errorf("PEM file did not include a private key")
	}
	rkp.Request = newReq
	rkp.PrivateKey = newKey
	return nil
}

// CA contains internal data for a certificate authority.
type CA struct {
	CertKeyPair
}

// CreateCA initializes a new CertKeyPair from given parameters.
func CreateCA(opts CertOptions) (*CA, error) {
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
	ca.PrivateKey, err = rsa.GenerateKey(rand.Reader, opts.Bits)
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

// CreateCertReq creates a new x.509 certificate request, potentially containing Receptor node ID names.
func CreateCertReq(opts CertOptions) (*ReqKeyPair, error) {
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

	var reqPrivKey *rsa.PrivateKey
	reqPrivKey, err = rsa.GenerateKey(rand.Reader, opts.Bits)
	if err != nil {
		return nil, err
	}

	var reqBytes []byte
	reqBytes, err = x509.CreateCertificateRequest(rand.Reader, reqTemplate, reqPrivKey)
	if err != nil {
		return nil, err
	}

	var req *x509.CertificateRequest
	req, err = x509.ParseCertificateRequest(reqBytes)
	if err != nil {
		return nil, err
	}

	return &ReqKeyPair{
		Request:    req,
		PrivateKey: reqPrivKey,
	}, nil
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
