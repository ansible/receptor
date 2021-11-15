package netceptor

import (
	"io/ioutil"
	"os"
	"testing"
)

var badCertFile = []byte(`
-----BEGIN CERTIFICATE-----
MIIEvzCCA6egAwIBAgIUJN/WE3Hk2tAQ6NdpylQpzl+p2PQwDQYJKoZIhvcNAQEL
BQAwSjELMAkGA1UEBhMCVVMxGTAXBgNVBAoTEEZsZWV0c21pdGgsIEluYy4xIDAe
BgNVBAMTF0ZsZWV0c21pdGggSXNzdWluZyBDQSAxMB4XDTE5MDkwMzE4NTEwMFoX
DTI0MDkwMTE4NTEwMFowcjELMAkGA1UEBhMCVVMxGTAXBgNVBAoTEEZsZWV0c21p
dGgsIEluYy4xGTAXBgNVBAsTEHVrZ294dnU0dGMzZW1zNXAxLTArBgNVBAMTJDMw
MDdlNzUwLWU3NjktNDQwYi05MDc1LTQxZGMyYjViMTc4NzCCAiIwDQYJKoZIhvcN
AQEBBQADggIPADCCAgoCggIBAOW4/sK8lPA4c/VgE7pMBkxcBD8bvLPZaqlrHxYz
8Q0UxHt4/osUNQTmOMeQkJo5hQyI0Y7Luvtp4OR6hU3IC2qkrwo6qUyu2xBhZVXX
e9UAuDjsR6KQVGvVH/v9ferJuisvEl9G+/Mh8P0hQ1Q10x7iuaQLuY8lcpr++sDK
a6tC183NqWH+NZR4UeOJq57ne0PmTxc3hOAgWZX8HZURB+IK3JewaHBLrjoNV+2h
9u7LbXwXVRPU6YT5M/WyFq88YZ9oQdaNLzMbRiI6yxL36bCMumJG6I8DWYDj0Tca
+sokW70eoAJbnGRPfH8eJzVTv/19lDtpZy1QjD4xFuKCVJyzHfsDWIfMVeqeLrpA
Djb3YWRlMg51YZDRWZwdnDsg4QbDxqA4CH/JMK9/7CEFMH1rg6tJ1dXzpZJm/x9M
4ZrTP/PQSIlLwLXPdMsX80l5g7PDPaPabdWcMi/iIlbllhdhjCuAkGwSAG/mmhWh
x4r6r1VbnF017XASNDjfCMGpQNOWB01CreVsxBLNPdi04dlk5gijb4qBDmoTOu5J
AAhORyHuRHN5BrKYmg5BD3SjTa+/WxdB0a+watHYynkSW/5uc1hulYTDr1uK3CSH
Osa8r2TwYOMYR2tvzjcG/rXK8sTNfamjQbBAnPrC9nK02Hw85BMFBqd3UtdeNkEA
Cpa1AgMBAAGjdTBzMA4GA1UdDwEB/wQEAwIFoDATBgNVHSUEDDAKBggrBgEFBQcD
AjAMBgNVHRMBAf8EAjAAMB0GA1UdDgQWBBRGNrny9ZloTqCW4P4uOQ+6NJV3jTAf
BgNVHSMEGDAWgBShyPmlTZv0tPOe0Sm6gDaz5KxPATANBgkqhkiG9w0BAQsFAAOC
AQEADmA6wIuyCjtJ5o93tycidD63lRdxK62SSso5ilbtbRfyYfm7z8zxh3fRdkey
AMvkHe6P3ytJ9ZnGmp1g7wqTRO7qt4lyyqryeGkgXReaJBdWVow+bncmhutEjP91
KHQPo9y7MAZ38sgKO1AgH1KCPcMkDX/Wk6t+m8lw4+nVCbajdo5XLTboyyrSaIzr
JPbYIpEP2yqcJwzCjYVUUXoAfs2Ccgtj89nrlODq5cyUdEq4oe0ugrHDdI+5V+F+
hif61y0kJPiWj1IC6qS+rb4lmTM85ZTYkPuH++EXOje9SaGIM69L5g461GuB6K4A
TyF2XS4a4JLQAz1FM6sYLkJvKw==
-----END CERTIFICATE-----
`)

