//go:build !no_tls_config
// +build !no_tls_config

package netceptor

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/ansible/receptor/pkg/utils"
	"github.com/ghjm/cmdline"
)

// **************************************************************************
// Command line
// **************************************************************************

var configSection = &cmdline.ConfigSection{
	Description: "Commands that configure resources used by other commands:",
	Order:       5,
}

func decodeFingerprints(fingerprints []string) ([][]byte, error) {
	fingerprintBytes := make([][]byte, 0, len(fingerprints))
	for _, fingStr := range fingerprints {
		fingBytes, err := hex.DecodeString(strings.ReplaceAll(fingStr, ":", ""))
		if err != nil {
			return nil, fmt.Errorf("error decoding fingerprint")
		}
		if len(fingBytes) != 32 && len(fingBytes) != 64 {
			return nil, fmt.Errorf("fingerprints must be 32 or 64 bytes for sha256 or sha512")
		}
		fingerprintBytes = append(fingerprintBytes, fingBytes)
	}

	return fingerprintBytes, nil
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

	block, _ := pem.Decode(certbytes)
	if block == nil {
		return fmt.Errorf("failed to parse certificate PEM")
	}

	parsedCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}

	s, err := utils.ReceptorNames(parsedCert.Extensions)
	if err != nil {
		return err
	}

	found := false

	for _, name := range s {
		if MainInstance.nodeID == name {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("nodeID=%s not found in certificate name(s); cfg section=%s; server cert=%s", MainInstance.nodeID, cfg.Name, cfg.Cert)
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

	switch {
	case cfg.RequireClientCert:
		tlscfg.ClientAuth = tls.RequireAndVerifyClientCert
	case cfg.ClientCAs != "":
		tlscfg.ClientAuth = tls.VerifyClientCertIfGiven
	default:
		tlscfg.ClientAuth = tls.NoClientCert
	}

	var pinnedFingerprints [][]byte
	pinnedFingerprints, err = decodeFingerprints(cfg.PinnedClientCert)
	if err != nil {
		return fmt.Errorf("error decoding fingerprints: %s", err)
	}

	if tlscfg.ClientAuth != tls.NoClientCert {
		tlscfg.VerifyPeerCertificate = ReceptorVerifyFunc(tlscfg, pinnedFingerprints,
			"", ExpectedHostnameTypeDNS, VerifyClient)
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

	pinnedFingerprints, err := decodeFingerprints(cfg.PinnedServerCert)
	if err != nil {
		return fmt.Errorf("error decoding fingerprints: %s", err)
	}

	tlscfg.InsecureSkipVerify = cfg.InsecureSkipVerify

	return MainInstance.SetClientTLSConfig(cfg.Name, tlscfg, pinnedFingerprints)
}

func init() {
	cmdline.RegisterConfigTypeForApp("receptor-tls",
		"tls-server", "Define a TLS server configuration", tlsServerCfg{}, cmdline.Section(configSection))
	cmdline.RegisterConfigTypeForApp("receptor-tls",
		"tls-client", "Define a TLS client configuration", tlsClientConfig{}, cmdline.Section(configSection))
}
