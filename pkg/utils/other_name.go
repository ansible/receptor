package utils

import (
	"crypto/x509/pkix"
	"encoding/asn1"
	"net"
)

var (
	// OIDSubjectAltName is the OID for subjectAltName.
	OIDSubjectAltName = asn1.ObjectIdentifier{2, 5, 29, 17}
	// OIDReceptorName is the OID for a Receptor node ID.
	OIDReceptorName = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 2312, 19, 1}
)

// OtherNameDecode is used for decoding the OtherName field type of an x.509 subjectAltName.
type OtherNameDecode struct {
	ID    asn1.ObjectIdentifier
	Value asn1.RawValue
}

// decodeReceptorName decodes a single ASN.1 value and extracts the Receptor name if present.
func decodeReceptorName(value asn1.RawValue) (string, bool, error) {
	if value.Tag != 0 {
		return "", false, nil
	}

	on := OtherNameDecode{}
	_, err := asn1.UnmarshalWithParams(value.FullBytes, &on, "tag:0")
	if err != nil {
		return "", false, err
	}

	if !on.ID.Equal(OIDReceptorName) {
		return "", false, nil
	}

	var name string
	_, err = asn1.Unmarshal(on.Value.Bytes, &name)
	if err != nil {
		return "", false, err
	}

	return name, true, nil
}

// extractNamesFromExtension extracts Receptor names from a single extension.
func extractNamesFromExtension(extension pkix.Extension) ([]string, error) {
	if !extension.Id.Equal(OIDSubjectAltName) {
		return nil, nil
	}

	values := make([]asn1.RawValue, 0)
	_, err := asn1.Unmarshal(extension.Value, &values)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0)
	for _, value := range values {
		name, found, err := decodeReceptorName(value)
		if err != nil {
			return nil, err
		}
		if found {
			names = append(names, name)
		}
	}

	return names, nil
}

// ReceptorNames returns a list of Receptor node IDs found in the subjectAltName field of an x.509 certificate.
func ReceptorNames(extensions []pkix.Extension) ([]string, error) {
	names := make([]string, 0)
	for _, extension := range extensions {
		extensionNames, err := extractNamesFromExtension(extension)
		if err != nil {
			return nil, err
		}
		names = append(names, extensionNames...)
	}

	return names, nil
}

// UTFString is used for encoding a UTF-8 string.
type UTFString struct {
	A string `asn1:"utf8"`
}

// DNSNameEncode is used for encoding the OtherName field of an x.509 subjectAltName.
type DNSNameEncode struct {
	Value string `asn1:"tag:2"`
}

// IPAddressEncode is used for encoding the OtherName field of an x.509 subjectAltName.
type IPAddressEncode struct {
	Value []byte `asn1:"tag:7"`
}

// OtherNameEncode is used for encoding the OtherName field of an x.509 subjectAltName.
type OtherNameEncode struct {
	OID   asn1.ObjectIdentifier
	Value UTFString `asn1:"tag:0"`
}

// GeneralNameEncode is used for encoding a GeneralName in an x.509 certificate.
type GeneralNameEncode struct {
	Names []interface{} `asn1:"tag:0"`
}

// MakeReceptorSAN generates a subjectAltName extension, optionally containing Receptor names.
func MakeReceptorSAN(dnsNames []string, ipAddresses []net.IP, nodeIDs []string) (*pkix.Extension, error) {
	rawValues := []asn1.RawValue{}
	for _, name := range dnsNames {
		rawValues = append(rawValues, asn1.RawValue{Tag: 2, Class: 2, Bytes: []byte(name)})
	}
	for _, rawIP := range ipAddresses {
		ip := rawIP.To4()
		if ip == nil {
			ip = rawIP
		}
		rawValues = append(rawValues, asn1.RawValue{Tag: 7, Class: 2, Bytes: ip})
	}
	for _, nodeID := range nodeIDs {
		var err error
		var asnOtherName []byte
		asnOtherName, err = asn1.Marshal(OtherNameEncode{
			OID:   OIDReceptorName,
			Value: UTFString{A: nodeID},
		})
		if err != nil {
			return nil, err
		}
		rawValues = append(rawValues, asn1.RawValue{Tag: 0, Class: 2, IsCompound: true, Bytes: asnOtherName[2:]})
	}
	sanBytes, err := asn1.Marshal(rawValues)
	if err != nil {
		return nil, err
	}
	sanExt := pkix.Extension{
		Id:       OIDSubjectAltName,
		Critical: false,
		Value:    sanBytes,
	}

	return &sanExt, nil
}