var goodCertFile = []byte(`
-----BEGIN CERTIFICATE-----
MIIERzCCAi8CFCwli9Kw22bwDQIaygLNEbVxBmCNMA0GCSqGSIb3DQEBCwUAMGAx
CzAJBgNVBAYTAkNBMQ8wDQYDVQQIDAZRdWViZWMxETAPBgNVBAcMCE1vbnRyZWFs
MRwwGgYDVQQKDBNEZWZhdWx0IENvbXBhbnkgTHRkMQ8wDQYDVQQDDAZmZWRvcmEw
HhcNMjExMTEyMjAzMjE4WhcNMjMwMzI3MjAzMjE4WjBgMQswCQYDVQQGEwJDQTEP
MA0GA1UECAwGUXVlYmVjMREwDwYDVQQHDAhNb250cmVhbDEcMBoGA1UECgwTRGVm
YXVsdCBDb21wYW55IEx0ZDEPMA0GA1UEAwwGZmVkb3JhMIIBIjANBgkqhkiG9w0B
AQEFAAOCAQ8AMIIBCgKCAQEAwzHIg8cLA/DyKlDmQV6hFV9Yk4LDSwXdbso5zCJ6
TLa07hNVOcgBfaqDSm7I7yxRHmGrRipdhq/ntJeziXiBOPidKII3XWTPI9yhcFRL
iFp+RxMnWcC9TPzJ8tvs6a91SH+BhSV3ROeGVxjiPgAiahS5aPbBKpYrY/ek3/4c
ZPFZS5urr+vVXnOnnk5R1oPWUKL/hcO5eS42bR6z4oD/f7KVir6GEjVMF6yM2wiu
D6ocrUQsSxxAcSqmzjHfC+Gzf/qt2oThx68iUNYu/yNU5j2OtHHNsxttxzra2MGJ
2b7bcsASjn+5ZWrkPb2MiCNEJwka05QlDelhZVcPXo/SkQIDAQABMA0GCSqGSIb3
DQEBCwUAA4ICAQBoYbQNNLh/W+QxkjrxfzmZ6iVxrX92HFUMjWIUPhVEe7yH5A72
9JQBl1yV2auKRF5jVQt9MvHY4ayzVbLyAJrT/uXBhi2U6KVyrgPJY+phaOpKBCwd
4qiv4UMEc82NELUImNG7a2xXI2G8tdlKUBoXR9k4doS8X7iZbCq4PI9Q52D04h3Y
oJQA/IL8icWJXHjkW5+VDgMhwsfdhQBPIUU0qhuZF8hK74jjD1BuRALC8ykSAdzW
Ae77MEirMcNXkDsmaQJPx7V+VEmVBuBUDPZC5pOyTfzFDkJPfYo2FFESGqwS3Ix/
PDhruIMQY6m/haUj2rCYL6J/TM3kejyDqOQ+UvZpw+0Vf1RpxkUadgsd9vMreGSk
JMVFGk6ro5nHyu3Sig/rY4Qmhh517H1c8JRGf4vj73S7XH9qEFTLcp6y5/JcAAnS
4cI6iEg6RzUaIGqiMJ6weE6bIz5PdNMLI0KJdt/5ng8ArGvJ6EpQxAqPRTx0+Evf
xF4Nj/hEN1dbatRxtmBr4IGnpgOgf3zyqNusr7SBr36XvuVzFm1xINWcxFzaD5TU
tYp6qVTQ72lrWM+VYj0aFpo4K77wHz0/jX3lk9pEt9P6IpPfl56A9y2634ntDGww
1Lr9qLhjg8rvyQZpAtypXllz78b0XySGoE0p8nlZwCN1sy6SMeGxTj3yLA==
-----END CERTIFICATE-----
`)

