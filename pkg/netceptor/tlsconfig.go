//go:build !no_tls_config
// +build !no_tls_config

package netceptor

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/ghjm/cmdline"
)

// **************************************************************************
// Command line
// **************************************************************************

var configSection = &cmdline.ConfigSection{
	Description: "Commands that configure resources used by other commands:",
	Order:       5,
}

// tlsServerCfg stores the configuration options for a TLS server.
type tlsServerCfg struct {
	Name              string   `required:"true" description:"Name of this TLS server configuration"`
	Cert              string   `required:"true" description:"Server certificate filename"`
	Key               string   `required:"true" description:"Server private key filename"`
	RequireClientCert bool     `description:"Require client certificates" default:"false"`
	ClientCAs         string   `description:"Filename of CA bundle to verify client certs with"`
	PinnedClientCert  []string `description:"Pinned fingerprint of required client certificate"`
}

// Prepare creates the tls.config and stores it in the global map.
func (cfg tlsServerCfg) Prepare() error {
	tlscfg := &tls.Config{
		PreferServerCipherSuites: true,
		MinVersion:               tls.VersionTLS12,
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

	if cfg.ClientCAs != "" {
		caBytes, err := ioutil.ReadFile(cfg.ClientCAs)
		if err != nil {
			return fmt.Errorf("error reading client CAs file: %s", err)
		}
		clientCAs := x509.NewCertPool()
		clientCAs.AppendCertsFromPEM(caBytes)
		tlscfg.ClientCAs = clientCAs
	}

	if len(cfg.PinnedClientCert) > 0 {
		tlscfg.VerifyPeerCertificate, err = GetPeerCertificatePinVerifier(cfg.PinnedClientCert)
		if err != nil {
			return err
		}
	}

	switch {
	case cfg.RequireClientCert:
		tlscfg.ClientAuth = tls.RequireAndVerifyClientCert
	case cfg.ClientCAs != "":
		tlscfg.ClientAuth = tls.VerifyClientCertIfGiven
	default:
		tlscfg.ClientAuth = tls.NoClientCert
	}

	return MainInstance.SetServerTLSConfig(cfg.Name, tlscfg)
}

// tlsClientConfig stores the configuration options for a TLS client.
type tlsClientConfig struct {
	Name               string   `required:"true" description:"Name of this TLS client configuration"`
	Cert               string   `required:"false" description:"Client certificate filename"`
	Key                string   `required:"false" description:"Client private key filename"`
	RootCAs            string   `required:"false" description:"Root CA bundle to use instead of system trust"`
	InsecureSkipVerify bool     `required:"false" description:"Accept any server cert" default:"false"`
	PinnedServerCert   []string `required:"false" description:"Pinned fingerprint of required server certificate"`
}

// Prepare creates the tls.config and stores it in the global map.
func (cfg tlsClientConfig) Prepare() error {
	tlscfg := &tls.Config{
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
	}

	if cfg.Cert != "" || cfg.Key != "" {
		if cfg.Cert == "" || cfg.Key == "" {
			return fmt.Errorf("cert and key must both be supplied or neither")
		}
		certBytes, err := ioutil.ReadFile(cfg.Cert)
		if err != nil {
			return err
		}
		keyBytes, err := ioutil.ReadFile(cfg.Key)
		if err != nil {
			return err
		}
		cert, err := tls.X509KeyPair(certBytes, keyBytes)
		if err != nil {
			return err
		}
		tlscfg.Certificates = []tls.Certificate{cert}
	}

	if cfg.RootCAs != "" {
		caBytes, err := ioutil.ReadFile(cfg.RootCAs)
		if err != nil {
			return fmt.Errorf("error reading root CAs file: %s", err)
		}

		rootCAs := x509.NewCertPool()
		rootCAs.AppendCertsFromPEM(caBytes)
		tlscfg.RootCAs = rootCAs
	}

	if len(cfg.PinnedServerCert) > 0 {
		var err error
		tlscfg.VerifyPeerCertificate, err = GetPeerCertificatePinVerifier(cfg.PinnedServerCert)
		if err != nil {
			return err
		}
	}

	tlscfg.InsecureSkipVerify = cfg.InsecureSkipVerify

	return MainInstance.SetClientTLSConfig(cfg.Name, tlscfg)
}

func GetPeerCertificatePinVerifier(pinnedFingerprints []string) (func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error, error) {
	fingerprintBytes := make([][]byte, 0, len(pinnedFingerprints))
	for _, fingStr := range pinnedFingerprints {
		fingBytes, err := hex.DecodeString(strings.ReplaceAll(fingStr, ":", ""))
		if err != nil {
			return nil, fmt.Errorf("error decoding fingerprint")
		}
		if len(fingBytes) != 32 && len(fingBytes) != 64 {
			return nil, fmt.Errorf("fingerprints must be 32 or 64 bytes for sha256 or sha512")
		}
		fingerprintBytes = append(fingerprintBytes, fingBytes)
	}
	verifier := func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		if len(rawCerts) == 0 {
			return fmt.Errorf("peer certificate missing")
		}
		cert, err := x509.ParseCertificate(rawCerts[0])
		if err != nil {
			return fmt.Errorf("error parsing certificate: %w", err)
		}
		var sha256sum []byte
		var sha512sum []byte
		for _, fing := range fingerprintBytes {
			switch len(fing) {
			case 32:
				if sha256sum == nil {
					sum := sha256.Sum256(cert.Raw)
					sha256sum = sum[:]
				}
				if bytes.Equal(fing, sha256sum) {
					return nil
				}
			case 64:
				if sha512sum == nil {
					sum := sha512.Sum512(cert.Raw)
					sha512sum = sum[:]
				}
				if bytes.Equal(fing, sha256sum) {
					return nil
				}
			}
		}
		
		return fmt.Errorf("peer certificate did not match any pinned fingerprint")
	}

	return verifier, nil
}

func init() {
	cmdline.RegisterConfigTypeForApp("receptor-tls",
		"tls-server", "Define a TLS server configuration", tlsServerCfg{}, cmdline.Section(configSection))
	cmdline.RegisterConfigTypeForApp("receptor-tls",
		"tls-client", "Define a TLS client configuration", tlsClientConfig{}, cmdline.Section(configSection))
}
