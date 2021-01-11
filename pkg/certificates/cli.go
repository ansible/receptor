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
	OutFile    string `description:"File to save the certificate and key to" required:"Yes"`
}

func (ica initCA) Run() error {
	opts := CertOptions{
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
		err = ca.SaveToFile(ica.OutFile)
	}
	return err
}

type makeReq struct {
	CommonName string   `description:"Common name to assign to the certificate" required:"Yes"`
	Bits       int      `description:"Bit length of the encryption keys of the certificate" required:"Yes"`
	DNSName    []string `description:"DNS names to add to the certificate"`
	IPAddress  []string `description:"IP addresses to add to the certificate"`
	NodeID     []string `description:"Receptor node IDs to add to the certificate"`
	OutFile    string   `description:"File to save the certificate and key to" required:"Yes"`
}

func (mr makeReq) Run() error {
	opts := CertOptions{
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
	req, err := CreateCertReq(opts)
	if err == nil {
		err = req.SaveToFile(mr.OutFile)
	}
	return err
}

type signReq struct {
	Req       string `description:"Certificate Request PEM filename" required:"Yes"`
	CA        string `description:"Certificate Authority PEM filename" required:"Yes"`
	NotBefore string `description:"Effective (NotBefore) date/time, in RFC3339 format"`
	NotAfter  string `description:"Expiration (NotAfter) date/time, in RFC3339 format"`
	OutFile   string `description:"File to save the signed certificate to" required:"Yes"`
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
	err := ca.LoadFromFile(sr.CA)
	if err != nil {
		return err
	}
	req := &ReqKeyPair{}
	err = req.LoadFromFile(sr.Req)
	if err != nil {
		return err
	}
	var names *CertNames
	names, err = GetReqNames(req.Request)
	if err != nil {
		return err
	}
	if len(names.DNSNames) == 0 && len(names.IPAddresses) == 0 && len(names.NodeIDs) == 0 {
		return fmt.Errorf("cannot sign: no names found in certificate")
	}
	fmt.Printf("Requested certificate:\n")
	fmt.Printf("  Subject: %s\n", req.Request.Subject)
	algo := req.Request.PublicKeyAlgorithm.String()
	if algo == "RSA" {
		rpk := req.Request.PublicKey.(*rsa.PublicKey)
		algo = fmt.Sprintf("%s (%d bits)", algo, rpk.Size()*8)
	}
	fmt.Printf("  Encryption Algorithm: %s\n", algo)
	fmt.Printf("  Signature Algorithm: %s\n", req.Request.SignatureAlgorithm.String())
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
	cert, err = SignCertReq(req.Request, ca, opts)
	if err != nil {
		return err
	}
	certData := []interface{}{cert}
	return SaveToPEMFile(sr.OutFile, certData)
}

type completeReq struct {
	Req      string `description:"Certificate Request PEM filename" required:"Yes"`
	Response string `description:"Signed certificate from CA" required:"Yes"`
	OutFile  string `description:"File to save the signed certificate to" required:"Yes"`
}

func (sr completeReq) Run() error {
	req := &ReqKeyPair{}
	err := req.LoadFromFile(sr.Req)
	if err != nil {
		return err
	}
	var certData []interface{}
	certData, err = LoadFromPEMFile(sr.Response)
	if len(certData) != 1 {
		return fmt.Errorf("response file should contain only one item")
	}
	cert, ok := certData[0].(*x509.Certificate)
	if !ok {
		return fmt.Errorf("response file should contain a certificate")
	}
	certPair := CertKeyPair{
		Certificate: cert,
		PrivateKey:  req.PrivateKey,
	}
	return certPair.SaveToFile(sr.OutFile)
}

func init() {
	cmdline.AddConfigType("cert-init", "Initialize PKI CA", initCA{}, cmdline.Exclusive, cmdline.Section(certSection))
	cmdline.AddConfigType("cert-makereq", "Create certificate request", makeReq{}, cmdline.Exclusive, cmdline.Section(certSection))
	cmdline.AddConfigType("cert-signreq", "Sign request and produce certificate", signReq{}, cmdline.Exclusive, cmdline.Section(certSection))
	cmdline.AddConfigType("cert-complete", "Complete a request based on cert returned from CA", completeReq{}, cmdline.Exclusive, cmdline.Section(certSection))
}
