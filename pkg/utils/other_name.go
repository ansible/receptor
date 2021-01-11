package utils

import (
	"crypto/x509/pkix"
	"encoding/asn1"
	"net"
)

var (
	// OIDSubjectAltName is the OID for subjectAltName
	OIDSubjectAltName = asn1.ObjectIdentifier{2, 5, 29, 17}
	// OIDReceptorName is the OID for a Receptor node ID
	OIDReceptorName = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 2312, 19, 1}
)

// OtherNameDecode is used for decoding the OtherName field type of an x.509 subjectAltName
type OtherNameDecode struct {
	ID    asn1.ObjectIdentifier
	Value asn1.RawValue
}

// ReceptorNames returns a list of Receptor node IDs found in the subjectAltName field of an x.509 certificate
func ReceptorNames(extensions []pkix.Extension) ([]string, error) {
	names := make([]string, 0)
	for _, extension := range extensions {
		if extension.Id.Equal(OIDSubjectAltName) {
			values := make([]asn1.RawValue, 0)
			_, err := asn1.Unmarshal(extension.Value, &values)
			if err != nil {
				return nil, err
			}
			for _, value := range values {
				if value.Tag == 0 {
					on := OtherNameDecode{}
					_, err = asn1.UnmarshalWithParams(value.FullBytes, &on, "tag:0")
					if err != nil {
						return nil, err
					}
					if on.ID.Equal(OIDReceptorName) {
						var name string
						_, err = asn1.Unmarshal(on.Value.Bytes, &name)
						if err != nil {
							return nil, err
						}
						names = append(names, name)
					}
				}
			}
		}
	}
	return names, nil
}

// UTFString is used for encoding a UTF-8 string
type UTFString struct {
	A string `asn1:"utf8"`
}

// DNSNameEncode is used for encoding the OtherName field of an x.509 subjectAltName
type DNSNameEncode struct {
	Value string `asn1:"tag:2"`
}

// IPAddressEncode is used for encoding the OtherName field of an x.509 subjectAltName
type IPAddressEncode struct {
	Value []byte `asn1:"tag:7"`
}

// OtherNameEncode is used for encoding the OtherName field of an x.509 subjectAltName
type OtherNameEncode struct {
	OID   asn1.ObjectIdentifier
	Value UTFString `asn1:"tag:0"`
}

// GeneralNameEncode is used for encoding a GeneralName in an x.509 certificate
type GeneralNameEncode struct {
	Names []interface{} `asn1:"tag:0"`
}

// MakeReceptorSAN generates a subjectAltName extension, optionally containing Receptor names
func MakeReceptorSAN(DNSNames []string, IPAddresses []net.IP, NodeIDs []string) (*pkix.Extension, error) {
	var rawValues []asn1.RawValue
	for _, name := range DNSNames {
		rawValues = append(rawValues, asn1.RawValue{Tag: 2, Class: 2, Bytes: []byte(name)})
	}
	for _, rawIP := range IPAddresses {
		ip := rawIP.To4()
		if ip == nil {
			ip = rawIP
		}
		rawValues = append(rawValues, asn1.RawValue{Tag: 7, Class: 2, Bytes: ip})
	}
	for _, nodeID := range NodeIDs {
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
