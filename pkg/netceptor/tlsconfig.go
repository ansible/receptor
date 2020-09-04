package netceptor

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/project-receptor/receptor/pkg/cmdline"
	"io/ioutil"
)

var serverConfigs = make(map[string]*tls.Config)
var clientConfigs = make(map[string]*tls.Config)

// GetServerTLSConfig retrieves a server TLS config by name
func GetServerTLSConfig(name string) (*tls.Config, error) {
	if name == "" {
		return nil, nil
	}
	sc, ok := serverConfigs[name]
	if !ok {
		return nil, fmt.Errorf("unknown TLS config %s", name)
	}
	return sc.Clone(), nil
}

// GetClientTLSConfig retrieves a server TLS config by name
func GetClientTLSConfig(name string, expectedHostName string) (*tls.Config, error) {
	if name == "" {
		return nil, nil
	}
	cc, ok := clientConfigs[name]
	if !ok {
		return nil, fmt.Errorf("unknown TLS config %s", name)
	}
	cc = cc.Clone()
	cc.ServerName = expectedHostName
	return cc, nil
}

// **************************************************************************
// Command line
// **************************************************************************

var configSection = &cmdline.Section{
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

	serverConfigs[cfg.Name] = tlscfg
	return nil
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

	clientConfigs[cfg.Name] = tlscfg
	return nil
}

func init() {
	err := TLSClientCfg{
		Name:               "default",
		Cert:               "",
		Key:                "",
		RootCAs:            "",
		InsecureSkipVerify: false,
	}.Prepare()
	if err != nil {
		panic(err)
	}
	cmdline.AddConfigType("tls-server", "Define a TLS server configuration", TLSServerCfg{}, false, false, false, false, configSection)
	cmdline.AddConfigType("tls-client", "Define a TLS client configuration", TLSClientCfg{}, false, false, false, false, configSection)
}
