// +build !no_tls_config

package netceptor

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/project-receptor/receptor/pkg/cmdline"
	"io/ioutil"
	"strings"
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
	Name               string `required:"true" description:"Name of this TLS server configuration"`
	Cert               string `required:"true" description:"Server certificate filename"`
	Key                string `required:"true" description:"Server private key filename"`
	RequireClientCert  bool   `description:"Require client certificates" default:"false"`
	VerifyClientNodeID bool   `description:"Verify certificate CA matches client node id" default:"true"`
	ClientCAs          string `description:"Filename of CA bundle to verify client certs with"`
}

func getClientValidator(helloInfo *tls.ClientHelloInfo, clientCAs *x509.CertPool) func([][]byte, [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		opts := x509.VerifyOptions{
			Roots:     clientCAs,
			KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			DNSName:   strings.Split(helloInfo.Conn.RemoteAddr().String(), ":")[0],
		}
		_, err := verifiedChains[0][0].Verify(opts)
		return err
	}
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

	tlscfg.NextProtos = []string{"netceptor"}

	serverConf := tlscfg
	if cfg.VerifyClientNodeID {
		serverConf = &tls.Config{}
		serverConf.GetConfigForClient = func(hi *tls.ClientHelloInfo) (*tls.Config, error) {
			tlscfg.VerifyPeerCertificate = getClientValidator(hi, tlscfg.ClientCAs)
			return tlscfg, nil
		}
	}
	return MainInstance.SetServerTLSConfig(cfg.Name, serverConf)
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
	tlscfg.NextProtos = []string{"netceptor"}
	return MainInstance.SetClientTLSConfig(cfg.Name, tlscfg)
}

func init() {
	cmdline.AddConfigType("tls-server", "Define a TLS server configuration", TLSServerCfg{}, cmdline.Section(configSection))
	cmdline.AddConfigType("tls-client", "Define a TLS client configuration", TLSClientCfg{}, cmdline.Section(configSection))
}
