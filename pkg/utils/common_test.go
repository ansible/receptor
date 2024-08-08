package utils_test

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"math/big"
	"reflect"
	"testing"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/utils"
)

func TestParseReceptorNamesFromCert(t *testing.T) {
	type args struct {
		cert             *x509.Certificate
		expectedHostname string
		logger           *logger.ReceptorLogger
	}

	const testGoodCertPEMData = `
-----BEGIN CERTIFICATE-----
MIIFRDCCAyygAwIBAgIEYiZSQDANBgkqhkiG9w0BAQsFADA7MTkwNwYDVQQDEzBB
bnNpYmxlIEF1dG9tYXRpb24gQ29udHJvbGxlciBOb2RlcyBNZXNoIFJPT1QgQ0Ew
HhcNMjIwMzA3MTg0MzEyWhcNMzExMjI4MDUwMzUxWjARMQ8wDQYDVQQDEwZmb29i
YXIwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQCnXsRTTIoV2Oqh5zvN
JzQBYOZPpxmnKzwLvgeop44Csk++zARvg5XIpmPbSEU2PY3pNGvLTH6nD54/ZfOI
RzSN0ipvfcrpJtkrJ7OYo1gX7ROXM30x3bj2KcJ/cMgMiZMQLqPegKhtMHLGz0TX
+0MfJ5jqTlowVjSAyUhK6pMtf2ISpHqOA6uvmLhUhkruUrUkHMnbwWMTzrO8QDMa
dLvV+hiWZNZFaf6Xt3lNBRY+yrXuSG7ZOc/6UsWDb4NVALL1mJ0IjfSeiK58Sf8V
HUY4MEjy8VW2lfARU/mcNkxrUY1DBNp5zcHMhwoLkLId90PyFyzXMDCvZxHrGEwt
Z23UAYY/wAvw1XWm5XJBiLzaL12dStuHeZgtAUOucQHvEOglvPilU6vKf5PFdxqo
KEOwXtgLUTlw4otm2bWx5p2LPlxkPApbAv7UaxiTbcpuIMh8WTTSk/EUgpyEUjph
iN0uqnp2fH9Mmyn8hgSB/Kf6FhIZFl3VMNN6x8VTkqLkzVG8Ud48gFHfraVQXvaL
cDDCLxTeda6Th6uTw2zCifBzXbWxZKjlinx8MEM/kIA1we/wlwsYwpQNhkbOorR3
eJ6Exdl1Ar8l3jHp293hCvxUNuzG5Z9oPDMQ6MSm8xxrBN2pYZNL3DCWaJ0njuNj
YeNR7l7s+9ibX5RD+uASiC6hOwIDAQABo3oweDAOBgNVHQ8BAf8EBAMCB4AwHQYD
VR0lBBYwFAYIKwYBBQUHAwIGCCsGAQUFBwMBMB8GA1UdIwQYMBaAFFAlC81rH211
fPJoWglERKb/7/NfMCYGA1UdEQQfMB2HBH8AAAGgFQYJKwYBBAGSCBMBoAgMBmZv
b2JhcjANBgkqhkiG9w0BAQsFAAOCAgEAbzKRqx2i8S0Kuu0bIX094EoGiGSTWW4l
YNHwn9mC/5KgzjSvxTkD0pInt31d5O27rK7/wMVezeqBIG92uwwZr7ndS6Fe0FT1
7tMZ1VH5VetIiicbu3AYssqMs/JYEocqOngLh/pGHmlwcnmPpCltipcE50bv9YWn
O8Yc5O7v16SxHzGsDUDO5eQAe2qvBaE5F5SBCVkjSoajmh3fdx/4eSzoF2wrug3/
O+WAb70UXX6r8dmRpr4RezQ6XPWAG57BgU3g0NUkczFo5gFndBUJngLhR6wr14xB
st21haZ65XIA46PB8jY04l/H2INwCzo++PlKJ3ROKwLXYDSZlgQ3X9XxsSzCX3Hs
viK9Ybzp2W8sl1Pvtb/jodcNTpD2IB8IrWnvuOgnwVmewqAqlxM7Ers9kC83lBpt
EhAXh0QyJ5BpHOkpm4jpVhOx1swHTBDoibysvpdr5KuuOm1JTr7cYRYhIe65rVz3
aL0PryzHdvQB97LhYAaUPtFnxNxUIeXKZO3Ndg/KSrSe4IqGz51uKjxJy+MnH9//
nnG0JqlerSVvSPSiZ2kdn4OwzV2eA3Gj3uyTSGsjjoj82bhhRwKaSWmUh+AJByQ9
kE6r/6za1Hvm+i/mz8f1cTUxFjF5pKzrprNRz5NMzs6NkQ0pg+mq5CNzav1ATSyv
Bdt96MbGrC0=
-----END CERTIFICATE-----`
	goodBlock, _ := pem.Decode([]byte(testGoodCertPEMData))
	testGoodCert, _ := x509.ParseCertificate(goodBlock.Bytes)

	extSubjectAltName := pkix.Extension{}
	extSubjectAltName.Id = asn1.ObjectIdentifier{2, 5, 29, 17}
	extSubjectAltName.Critical = false

	testUglyCert, _ := x509.ParseCertificate(goodBlock.Bytes)

	pi, _ := new(big.Int).SetString("3.14159", 10)
	extSubjectAltName.Value, _ = asn1.Marshal(pi)

	testUglyCert.Extensions = append(testUglyCert.Extensions, extSubjectAltName)
	var uglyWant1 []string

	tests := []struct {
		name    string
		args    args
		want    bool
		want1   []string
		wantErr bool
	}{
		{
			name: "Positive test",
			args: args{
				cert:             testGoodCert,
				expectedHostname: "foobar",
				logger:           logger.NewReceptorLogger("test"),
			},
			want:    true,               // found
			want1:   []string{"foobar"}, // ReceptorNames
			wantErr: false,
		},
		{
			name: "Non-matching hostname test",
			args: args{
				cert:             testGoodCert,
				expectedHostname: "foobaz-server",
				logger:           logger.NewReceptorLogger("test"),
			},
			want:    false,
			want1:   []string{"foobar"},
			wantErr: false,
		},
		{
			name: "Error test",
			args: args{
				cert:             testUglyCert,
				expectedHostname: "foobaz-server",
				logger:           logger.NewReceptorLogger("test"),
			},
			want:    false,
			want1:   uglyWant1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := utils.ParseReceptorNamesFromCert(tt.args.cert, tt.args.expectedHostname, tt.args.logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseReceptorNamesFromCert() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if got != tt.want {
				t.Errorf("ParseReceptorNamesFromCert() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("ParseReceptorNamesFromCert() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
