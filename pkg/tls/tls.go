package tls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
)

type ServerConf struct {
	// Path to file containing certificate.
	Cert string `mapstructure:"cert"`
	// Path to file containing key.
	Key string `mapstructure:"key"`
	// Path to file containing CA for client validation.
	CA string `mapstructure:"ca"`
	// Do not verify clients.
	SkipVerify bool `mapstructure:"insecure-no-verify"`
}

func (c ServerConf) TLSConfig() (*tls.Config, error) {
	tlscfg := &tls.Config{
		ClientAuth: tls.RequireAndVerifyClientCert,
	}

	if c.SkipVerify {
		tlscfg.ClientAuth = tls.NoClientCert
	} else {
		bytes, err := ioutil.ReadFile(c.CA)
		if err != nil {
			return nil, fmt.Errorf("error reading client CAs file: %s", err)
		}
		clientCAs := x509.NewCertPool()
		clientCAs.AppendCertsFromPEM(bytes)
		tlscfg.ClientCAs = clientCAs
	}

	certbytes, err := ioutil.ReadFile(c.Cert)
	if err != nil {
		return nil, err
	}
	keybytes, err := ioutil.ReadFile(c.Key)
	if err != nil {
		return nil, err
	}
	cert, err := tls.X509KeyPair(certbytes, keybytes)
	if err != nil {
		return nil, err
	}

	tlscfg.Certificates = []tls.Certificate{cert}

	return tlscfg, nil
}

type ClientConf struct {
	// Path to file containing certificate.
	Cert string `mapstructure:"cert"`
	// Path to file containing key.
	Key string `mapstructure:"key"`
	// Path to file containing CA for server validation.
	CA string `mapstructure:"ca"`
	// Do not verify server.
	SkipVerify bool `mapstructure:"insecure-no-verify"`
}

func (c ClientConf) TLSConfig() (*tls.Config, error) {
	tlscfg := &tls.Config{}

	if c.SkipVerify {
		tlscfg.InsecureSkipVerify = true
	} else {
		bytes, err := ioutil.ReadFile(c.CA)
		if err != nil {
			return nil, fmt.Errorf("error reading root CAs file: %s", err)
		}

		rootCAs := x509.NewCertPool()
		rootCAs.AppendCertsFromPEM(bytes)
		tlscfg.RootCAs = rootCAs
	}

	if (c.Cert == "") != (c.Key == "") {
		return nil, fmt.Errorf("cert and key must both be supplied or neither")
	}

	if c.Cert != "" {
		certbytes, err := ioutil.ReadFile(c.Cert)
		if err != nil {
			return nil, err
		}
		keybytes, err := ioutil.ReadFile(c.Key)
		if err != nil {
			return nil, err
		}
		cert, err := tls.X509KeyPair(certbytes, keybytes)
		if err != nil {
			return nil, err
		}
		tlscfg.Certificates = []tls.Certificate{cert}
	}

	return tlscfg, nil
}

// Alias some contents of crypto/tls to avoid import madness.

type (
	Config = tls.Config
	Conn   = tls.Conn
)

var (
	NewListener    = tls.NewListener
	Dial           = tls.Dial
	DialWithDialer = tls.DialWithDialer
)
