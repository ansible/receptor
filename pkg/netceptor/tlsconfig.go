package netceptor

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/utils"
	"github.com/ghjm/cmdline"
	"github.com/spf13/viper"
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

func checkCertificatesMatchNodeID(certbytes []byte, n *Netceptor, certName string, certPath string) error {
	block, _ := pem.Decode(certbytes)

	if block == nil {
		return fmt.Errorf("failed to parse certfifcate PEM")
	}

	parsedCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}

	found, receptorNames, err := utils.ParseReceptorNamesFromCert(parsedCert, n.nodeID, n.Logger)
	if err != nil {
		return err
	}

	if !found {
		return fmt.Errorf("nodeID=%s not found in certificate name(s); names found=%s; cfg section=%s; server cert=%s", n.nodeID, fmt.Sprint(receptorNames), certName, certPath)
	}

	return nil
}

func baseTLS(minTLS13 bool) *tls.Config {
	var tlscfg *tls.Config
	if minTLS13 {
		tlscfg = &tls.Config{
			PreferServerCipherSuites: true,
			MinVersion:               tls.VersionTLS13,
		}
	} else {
		tlscfg = &tls.Config{
			MinVersion:               tls.VersionTLS12,
			CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
			PreferServerCipherSuites: true,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			},
		}
	}

	return tlscfg
}

// TLSServerConfig stores the configuration options for a TLS server.
type TLSServerConfig struct {
	Name                   string   `required:"true" description:"Name of this TLS server configuration"`
	Cert                   string   `required:"true" description:"Server certificate filename"`
	Key                    string   `required:"true" description:"Server private key filename"`
	RequireClientCert      bool     `required:"false" description:"Require client certificates" default:"false"`
	ClientCAs              string   `required:"false" description:"Filename of CA bundle to verify client certs with"`
	PinnedClientCert       []string `required:"false" description:"Pinned fingerprint of required client certificate"`
	SkipReceptorNamesCheck bool     `required:"false" description:"Skip verifying ReceptorNames OIDs in certificate at startup" default:"false"`
	MinTLS13               bool     `required:"false" description:"Set minimum TLS version to 1.3. Otherwise the minimum is 1.2" default:"false"`
}

