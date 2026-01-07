package netceptor

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
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

	// Use the extracted helper for validation.
	err = verifyReceptorNodeID(parsedCert, n.nodeID, n.Logger)
	if err != nil {
		// Add startup-specific context to the error.
		var certErr ReceptorCertNameError
		if errors.As(err, &certErr) {
			return fmt.Errorf("nodeID=%s not found in certificate name(s); names found=%s; cfg section=%s; server cert=%s",
				n.nodeID, fmt.Sprint(certErr.ValidNodes), certName, certPath)
		}

		return err
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
	MinTLS13               bool     `required:"false" description:"Set minimum TLS version to 1.3. Otherwise the minimum is 1.2" default:"true"`
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
	MinTLS13               bool     `required:"false" description:"Set minimum TLS version to 1.3. Otherwise the minimum is 1.2" default:"true"`
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

// hashAlgorithm represents a supported hash algorithm for fingerprint verification.
type hashAlgorithm struct {
	length  int
	compute func([]byte) []byte
}

// getSupportedHashAlgorithms returns the list of supported hash algorithms for certificate fingerprints.
func getSupportedHashAlgorithms() []hashAlgorithm {
	return []hashAlgorithm{
		{28, func(data []byte) []byte {
			sum := sha256.Sum224(data)

			return sum[:]
		}},
		{32, func(data []byte) []byte {
			sum := sha256.Sum256(data)

			return sum[:]
		}},
		{48, func(data []byte) []byte {
			sum := sha512.Sum384(data)

			return sum[:]
		}},
		{64, func(data []byte) []byte {
			sum := sha512.Sum512(data)

			return sum[:]
		}},
	}
}

// computeHashForFingerprint computes the hash for a certificate based on fingerprint length.
// Returns the hash and true if a matching algorithm was found, nil and false otherwise.
func computeHashForFingerprint(certRaw []byte, fingerprintLen int, algorithms []hashAlgorithm) ([]byte, bool) {
	for _, algo := range algorithms {
		if fingerprintLen == algo.length {
			return algo.compute(certRaw), true
		}
	}

	return nil, false
}

// verifyPinnedFingerprint checks if a certificate matches any of the provided pinned fingerprints.
// Returns nil if fingerprints match or if no fingerprints are provided.
func verifyPinnedFingerprint(cert *x509.Certificate, pinnedFingerprints [][]byte) error {
	if len(pinnedFingerprints) == 0 {
		return nil
	}

	algorithms := getSupportedHashAlgorithms()
	hashCache := make(map[int][]byte)

	for _, fing := range pinnedFingerprints {
		hash, exists := hashCache[len(fing)]
		if !exists {
			var valid bool
			hash, valid = computeHashForFingerprint(cert.Raw, len(fing), algorithms)
			if !valid {
				return fmt.Errorf("RVF failed: pinned certificate must be sha224, sha256, sha384 or sha512")
			}
			hashCache[len(fing)] = hash
		}

		if bytes.Equal(fing, hash) {
			return nil
		}
	}

	return fmt.Errorf("RVF failed: presented certificate does not match any pinned fingerprint")
}

// buildVerifyOptions creates x509.VerifyOptions based on the verification type and hostname.
func buildVerifyOptions(tlscfg *tls.Config, verifyType VerifyType, expectedHostname string, expectedHostnameType ExpectedHostnameType) (x509.VerifyOptions, error) {
	var roots *x509.CertPool
	var keyUsage x509.ExtKeyUsage

	switch verifyType {
	case VerifyServer:
		roots = tlscfg.RootCAs
		keyUsage = x509.ExtKeyUsageServerAuth
	case VerifyClient:
		roots = tlscfg.ClientCAs
		keyUsage = x509.ExtKeyUsageClientAuth
	default:
		return x509.VerifyOptions{}, fmt.Errorf("RVF failed: invalid verification type: must be client or server")
	}

	opts := x509.VerifyOptions{
		Intermediates: x509.NewCertPool(),
		Roots:         roots,
		CurrentTime:   time.Now(),
		KeyUsages:     []x509.ExtKeyUsage{keyUsage},
	}

	if expectedHostnameType == ExpectedHostnameTypeDNS && expectedHostname != "" {
		opts.DNSName = expectedHostname
	}

	return opts, nil
}

// addIntermediateCerts adds intermediate certificates from the peer's chain to the verify options.
// Note: Certificate deduplication was considered but rejected to avoid invalidating user-provided certificate bundles.
func addIntermediateCerts(certs []*x509.Certificate, opts *x509.VerifyOptions) {
	for _, cert := range certs[1:] {
		opts.Intermediates.AddCert(cert)
	}
}

// verifyReceptorNodeID validates that a certificate contains the expected Receptor node ID.
func verifyReceptorNodeID(cert *x509.Certificate, expectedHostname string, logger *logger.ReceptorLogger) error {
	found, receptorNames, err := utils.ParseReceptorNamesFromCert(cert, expectedHostname, logger)
	if err != nil {
		return err
	}

	if !found {
		return ReceptorCertNameError{ValidNodes: receptorNames, ExpectedNode: expectedHostname}
	}

	return nil
}

// ReceptorVerifyFunc generates a function that verifies a Receptor node ID.
func ReceptorVerifyFunc(tlscfg *tls.Config, pinnedFingerprints [][]byte, expectedHostname string,
	expectedHostnameType ExpectedHostnameType, verifyType VerifyType, logger *logger.ReceptorLogger,
) func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		// Validate certificates are provided.
		if len(rawCerts) == 0 {
			logger.Error("RVF failed: peer certificate missing")

			return fmt.Errorf("RVF failed: peer certificate missing")
		}

		// Parse raw certificates.
		certs := make([]*x509.Certificate, len(rawCerts))
		for i, asn1Data := range rawCerts {
			cert, err := x509.ParseCertificate(asn1Data)
			if err != nil {
				logger.Error("RVF failed to parse: %s", err)

				return fmt.Errorf("failed to parse certificate from server: %w", err)
			}
			certs[i] = cert
		}

		// Check pinned fingerprints if provided.
		if err := verifyPinnedFingerprint(certs[0], pinnedFingerprints); err != nil {
			logger.Error("%s", err)

			return err
		}

		// Build verification options.
		opts, err := buildVerifyOptions(tlscfg, verifyType, expectedHostname, expectedHostnameType)
		if err != nil {
			logger.Error("%s", err)

			return err
		}

		// Add intermediate certificates to the verification options.
		addIntermediateCerts(certs, &opts)

		// Verify the certificate chain.
		_, err = certs[0].Verify(opts)
		if err != nil {
			var hostnameError x509.HostnameError
			handledError := err
			if errors.As(err, &hostnameError) {
				handledError = handleHostnameError(hostnameError)
			}
			logger.Error("RVF failed verify: %s\nRootCAs: %v\nServerName: %s", handledError, tlscfg.RootCAs, tlscfg.ServerName)

			return handledError
		}

		// Verify Receptor node ID if required.
		if expectedHostnameType == ExpectedHostnameTypeReceptor {
			if err := verifyReceptorNodeID(certs[0], expectedHostname, logger); err != nil {
				logger.Error("%s", err)

				return err
			}
		}

		return nil
	}
}

