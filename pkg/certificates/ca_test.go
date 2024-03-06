//go:build !no_cert_auth
// +build !no_cert_auth

package certificates_test

import (
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/certificates"
	"github.com/ansible/receptor/pkg/certificates/mock_certificates"
	"github.com/ansible/receptor/pkg/utils"
	"go.uber.org/mock/gomock"
)

func TestCreateCAValid(t *testing.T) {
	type args struct {
		opts *certificates.CertOptions
	}
	goodCaBlock, _ := pem.Decode([]byte(`
-----BEGIN CERTIFICATE-----
MIIFVTCCAz2gAwIBAgIEYdeDaTANBgkqhkiG9w0BAQsFADA7MTkwNwYDVQQDEzBB
bnNpYmxlIEF1dG9tYXRpb24gQ29udHJvbGxlciBOb2RlcyBNZXNoIFJPT1QgQ0Ew
HhcNMjIwMTA3MDAwMzUxWhcNMzIwMTA3MDAwMzUxWjA7MTkwNwYDVQQDEzBBbnNp
YmxlIEF1dG9tYXRpb24gQ29udHJvbGxlciBOb2RlcyBNZXNoIFJPT1QgQ0EwggIi
MA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQCxAErOWvVDU8mfZgtE6BSygTWU
MkPxxIEQSYs/UesRAHaB+QXa7/0Foa0VUJKcWwUE+2yYkNRrg8MmE8VWMSewcaNI
As407stFXP+A2anPEglwemTskpO72sigiYDKShC5n5ciyPsHckwVlOCTtac5TwFe
eTmGnHWRcd4uBGvaEXx98fw/wLgYtr9vmKTdnOQjriX9EaAWrjlrlzm54Bs3uVUj
GSL7zY381EuUVV4AjbqQyThbY9cVfsK0nmzLUqpiHG2IhGZDZA9+jxtz2wJWFkNQ
nWA3afCUjcWV+4FpP3p1U1myCeh2yR2uCHs9pkUK3ts9uD/Wd5j9M1oBMlymbN/C
5Fahd+cTXrPAjsoRqCso9TBP4mIlNl1Jq8MRUWTL5HOuwn+KnufBtuQ1hIb71Eso
kj90eWeo/P+temYAEquUVWiej7lnHyZVW647lE+o+xJEOmW+tY5H4jgA/twP4s7U
BgR545usWF9/utvnhsGSkg1EYcdzaM01pkrWrw1GvHT++HshsrG6Tse8gY7JrTds
LDtU8LPhPUEnSfVBcgcMg2Dg8lEbaODtdp2xtCJwZHy9CiAx3CKcogVEfecrKSSr
2iSocft9/l8J+mhVG2CI6ekG6Cy9hDct/3SV01Dfd4FG7xXJIE3mTDLILpi1AVIW
MHxYD3skuQMyLhJDAwIDAQABo2EwXzAOBgNVHQ8BAf8EBAMCAoQwHQYDVR0lBBYw
FAYIKwYBBQUHAwIGCCsGAQUFBwMBMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYE
FFAlC81rH211fPJoWglERKb/7/NfMA0GCSqGSIb3DQEBCwUAA4ICAQCWCP/O6YQ9
jhae2/BeUeeKsnoxf90prg3o6/QHbelF6yL+MvGg5ZbPSlt+ywLDNR2CYvXk/4SD
5To7CPKhSPBwwUJpafvQfAOZijU30fvXvp5yZEFoOzOyvBP58NfzL5qH6Pf5A6i3
rHvtR1v7DgS7u2qWWcSimIM0UPoV3JubLTEORjOR6FIyNkIxdjhrP3SxyZ54xxde
G3bchKaRcGVNoFYSDN4bAA22JAjlD8kXNYKzIS/0cOR/9SnHd1wMIQ2trx0+TfyG
FAA1mW1mjzQd+h5SGBVeCz2W2XttNSIfQDndJCsyACxmIaOK99AQxdhZsWfHtGO1
3TjnyoiHjf8rozJbAVYqrIdB6GDf6fUlxwhUXT0qkgOvvAzjNnLoOBUkE4TWqXHl
38a+ITDNVzaUlrTd63eexS69V6kHe7mrqjywNQ9EXF9kaVeoNTzRf/ztT/DEVAl+
rKshMt4IOKQf1ScE+EJe1njpREHV+fa+kYvQB6cRuxW9a8sOSeQNaSL73Zv54elZ
xffYhMv6yXvVxVnJHEsG3kM/CsvsU364BBd9kDcZbHpjNcDHMu+XxECJjD2atVtu
FdaOLykGKfMCYVBP+xs97IJO8En/5N9QQwc+N4cfCg9/BWoZKHPbRx/V+57VEj0m
69EpJXbL15ZQLCPsaIcqJqpK23VyJKc8fA==
-----END CERTIFICATE-----
`))

	goodCaCertificate, err := x509.ParseCertificate(goodCaBlock.Bytes)
	if err != nil {
		t.Errorf("Error parsing certificate: %v", err)
	}

	goodCaTimeAfterString := "2032-01-07T00:03:51Z"
	goodCaTimeAfter, err := time.Parse(time.RFC3339, goodCaTimeAfterString)
	if err != nil {
		t.Errorf("Error parsing time %s: %v", goodCaTimeAfterString, err)
	}
	goodCaTimeBeforeString := "2022-01-07T00:03:51Z"
	goodCaTimeBefore, err := time.Parse(time.RFC3339, goodCaTimeBeforeString)
	if err != nil {
		t.Errorf("Error parsing time %s: %v", goodCaTimeBeforeString, err)
	}

	goodCertOptions := certificates.CertOptions{
		Bits:       4096,
		CommonName: "Ansible Automation Controller Nodes Mesh ROOT CA",
		NotAfter:   goodCaTimeAfter,
		NotBefore:  goodCaTimeBefore,
	}

	goodCaPrivateKey, err := setupGoodPrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		args args
		want *certificates.CA
	}{
		{
			name: "Positive test",
			args: args{
				opts: &goodCertOptions,
			},
			want: &certificates.CA{
				Certificate: goodCaCertificate,
				PrivateKey:  goodCaPrivateKey,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRsa := mock_certificates.NewMockRsaer(ctrl)

			mockRsa.EXPECT().GenerateKey(gomock.Any(), gomock.Any()).DoAndReturn(
				func(random io.Reader, bits int) (*rsa.PrivateKey, error) {
					return goodCaPrivateKey, nil
				},
			)

			got, _ := certificates.CreateCA(tt.args.opts, mockRsa)
			if !reflect.DeepEqual(got.PrivateKey, tt.want.PrivateKey) {
				t.Errorf("CreateCA() Private Key got = %v, want = %v", got.PrivateKey, tt.want.PrivateKey)

				return
			}

			certWant := tt.want.Certificate
			certGot := got.Certificate
			if certGot.BasicConstraintsValid != true {
				t.Errorf("CreateCA() Certificate BasicConstraintsValid got = %v, want = %v", certGot.BasicConstraintsValid, true)

				return
			}

			if !reflect.DeepEqual(certGot.ExtraExtensions, certWant.ExtraExtensions) {
				t.Errorf("CreateCA() Certificate ExtraExtensions got = %v, want = %v", certGot.ExtraExtensions, certWant.ExtraExtensions)

				return
			}

			if !reflect.DeepEqual(certGot.ExtKeyUsage, certWant.ExtKeyUsage) {
				t.Errorf("CreateCA() Certificate ExtKeyUsage got = %v, want = %v", certGot.ExtKeyUsage, certWant.ExtKeyUsage)

				return
			}

			if certGot.IsCA != true {
				t.Errorf("CreateCA() Certificate IsCA got = %v, want = %v", certGot.IsCA, true)

				return
			}

			if !reflect.DeepEqual(certGot.Issuer, certWant.Issuer) {
				t.Errorf("CreateCA() Certificate Issuer got = %v, want = %v", certGot.Issuer, certWant.Issuer)

				return
			}

			if !reflect.DeepEqual(certGot.KeyUsage, certWant.KeyUsage) {
				t.Errorf("CreateCA() Certificate KeyUsage got = %v, want = %v", certGot.KeyUsage, certWant.KeyUsage)

				return
			}

			if !reflect.DeepEqual(certGot.NotAfter, certWant.NotAfter) {
				t.Errorf("CreateCA() Certificate NotAfter got = %v, want = %v", certGot.NotAfter, certWant.NotAfter)

				return
			}

			if !reflect.DeepEqual(certGot.NotBefore, certWant.NotBefore) {
				t.Errorf("CreateCA() Certificate NotBefore got = %v, want = %v", certGot.NotBefore, certWant.NotBefore)

				return
			}

			if !reflect.DeepEqual(certGot.PublicKeyAlgorithm, certWant.PublicKeyAlgorithm) {
				t.Errorf("CreateCA() Certificate PublicKeyAlgorithm got = %v, want = %v", certGot.PublicKeyAlgorithm, certWant.PublicKeyAlgorithm)

				return
			}

			if !reflect.DeepEqual(certGot.SignatureAlgorithm, certWant.SignatureAlgorithm) {
				t.Errorf("CreateCA() Certificate SignatureAlgorithm got = %v, want = %v", certGot.SignatureAlgorithm, certWant.SignatureAlgorithm)

				return
			}

			if !reflect.DeepEqual(certGot.Subject, certWant.Subject) {
				t.Errorf("CreateCA() Certificate Subject got = %v, want = %v", certGot.Subject, certWant.Subject)

				return
			}

			if !reflect.DeepEqual(certGot.Version, certWant.Version) {
				t.Errorf("CreateCA() Certificate Version got = %v, want = %v", certGot.Version, certWant.Version)

				return
			}
		})
	}
}