var goodCertKey = []byte(`
-----BEGIN RSA PRIVATE KEY-----
MIIEpQIBAAKCAQEAwzHIg8cLA/DyKlDmQV6hFV9Yk4LDSwXdbso5zCJ6TLa07hNV
OcgBfaqDSm7I7yxRHmGrRipdhq/ntJeziXiBOPidKII3XWTPI9yhcFRLiFp+RxMn
WcC9TPzJ8tvs6a91SH+BhSV3ROeGVxjiPgAiahS5aPbBKpYrY/ek3/4cZPFZS5ur
r+vVXnOnnk5R1oPWUKL/hcO5eS42bR6z4oD/f7KVir6GEjVMF6yM2wiuD6ocrUQs
SxxAcSqmzjHfC+Gzf/qt2oThx68iUNYu/yNU5j2OtHHNsxttxzra2MGJ2b7bcsAS
jn+5ZWrkPb2MiCNEJwka05QlDelhZVcPXo/SkQIDAQABAoIBAGrci2ERjEdJFtrx
1Uz+aIPR4iVH1nDxMgwgbEoEqh3rfNxF+0eZ5q8Mtbn/MsQ15+cRI3pTYUaGcPye
fK0LKvusqCVwPK1Frl18wWlEzOFGYZG5u7ZYXYqTbmAl5Or+ot/g5mClZUl000hF
mD7HRg/7bpI3XQNamUCuaDY04QilXgfkYYz5JjfoBLw0+XoxLHnxtcGgH7oGWr/T
S1vM3mziY9aH9lz2XSMz4kgVCkTCmgpyY/KrVZ1nF4X/zHD5H6t4Nn9y2RDB9JrJ
1l9v9BpoRIczGWzqA7fLW2Gog6cocEmL/01aqBrM8nnDM3u494urpG5QTcC82KgH
x3lVE8ECgYEA/0tzss3S6GQN7ECJdkZyG4bk6EUVQGCvpFWyKZIO2fzYw/E0YQpQ
55+bqytoLU5A3P5pDbvsydfi9sOPaA5wYKBKLmzhSf842CUnisKkiDqRhWu9jYkQ
J7+Nm5NGKD3cVqI0Pj/BB0QlN2r5eOjD0VKJg7jOQEklATc/xHVnTikCgYEAw7vT
2npgbNMGNbeduydr3MzaJBm/BiB6XqVtIZWtEtrI+YrmW/40Y/GwyjKTNTBtTYvR
ziV6G8ynoGwTiA8MTO0PuubVUE53Ul0Rcnll7DZXA9RXLRBBMjIfdRO4+fA4km2x
0VDDsbol9vr7SxsHJl2pfnIuB6M0243Gz/NvnikCgYEAoh+1auCAxqn7mYPmh+D2
x/pjVgnBFDASescdLH8fxVloAw8jl1ioxl86yXn4X4Upde5uopUsK4ZZESJh6M/6
l7JTSaZFb+uPmtwFf35aZFTlYxhnuQFI7CUedaUSUa3vRmkdykZMfCPPiqD5qsoO
yNikka0D9hk9UcdXTJjOMPkCgYEAnnIh4S5HeFCrKjjlWmdjDW5W9/pPhGouJQfM
++7qF+U746XpLHbveihgcI3YVKpLEQcqY7I60t4so9RZSz7DOlJ89VN/Qm8CcI4e
XYigVUL31YXCvBv4auXgSSoaB1nKsd5Sn5q9j9Wcff5WBkpu00PmvRE46b3YQBLY
6eWqaWECgYEApGxBlEYRAjcEt5bg2siE9QcvXtmMlS5QRNTPGD7DEE6BxphphdfI
dpD9Q0E8A2xq9Wvd3OZfRi11SNYrunI4K8FLgcB7e4V+Ue5oOkY0joO45lD4qz0S
nWvGPVGHjq3s8fyjh1wNR1JLXevUgQmel1BMUjh0AKOw0SYaH4ET1J4=
-----END RSA PRIVATE KEY-----
`)

