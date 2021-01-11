// +build !no_cert_auth

package certificates

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"github.com/project-receptor/receptor/pkg/cmdline"
	"net"
	"strings"
	"time"
)

type initCA struct {
	CommonName string `description:"Common name to assign to the certificate" required:"Yes"`
	Bits       int    `description:"Bit length of the encryption keys of the certificate" required:"Yes"`
	NotBefore  string `description:"Effective (NotBefore) date/time, in RFC3339 format"`
	NotAfter   string `description:"Expiration (NotAfter) date/time, in RFC3339 format"`
	OutCert    string `description:"File to save the CA certificate to" required:"Yes"`
	OutKey     string `description:"File to save the CA private key to" required:"Yes"`
}

func (ica initCA) Run() error {
	opts := &CertOptions{
		CommonName: ica.CommonName,
		Bits:       ica.Bits,
	}
	if ica.NotBefore != "" {
		t, err := time.Parse(time.RFC3339, ica.NotBefore)
		if err != nil {
			return err
		}
		opts.NotBefore = t
	}
	if ica.NotAfter != "" {
		t, err := time.Parse(time.RFC3339, ica.NotAfter)
		if err != nil {
			return err
		}
		opts.NotAfter = t
	}
	ca, err := CreateCA(opts)
	if err == nil {
		err = SaveToPEMFile(ica.OutCert, []interface{}{ca.Certificate})
	}
	if err == nil {
		err = SaveToPEMFile(ica.OutKey, []interface{}{ca.PrivateKey})
	}
	return err
}

type makeReq struct {
	CommonName string   `description:"Common name to assign to the certificate" required:"Yes"`
	Bits       int      `description:"Bit length of the encryption keys of the certificate" required:"Yes"`
	DNSName    []string `description:"DNS names to add to the certificate"`
	IPAddress  []string `description:"IP addresses to add to the certificate"`
	NodeID     []string `description:"Receptor node IDs to add to the certificate"`
	OutReq     string   `description:"File to save the certificate request to" required:"Yes"`
	OutKey     string   `description:"File to save the private key to" required:"Yes"`
}

func (mr makeReq) Run() error {
	opts := &CertOptions{
		CommonName: mr.CommonName,
		Bits:       mr.Bits,
	}
	opts.DNSNames = mr.DNSName
	opts.NodeIDs = mr.NodeID
	for _, ipstr := range mr.IPAddress {
		ip := net.ParseIP(ipstr)
		if ip == nil {
			return fmt.Errorf("invalid IP address: %s", ipstr)
		}
		if opts.IPAddresses == nil {
			opts.IPAddresses = make([]net.IP, 0)
		}
		opts.IPAddresses = append(opts.IPAddresses, ip)
	}
	req, key, err := CreateCertReq(opts)
	if err == nil {
		err = SaveToPEMFile(mr.OutReq, []interface{}{req})
	}
	if err == nil {
		err = SaveToPEMFile(mr.OutKey, []interface{}{key})
	}
	return err
}

type signReq struct {
	Req       string `description:"Certificate Request PEM filename" required:"Yes"`
	CACert    string `description:"CA certificate PEM filename" required:"Yes"`
	CAKey     string `description:"CA private key PEM filename" required:"Yes"`
	NotBefore string `description:"Effective (NotBefore) date/time, in RFC3339 format"`
	NotAfter  string `description:"Expiration (NotAfter) date/time, in RFC3339 format"`
	OutCert   string `description:"File to save the signed certificate to" required:"Yes"`
}

func (sr signReq) Run() error {
	opts := &CertOptions{}
	if sr.NotBefore != "" {
		t, err := time.Parse(time.RFC3339, sr.NotBefore)
		if err != nil {
			return err
		}
		opts.NotBefore = t
	}
	if sr.NotAfter != "" {
		t, err := time.Parse(time.RFC3339, sr.NotAfter)
		if err != nil {
			return err
		}
		opts.NotAfter = t
	}
	ca := &CA{}
	var err error
	ca.Certificate, err = LoadCertificate(sr.CACert)
	if err != nil {
		return err
	}
	ca.PrivateKey, err = LoadPrivateKey(sr.CAKey)
	if err != nil {
		return err
	}
	var req *x509.CertificateRequest
	req, err = LoadRequest(sr.Req)
	if err != nil {
		return err
	}
	var names *CertNames
	names, err = GetReqNames(req)
	if err != nil {
		return err
	}
	if len(names.DNSNames) == 0 && len(names.IPAddresses) == 0 && len(names.NodeIDs) == 0 {
		return fmt.Errorf("cannot sign: no names found in certificate")
	}
	fmt.Printf("Requested certificate:\n")
	fmt.Printf("  Subject: %s\n", req.Subject)
	algo := req.PublicKeyAlgorithm.String()
	if algo == "RSA" {
		rpk := req.PublicKey.(*rsa.PublicKey)
		algo = fmt.Sprintf("%s (%d bits)", algo, rpk.Size()*8)
	}
	fmt.Printf("  Encryption Algorithm: %s\n", algo)
	fmt.Printf("  Signature Algorithm: %s\n", req.SignatureAlgorithm.String())
	fmt.Printf("  Names:\n")
	for _, name := range names.DNSNames {
		fmt.Printf("    DNS Name: %s\n", name)
	}
	for _, ip := range names.IPAddresses {
		fmt.Printf("    IP Address: %v\n", ip)
	}
	for _, node := range names.NodeIDs {
		fmt.Printf("    Receptor Node ID: %s\n", node)
	}
	fmt.Printf("Sign certificate (yes/no)? ")
	var response string
	_, err = fmt.Scanln(&response)
	if err != nil {
		return err
	}
	response = strings.ToLower(response)
	if response != "y" && response != "yes" {
		return fmt.Errorf("user declined")
	}
	var cert *x509.Certificate
	cert, err = SignCertReq(req, ca, opts)
	if err != nil {
		return err
	}
	return SaveToPEMFile(sr.OutCert, []interface{}{cert})
}

func init() {
	cmdline.AddConfigType("cert-init", "Initialize PKI CA", initCA{}, cmdline.Exclusive, cmdline.Section(certSection))
	cmdline.AddConfigType("cert-makereq", "Create certificate request", makeReq{}, cmdline.Exclusive, cmdline.Section(certSection))
	cmdline.AddConfigType("cert-signreq", "Sign request and produce certificate", signReq{}, cmdline.Exclusive, cmdline.Section(certSection))
}