func TestCreateCANegative(t *testing.T) {
	type args struct {
		opts *certificates.CertOptions
	}
	badCertOptions := certificates.CertOptions{
		Bits: -1,
	}
	tests := []struct {
		name    string
		args    args
		want    *certificates.CA
		wantErr error
	}{
		{
			name: "Negative test for Common Name",
			args: args{
				opts: &badCertOptions,
			},
			want:    nil,
			wantErr: fmt.Errorf("must provide CommonName"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, gotErr := certificates.CreateCA(tt.args.opts, &certificates.RsaWrapper{})
			if gotErr == nil || gotErr.Error() != tt.wantErr.Error() {
				t.Errorf("CreateCA() error = %v, wantErr = %v", gotErr, tt.wantErr)

				return
			}
		})
	}
}

func setupGoodCertRequest() (certificates.CertOptions, *x509.CertificateRequest, error) {
	goodCaTimeAfterString := "2032-01-07T00:03:51Z"
	goodCaTimeAfter, err := time.Parse(time.RFC3339, goodCaTimeAfterString)
	if err != nil {
		return certificates.CertOptions{}, &x509.CertificateRequest{}, err
	}

	goodCaTimeBeforeString := "2022-01-07T00:03:51Z"
	goodCaTimeBefore, err := time.Parse(time.RFC3339, goodCaTimeBeforeString)
	if err != nil {
		return certificates.CertOptions{}, &x509.CertificateRequest{}, err
	}

	goodDNSName := "receptor.TEST"
	goodIPAddress := net.ParseIP("127.0.0.1").To4()
	goodNodeIDs := goodDNSName

	goodCertOptions := certificates.CertOptions{
		Bits:       4096,
		CommonName: "Ansible Automation Controller Nodes Mesh",
		CertNames: certificates.CertNames{
			DNSNames:    []string{goodDNSName},
			IPAddresses: []net.IP{goodIPAddress},
			NodeIDs:     []string{goodNodeIDs},
		},
		NotAfter:  goodCaTimeAfter,
		NotBefore: goodCaTimeBefore,
	}

	goodSubjectAltNamesExtension, err := utils.MakeReceptorSAN(goodCertOptions.CertNames.DNSNames, goodCertOptions.IPAddresses, goodCertOptions.NodeIDs)
	if err != nil {
		return goodCertOptions, &x509.CertificateRequest{}, err
	}

	goodCertificateRequest := &x509.CertificateRequest{
		Attributes:     nil,
		DNSNames:       []string{goodDNSName},
		EmailAddresses: nil,
		Extensions: []pkix.Extension{
			*goodSubjectAltNamesExtension,
		},
		ExtraExtensions:    nil,
		IPAddresses:        []net.IP{goodIPAddress},
		PublicKeyAlgorithm: x509.RSA,
		SignatureAlgorithm: x509.SHA256WithRSA,
		Subject: pkix.Name{
			CommonName:         goodCertOptions.CommonName,
			Country:            nil,
			ExtraNames:         []pkix.AttributeTypeAndValue{},
			Locality:           nil,
			Names:              []pkix.AttributeTypeAndValue{},
			Organization:       nil,
			OrganizationalUnit: nil,
			PostalCode:         nil,
			Province:           nil,
			SerialNumber:       "",
			StreetAddress:      nil,
		},
		URIs:    nil,
		Version: 0,
	}

	return goodCertOptions, goodCertificateRequest, nil
}

