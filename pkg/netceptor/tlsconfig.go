// +build !no_tls_config

package netceptor

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/ghjm/cmdline"
	"io/ioutil"
)

// **************************************************************************
// Command line
// **************************************************************************

var configSection = &cmdline.ConfigSection{
	Description: "Commands that configure resources used by other commands:",
	Order:       5,
}

// TLSServerCfg stores the configuration options for a TLS server
type TLSServerCfg struct {
	Name              string `required:"true" description:"Name of this TLS server configuration"`
	Cert              string `required:"true" description:"Server certificate filename"`
	Key               string `required:"true" description:"Server private key filename"`
	RequireClientCert bool   `description:"Require client certificates" default:"false"`
	ClientCAs         string `description:"Filename of CA bundle to verify client certs with"`
}

// Prepare creates the tls.config and stores it in the global map
func (cfg TLSServerCfg) Prepare() error {
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
			return fmt.Errorf("error reading client CAs file: %s", err)
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

// TLSClientCfg stores the configuration options for a TLS client
type TLSClientCfg struct {
	Name               string `required:"true" description:"Name of this TLS client configuration"`
	Cert               string `required:"false" description:"Client certificate filename"`
	Key                string `required:"false" description:"Client private key filename"`
	RootCAs            string `required:"false" description:"Root CA bundle to use instead of system trust"`
	InsecureSkipVerify bool   `required:"false" description:"Accept any server cert" default:"false"`
}

// Prepare creates the tls.config and stores it in the global map
func (cfg TLSClientCfg) Prepare() error {
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
			return fmt.Errorf("error reading root CAs file: %s", err)
		}

		rootCAs := x509.NewCertPool()
		rootCAs.AppendCertsFromPEM(bytes)
		tlscfg.RootCAs = rootCAs
	}

	tlscfg.InsecureSkipVerify = cfg.InsecureSkipVerify
	return MainInstance.SetClientTLSConfig(cfg.Name, tlscfg)
}

func init() {
	cl := cmdline.GlobalInstance()
	cl.AddConfigType("tls-server", "Define a TLS server configuration", TLSServerCfg{}, cmdline.Section(configSection))
	cl.AddConfigType("tls-client", "Define a TLS client configuration", TLSClientCfg{}, cmdline.Section(configSection))
}
