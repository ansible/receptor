// +build !no_tls_config

package netceptor

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"

	"github.com/ghjm/cmdline"
)

// **************************************************************************
// Command line
// **************************************************************************

var configSection = &cmdline.ConfigSection{
	Description: "Commands that configure resources used by other commands:",
	Order:       5,
}

// tlsServerCfg stores the configuration options for a TLS server
type tlsServerCfg struct {
	Name              string `required:"true" description:"Name of this TLS server configuration"`
	Cert              string `required:"true" description:"Server certificate filename"`
	Key               string `required:"true" description:"Server private key filename"`
	RequireClientCert bool   `description:"Require client certificates" default:"false"`
	ClientCAs         string `description:"Filename of CA bundle to verify client certs with"`
}

// Prepare creates the tls.config and stores it in the global map
func (cfg tlsServerCfg) Prepare() error {
	tlscfg := &tls.Config{}

	certbytes, err := ioutil.ReadFile(cfg.Cert)
	if err != nil {
		return err
	}
	keybytes, err := ioutil.ReadFile(cfg.Key)
	if err != nil {
		return err
	}
	cert, err := tls.X509KeyPair(certbytes, keybytes)
	if err != nil {
		return err
	}

	tlscfg.Certificates = []tls.Certificate{cert}

	if cfg.ClientCAs != "" {
		bytes, err := ioutil.ReadFile(cfg.ClientCAs)
		if err != nil {
			return fmt.Errorf("error reading client CAs file: %w", err)
		}
		clientCAs := x509.NewCertPool()
		clientCAs.AppendCertsFromPEM(bytes)
		tlscfg.ClientCAs = clientCAs
	}

	if cfg.RequireClientCert {
		tlscfg.ClientAuth = tls.RequireAndVerifyClientCert
	} else if cfg.ClientCAs != "" {
		tlscfg.ClientAuth = tls.VerifyClientCertIfGiven
	} else {
		tlscfg.ClientAuth = tls.NoClientCert
	}

	return MainInstance.SetServerTLSConfig(cfg.Name, tlscfg)
}

// tlsClientConfig stores the configuration options for a TLS client
type tlsClientConfig struct {
	Name               string `required:"true" description:"Name of this TLS client configuration"`
	Cert               string `required:"false" description:"Client certificate filename"`
	Key                string `required:"false" description:"Client private key filename"`
	RootCAs            string `required:"false" description:"Root CA bundle to use instead of system trust"`
	InsecureSkipVerify bool   `required:"false" description:"Accept any server cert" default:"false"`
}

// Prepare creates the tls.config and stores it in the global map
func (cfg tlsClientConfig) Prepare() error {
	tlscfg := &tls.Config{}

	if cfg.Cert != "" || cfg.Key != "" {
		if cfg.Cert == "" || cfg.Key == "" {
			return fmt.Errorf("cert and key must both be supplied or neither")
		}
		certbytes, err := ioutil.ReadFile(cfg.Cert)
		if err != nil {
			return err
		}
		keybytes, err := ioutil.ReadFile(cfg.Key)
		if err != nil {
			return err
		}
		cert, err := tls.X509KeyPair(certbytes, keybytes)
		if err != nil {
			return err
		}
		tlscfg.Certificates = []tls.Certificate{cert}
	}

	if cfg.RootCAs != "" {
		bytes, err := ioutil.ReadFile(cfg.RootCAs)
		if err != nil {
			return fmt.Errorf("error reading root CAs file: %w", err)
		}

		rootCAs := x509.NewCertPool()
		rootCAs.AppendCertsFromPEM(bytes)
		tlscfg.RootCAs = rootCAs
	}

	tlscfg.InsecureSkipVerify = cfg.InsecureSkipVerify
	return MainInstance.SetClientTLSConfig(cfg.Name, tlscfg)
}

func init() {
	cmdline.RegisterConfigTypeForApp("receptor-tls",
		"tls-server", "Define a TLS server configuration", tlsServerCfg{}, cmdline.Section(configSection))
	cmdline.RegisterConfigTypeForApp("receptor-tls",
		"tls-client", "Define a TLS client configuration", tlsClientConfig{}, cmdline.Section(configSection))
}