func (cfg TLSServerConfig) PrepareTLSServerConfig(n *Netceptor) (*tls.Config, error) {
	tlscfg := baseTLS(cfg.MinTLS13)
	certBytes, err := os.ReadFile(cfg.Cert)
	if err != nil {
		return nil, err
	}
	keybytes, err := os.ReadFile(cfg.Key)
	if err != nil {
		return nil, err
	}
	cert, err := tls.X509KeyPair(certBytes, keybytes)
	if err != nil {
		return nil, err
	}
	// check server crt to ensure that the receptor NodeID is in the client certificate as an OID
	if !cfg.SkipReceptorNamesCheck {
		if err := checkCertificatesMatchNodeID(certBytes, n, cfg.Name, cfg.Cert); err != nil {
			return nil, err
		}
	}

	tlscfg.Certificates = []tls.Certificate{cert}

	if cfg.ClientCAs != "" {
		caBytes, err := os.ReadFile(cfg.ClientCAs)
		if err != nil {
			return nil, fmt.Errorf("error reading client CAs file: %s", err)
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
		return nil, fmt.Errorf("error decoding fingerprints: %s", err)
	}

	if tlscfg.ClientAuth != tls.NoClientCert {
		tlscfg.VerifyPeerCertificate = ReceptorVerifyFunc(tlscfg, pinnedFingerprints,
			"", ExpectedHostnameTypeDNS, VerifyClient, n.Logger)
	}

	return tlscfg, nil
}

// Prepare creates the tls.config and stores it in the global map.
func (cfg TLSServerConfig) Prepare() error {
	tlscfg, err := cfg.PrepareTLSServerConfig(MainInstance)
	if err != nil {
		return fmt.Errorf("error preparing tls server config: %s", err)
	}

	return MainInstance.SetServerTLSConfig(cfg.Name, tlscfg)
}

// TLSClientConfig stores the configuration options for a TLS client.
type TLSClientConfig struct {
	Name                   string   `required:"true" description:"Name of this TLS client configuration"`
	Cert                   string   `required:"true" description:"Client certificate filename"`
	Key                    string   `required:"true" description:"Client private key filename"`
	RootCAs                string   `required:"false" description:"Root CA bundle to use instead of system trust"`
	InsecureSkipVerify     bool     `required:"false" description:"Accept any server cert" default:"false"`
	PinnedServerCert       []string `required:"false" description:"Pinned fingerprint of required server certificate"`
	SkipReceptorNamesCheck bool     `required:"false" description:"if true, skip verifying ReceptorNames OIDs in certificate at startup"`
	MinTLS13               bool     `required:"false" description:"Set minimum TLS version to 1.3. Otherwise the minimum is 1.2" default:"false"`
}

func (cfg TLSClientConfig) PrepareTLSClientConfig(n *Netceptor) (tlscfg *tls.Config, pinnedFingerprints [][]byte, err error) {
	tlscfg = baseTLS(cfg.MinTLS13)
	if cfg.Cert != "" || cfg.Key != "" {
		if cfg.Cert == "" || cfg.Key == "" {
			return nil, nil, fmt.Errorf("cert and key must both be supplied or neither")
		}
		certBytes, err := os.ReadFile(cfg.Cert)
		if err != nil {
			return nil, nil, err
		}
		keyBytes, err := os.ReadFile(cfg.Key)
		if err != nil {
			return nil, nil, err
		}
		cert, err := tls.X509KeyPair(certBytes, keyBytes)
		if err != nil {
			return nil, nil, err
		}

		// check client crt to ensure that the receptor NodeID is in the client certificate as an OID
		if !cfg.SkipReceptorNamesCheck {
			if err := checkCertificatesMatchNodeID(certBytes, n, cfg.Name, cfg.Cert); err != nil {
				return nil, nil, err
			}
		}
		tlscfg.Certificates = []tls.Certificate{cert}
	}

	if cfg.RootCAs != "" {
		caBytes, err := os.ReadFile(cfg.RootCAs)
		if err != nil {
			return nil, nil, fmt.Errorf("error reading root CAs file: %s", err)
		}

		rootCAs := x509.NewCertPool()
		rootCAs.AppendCertsFromPEM(caBytes)
		tlscfg.RootCAs = rootCAs
	}

	pinnedFingerprints, err = decodeFingerprints(cfg.PinnedServerCert)
	if err != nil {
		return nil, nil, fmt.Errorf("error decoding fingerprints: %s", err)
	}

	tlscfg.InsecureSkipVerify = cfg.InsecureSkipVerify

	return tlscfg, pinnedFingerprints, nil
}

// Prepare creates the tls.config and stores it in the global map.
func (cfg TLSClientConfig) Prepare() error {
	tlscfg, pinnedFingerprints, err := cfg.PrepareTLSClientConfig(MainInstance)
	if err != nil {
		return fmt.Errorf("error preparing tls client config: %s", err)
	}

	return MainInstance.SetClientTLSConfig(cfg.Name, tlscfg, pinnedFingerprints)
}

// **************************************************************************
// Certificate Verification
// **************************************************************************

// ReceptorCertNameError represents an error when a certificate doesn't match expected Receptor node IDs.
type ReceptorCertNameError struct {
	ValidNodes   []string
	ExpectedNode string
}

func (rce ReceptorCertNameError) Error() string {
	if len(rce.ValidNodes) == 0 {
		return fmt.Sprintf("x509: certificate is not valid for any Receptor node IDs, but wanted to match %s",
			rce.ExpectedNode)
	}
	var plural string
	if len(rce.ValidNodes) > 1 {
		plural = "s"
	}

	return fmt.Sprintf("x509: certificate is valid for Receptor node ID%s %s, not %s",
		plural, strings.Join(rce.ValidNodes, ", "), rce.ExpectedNode)
}

// VerifyType indicates whether we are verifying a server or client.
type VerifyType int

const (
	// VerifyServer indicates we are the client, verifying a server.
	VerifyServer VerifyType = 1
	// VerifyClient indicates we are the server, verifying a client.
	VerifyClient = 2
)

// ExpectedHostnameType indicates whether we are connecting to a DNS hostname or a Receptor Node ID.
type ExpectedHostnameType int

const (
	// ExpectedHostnameTypeDNS indicates we are expecting a DNS style hostname.
	ExpectedHostnameTypeDNS ExpectedHostnameType = 1
	// ExpectedHostnameTypeReceptor indicates we are expecting a Receptor node ID.
	ExpectedHostnameTypeReceptor = 2
)

// ReceptorVerifyFunc generates a function that verifies a Receptor node ID.
func ReceptorVerifyFunc(tlscfg *tls.Config, pinnedFingerprints [][]byte, expectedHostname string,
	expectedHostnameType ExpectedHostnameType, verifyType VerifyType, logger *logger.ReceptorLogger,
) func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		if len(rawCerts) == 0 {
			logger.Error("RVF failed: peer certificate missing")

			return fmt.Errorf("RVF failed: peer certificate missing")
		}
		certs := make([]*x509.Certificate, len(rawCerts))
		for i, asn1Data := range rawCerts {
			cert, err := x509.ParseCertificate(asn1Data)
			if err != nil {
				logger.Error("RVF failed to parse: %s", err)

				return fmt.Errorf("failed to parse certificate from server: " + err.Error()) //nolint:govet
			}
			certs[i] = cert
		}
		var opts x509.VerifyOptions
		switch verifyType {
		case VerifyServer:
			opts = x509.VerifyOptions{
				Intermediates: x509.NewCertPool(),
				Roots:         tlscfg.RootCAs,
				CurrentTime:   time.Now(),
				KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			}
			if expectedHostnameType == ExpectedHostnameTypeDNS && expectedHostname != "" {
				opts.DNSName = expectedHostname
			}
		case VerifyClient:
			opts = x509.VerifyOptions{
				Intermediates: x509.NewCertPool(),
				Roots:         tlscfg.ClientCAs,
				CurrentTime:   time.Now(),
				KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			}
			if expectedHostnameType == ExpectedHostnameTypeDNS && expectedHostname != "" {
				opts.DNSName = expectedHostname
			}
		default:
			logger.Error("RVF failed: invalid verification type: must be client or server")

			return fmt.Errorf("RVF failed: invalid verification type: must be client or server")
		}

		if len(pinnedFingerprints) > 0 {
			var sha224sum []byte
			var sha256sum []byte
			var sha384sum []byte
			var sha512sum []byte
			fingerprintOK := false
			for _, fing := range pinnedFingerprints {
				fingLenFound := false
				for _, s := range []struct {
					len     int
					sum     *[]byte
					sumFunc func(data []byte) []byte
				}{
					{28, &sha224sum, func(data []byte) []byte {
						sum := sha256.Sum224(data)

						return sum[:]
					}},
					{32, &sha256sum, func(data []byte) []byte {
						sum := sha256.Sum256(data)

						return sum[:]
					}},
					{48, &sha384sum, func(data []byte) []byte {
						sum := sha512.Sum384(data)

						return sum[:]
					}},
					{64, &sha512sum, func(data []byte) []byte {
						sum := sha512.Sum512(data)

						return sum[:]
					}},
				} {
					if len(fing) == s.len {
						fingLenFound = true
						if *s.sum == nil {
							*s.sum = s.sumFunc(certs[0].Raw)
						}
						if bytes.Equal(fing, *s.sum) {
							fingerprintOK = true

							break
						}
					}
				}
				if !fingLenFound {
					logger.Error("RVF failed: pinned certificate must be sha224, sha256, sha384 or sha512")

					return fmt.Errorf("RVF failed: pinned certificate must be sha224, sha256, sha384 or sha512")
				}
			}
			if !fingerprintOK {
				logger.Error("RVF failed: presented certificate does not match any pinned fingerprint")

				return fmt.Errorf("RVF failed: presented certificate does not match any pinned fingerprint")
			}
		}

		for _, cert := range certs[1:] {
			opts.Intermediates.AddCert(cert)
		}
		_, err := certs[0].Verify(opts)
		if err != nil {
			logger.Error("RVF failed verify: %s\nRootCAs: %v\nServerName: %s", err, tlscfg.RootCAs, tlscfg.ServerName)

			return err
		}

		if expectedHostnameType == ExpectedHostnameTypeReceptor {
			found, receptorNames, err := utils.ParseReceptorNamesFromCert(certs[0], expectedHostname, logger)
			if err != nil {
				return err
			}
			if !found {
				logger.Error("RVF ReceptorNameError: expected %s but found %s", expectedHostname, strings.Join(receptorNames, ", "))

				return ReceptorCertNameError{ValidNodes: receptorNames, ExpectedNode: expectedHostname}
			}
		}

		return nil
	}
}

func init() {
	version := viper.GetInt("version")
	if version > 1 {
		return
	}
	cmdline.RegisterConfigTypeForApp("receptor-tls",
		"tls-server", "Define a TLS server configuration", TLSServerConfig{}, cmdline.Section(configSection))
	cmdline.RegisterConfigTypeForApp("receptor-tls",
		"tls-client", "Define a TLS client configuration", TLSClientConfig{}, cmdline.Section(configSection))
}