func setupGoodPrivateKey() (*rsa.PrivateKey, error) {
	goodPrivateKeyBlock, rest := pem.Decode([]byte(`-----BEGIN RSA PRIVATE KEY-----
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
-----END RSA PRIVATE KEY-----`))

	if goodPrivateKeyBlock == nil {
		return &rsa.PrivateKey{}, fmt.Errorf("Failed to decode PEM block containing private key")
	}

	wantBlockType := "RSA PRIVATE KEY"
	if goodPrivateKeyBlock.Type != wantBlockType {
		return &rsa.PrivateKey{}, fmt.Errorf("Decoded PEM block: got: %v, want: %v", goodPrivateKeyBlock.Type, wantBlockType)
	}

	if len(rest) > 0 {
		return &rsa.PrivateKey{}, fmt.Errorf("Unexpected extra data in PEM block: %q", rest)
	}

	goodPrivateKey, err := x509.ParsePKCS1PrivateKey(goodPrivateKeyBlock.Bytes)
	if err != nil {
		return goodPrivateKey, fmt.Errorf("Error parsing Private Key: %v", err)
	}

	return goodPrivateKey, nil
}

func TestCreateCertReqValid(t *testing.T) {
	type args struct {
		opts       *certificates.CertOptions
		privateKey *rsa.PrivateKey
	}

	goodCertOptions, goodCertificateRequest, err := setupGoodCertRequest()
	if err != nil {
		t.Fatal(err)
	}

	goodPrivateKey, err := setupGoodPrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		args    args
		want    *x509.CertificateRequest
		wantErr bool
	}{
		{
			name: "Positive test",
			args: args{
				opts:       &goodCertOptions,
				privateKey: goodPrivateKey,
			},
			want: goodCertificateRequest,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := certificates.CreateCertReq(tt.args.opts, tt.args.privateKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateCertReq() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !reflect.DeepEqual(got.DNSNames, tt.want.DNSNames) {
				t.Errorf("CreateCertReq() DNSNames got = %v, want = %v", got.DNSNames, tt.want.DNSNames)

				return
			}

			if !reflect.DeepEqual(got.EmailAddresses, tt.want.EmailAddresses) {
				t.Errorf("CreateCertReq() EmailAddresses got = %v, want = %v", got.EmailAddresses, tt.want.EmailAddresses)

				return
			}

			if !reflect.DeepEqual(got.ExtraExtensions, tt.want.ExtraExtensions) {
				t.Errorf("CreateCertReq() ExtraExtensions got = %v, want = %v", got.ExtraExtensions, tt.want.ExtraExtensions)

				return
			}

			if !reflect.DeepEqual(got.IPAddresses, tt.want.IPAddresses) {
				t.Errorf("CreateCertReq() IPAddresses got = %v, want = %v", got.IPAddresses, tt.want.IPAddresses)

				return
			}

			if !reflect.DeepEqual(got.PublicKeyAlgorithm, tt.want.PublicKeyAlgorithm) {
				t.Errorf("CreateCertReq() PublicKeyAlgorithm = %v, want = %v", got.PublicKeyAlgorithm, tt.want.PublicKeyAlgorithm)

				return
			}

			if !reflect.DeepEqual(got.SignatureAlgorithm, tt.want.SignatureAlgorithm) {
				t.Errorf("CreateCertReq() SignatureAlgorithm got = %v, want = %v", got.SignatureAlgorithm, tt.want.SignatureAlgorithm)

				return
			}

			if !reflect.DeepEqual(got.URIs, tt.want.URIs) {
				t.Errorf("CreateCertReq() URIs got = %v, want = %v", got.URIs, tt.want.URIs)

				return
			}

			if !reflect.DeepEqual(got.Version, tt.want.Version) {
				t.Errorf("CreateCertReq() Version got = %v, want = %v", got.Version, tt.want.Version)

				return
			}
		})
	}
}