var rootCAFile = []byte(`
-----BEGIN CERTIFICATE-----
MIIFoTCCA4mgAwIBAgIUG7/RqzbOK1lj8kpSlHtqFQtLJEkwDQYJKoZIhvcNAQEL
BQAwYDELMAkGA1UEBhMCQ0ExDzANBgNVBAgMBlF1ZWJlYzERMA8GA1UEBwwITW9u
dHJlYWwxHDAaBgNVBAoME0RlZmF1bHQgQ29tcGFueSBMdGQxDzANBgNVBAMMBmZl
ZG9yYTAeFw0yMTExMTIyMDMxNDVaFw0yNDA5MDEyMDMxNDVaMGAxCzAJBgNVBAYT
AkNBMQ8wDQYDVQQIDAZRdWViZWMxETAPBgNVBAcMCE1vbnRyZWFsMRwwGgYDVQQK
DBNEZWZhdWx0IENvbXBhbnkgTHRkMQ8wDQYDVQQDDAZmZWRvcmEwggIiMA0GCSqG
SIb3DQEBAQUAA4ICDwAwggIKAoICAQCrKZXQGKliYxBfStd0eT4sh2/V/fJZJEVp
9wzkt53CPEO7soGCuunyQV9yBsFVmm2YInBGUGgPZ/q221612eRq2/72xoBNMSV0
hCnJbOlwiHL2W7cMTO1KFTCdH09Eprz6cC+scVM+65zFOTxL858K8jcc13q9dnUh
W/c9b0LdM/RLuyaV2OHJLCQEUZXzTEIiF2bFrTqOk7/1RmxtS/y6sjs5WqEu0k4M
OygrKKdyLv47ykfc04rEPGLDzQaA2Kw7SBirDTkE+zGQrABjveGAXY5z75/bLm4R
rTJgOJwZvbgLxz3YqNjqdM+w8hi+rGyS/aNd/rKe1D9HXLtXnzpo+C1DgVlYW4BX
TZuGJ+Hr4577q8b1K86Xz3TUNqBoGyWo4Z01zS0IRquHVXH8sXqvXBOOOLncuFEi
+DnKlbCc70TSekotGRz5rFEhXXiJZ17aMVo9jgrlRDe9G/Hlnw0VgdsItTFtpdBI
tMva4ss3zsE+oEAolSTulBW1Yk+OtrsUeQ3jPzPrD79Uc9yl4V4SEdDnsUjgJ9S4
GrH7/8PjR9HsyBd4qIU0I8BYzGHiFUgQ0a5kSfFtSsmtSQDgBd5BTwafsiqsbRe2
bzjPjIxtcxkQ1l+Cm5uk9wJ3YmurwAb6O1AwJmp6Vqd7AJ1dAmvwUQ7gn9RFIr0F
EpqKM8u9lQIDAQABo1MwUTAdBgNVHQ4EFgQUOlypBBKd121ckbmPgSJnnG3sCTgw
HwYDVR0jBBgwFoAUOlypBBKd121ckbmPgSJnnG3sCTgwDwYDVR0TAQH/BAUwAwEB
/zANBgkqhkiG9w0BAQsFAAOCAgEAPHUVHtP9w7qYwoLuANG8FkVoJ8NkZLS4olcK
AjGDo9aFV5SavEOud3d8JLaKY4x2NXySJ8HCJW4ry822RgHo3k52PEJrQXTqG5ZQ
/qLT+qoAmakBKNpzqmJosWm5zbXH8hqhGGUxWOza73bc31JdKEArZUExB0Q8q2Sx
EX19M23NBqY2uw3TbXPaw/lIt74rOuNzgGsABYiQ+kJOD8cvdt+dgHIB10oLUXbO
ffPvR5+qBeUjgCbJlipvHRz96Jg5Gywg2LcF2/k4LAQeduRPMSm8dhSfAuGGZ/Pb
NQYdddSf+iOtj7NAHYgSFKHH63FwWFq+agub65YnUBNLqOnnXc0OWk+HSxUEnqEl
PQbmaxJaScNP/njtmhPgrX+Q8v5+hxYkdf3AH33jlivC5JxopU4Ngi+vhXUBz+CS
EUcG7boRHKkgbtQcMZMqAKXFIMZ62yu8Ym0nB4zCNtW/5XqzJ6fDaXdhI0pxV7dL
pKs91vbQHhN0iM0JDOo90vwmJISeRIcqLjXDOH3uvZ4afYaeiy2eFiHZJKOpqZLl
1tZfTQsJwhbRHAqOeVFr0aZShrpq3n9lCp3TYKNVjI9KgWVEzNPH2cYxgKQSRCOc
hx4qYWNRe+DnbOr9xj+Fsd1kDQ3k+FyxIWz5lcXx8Tk7nFjfxhs+4yfHx3xuea58
BNsxjLo=
-----END CERTIFICATE-----
`)

