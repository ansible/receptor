package mesh

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"path/filepath"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/certificates"
	pkgutils "github.com/ansible/receptor/pkg/utils"
)

// malformedReceptorSAN builds a subjectAltName extension whose otherName value
// is not valid ASN.1. The outer GeneralName TLV stays valid so
// x509.ParseCertificate accepts the certificate, but ReceptorNames fails to
// decode the [0] EXPLICIT value.
func malformedReceptorSAN(t *testing.T) pkix.Extension {
	t.Helper()

	oidDER, err := asn1.Marshal(pkgutils.OIDReceptorName)
	if err != nil {
		t.Fatalf("failed to marshal receptor OID: %v", err)
	}

	// UTF8String (tag 0x0c) declaring a 4-byte length but carrying one byte.
	inner := asn1.RawValue{
		Class:      asn1.ClassContextSpecific,
		Tag:        0,
		IsCompound: true,
		Bytes:      []byte{0x0c, 0x04, 'x'},
	}
	innerDER, err := asn1.Marshal(inner)
	if err != nil {
		t.Fatalf("failed to marshal malformed otherName value: %v", err)
	}

	gn := asn1.RawValue{
		Class:      asn1.ClassContextSpecific,
		Tag:        0,
		IsCompound: true,
		Bytes:      append(oidDER, innerDER...),
	}
	sanDER, err := asn1.Marshal([]asn1.RawValue{gn})
	if err != nil {
		t.Fatalf("failed to marshal subjectAltName: %v", err)
	}

	return pkix.Extension{Id: pkgutils.OIDSubjectAltName, Value: sanDER}
}

// generateMalformedSANCert creates a CA-signed certificate whose subjectAltName
// contains a malformed Receptor otherName, and returns its key and cert paths.
func generateMalformedSANCert(t *testing.T, caKeyPath, caCrtPath string) (string, string) {
	t.Helper()

	caCert, err := certificates.LoadCertificate(caCrtPath, &certificates.OsWrapper{})
	if err != nil {
		t.Fatalf("failed to load CA certificate: %v", err)
	}
	caKey, err := certificates.LoadPrivateKey(caKeyPath, &certificates.OsWrapper{})
	if err != nil {
		t.Fatalf("failed to load CA key: %v", err)
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	san := malformedReceptorSAN(t)
	template := &x509.Certificate{
		SerialNumber:    big.NewInt(time.Now().UnixNano()),
		Subject:         pkix.Name{CommonName: "malformed-san"},
		NotBefore:       time.Now().Add(-time.Hour),
		NotAfter:        time.Now().Add(time.Hour),
		KeyUsage:        x509.KeyUsageDigitalSignature,
		ExtKeyUsage:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		ExtraExtensions: []pkix.Extension{san},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("certificate with malformed otherName should still parse: %v", err)
	}
	if _, err := pkgutils.ReceptorNames(cert.Extensions); err == nil {
		t.Fatal("ReceptorNames unexpectedly accepted the malformed otherName")
	}

	dir := t.TempDir()
	keyPath := filepath.Join(dir, "malformed.key")
	crtPath := filepath.Join(dir, "malformed.crt")
	if err := certificates.SaveToPEMFile(crtPath, []interface{}{cert}, &certificates.OsWrapper{}); err != nil {
		t.Fatalf("failed to save certificate: %v", err)
	}
	if err := certificates.SaveToPEMFile(keyPath, []interface{}{key}, &certificates.OsWrapper{}); err != nil {
		t.Fatalf("failed to save key: %v", err)
	}

	return keyPath, crtPath
}