func TestCreateCertReqNegative(t *testing.T) {
	type args struct {
		opts       *certificates.CertOptions
		privateKey *rsa.PrivateKey
	}

	badCertOptions := certificates.CertOptions{
		Bits: -1,
	}
	tests := []struct {
		name    string
		args    args
		want    *x509.CertificateRequest
		wantErr error
	}{
		{
			name: "Negative test for Common Name",
			args: args{
				opts:       &badCertOptions,
				privateKey: nil,
			},
			want:    nil,
			wantErr: fmt.Errorf("must provide CommonName"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, gotErr := certificates.CreateCertReq(tt.args.opts, tt.args.privateKey)
			if gotErr == nil || gotErr.Error() != tt.wantErr.Error() {
				t.Errorf("CreateCertReq() error = %v, wantErr = %v", gotErr, tt.wantErr)
			}
		})
	}
}

func TestCreateCertReqWithKey(t *testing.T) {
	type args struct {
		opts *certificates.CertOptions
	}

	goodCertOptions, goodCertificateRequest, err := setupGoodCertRequest()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		args    args
		want    *x509.CertificateRequest
		want1   *rsa.PrivateKey
		wantErr bool
	}{
		{
			name: "Positive test",
			args: args{
				opts: &goodCertOptions,
			},
			want:    goodCertificateRequest,
			want1:   nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := certificates.CreateCertReqWithKey(tt.args.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateCertReqWithKey() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
		})
	}
}