func setupTest(tb testing.TB) (func(tb testing.TB), os.File, os.File, os.File, os.File) {
	tb.Log("test setup")

	// create bad cert and werite bad cert contents to it
	badCert, err := ioutil.TempFile("", "*")
	if err != nil {
		tb.Log(err.Error())
	}

	defer os.Remove(badCert.Name())

	_, err = badCert.Write(badCertFile)
	if err != nil {
		tb.Log(err.Error())
	}

	// create good cert and write good cert contents to it
	goodCert, err := ioutil.TempFile("", "*")
	if err != nil {
		tb.Log(err.Error())
	}

	defer os.Remove(goodCert.Name())

	_, err = goodCert.Write(goodCertFile)
	if err != nil {
		tb.Log(err.Error())
	}

	// create good cert key and write good cert key contents to it
	goodKey, err := ioutil.TempFile("", "*")
	if err != nil {
		tb.Logf(err.Error())
	}

	defer os.Remove(goodKey.Name())

	_, err = goodKey.Write(goodCertKey)
	if err != nil {
		tb.Log(err.Error())
	}

	// create root ca and write good root CA contents to it
	goodCA, err := ioutil.TempFile("", "*")
	if err != nil {
		tb.Log(err.Error())
	}

	defer os.Remove(goodCA.Name())

	_, err = goodCA.Write(rootCAFile)
	if err != nil {
		tb.Log(err.Error())
	}

	return func(tb testing.TB) {
		tb.Log("teardown")
	}, *badCert, *goodCert, *goodKey, *goodCA
}

func TestBadClientCert(t *testing.T) {
	tearDownSuite, badCertFile, _, goodCertKey, rootCAFile := setupTest(t)
	defer tearDownSuite(t)

	badClientConfig := &tlsClientConfig{
		Cert:    badCertFile.Name(),
		Key:     goodCertKey.Name(),
		RootCAs: rootCAFile.Name(),
	}
	err := badClientConfig.Prepare()

	if err == nil {
		t.Errorf("this should have failed")
	}
}

func TestBadServerCert(t *testing.T) {
	tearDownSuite, badCertFile, _, goodCertKey, rootCAFile := setupTest(t)
	defer tearDownSuite(t)

	goodClientConfig := &tlsClientConfig{
		Cert:    badCertFile.Name(),
		Key:     goodCertKey.Name(),
		RootCAs: rootCAFile.Name(),
	}
	err := goodClientConfig.Prepare()

	if err == nil {
		t.Errorf("this should have failed")
	}
}

// TODO Create positive path forward with unit tests (assert that good certs pass Prepare())
// Needs proper Netceptor instantiation
