package netceptor

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/ansible/receptor/tests/functional/lib/utils"
)

// setup handle using hardcoded PEMs.
func setupSuite(t *testing.T) (*os.File, *os.File, *os.File, func(t *testing.T)) {
	tempCertFile, err := ioutil.TempFile("", "")
	if err != nil {
		t.Error(err.Error())
	}

	tempCertFile.Write([]byte(`-----BEGIN CERTIFICATE-----
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
-----END CERTIFICATE-----
`))

	tempCertKey, err := ioutil.TempFile("", "")
	if err != nil {
		t.Error(err.Error())
	}

	tempCertKey.Write([]byte(`-----BEGIN RSA PRIVATE KEY-----
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
-----END RSA PRIVATE KEY-----
`))

	tempCA, err := ioutil.TempFile("", "")
	if err != nil {
		t.Error(err.Error())
	}

	tempCA.Write([]byte(`-----BEGIN CERTIFICATE-----
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

	return tempCertFile, tempCertKey, tempCA, func(t *testing.T) {
		defer os.Remove(tempCertFile.Name())
		defer os.Remove(tempCertKey.Name())
		defer os.Remove(tempCA.Name())
	}
}

// another setup handle using Receptor-like cert-generation.
func useUtilsSetupSuite(t *testing.T, name string) (string, string, string, func(t *testing.T)) {
	_, ca, err := utils.GenerateCA(name, name)
	if err != nil {
		t.Error(err.Error())
	}
	certKey, cert, err := utils.GenerateCert(name, name, []string{name}, []string{name})
	if err != nil {
		t.Error(err.Error())
	}

	return ca, certKey, cert, func(t *testing.T) {
		defer os.Remove(ca)
		defer os.Remove(certKey)
		defer os.Remove(cert)
	}
}

func useUtilsSetupSuiteWithGenerateWithCA(t *testing.T, name string) (string, string, string, func(t *testing.T)) {
	caKey, caCert, err := utils.GenerateCA(name, name)
	if err != nil {
		t.Error(err.Error())
	}
	certKey, cert, err := utils.GenerateCertWithCA(name, caKey, caCert, name, nil, []string{"foobar"})
	if err != nil {
		t.Error(err.Error())
	}

	return caCert, cert, certKey, func(t *testing.T) {
		defer os.Remove(caCert)
		defer os.Remove(caKey)
		defer os.Remove(certKey)
		defer os.Remove(cert)
	}
}

func TestPrepareGoodTlsServerCfg(t *testing.T) {
	tempCertFile, tempCertKey, tempCA, teardownSuite := setupSuite(t)
	defer teardownSuite(t)

	cfg := &tlsServerCfg{
		Name:              "foobar",
		Cert:              tempCertFile.Name(),
		Key:               tempCertKey.Name(),
		RequireClientCert: false,
		ClientCAs:         tempCA.Name(),
	}

	MainInstance = New(context.Background(), "foobar")

	if err := cfg.Prepare(); err != nil {
		t.Errorf("nodeId=%s not found in certificate", MainInstance.nodeID)
	}
}

func TestNodeIDMismatch(t *testing.T) {
	tempCertFile, tempCertKey, tempCA, teardownSuite := setupSuite(t)
	defer teardownSuite(t)

	cfg := &tlsServerCfg{
		Name:              "foobar",
		Cert:              tempCertFile.Name(),
		Key:               tempCertKey.Name(),
		RequireClientCert: false,
		ClientCAs:         tempCA.Name(),
	}

	MainInstance = New(context.Background(), "barfoo")

	if err := cfg.Prepare(); err == nil {
		t.Errorf("nodeId=%s; ReceptorName=foobar; this should have failed out", MainInstance.nodeID)
	}
}

func TestNodeIDWithUtilsGenerateCert(t *testing.T) {
	tempCa, tempCertKey, tempCert, tearDownSuite := useUtilsSetupSuite(t, "foobar")
	defer tearDownSuite(t)

	cfg := tlsServerCfg{
		Name:              "foobar",
		Cert:              tempCert,
		Key:               tempCertKey,
		RequireClientCert: false,
		ClientCAs:         tempCa,
	}

	MainInstance = New(context.Background(), "foobar")

	if err := cfg.Prepare(); err != nil {
		t.Errorf("nodeId=%s; ReceptorName=foobar; this shouldn't have failed", MainInstance.nodeID)
	}
}

func TestBadNodeIDWithUtilsGenerateCert(t *testing.T) {
	tempCa, tempCertKey, tempCert, tearDownSuite := useUtilsSetupSuite(t, "foobar")
	defer tearDownSuite(t)

	cfg := tlsServerCfg{
		Name:              "foobar",
		Cert:              tempCert,
		Key:               tempCertKey,
		RequireClientCert: false,
		ClientCAs:         tempCa,
	}

	MainInstance = New(context.Background(), "barfoo")

	if err := cfg.Prepare(); err == nil {
		t.Errorf("nodeId=%s; ReceptorName=foobar; this should have failed", MainInstance.nodeID)
	}
}

func TestNodeIDWithUtilsGenerateCertWithCA(t *testing.T) {
	caCert, tempCert, tempCertKey, tearDownSuite := useUtilsSetupSuiteWithGenerateWithCA(t, "foobar")
	defer tearDownSuite(t)

	cfg := &tlsClientConfig{
		Name:               "foobar",
		Cert:               tempCert,
		Key:                tempCertKey,
		RootCAs:            caCert,
	}

	MainInstance = New(context.Background(), "foobar")

	if err := cfg.Prepare(); err != nil {
		t.Errorf("nodeId=%s; ReceptorName=foobar; this should have not failed", MainInstance.nodeID)
	}
}

func TestNodeIDWIthSkipReceptorNamesCheckTrue(t *testing.T) {
	caCert, tempCert, tempCertKey, tearDownSuite := useUtilsSetupSuiteWithGenerateWithCA(t, "foobaz")
	defer tearDownSuite(t)

	clientCfg := &tlsClientConfig{
		Name:                   "foobaz-client",
		Cert:                   tempCert,
		Key:                    tempCertKey,
		RootCAs:                caCert,
		SkipReceptorNamesCheck: true,
	}

	serverCfg := &tlsServerCfg{
		Name:                   "foobaz-server",
		Cert:                   tempCert,
		Key:                    tempCertKey,
		RequireClientCert:      false,
		ClientCAs:              caCert,
		SkipReceptorNamesCheck: true,
	}

	MainInstance = New(context.Background(), "foobar")

	if err := clientCfg.Prepare(); err != nil {
		t.Errorf("nodeId=%s; ReceptorName=foobar; this should have not failed", MainInstance.nodeID)
	}
	if err := serverCfg.Prepare(); err != nil {
		t.Errorf("nodeId=%s; ReceptorName=foobar; this should have not failed", MainInstance.nodeID)
	}
}
