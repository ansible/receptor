package main

import (
	"fmt"
	"github.com/project-receptor/receptor/pkg/certificates"
	"github.com/project-receptor/receptor/pkg/cmdline"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"time"
)

type certMaker struct {
	CommonName string `description:"Common name to assign to the certificate" required:"Yes"`
	Bits       int    `description:"Bit length of the encryption keys of the certificate" required:"Yes"`
	NotBefore  string `description:"Effective (NotBefore) date/time, in RFC3339 format"`
	NotAfter   string `description:"Expiration (NotAfter) date/time, in RFC3339 format"`
	OutFile    string `description:"File to save the certificate and key to" required:"Yes"`
}

type initCA struct {
	certMaker
}

func (c certMaker) prepOpts() (certificates.CertOptions, error) {
	opts := certificates.CertOptions{
		CommonName: c.CommonName,
		Bits:       c.Bits,
	}
	if c.NotBefore != "" {
		t, err := time.Parse(time.RFC3339, c.NotBefore)
		if err != nil {
			return opts, err
		}
		opts.NotBefore = t
	}
	if c.NotAfter != "" {
		t, err := time.Parse(time.RFC3339, c.NotAfter)
		if err != nil {
			return opts, err
		}
		opts.NotAfter = t
	}
	return opts, nil
}

func (c certMaker) saveCert(cert string, key string) error {
	if c.OutFile == "-" {
		fmt.Printf("%s\n%s\n", cert, key)
	} else {
		content := strings.Join([]string{cert, key}, "\n")
		return ioutil.WriteFile(c.OutFile, []byte(content), 0600)
	}
	return nil
}

func (ica initCA) Run() error {
	opts, err := ica.prepOpts()
	var cert, key string
	if err == nil {
		cert, key, err = certificates.CreateCA(opts)
	}
	if err == nil {
		err = ica.saveCert(cert, key)
	}
	return err
}

type makeReq struct {
	certMaker
	DNSName   []string `description:"DNS names to add to the certificate"`
	IPAddress []string `description:"IP addresses to add to the certificate"`
	NodeID    []string `description:"Receptor node IDs to add to the certificate"`
}

func (mr makeReq) Run() error {
	opts, err := mr.prepOpts()
	if err != nil {
		return err
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
	cert, key, err := certificates.CreateCertReq(opts)
	if err == nil {
		err = mr.saveCert(cert, key)
	}
	return err
}

func main() {
	if len(os.Args) <= 1 {
		fmt.Printf("No command specified.\n")
		os.Exit(1)
	}
	cmdline.AddConfigType("init-ca", "Initialize PKI CA", initCA{}, cmdline.Singleton)
	cmdline.AddConfigType("make-req", "Create signed Receptor node certificate", makeReq{}, cmdline.Singleton)
	cmdline.ParseAndRun(os.Args[1:], []string{"Run"})
}
