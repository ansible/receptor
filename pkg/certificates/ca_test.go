//go:build !no_cert_auth
// +build !no_cert_auth

package certificates_test

import (
	"crypto/rand"
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

var excessPEMDataFormatString string = "Excess PEM Data: %v"

var wrongPEMBlockTypeFormatString string = "PEM Block is not a %s: %v"

func setupGoodCA(certOpts *certificates.CertOptions, rsaer certificates.Rsaer) (*certificates.CA, error) {
	certOpts.Bits = 4096
	certOpts.CommonName = "Ansible Automation Controller Nodes Mesh ROOT CA"
	certOpts.DNSNames = nil
	certOpts.NodeIDs = nil
	certOpts.IPAddresses = nil

	goodCaTimeAfterString := "2032-01-07T00:03:51Z"
	goodCaTimeAfter, err := time.Parse(time.RFC3339, goodCaTimeAfterString)
	if err != nil {
		return nil, err
	}
	certOpts.NotAfter = goodCaTimeAfter

	goodCaTimeBeforeString := "2022-01-07T00:03:51Z"
	goodCaTimeBefore, err := time.Parse(time.RFC3339, goodCaTimeBeforeString)
	if err != nil {
		return nil, err
	}
	certOpts.NotBefore = goodCaTimeBefore

	goodCA, err := certificates.CreateCA(certOpts, rsaer)
	if err != nil {
		return nil, err
	}

	return goodCA, nil
}

func setupGoodCertificate() (*x509.Certificate, error) {
	goodCertificatePEMData := setupGoodCertificatePEMData()
	goodCertificateBlock, rest := pem.Decode(goodCertificatePEMData)
	if len(rest) != 0 {
		return &x509.Certificate{}, fmt.Errorf(excessPEMDataFormatString, rest)
	}

	if goodCertificateBlock.Type != "CERTIFICATE" {
		return &x509.Certificate{}, fmt.Errorf(wrongPEMBlockTypeFormatString, "certificate", goodCertificateBlock.Type)
	}

	goodCertificate, err := x509.ParseCertificate(goodCertificateBlock.Bytes)
	if err != nil {
		return &x509.Certificate{}, err
	}

	return goodCertificate, nil
}

func setupGoodCertificatePEMData() []byte {
	return []byte(`-----BEGIN CERTIFICATE-----
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
-----END CERTIFICATE-----`)
}

func setupGoodCertificateRequest() (*x509.CertificateRequest, error) {
	goodCertificateRequestPEMData := setupGoodCertificateRequestPEMData()

	goodCertificateRequestBlock, rest := pem.Decode(goodCertificateRequestPEMData)
	if len(rest) != 0 {
		return &x509.CertificateRequest{}, fmt.Errorf(excessPEMDataFormatString, rest)
	}

	if goodCertificateRequestBlock.Type != "CERTIFICATE REQUEST" {
		return &x509.CertificateRequest{}, fmt.Errorf(wrongPEMBlockTypeFormatString, "certificate request", goodCertificateRequestBlock.Type)
	}

	goodCertificateRequest, err := x509.ParseCertificateRequest(goodCertificateRequestBlock.Bytes)
	if err != nil {
		return &x509.CertificateRequest{}, err
	}

	return goodCertificateRequest, nil
}

func setupGoodCertificateRequestPEMData() []byte {
	return []byte(`-----BEGIN CERTIFICATE REQUEST-----
MIICUjCCAToCAQAwDTELMAkGA1UEBhMCVVMwggEiMA0GCSqGSIb3DQEBAQUAA4IB
DwAwggEKAoIBAQDQUtV5/hb2b2EwJUIhhMtGK42qAzWmd+n0zJTifiMIUYdMRmJS
1OGfzk9aNyuxYXeT5rWRD64o2Zh7KdFhNtLOgYjIczeYQXH8zdYxHtyJjkXaYZNC
8VavhCNriRWP+BET7bAFmDIRvSoWf/ZzjCYS24XI2iR3bTxevW8avE3KwiUYe+sR
OfxT6g86MojD3T7Sv8aaR1w8CoP7GceibYwVG53UqrZb9LTBPP6FgHNCTZmpIclH
o6YFpObpsttbbNIaHXRwIVRaRKHgPc2vY/yt0HgDgk1Qke8LRMUGdR+5P8VJL1k1
618t2uc6nK/OOKRuED9ovbJ29TKhVzWrSc5JAgMBAAGgADANBgkqhkiG9w0BAQsF
AAOCAQEAF/V/ZE6kLbVzH3wgpLbBweotKmA68pPMsYmouvhGsecYWG/Dk6x51r2I
45HT9RfRIfIFKJEso46OzWxPKlmjVIYnqsQHjMQWSIX9FthvqZUrw+R0GTctJymo
qhMS+OWFzbq/eYBZ/kfCIErlyytwAu7TqknAbwUBsyOonLp9ViLepTihawkKKyUk
JvML8+p5qpQEwZ+shqr1J5kkqU44KHiADslqY6Af8q5LPHaLCj0STeTlJ/pgL9I3
IaXWfGohy0VQ0OTftwnNFo48r/KE9JRbYJCAvN4yzUx1zuBgCOy2y6JYeXV3eFTJ
v8wHZVYzsGTM7Rfr6EbECCPahojLDw==
-----END CERTIFICATE REQUEST-----`)
}

func setupGoodRSAPrivateKey() (*rsa.PrivateKey, error) {
	goodRSAPrivateKeyPEMData := setupGoodRSAPrivateKeyPEMData()
	goodRSAPrivateKeyBlock, rest := pem.Decode(goodRSAPrivateKeyPEMData)
	if len(rest) != 0 {
		return &rsa.PrivateKey{}, fmt.Errorf(excessPEMDataFormatString, rest)
	}

	if goodRSAPrivateKeyBlock.Type != "RSA PRIVATE KEY" {
		return &rsa.PrivateKey{}, fmt.Errorf(wrongPEMBlockTypeFormatString, "RSA Private Key", goodRSAPrivateKeyBlock.Type)
	}

	goodRSAPrivateKey, err := x509.ParsePKCS1PrivateKey(goodRSAPrivateKeyBlock.Bytes)
	if err != nil {
		return &rsa.PrivateKey{}, err
	}

	return goodRSAPrivateKey, nil
}

func setupGoodRSAPrivateKeyPEMData() []byte {
	return []byte(`-----BEGIN RSA PRIVATE KEY-----
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
-----END RSA PRIVATE KEY-----`)
}

func setupGoodPrivateKey() (*rsa.PrivateKey, error) {
	goodPrivateKeyPEMData := setupGoodPrivateKeyPEMData()

	goodPrivateKeyBlock, rest := pem.Decode(goodPrivateKeyPEMData)
	if len(rest) != 0 {
		return &rsa.PrivateKey{}, fmt.Errorf(excessPEMDataFormatString, rest)
	}

	if goodPrivateKeyBlock.Type != "PRIVATE KEY" {
		return &rsa.PrivateKey{}, fmt.Errorf(wrongPEMBlockTypeFormatString, "private key", goodPrivateKeyBlock.Type)
	}

	goodResult, err := x509.ParsePKCS8PrivateKey(goodPrivateKeyBlock.Bytes)
	if err != nil {
		return &rsa.PrivateKey{}, err
	}

	goodPrivateKey := goodResult.(*rsa.PrivateKey)

	return goodPrivateKey, nil
}

func setupGoodPrivateKeyPEMData() []byte {
	return []byte(`-----BEGIN PRIVATE KEY-----
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
FYeY7rWtiZX43Wq4szS4xWy7nlk0Jg==
-----END PRIVATE KEY-----`)
}

func setupGoodPublicKey() (*rsa.PublicKey, error) {
	goodPublicKeyPEMData := setupGoodPublicKeyPEMData()

	goodPublicKeyBlock, rest := pem.Decode(goodPublicKeyPEMData)
	if len(rest) != 0 {
		return &rsa.PublicKey{}, fmt.Errorf(excessPEMDataFormatString, rest)
	}

	if goodPublicKeyBlock.Type != "PUBLIC KEY" {
		return &rsa.PublicKey{}, fmt.Errorf(wrongPEMBlockTypeFormatString, "public key", goodPublicKeyBlock.Type)
	}

	goodResult, err := x509.ParsePKIXPublicKey(goodPublicKeyBlock.Bytes)
	if err != nil {
		return &rsa.PublicKey{}, err
	}

	goodPublicKey := goodResult.(*rsa.PublicKey)

	return goodPublicKey, nil
}

func setupGoodPublicKeyPEMData() []byte {
	return []byte(`-----BEGIN PUBLIC KEY-----
MIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAp17EU0yKFdjqoec7zSc0
AWDmT6cZpys8C74HqKeOArJPvswEb4OVyKZj20hFNj2N6TRry0x+pw+eP2XziEc0
jdIqb33K6SbZKyezmKNYF+0TlzN9Md249inCf3DIDImTEC6j3oCobTByxs9E1/tD
HyeY6k5aMFY0gMlISuqTLX9iEqR6jgOrr5i4VIZK7lK1JBzJ28FjE86zvEAzGnS7
1foYlmTWRWn+l7d5TQUWPsq17khu2TnP+lLFg2+DVQCy9ZidCI30noiufEn/FR1G
ODBI8vFVtpXwEVP5nDZMa1GNQwTaec3BzIcKC5CyHfdD8hcs1zAwr2cR6xhMLWdt
1AGGP8AL8NV1puVyQYi82i9dnUrbh3mYLQFDrnEB7xDoJbz4pVOryn+TxXcaqChD
sF7YC1E5cOKLZtm1seadiz5cZDwKWwL+1GsYk23KbiDIfFk00pPxFIKchFI6YYjd
Lqp6dnx/TJsp/IYEgfyn+hYSGRZd1TDTesfFU5Ki5M1RvFHePIBR362lUF72i3Aw
wi8U3nWuk4erk8Nswonwc121sWSo5Yp8fDBDP5CANcHv8JcLGMKUDYZGzqK0d3ie
hMXZdQK/Jd4x6dvd4Qr8VDbsxuWfaDwzEOjEpvMcawTdqWGTS9wwlmidJ47jY2Hj
Ue5e7PvYm1+UQ/rgEoguoTsCAwEAAQ==
-----END PUBLIC KEY-----`)
}

func TestCreateCAValid(t *testing.T) {
	type args struct {
		opts *certificates.CertOptions
	}
	goodCaCertificate, err := setupGoodCertificate()
	if err != nil {
		t.Errorf("Error setting up certificate: %v", err)
	}

	goodCaCertificate.ExtKeyUsage = []x509.ExtKeyUsage{
		x509.ExtKeyUsageClientAuth,
		x509.ExtKeyUsageServerAuth,
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

	goodCaPrivateKey, err := setupGoodRSAPrivateKey()
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
				t.Errorf("CreateCA() Private Key got = %+v, want = %+v", got.PrivateKey, tt.want.PrivateKey)

				return
			}

			certWant := tt.want.Certificate
			certGot := got.Certificate
			if certGot.BasicConstraintsValid != true {
				t.Errorf("CreateCA() Certificate BasicConstraintsValid got = %+v, want = %+v", certGot.BasicConstraintsValid, true)

				return
			}

			if !reflect.DeepEqual(certGot.ExtraExtensions, certWant.ExtraExtensions) {
				t.Errorf("CreateCA() Certificate ExtraExtensions got = %+v, want = %+v", certGot.ExtraExtensions, certWant.ExtraExtensions)

				return
			}

			if !reflect.DeepEqual(certGot.ExtKeyUsage, certWant.ExtKeyUsage) {
				t.Errorf("CreateCA() Certificate ExtKeyUsage got = %+v, want = %+v", certGot.ExtKeyUsage, certWant.ExtKeyUsage)

				return
			}

			if certGot.IsCA != true {
				t.Errorf("CreateCA() Certificate IsCA got = %+v, want = %+v", certGot.IsCA, true)

				return
			}

			if !reflect.DeepEqual(certGot.Issuer, certWant.Issuer) {
				t.Errorf("CreateCA() Certificate Issuer got = %+v, want = %+v", certGot.Issuer, certWant.Issuer)

				return
			}

			if !reflect.DeepEqual(certGot.KeyUsage, certWant.KeyUsage) {
				t.Errorf("CreateCA() Certificate KeyUsage got = %+v, want = %+v", certGot.KeyUsage, certWant.KeyUsage)

				return
			}

			if !reflect.DeepEqual(certGot.NotAfter, certWant.NotAfter) {
				t.Errorf("CreateCA() Certificate NotAfter got = %+v, want = %+v", certGot.NotAfter, certWant.NotAfter)

				return
			}

			if !reflect.DeepEqual(certGot.NotBefore, certWant.NotBefore) {
				t.Errorf("CreateCA() Certificate NotBefore got = %+v, want = %+v", certGot.NotBefore, certWant.NotBefore)

				return
			}

			if !reflect.DeepEqual(certGot.PublicKeyAlgorithm, certWant.PublicKeyAlgorithm) {
				t.Errorf("CreateCA() Certificate PublicKeyAlgorithm got = %+v, want = %+v", certGot.PublicKeyAlgorithm, certWant.PublicKeyAlgorithm)

				return
			}

			if !reflect.DeepEqual(certGot.SignatureAlgorithm, certWant.SignatureAlgorithm) {
				t.Errorf("CreateCA() Certificate SignatureAlgorithm got = %+v, want = %+v", certGot.SignatureAlgorithm, certWant.SignatureAlgorithm)

				return
			}

			if !reflect.DeepEqual(certGot.Subject, certWant.Subject) {
				t.Errorf("CreateCA() Certificate Subject got = %+v, want = %+v", certGot.Subject, certWant.Subject)

				return
			}

			if !reflect.DeepEqual(certGot.Version, certWant.Version) {
				t.Errorf("CreateCA() Certificate Version got = %+v, want = %+v", certGot.Version, certWant.Version)

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

func setupBadCertRequest() (certificates.CertOptions, *x509.CertificateRequest, error) {
	badCaTimeAfterString := "2021-01-07T00:03:51Z"
	badCaTimeAfter, err := time.Parse(time.RFC3339, badCaTimeAfterString)
	if err != nil {
		return certificates.CertOptions{}, &x509.CertificateRequest{}, err
	}

	badCaTimeBeforeString := "2022-01-07T00:03:51Z"
	badCaTimeBefore, err := time.Parse(time.RFC3339, badCaTimeBeforeString)
	if err != nil {
		return certificates.CertOptions{}, &x509.CertificateRequest{}, err
	}

	badDNSName := "receptor.TEST.BAD"
	badIPAddress := net.ParseIP("127.0.0.1").To4()
	badNodeIDs := badDNSName

	badCertOptions := certificates.CertOptions{
		Bits:       -1,
		CommonName: "Ansible Automation Controller Nodes Mesh Bad",
		CertNames: certificates.CertNames{
			DNSNames:    []string{badDNSName},
			IPAddresses: []net.IP{badIPAddress},
			NodeIDs:     []string{badNodeIDs},
		},
		NotAfter:  badCaTimeAfter,
		NotBefore: badCaTimeBefore,
	}

	badSubjectAltNamesExtension, err := utils.MakeReceptorSAN(badCertOptions.CertNames.DNSNames,
		badCertOptions.IPAddresses,
		badCertOptions.NodeIDs)
	if err != nil {
		return badCertOptions, &x509.CertificateRequest{}, err
	}

	badCertificateRequest := &x509.CertificateRequest{
		Attributes:     nil,
		DNSNames:       []string{badDNSName},
		EmailAddresses: nil,
		Extensions: []pkix.Extension{
			*badSubjectAltNamesExtension,
		},
		ExtraExtensions:    nil,
		IPAddresses:        []net.IP{badIPAddress},
		PublicKeyAlgorithm: x509.RSA,
		SignatureAlgorithm: x509.SHA256WithRSA,
		Subject: pkix.Name{
			CommonName:         badCertOptions.CommonName,
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

	return badCertOptions, badCertificateRequest, nil
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

func TestCreateCertReqValid(t *testing.T) {
	type args struct {
		opts       *certificates.CertOptions
		privateKey *rsa.PrivateKey
	}

	goodCertOptions, goodCertificateRequest, err := setupGoodCertRequest()
	if err != nil {
		t.Fatal(err)
	}

	goodRSAPrivateKey, err := setupGoodRSAPrivateKey()
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
				privateKey: goodRSAPrivateKey,
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
				t.Errorf("CreateCertReq() DNSNames got = %+v, want = %+v", got.DNSNames, tt.want.DNSNames)

				return
			}

			if !reflect.DeepEqual(got.EmailAddresses, tt.want.EmailAddresses) {
				t.Errorf("CreateCertReq() EmailAddresses got = %+v, want = %+v", got.EmailAddresses, tt.want.EmailAddresses)

				return
			}

			if !reflect.DeepEqual(got.ExtraExtensions, tt.want.ExtraExtensions) {
				t.Errorf("CreateCertReq() ExtraExtensions got = %+v, want = %+v", got.ExtraExtensions, tt.want.ExtraExtensions)

				return
			}

			if !reflect.DeepEqual(got.IPAddresses, tt.want.IPAddresses) {
				t.Errorf("CreateCertReq() IPAddresses got = %+v, want = %+v", got.IPAddresses, tt.want.IPAddresses)

				return
			}

			if !reflect.DeepEqual(got.PublicKeyAlgorithm, tt.want.PublicKeyAlgorithm) {
				t.Errorf("CreateCertReq() PublicKeyAlgorithm = %+v, want = %+v", got.PublicKeyAlgorithm, tt.want.PublicKeyAlgorithm)

				return
			}

			if !reflect.DeepEqual(got.SignatureAlgorithm, tt.want.SignatureAlgorithm) {
				t.Errorf("CreateCertReq() SignatureAlgorithm got = %+v, want = %+v", got.SignatureAlgorithm, tt.want.SignatureAlgorithm)

				return
			}

			if !reflect.DeepEqual(got.URIs, tt.want.URIs) {
				t.Errorf("CreateCertReq() URIs got = %+v, want = %+v", got.URIs, tt.want.URIs)

				return
			}

			if !reflect.DeepEqual(got.Version, tt.want.Version) {
				t.Errorf("CreateCertReq() Version got = %+v, want = %+v", got.Version, tt.want.Version)

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
				t.Errorf("CreateCertReqWithKey() error = %v, wantErr = %v", gotErr, tt.wantErr)
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

func TestLoadCertificate(t *testing.T) {
	type args struct {
		filename string
	}

	goodCertificate, err := setupGoodCertificate()
	if err != nil {
		t.Fatal(err)
	}

	positiveTestFilename := "positive_test_filename"
	tests := []struct {
		name    string
		args    args
		want    *x509.Certificate
		wantErr bool
	}{
		{
			name: "Positive test",
			args: args{
				filename: positiveTestFilename,
			},
			want:    goodCertificate,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			o := mock_certificates.NewMockOser(ctrl)
			switch tt.args.filename {
			case positiveTestFilename:
				o.
					EXPECT().
					ReadFile(gomock.Eq(positiveTestFilename)).
					Return(setupGoodCertificatePEMData(), nil).
					Times(1)

			default:
				t.Errorf("Unexpected filename: %s", tt.args.filename)
			}

			got, err := certificates.LoadCertificate(tt.args.filename, o)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadCertificate() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadCertificate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadCertificatesNegative(t *testing.T) {
	type args struct {
		filename string
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

	negativeMultipleItemTest := "negative_multiple_item_test"

	multipleCertificates := setupGoodCertificatePEMData()
	multipleCertificates = append(multipleCertificates, multipleCertificates[0])

	negativeNoCertificateTest := "negative_no_certificate_test"
	noCertificates := []byte{
		0, 0, 0, 0,
	}

	tests := []struct {
		name    string
		args    args
		want    *x509.Certificate
		wantErr bool
	}{
		{
			name: "Negative multiple item test",
			args: args{
				filename: negativeMultipleItemTest,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Negative no certificate test",
			args: args{
				filename: negativeNoCertificateTest,
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			o := mock_certificates.NewMockOser(ctrl)

			switch tt.args.filename {
			case negativeMultipleItemTest:
				o.
					EXPECT().
					ReadFile(gomock.Eq(negativeMultipleItemTest)).
					Return(multipleCertificates, nil).
					Times(1)

			case negativeNoCertificateTest:
				o.
					EXPECT().
					ReadFile(gomock.Eq(negativeNoCertificateTest)).
					Return(noCertificates, nil).
					Times(1)

			default:
				t.Errorf("Unexpected filename: %s", tt.args.filename)
			}
			got, err := certificates.LoadCertificate(tt.args.filename, o)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadCertificate() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetReqNames() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadFromPEMFile(t *testing.T) {
	type args struct {
		filename string
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	o := mock_certificates.NewMockOser(ctrl)

	errorSettingUpTypeFormatString := "Error setting up %s: %v"

	certificateTestFilename := "certificate_test_filename"
	goodCertificate, err := setupGoodCertificate()
	if err != nil {
		t.Errorf(errorSettingUpTypeFormatString, "certificate", err)
	}

	certificateRequestTestFilename := "certificate_request_test_filename"
	goodCertificateRequest, err := setupGoodCertificateRequest()
	if err != nil {
		t.Errorf(errorSettingUpTypeFormatString, "certificate request", err)
	}

	rsaPrivateKeyTestFilename := "rsa_private_key_test_filename"
	goodRSAPrivateKey, err := setupGoodRSAPrivateKey()
	if err != nil {
		t.Errorf(errorSettingUpTypeFormatString, "rsa private key", err)
	}

	privateKeyTestFilename := "private_key_test_filename"
	goodPrivateKey, err := setupGoodPrivateKey()
	if err != nil {
		t.Errorf(errorSettingUpTypeFormatString, "private key", err)
	}

	publicKeyTestFilename := "public_key_test_filename"
	goodPublicKey, err := setupGoodPublicKey()
	if err != nil {
		t.Errorf(errorSettingUpTypeFormatString, "public key", err)
	}

	failedToDecodeTestFilename := "failed_to_decode_test_filename"
	failedToDecodeFileContents := []byte{
		0, 0, 0, 0,
	}

	unknownBlockTypeTestFilename := "unknown_block_type_test_filename"

	tests := []struct {
		name                  string
		args                  args
		wantOserReadfileCalls func()
		want                  []interface{}
		wantErr               bool
	}{
		{
			name: "Certificate",
			args: args{
				filename: certificateTestFilename,
			},
			wantOserReadfileCalls: func() {
				o.
					EXPECT().
					ReadFile(gomock.Any()).
					Return(interface{}(setupGoodCertificatePEMData()), nil).
					Times(1)
			},
			want: []interface{}{
				goodCertificate,
			},
			wantErr: false,
		},
		{
			name: "Certificate Request",
			args: args{
				filename: certificateRequestTestFilename,
			},
			wantOserReadfileCalls: func() {
				o.
					EXPECT().
					ReadFile(gomock.Any()).
					Return(interface{}(setupGoodCertificateRequestPEMData()), nil).
					Times(1)
			},
			want: []interface{}{
				goodCertificateRequest,
			},
			wantErr: false,
		},
		{
			name: "RSA Private Key",
			args: args{
				filename: rsaPrivateKeyTestFilename,
			},
			wantOserReadfileCalls: func() {
				o.
					EXPECT().
					ReadFile(gomock.Any()).
					Return(interface{}(setupGoodRSAPrivateKeyPEMData()), nil).
					Times(1)
			},
			want: []interface{}{
				goodRSAPrivateKey,
			},
			wantErr: false,
		},
		{
			name: "Private Key",
			args: args{
				filename: privateKeyTestFilename,
			},
			wantOserReadfileCalls: func() {
				o.
					EXPECT().
					ReadFile(gomock.Any()).
					Return(interface{}(setupGoodPrivateKeyPEMData()), nil).
					Times(1)
			},
			want: []interface{}{
				goodPrivateKey,
			},
			wantErr: false,
		},
		{
			name: "Public Key",
			args: args{
				filename: publicKeyTestFilename,
			},
			wantOserReadfileCalls: func() {
				o.
					EXPECT().
					ReadFile(gomock.Any()).
					Return(interface{}(setupGoodPublicKeyPEMData()), nil).
					Times(1)
			},
			want: []interface{}{
				goodPublicKey,
			},
			wantErr: false,
		},
		{
			name: "Failed to decode",
			args: args{
				filename: failedToDecodeTestFilename,
			},
			wantOserReadfileCalls: func() {
				o.
					EXPECT().
					ReadFile(gomock.Any()).
					Return(interface{}(failedToDecodeFileContents), nil).
					Times(1)
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Unknown block type",
			args: args{
				filename: unknownBlockTypeTestFilename,
			},
			wantOserReadfileCalls: func() {
				o.
					EXPECT().
					ReadFile(gomock.Any()).
					Return(interface{}(setupUnknownBlockTypeFileContents()), nil).
					Times(1)
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantOserReadfileCalls()

			got, err := certificates.LoadFromPEMFile(tt.args.filename, o)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadFromPEMFile() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadFromPEMFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func setupUnknownBlockTypeFileContents() []byte {
	return []byte(`-----BEGIN PKCS7-----
MIIFhAYJKoZIhvcNAQcCoIIFdTCCBXECAQExADALBgkqhkiG9w0BBwGgggVZMIIF
VTCCAz2gAwIBAgIEYdeDaTANBgkqhkiG9w0BAQsFADA7MTkwNwYDVQQDEzBBbnNp
YmxlIEF1dG9tYXRpb24gQ29udHJvbGxlciBOb2RlcyBNZXNoIFJPT1QgQ0EwHhcN
MjIwMTA3MDAwMzUxWhcNMzIwMTA3MDAwMzUxWjA7MTkwNwYDVQQDEzBBbnNpYmxl
IEF1dG9tYXRpb24gQ29udHJvbGxlciBOb2RlcyBNZXNoIFJPT1QgQ0EwggIiMA0G
CSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQCxAErOWvVDU8mfZgtE6BSygTWUMkPx
xIEQSYs/UesRAHaB+QXa7/0Foa0VUJKcWwUE+2yYkNRrg8MmE8VWMSewcaNIAs40
7stFXP+A2anPEglwemTskpO72sigiYDKShC5n5ciyPsHckwVlOCTtac5TwFeeTmG
nHWRcd4uBGvaEXx98fw/wLgYtr9vmKTdnOQjriX9EaAWrjlrlzm54Bs3uVUjGSL7
zY381EuUVV4AjbqQyThbY9cVfsK0nmzLUqpiHG2IhGZDZA9+jxtz2wJWFkNQnWA3
afCUjcWV+4FpP3p1U1myCeh2yR2uCHs9pkUK3ts9uD/Wd5j9M1oBMlymbN/C5Fah
d+cTXrPAjsoRqCso9TBP4mIlNl1Jq8MRUWTL5HOuwn+KnufBtuQ1hIb71Esokj90
eWeo/P+temYAEquUVWiej7lnHyZVW647lE+o+xJEOmW+tY5H4jgA/twP4s7UBgR5
45usWF9/utvnhsGSkg1EYcdzaM01pkrWrw1GvHT++HshsrG6Tse8gY7JrTdsLDtU
8LPhPUEnSfVBcgcMg2Dg8lEbaODtdp2xtCJwZHy9CiAx3CKcogVEfecrKSSr2iSo
cft9/l8J+mhVG2CI6ekG6Cy9hDct/3SV01Dfd4FG7xXJIE3mTDLILpi1AVIWMHxY
D3skuQMyLhJDAwIDAQABo2EwXzAOBgNVHQ8BAf8EBAMCAoQwHQYDVR0lBBYwFAYI
KwYBBQUHAwIGCCsGAQUFBwMBMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFFAl
C81rH211fPJoWglERKb/7/NfMA0GCSqGSIb3DQEBCwUAA4ICAQCWCP/O6YQ9jhae
2/BeUeeKsnoxf90prg3o6/QHbelF6yL+MvGg5ZbPSlt+ywLDNR2CYvXk/4SD5To7
CPKhSPBwwUJpafvQfAOZijU30fvXvp5yZEFoOzOyvBP58NfzL5qH6Pf5A6i3rHvt
R1v7DgS7u2qWWcSimIM0UPoV3JubLTEORjOR6FIyNkIxdjhrP3SxyZ54xxdeG3bc
hKaRcGVNoFYSDN4bAA22JAjlD8kXNYKzIS/0cOR/9SnHd1wMIQ2trx0+TfyGFAA1
mW1mjzQd+h5SGBVeCz2W2XttNSIfQDndJCsyACxmIaOK99AQxdhZsWfHtGO13Tjn
yoiHjf8rozJbAVYqrIdB6GDf6fUlxwhUXT0qkgOvvAzjNnLoOBUkE4TWqXHl38a+
ITDNVzaUlrTd63eexS69V6kHe7mrqjywNQ9EXF9kaVeoNTzRf/ztT/DEVAl+rKsh
Mt4IOKQf1ScE+EJe1njpREHV+fa+kYvQB6cRuxW9a8sOSeQNaSL73Zv54elZxffY
hMv6yXvVxVnJHEsG3kM/CsvsU364BBd9kDcZbHpjNcDHMu+XxECJjD2atVtuFdaO
LykGKfMCYVBP+xs97IJO8En/5N9QQwc+N4cfCg9/BWoZKHPbRx/V+57VEj0m69Ep
JXbL15ZQLCPsaIcqJqpK23VyJKc8fDEA
-----END PKCS7-----`)
}

func TestLoadPrivateKey(t *testing.T) {
	type args struct {
		filename string
	}

	positivePrivateKeyFilename := "private_key_test_filename"

	errorSettingUpTypeFormatString := "Error setting up %s: %v"

	goodPrivateKey, err := setupGoodPrivateKey()
	if err != nil {
		t.Errorf(errorSettingUpTypeFormatString, "private key", err)
	}

	negativeMultipleItemFilename := "negative_multiple_item_test"
	multiplePrivateKeys := setupGoodPrivateKeyPEMData()
	multiplePrivateKeys = append(multiplePrivateKeys, multiplePrivateKeys[0])

	negativeNoPrivateKeyFilename := "negative_no_private_key_test"
	noPrivateKey := []byte{
		0, 0, 0, 0, 0,
	}

	tests := []struct {
		name                   string
		args                   args
		wantOserReadfileArg    string
		wantOserReadfileResult []byte
		want                   *rsa.PrivateKey
		wantErr                bool
	}{
		{
			name: "Positive Private Key",
			args: args{
				filename: positivePrivateKeyFilename,
			},
			wantOserReadfileArg:    positivePrivateKeyFilename,
			wantOserReadfileResult: setupGoodPrivateKeyPEMData(),
			want:                   goodPrivateKey,
			wantErr:                false,
		},
		{
			name: "Negative multi item test",
			args: args{
				filename: negativeMultipleItemFilename,
			},
			wantOserReadfileArg:    negativeMultipleItemFilename,
			wantOserReadfileResult: multiplePrivateKeys,
			want:                   nil,
			wantErr:                true,
		},
		{
			name: "Negative no private key test",
			args: args{
				filename: negativeNoPrivateKeyFilename,
			},
			wantOserReadfileArg:    negativeNoPrivateKeyFilename,
			wantOserReadfileResult: noPrivateKey,
			want:                   nil,
			wantErr:                true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			o := mock_certificates.NewMockOser(ctrl)
			o.
				EXPECT().
				ReadFile(gomock.Eq(tt.wantOserReadfileArg)).
				Return(tt.wantOserReadfileResult, nil).
				Times(1)

			got, err := certificates.LoadPrivateKey(tt.args.filename, o)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadPrivateKey() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadPrivateKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadPublicKey(t *testing.T) {
	type args struct {
		filename string
	}

	errorSettingUpTypeFormatString := "Error setting up %s: %v"

	positivePublicKeyFilename := "public_key_test_filename"
	goodPublicKey, err := setupGoodPublicKey()
	if err != nil {
		t.Errorf(errorSettingUpTypeFormatString, "public key", err)
	}

	negativeMultipleItemFilename := "negative_multiple_item_test"
	multiplePublicKeys := setupGoodPublicKeyPEMData()
	multiplePublicKeys = append(multiplePublicKeys, multiplePublicKeys[0])

	negativeNoPublicKeyFilename := "negative_no_public_key_test"
	noPublicKey := []byte{
		0, 0, 0, 0,
	}

	tests := []struct {
		name                   string
		args                   args
		wantOserReadfileArg    string
		wantOserReadfileResult []byte
		want                   *rsa.PublicKey
		wantErr                bool
	}{
		{
			name: "Positive Public Key",
			args: args{
				filename: positivePublicKeyFilename,
			},
			wantOserReadfileArg:    positivePublicKeyFilename,
			wantOserReadfileResult: setupGoodPublicKeyPEMData(),
			want:                   goodPublicKey,
			wantErr:                false,
		},
		{
			name: "Negative multi item test",
			args: args{
				filename: negativeMultipleItemFilename,
			},
			wantOserReadfileArg:    negativeMultipleItemFilename,
			wantOserReadfileResult: multiplePublicKeys,
			want:                   nil,
			wantErr:                true,
		},
		{
			name: "Negative no public key test",
			args: args{
				filename: negativeNoPublicKeyFilename,
			},
			wantOserReadfileArg:    negativeNoPublicKeyFilename,
			wantOserReadfileResult: noPublicKey,
			want:                   nil,
			wantErr:                true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			o := mock_certificates.NewMockOser(ctrl)
			o.
				EXPECT().
				ReadFile(gomock.Eq(tt.wantOserReadfileArg)).
				Return(tt.wantOserReadfileResult, nil).
				Times(1)

			got, err := certificates.LoadPublicKey(tt.args.filename, o)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadPublicKey() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadPublicKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadRequest(t *testing.T) {
	type args struct {
		filename string
	}
	errorSettingUpTypeFormatString := "Error setting up %s: %v"

	positiveRequestFilename := "request_test_filename"
	goodRequest, err := setupGoodCertificateRequest()
	if err != nil {
		t.Errorf(errorSettingUpTypeFormatString, "request", err)
	}

	negativeMultipleItemFilename := "negative_multiple_item_test"
	multipleRequests := setupGoodCertificateRequestPEMData()
	multipleRequests = append(multipleRequests, multipleRequests[0])

	negativeNoRequestFilename := "negative_no_request_test"
	noRequest := []byte{
		0, 0, 0, 0,
	}

	tests := []struct {
		name                   string
		args                   args
		wantOserReadfileArg    string
		wantOserReadfileResult []byte
		want                   *x509.CertificateRequest
		wantErr                bool
	}{
		{
			name: "Positive Request",
			args: args{
				filename: positiveRequestFilename,
			},
			wantOserReadfileArg:    positiveRequestFilename,
			wantOserReadfileResult: setupGoodCertificateRequestPEMData(),
			want:                   goodRequest,
			wantErr:                false,
		},
		{
			name: "Negative multi item test",
			args: args{
				filename: negativeMultipleItemFilename,
			},
			wantOserReadfileArg:    negativeMultipleItemFilename,
			wantOserReadfileResult: multipleRequests,
			want:                   nil,
			wantErr:                true,
		},
		{
			name: "Negative no request test",
			args: args{
				filename: negativeNoRequestFilename,
			},
			wantOserReadfileArg:    negativeNoRequestFilename,
			wantOserReadfileResult: noRequest,
			want:                   nil,
			wantErr:                true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			o := mock_certificates.NewMockOser(ctrl)
			o.
				EXPECT().
				ReadFile(gomock.Eq(tt.wantOserReadfileArg)).
				Return(tt.wantOserReadfileResult, nil).
				Times(1)

			got, err := certificates.LoadRequest(tt.args.filename, o)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadRequest() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRsaWrapper_GenerateKey(t *testing.T) {
	type args struct {
		random io.Reader
		bits   int
	}

	errorSettingUpTypeFormatString := "Error setting up %s: %v"

	goodPrivateKey, err := setupGoodPrivateKey()
	if err != nil {
		t.Errorf(errorSettingUpTypeFormatString, "private key", err)
	}

	tests := []struct {
		name                            string
		args                            args
		wantGenerateKeyRandomArg        io.Reader
		wantGenerateKeyBitsArg          int
		wantGenerateKeyPrivateKeyResult *rsa.PrivateKey
		wantGenerateKeyErrorResult      error
		want                            *rsa.PrivateKey
		wantErr                         bool
	}{
		{
			name: "Positive test",
			args: args{
				random: rand.Reader,
				bits:   2048,
			},
			wantGenerateKeyRandomArg:        nil,
			wantGenerateKeyBitsArg:          2048,
			wantGenerateKeyPrivateKeyResult: goodPrivateKey,
			wantGenerateKeyErrorResult:      nil,
			want:                            goodPrivateKey,
			wantErr:                         false,
		},
		{
			name: "Negative test",
			args: args{
				random: rand.Reader,
				bits:   -1,
			},
			wantGenerateKeyRandomArg:        nil,
			wantGenerateKeyBitsArg:          -1,
			wantGenerateKeyPrivateKeyResult: nil,
			wantGenerateKeyErrorResult:      fmt.Errorf("Error result"),
			want:                            nil,
			wantErr:                         true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRsa := mock_certificates.NewMockRsaer(ctrl)
			mockRsa.
				EXPECT().
				GenerateKey(gomock.Any(), gomock.Eq(tt.wantGenerateKeyBitsArg)).
				Return(tt.wantGenerateKeyPrivateKeyResult, tt.wantGenerateKeyErrorResult).
				Times(1)

			got, err := mockRsa.GenerateKey(tt.args.random, tt.args.bits)
			if (err != nil) != tt.wantErr {
				t.Errorf("RsaWrapper.GenerateKey() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RsaWrapper.GenerateKey() = %v, want %v", got, tt.want)
				t.Errorf("LoadRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSaveToPEMFile(t *testing.T) {
	type args struct {
		filename string
		data     []interface{}
	}

	errorSettingUpTypeFormatString := "Error setting up %s: %v"

	certificateRequestTestFilename := "certificate_request_test_filename"
	goodRequest, err := setupGoodCertificateRequest()
	if err != nil {
		t.Errorf(errorSettingUpTypeFormatString, "request", err)
	}

	certificateTestFilename := "certificate_test_filename"
	goodCaCertificate, err := setupGoodCertificate()
	if err != nil {
		t.Errorf("Error setting up certificate: %v", err)
	}

	failedToEncodeTestFilename := "failed_to_encode_test_filename"

	privateKeyTestFilename := "private_key_test_filename"
	goodPrivateKey, err := setupGoodPrivateKey()
	if err != nil {
		t.Errorf(errorSettingUpTypeFormatString, "private key", err)
	}

	publicKeyTestFilename := "public_key_test_filename"
	goodPublicKey, err := setupGoodPublicKey()
	if err != nil {
		t.Errorf(errorSettingUpTypeFormatString, "public key", err)
	}

	rsaPrivateKeyTestFilename := "rsa_private_key_test_filename"
	goodCaPrivateKey, err := setupGoodRSAPrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	unknownBlockTypeTestFilename := "unknown_block_type_test_filename"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	o := mock_certificates.NewMockOser(ctrl)

	tests := []struct {
		name                   string
		args                   args
		wantOserWritefileCalls func()
		want                   []interface{}
		wantErr                bool
	}{
		{
			name: "Certificate",
			args: args{
				filename: certificateTestFilename,
				data: []interface{}{
					goodCaCertificate,
				},
			},
			wantOserWritefileCalls: func() {
				o.
					EXPECT().
					WriteFile(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(interface{}(nil)).
					Times(1)
			},
			want: []interface{}{
				nil,
			},
			wantErr: false,
		},
		{
			name: "Certificate Request",
			args: args{
				filename: certificateRequestTestFilename,
				data: []interface{}{
					goodRequest,
				},
			},
			wantOserWritefileCalls: func() {
				o.
					EXPECT().
					WriteFile(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(interface{}(nil)).
					Times(1)
			},
			want: []interface{}{
				nil,
			},
			wantErr: false,
		},
		{
			name: "RSA Private Key",
			args: args{
				filename: rsaPrivateKeyTestFilename,
				data: []interface{}{
					goodCaPrivateKey,
				},
			},
			wantOserWritefileCalls: func() {
				o.
					EXPECT().
					WriteFile(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(interface{}(nil)).
					Times(1)
			},
			want: []interface{}{
				nil,
			},
			wantErr: false,
		},
		{
			name: "Private Key",
			args: args{
				filename: privateKeyTestFilename,
				data: []interface{}{
					goodPrivateKey,
				},
			},
			wantOserWritefileCalls: func() {
				o.
					EXPECT().
					WriteFile(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(interface{}(nil)).
					Times(1)
			},
			want: []interface{}{
				nil,
			},
			wantErr: false,
		},
		{
			name: "Public Key",
			args: args{
				filename: publicKeyTestFilename,
				data: []interface{}{
					goodPublicKey,
				},
			},
			wantOserWritefileCalls: func() {
				o.
					EXPECT().
					WriteFile(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(interface{}(nil)).
					Times(1)
			},
			want: []interface{}{
				nil,
			},
			wantErr: false,
		},
		{
			name: "Failed to encode",
			args: args{
				filename: failedToEncodeTestFilename,
				data: []interface{}{
					[]byte{
						0, 0, 0, 0,
					},
				},
			},
			wantOserWritefileCalls: func() {
				o.
					EXPECT().
					WriteFile(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(interface{}(nil)).
					Times(0)
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Unknown block type",
			args: args{
				filename: unknownBlockTypeTestFilename,
				data: []interface{}{
					nil,
				},
			},
			wantOserWritefileCalls: func() {
				o.
					EXPECT().
					WriteFile(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(interface{}(nil)).
					Times(0)
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantOserWritefileCalls()

			if err := certificates.SaveToPEMFile(tt.args.filename, tt.args.data, o); (err != nil) != tt.wantErr {
				t.Errorf("SaveToPEMFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSignCertReq(t *testing.T) {
	type args struct {
		req *x509.CertificateRequest
		// ca   *certificates.CA
		caOpts *certificates.CertOptions
		opts   *certificates.CertOptions
	}

	badCertOptions, badCertificateRequest, err := setupBadCertRequest()
	if err != nil {
		t.Fatal(err)
	}

	badCertOptions.Bits = -1
	badCertificateRequest.SignatureAlgorithm = x509.DSAWithSHA1

	goodCaCertOptions, _, err := setupGoodCertRequest()
	if err != nil {
		t.Fatal(err)
	}

	goodCaPrivateKey, err := setupGoodRSAPrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	goodCaPublicKey := goodCaPrivateKey.Public()

	errorSettingUpTypeFormatString := "Error setting up %s: %v"

	goodCertificate, err := setupGoodCertificate()
	if err != nil {
		t.Errorf(errorSettingUpTypeFormatString, "certificate", err)
	}
	goodCertificate.KeyUsage = 1

	goodCertOptions, goodCertificateRequest, err := setupGoodCertRequest()
	if err != nil {
		t.Fatal(err)
	}

	goodCertOptions.CommonName = "Ansible Automation Controller Nodes Mesh"
	goodCertificateRequest.PublicKey = goodCaPublicKey

	tests := []struct {
		name    string
		args    args
		want    *x509.Certificate
		wantErr bool
	}{
		{
			name: "Positive test",
			args: args{
				req:    goodCertificateRequest,
				caOpts: &goodCaCertOptions,
				opts:   &goodCertOptions,
			},
			want:    goodCertificate,
			wantErr: false,
		},
		{
			name: "Negative test",
			args: args{
				req:    badCertificateRequest,
				caOpts: &goodCaCertOptions,
				opts:   &badCertOptions,
			},
			want:    nil,
			wantErr: true,
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

			CA, err := setupGoodCA(tt.args.caOpts, mockRsa)
			if err != nil {
				t.Fatal(err)
			}

			certGot, err := certificates.SignCertReq(tt.args.req, CA, tt.args.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("SignCertReq() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if ( err == nil ) {
				if certGot.BasicConstraintsValid != false {
					t.Errorf("CreateCA() Certificate BasicConstraintsValid got = %+v, want = %+v", certGot.BasicConstraintsValid, false)

					return
				}

				if !reflect.DeepEqual(certGot.ExtraExtensions, tt.want.ExtraExtensions) {
					t.Errorf("CreateCA() Certificate ExtraExtensions got = %+v, want = %+v", certGot.ExtraExtensions, tt.want.ExtraExtensions)

					return
				}

				if !reflect.DeepEqual(certGot.ExtKeyUsage, tt.want.ExtKeyUsage) {
					t.Errorf("CreateCA() Certificate ExtKeyUsage got = %+v, want = %+v", certGot.ExtKeyUsage, tt.want.ExtKeyUsage)

					return
				}

				if certGot.IsCA != false {
					t.Errorf("CreateCA() Certificate IsCA got = %+v, want = %+v", certGot.IsCA, false)

					return
				}

				if !reflect.DeepEqual(certGot.Issuer, tt.want.Issuer) {
					t.Errorf("CreateCA() Certificate Issuer got = %+v, want = %+v", certGot.Issuer, tt.want.Issuer)

					return
				}

				if !reflect.DeepEqual(certGot.KeyUsage, tt.want.KeyUsage) {
					t.Errorf("CreateCA() Certificate KeyUsage got = %+v, want = %+v", certGot.KeyUsage, tt.want.KeyUsage)

					return
				}

				if !reflect.DeepEqual(certGot.NotAfter, tt.want.NotAfter) {
					t.Errorf("CreateCA() Certificate NotAfter got = %+v, want = %+v", certGot.NotAfter, tt.want.NotAfter)

					return
				}

				if !reflect.DeepEqual(certGot.NotBefore, tt.want.NotBefore) {
					t.Errorf("CreateCA() Certificate NotBefore got = %+v, want = %+v", certGot.NotBefore, tt.want.NotBefore)

					return
				}

				if !reflect.DeepEqual(certGot.PublicKeyAlgorithm, tt.want.PublicKeyAlgorithm) {
					t.Errorf("CreateCA() Certificate PublicKeyAlgorithm got = %+v, want = %+v", certGot.PublicKeyAlgorithm, tt.want.PublicKeyAlgorithm)

					return
				}

				if !reflect.DeepEqual(certGot.SignatureAlgorithm, tt.want.SignatureAlgorithm) {
					t.Errorf("CreateCA() Certificate SignatureAlgorithm got = %+v, want = %+v", certGot.SignatureAlgorithm, tt.want.SignatureAlgorithm)

					return
				}

				if certGot.Subject.String() != "CN=Ansible Automation Controller Nodes Mesh" {
					t.Errorf("CreateCA() Certificate Subject got = %+v, want = %+v", certGot.Subject, "CN=Ansible Automation Controller Nodes Mesh")

					return
				}

				if !reflect.DeepEqual(certGot.Version, tt.want.Version) {
					t.Errorf("CreateCA() Certificate Version got = %+v, want = %+v", certGot.Version, tt.want.Version)

					return
				}
			}
		})
	}
}