// handleHostnameError handles a hostname error from the Certificate.Verify function. It
// was put in place temporarily to mitigate CVE-2025-61729 until the project is able to
// upgrade to a Golang version containing a fix.
//
// This function should be removed once the project can upgrade to a Golang version with
// a fix for the CVE.
func handleHostnameError(h x509.HostnameError) error {
	c := h.Certificate
	maxNamesIncluded := 100

	var valid strings.Builder
	if ip := net.ParseIP(h.Host); ip != nil {
		// Trying to validate an IP
		if len(c.IPAddresses) == 0 {
			return errors.New("x509: cannot validate certificate for " + h.Host + " because it doesn't contain any IP SANs")
		}
		if len(c.IPAddresses) >= maxNamesIncluded {
			return fmt.Errorf("x509: certificate is valid for %d IP SANs, but none matched %s", len(c.IPAddresses), h.Host)
		}
		for _, san := range c.IPAddresses {
			if valid.Len() > 0 {
				valid.WriteString(", ")
			}
			valid.WriteString(san.String())
		}
	} else {
		if len(c.DNSNames) >= maxNamesIncluded {
			return fmt.Errorf("x509: certificate is valid for %d names, but none matched %s", len(c.DNSNames), h.Host)
		}
		valid.WriteString(strings.Join(c.DNSNames, ", "))
	}

	if valid.Len() == 0 {
		return errors.New("x509: certificate is not valid for any names, but wanted to match " + h.Host)
	}
	return errors.New("x509: certificate is valid for " + valid.String() + ", not " + h.Host)
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
