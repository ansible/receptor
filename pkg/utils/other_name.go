package utils

import (
	"crypto/x509"
	"encoding/asn1"
)

var (
	oidSubjectAltName = asn1.ObjectIdentifier{2, 5, 29, 17}
	oidReceptorName   = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 2312, 19, 1}
)

// OtherName represents the OtherName field type of an x.509 subjectAltName
type OtherName struct {
	ID    asn1.ObjectIdentifier
	Value asn1.RawValue
}

// ReceptorNames returns a list of Receptor node IDs found in the subjectAltName field of an x.509 certificate
func ReceptorNames(cert *x509.Certificate) ([]string, error) {
	names := make([]string, 0)
	for _, extension := range cert.Extensions {
		if extension.Id.Equal(oidSubjectAltName) {
			values := make([]asn1.RawValue, 0)
			_, err := asn1.Unmarshal(extension.Value, &values)
			if err != nil {
				return nil, err
			}
			for _, value := range values {
				if value.Tag == 0 {
					on := OtherName{}
					_, err = asn1.UnmarshalWithParams(value.FullBytes, &on, "tag:0")
					if err != nil {
						return nil, err
					}
					if on.ID.Equal(oidReceptorName) {
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