func TestCreateCertReqWithKeyNegative(t *testing.T) {
	type args struct {
		opts *certificates.CertOptions
	}

	badCertOptions := certificates.CertOptions{
		Bits: -1,
	}

	tests := []struct {
		name    string
		args    args
		want    *x509.CertificateRequest
		want1   *rsa.PrivateKey
		wantErr error
	}{
		{
			name: "Negative test for Bits",
			args: args{
				opts: &badCertOptions,
			},
			want:    nil,
			want1:   nil,
			wantErr: fmt.Errorf("crypto/rsa: too few primes of given length to generate an RSA key"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, gotErr := certificates.CreateCertReqWithKey(tt.args.opts)
			if gotErr == nil || gotErr.Error() != tt.wantErr.Error() {
				t.Errorf("CreateCA() error = %v, wantErr = %v", gotErr, tt.wantErr)
			}
		})
	}
}

func TestGetReqNames(t *testing.T) {
	type args struct {
		request *x509.CertificateRequest
	}

	_, goodCertificateRequest, err := setupGoodCertRequest()
	if err != nil {
		t.Fatal(err)
	}

	goodDNSNames := goodCertificateRequest.DNSNames
	goodIPAddresses := goodCertificateRequest.IPAddresses
	goodNodeIDs := goodDNSNames

	tests := []struct {
		name    string
		args    args
		want    *certificates.CertNames
		wantErr bool
	}{
		{
			name: "Positive test",
			args: args{
				request: goodCertificateRequest,
			},
			want: &certificates.CertNames{
				DNSNames:    goodDNSNames,
				NodeIDs:     goodNodeIDs,
				IPAddresses: goodIPAddresses,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := certificates.GetReqNames(tt.args.request)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetReqNames() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetReqNames() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetReqNamesNegative(t *testing.T) {
	type args struct {
		request *x509.CertificateRequest
	}

	_, goodCertificateRequest, err := setupGoodCertRequest()
	if err != nil {
		t.Fatal(err)
	}

	goodCertificateRequest.Extensions = []pkix.Extension{
		{
			Id:       utils.OIDSubjectAltName,
			Critical: true,
			Value:    nil,
		},
	}

	tests := []struct {
		name    string
		args    args
		want    *certificates.CertNames
		wantErr bool
	}{
		{
			name: "Negative test",
			args: args{
				request: goodCertificateRequest,
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := certificates.GetReqNames(tt.args.request)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetReqNames() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetReqNames() = %v, want %v", got, tt.want)
			}
		})
	}
}
