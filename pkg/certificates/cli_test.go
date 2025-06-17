//go:build !no_cert_auth
// +build !no_cert_auth

package certificates_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io/fs"
	"net"
	"os"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/certificates"
	"github.com/ansible/receptor/pkg/certificates/mock_certificates"
	"go.uber.org/mock/gomock"
)

// setupEmptyNamesCertificateRequestPEMData returns a PEM-encoded certificate request with no names.
func setupEmptyNamesCertificateRequestPEMData() []byte {
	csr := &x509.CertificateRequest{
		Subject:     pkix.Name{CommonName: "empty-names"},
		DNSNames:    nil,
		IPAddresses: nil,
	}
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	csrBytes, _ := x509.CreateCertificateRequest(rand.Reader, csr, priv)
	pemBlock := &pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrBytes,
	}
	var buf bytes.Buffer
	pem.Encode(&buf, pemBlock)

	return buf.Bytes()
}

func TestInitCA(t *testing.T) {
	type args struct {
		opts    *certificates.CertOptions
		certOut string
		keyOut  string
	}

	positiveCertOut := "/tmp/receptor_ca_cert.pem"
	positiveKeyOut := "/tmp/receptor_ca_key.pem"
	positiveCaTimeNotAfterString := "2032-01-07T00:03:51Z"
	positiveCaTimeNotAfter, err := time.Parse(time.RFC3339, positiveCaTimeNotAfterString)
	if err != nil {
		t.Errorf("Invalid CA NOT after time: %+v", err)
	}

	positiveCaTimeNotBeforeString := "2022-01-07T00:03:51Z"
	positiveCaTimeNotBefore, err := time.Parse(time.RFC3339, positiveCaTimeNotBeforeString)
	if err != nil {
		t.Errorf("Invalid CA NOT before time: %+v", err)
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Positive test",
			args: args{
				opts: &certificates.CertOptions{
					Bits:       8192,
					CommonName: "Ansible Automation Controller Nodes Mesh CA",
					NotAfter:   positiveCaTimeNotAfter,
					NotBefore:  positiveCaTimeNotBefore,
				},
				certOut: positiveCertOut,
				keyOut:  positiveKeyOut,
			},
			wantErr: false,
		},
		{
			name: "Negative test",
			args: args{
				opts: &certificates.CertOptions{
					Bits:       -1,
					CommonName: "Ansible Automation Controller Nodes Mesh CA",
					NotAfter:   positiveCaTimeNotAfter,
					NotBefore:  positiveCaTimeNotBefore,
				},
				certOut: positiveCertOut,
				keyOut:  positiveKeyOut,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			o := mock_certificates.NewMockOser(ctrl)

			switch tt.args.certOut {
			case positiveCertOut:
				o.
					EXPECT().
					WriteFile(gomock.Eq(positiveCertOut), gomock.Any(), gomock.Any()).
					Return(nil).
					MaxTimes(1).
					MinTimes(0)

			default:
				t.Errorf("Unexpected certOut filename: %s", tt.args.certOut)
			}

			switch tt.args.keyOut {
			case positiveKeyOut:
				o.
					EXPECT().
					WriteFile(gomock.Eq(positiveKeyOut), gomock.Any(), gomock.Any()).
					Return(nil).
					MaxTimes(1).
					MinTimes(0)

			default:
				t.Errorf("Unexpected keyOut filename: %s", tt.args.keyOut)
			}

			if err := certificates.InitCA(tt.args.opts, tt.args.certOut, tt.args.keyOut, o); (err != nil) != tt.wantErr {
				t.Errorf("InitCA() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// setupMultiplePrivateKeysPEMData returns a PEM-encoded byte slice containing two RSA private keys.
func setupMultiplePrivateKeysPEMData() []byte {
	var buf bytes.Buffer

	// Generate first RSA private key
	key1, _ := rsa.GenerateKey(rand.Reader, 2048)
	pemBlock1 := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key1),
	}
	pem.Encode(&buf, pemBlock1)

	// Generate second RSA private key
	key2, _ := rsa.GenerateKey(rand.Reader, 2048)
	pemBlock2 := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key2),
	}
	pem.Encode(&buf, pemBlock2)

	return buf.Bytes()
}

func TestSignReq_Original(t *testing.T) {
	type args struct {
		opts      *certificates.CertOptions
		caCrtPath string
		caKeyPath string
		reqPath   string
		certOut   string
		verify    bool
	}

	positiveCaCrtPath := "/tmp/receptor_ca_cert.pem"

	positiveCaKeyPath := "/tmp/receptor_ca_key.pem"

	positiveCertOut := "/tmp/receptor_cert_out.pem"

	positiveReqPath := "/tmp/receptor_request.pem"

	positiveCertOptions, _, err := setupGoodCertRequest()
	if err != nil {
		t.Errorf("Invalid good Certificate Request: %+v", err)
	}

	negativeCaTimeNotAfterString := "2021-01-07T00:03:51Z"
	negativeCaTimeNotAfter, err := time.Parse(time.RFC3339, negativeCaTimeNotAfterString)
	if err != nil {
		t.Errorf("Invalid CA after time: %+v", err)
	}

	negativeCaTimeNotBeforeString := "2022-01-07T00:03:51Z"
	negativeCaTimeNotBefore, err := time.Parse(time.RFC3339, negativeCaTimeNotBeforeString)
	if err != nil {
		t.Errorf("Invalid CA before time: %+v", err)
	}

	negativeReqPath := "/tmp/receptor_request_bad.pem"
	negativeDNSName := "receptor.TEST.BAD"
	negativeIPAddress := net.ParseIP("127.0.0.1").To4()
	negativeNodeIDs := negativeDNSName

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Positive test",
			args: args{
				opts:      &positiveCertOptions,
				caCrtPath: positiveCaCrtPath,
				caKeyPath: positiveCaKeyPath,
				reqPath:   positiveReqPath,
				certOut:   positiveCertOut,
				verify:    true,
			},
			wantErr: false,
		},
		{
			name: "Negative test",
			args: args{
				opts: &certificates.CertOptions{
					Bits: -1,
					CertNames: certificates.CertNames{
						DNSNames: []string{
							negativeDNSName,
						},
						IPAddresses: []net.IP{
							negativeIPAddress,
						},
						NodeIDs: []string{
							negativeNodeIDs,
						},
					},
					CommonName: "Ansible Automation Controller Nodes Mesh",
					NotAfter:   negativeCaTimeNotAfter,
					NotBefore:  negativeCaTimeNotBefore,
				},
				caCrtPath: positiveCaCrtPath,
				caKeyPath: positiveCaKeyPath,
				reqPath:   negativeReqPath,
				certOut:   positiveCertOut,
				verify:    true,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			o := mock_certificates.NewMockOser(ctrl)

			switch tt.args.caCrtPath {
			case positiveCaCrtPath:
				o.
					EXPECT().
					ReadFile(gomock.Eq(positiveCaCrtPath)).
					Return(setupGoodCaCertificatePEMData(), nil).
					Times(1)
				o.EXPECT(). // I can't see this as best practice,
					// but it is what the original code expected because it didn't used to mock WriteFile()
					WriteFile(gomock.Eq(positiveCertOut), gomock.Any(), gomock.Any()).
					Return(nil).
					MinTimes(0).
					MaxTimes(1)

			default:
				t.Errorf("Unexpected filename: %s", tt.args.caCrtPath)
			}

			switch tt.args.caKeyPath {
			case positiveCaKeyPath:
				o.
					EXPECT().
					ReadFile(gomock.Eq(positiveCaKeyPath)).
					Return(setupGoodCaRsaPrivateKeyPEMData(), nil).
					Times(1)

			default:
				t.Errorf("Unexpected filename: %s", tt.args.reqPath)
			}

			switch tt.args.reqPath {
			case negativeReqPath:
				o.
					EXPECT().
					ReadFile(gomock.Eq(negativeReqPath)).
					Return(setupGoodCertificatePEMData(), nil).
					Times(1)

			case positiveReqPath:
				o.
					EXPECT().
					ReadFile(gomock.Eq(positiveReqPath)).
					Return(setupGoodCertificateRequestPEMData(), nil).
					Times(1)

			default:
				t.Errorf("Unexpected filename: %s", tt.args.reqPath)
			}

			if err := certificates.SignReq(tt.args.opts, tt.args.caCrtPath, tt.args.caKeyPath, tt.args.reqPath, tt.args.certOut, tt.args.verify, o); (err != nil) != tt.wantErr {
				t.Errorf("SignReq() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMakeReq(t *testing.T) {
	type args struct {
		opts   *certificates.CertOptions
		keyIn  string
		keyOut string
		reqOut string
	}

	// Helper variables for filenames and data
	positiveKeyIn := "/tmp/receptor_key.pem"
	positiveKeyOut := "/tmp/receptor_key_out.pem"
	positiveReqOut := "/tmp/receptor_request_out.pem"
	negativeKeyIn := "/tmp/bad_key.pem"
	badKeyFile := "/tmp/bad_keyfile.pem"
	missingKeyFile := "/tmp/missing_keyfile.pem"
	invalidReqOut := "/dev/null/req.pem"
	invalidKeyOut := "/dev/null/key.pem"

	tests := []struct {
		name       string
		args       args
		wantErr    bool
		setupMocks func(mockOs *mock_certificates.MockOser)
	}{
		// Original positive test
		{
			name: "Positive test with keyIn",
			args: args{
				opts: &certificates.CertOptions{
					Bits:       8192,
					CommonName: "Ansible Automation Controller Nodes Mesh",
				},
				keyIn:  positiveKeyIn,
				keyOut: positiveKeyOut,
				reqOut: positiveReqOut,
			},
			wantErr: false,
			setupMocks: func(mockOs *mock_certificates.MockOser) {
				mockOs.EXPECT().ReadFile(gomock.Eq(positiveKeyIn)).Return(setupGoodPrivateKeyPEMData(), nil).Times(1)
				mockOs.EXPECT().WriteFile(gomock.Eq(positiveKeyOut), gomock.Any(), gomock.Any()).Return(nil).MinTimes(0).MaxTimes(1)
				mockOs.EXPECT().WriteFile(gomock.Eq(positiveReqOut), gomock.Any(), gomock.Any()).Return(nil).MinTimes(0).MaxTimes(1)
			},
		},
		// Original negative test: invalid key bits
		{
			name: "Negative test with bad key bits",
			args: args{
				opts: &certificates.CertOptions{
					Bits:       -1,
					CommonName: "Ansible Automation Controller Nodes Mesh",
				},
				keyIn:  negativeKeyIn,
				keyOut: positiveKeyOut,
				reqOut: positiveReqOut,
			},
			wantErr: true,
			setupMocks: func(mockOs *mock_certificates.MockOser) {
				mockOs.EXPECT().ReadFile(gomock.Eq(negativeKeyIn)).Return(nil, fs.ErrInvalid).Times(1)
				mockOs.EXPECT().WriteFile(gomock.Eq(positiveKeyOut), gomock.Any(), gomock.Any()).MinTimes(0).MaxTimes(1)
				mockOs.EXPECT().WriteFile(gomock.Eq(positiveReqOut), gomock.Any(), gomock.Any()).MinTimes(0).MaxTimes(1)
			},
		},
		// Test: Valid input, generates new key (no keyIn)
		{
			name: "Valid input, generates new key",
			args: args{
				opts: &certificates.CertOptions{
					Bits:       2048,
					CommonName: "example.com",
				},
				keyIn:  "",
				keyOut: "/tmp/test_key.pem",
				reqOut: "/tmp/test_req.pem",
			},
			wantErr: false,
			setupMocks: func(mockOs *mock_certificates.MockOser) {
				mockOs.EXPECT().WriteFile(gomock.Eq("/tmp/test_key.pem"), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				mockOs.EXPECT().WriteFile(gomock.Eq("/tmp/test_req.pem"), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
		},
		// Test: Invalid key bits with new key
		{
			name: "Invalid key bits, new key",
			args: args{
				opts: &certificates.CertOptions{
					Bits:       -1,
					CommonName: "example.com",
				},
				keyIn:  "",
				keyOut: "/tmp/test_key.pem",
				reqOut: "/tmp/test_req.pem",
			},
			wantErr: true,
			setupMocks: func(mockOs *mock_certificates.MockOser) {
				mockOs.EXPECT().WriteFile(gomock.Eq("/tmp/test_key.pem"), gomock.Any(), gomock.Any()).AnyTimes()
				mockOs.EXPECT().WriteFile(gomock.Eq("/tmp/test_req.pem"), gomock.Any(), gomock.Any()).AnyTimes()
			},
		},
		// Edge: keyIn file missing
		{
			name: "keyIn file missing",
			args: args{
				opts: &certificates.CertOptions{
					Bits:       2048,
					CommonName: "example.com",
				},
				keyIn:  missingKeyFile,
				keyOut: positiveKeyOut,
				reqOut: positiveReqOut,
			},
			wantErr: true,
			setupMocks: func(mockOs *mock_certificates.MockOser) {
				mockOs.EXPECT().ReadFile(gomock.Eq(missingKeyFile)).Return(nil, fs.ErrNotExist).Times(1)
			},
		},
		// Edge: keyIn file contains no private keys
		{
			name: "keyIn file contains no private keys",
			args: args{
				opts: &certificates.CertOptions{
					Bits:       2048,
					CommonName: "example.com",
				},
				keyIn:  badKeyFile,
				keyOut: positiveKeyOut,
				reqOut: positiveReqOut,
			},
			wantErr: true,
			setupMocks: func(mockOs *mock_certificates.MockOser) {
				mockOs.EXPECT().ReadFile(gomock.Eq(badKeyFile)).Return([]byte("not a private key"), nil).Times(1)
			},
		},
		// Edge: keyIn file contains multiple private keys
		{
			name: "keyIn file contains multiple private keys",
			args: args{
				opts: &certificates.CertOptions{
					Bits:       2048,
					CommonName: "example.com",
				},
				keyIn:  "/tmp/multi_key.pem",
				keyOut: positiveKeyOut,
				reqOut: positiveReqOut,
			},
			wantErr: true,
			setupMocks: func(mockOs *mock_certificates.MockOser) {
				mockOs.EXPECT().ReadFile(gomock.Eq("/tmp/multi_key.pem")).Return(setupMultiplePrivateKeysPEMData(), nil).Times(1)
			},
		},
		// Edge: WriteFile fails for reqOut
		{
			name: "WriteFile fails for reqOut",
			args: args{
				opts: &certificates.CertOptions{
					Bits:       2048,
					CommonName: "example.com",
				},
				keyIn:  positiveKeyIn,
				keyOut: positiveKeyOut,
				reqOut: invalidReqOut,
			},
			wantErr: true,
			setupMocks: func(mockOs *mock_certificates.MockOser) {
				mockOs.EXPECT().ReadFile(gomock.Eq(positiveKeyIn)).Return(setupGoodPrivateKeyPEMData(), nil).Times(1)
				mockOs.EXPECT().WriteFile(gomock.Eq(positiveKeyOut), gomock.Any(), gomock.Any()).Return(nil).MinTimes(0).MaxTimes(1)
				mockOs.EXPECT().WriteFile(gomock.Eq(invalidReqOut), gomock.Any(), gomock.Any()).Return(fs.ErrPermission).Times(1)
			},
		},
		// Edge: WriteFile fails for keyOut
		{
			name: "WriteFile fails for keyOut",
			args: args{
				opts: &certificates.CertOptions{
					Bits:       2048,
					CommonName: "example.com",
				},
				keyIn:  "",
				keyOut: invalidKeyOut,
				reqOut: positiveReqOut,
			},
			wantErr: true,
			setupMocks: func(mockOs *mock_certificates.MockOser) {
				mockOs.EXPECT().WriteFile(gomock.Eq(invalidKeyOut), gomock.Any(), gomock.Any()).Return(fs.ErrPermission).Times(1)
				mockOs.EXPECT().WriteFile(gomock.Eq(positiveReqOut), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
		},
		// Edge: Bits missing with keyOut only (should error)
		{
			name: "Bits missing with keyOut only",
			args: args{
				opts: &certificates.CertOptions{
					CommonName: "example.com",
				},
				keyIn:  "",
				keyOut: positiveKeyOut,
				reqOut: positiveReqOut,
			},
			wantErr:    true,
			setupMocks: func(mockOs *mock_certificates.MockOser) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockOs := mock_certificates.NewMockOser(ctrl)
			tt.setupMocks(mockOs)
			err := certificates.MakeReq(tt.args.opts, tt.args.keyIn, tt.args.keyOut, tt.args.reqOut, mockOs)
			if (err != nil) != tt.wantErr {
				t.Errorf("MakeReq() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPrepare(t *testing.T) {
	tests := []struct {
		name      string
		cfg       certificates.MakeReqConfig
		wantErr   bool
		errString string
	}{
		{
			name:      "Neither InKey nor OutKey set",
			cfg:       certificates.MakeReqConfig{InKey: "", OutKey: "", Bits: 0},
			wantErr:   true,
			errString: "must provide either InKey or OutKey",
		},
		{
			name:      "Both InKey and OutKey set",
			cfg:       certificates.MakeReqConfig{InKey: "akey", OutKey: "bkey", Bits: 0},
			wantErr:   true,
			errString: "cannot use both InKey and OutKey",
		},
		{
			name:      "InKey set with Bits",
			cfg:       certificates.MakeReqConfig{InKey: "akey", OutKey: "", Bits: 2048},
			wantErr:   true,
			errString: "cannot specify key bits when reading an already-existing key",
		},
		{
			name:      "OutKey set without Bits",
			cfg:       certificates.MakeReqConfig{InKey: "", OutKey: "bkey", Bits: 0},
			wantErr:   true,
			errString: "must specify key bits when creating a new key",
		},
		{
			name:      "Valid: only InKey set",
			cfg:       certificates.MakeReqConfig{InKey: "akey", OutKey: "", Bits: 0},
			wantErr:   false,
			errString: "",
		},
		{
			name:      "Valid: only OutKey set with Bits",
			cfg:       certificates.MakeReqConfig{InKey: "", OutKey: "bkey", Bits: 2048},
			wantErr:   false,
			errString: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Prepare()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Prepare() error = nil, want error %q", tt.errString)
				} else if err.Error() != tt.errString {
					t.Errorf("Prepare() error = %q, want %q", err.Error(), tt.errString)
				}
			} else {
				if err != nil {
					t.Errorf("Prepare() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestSignReq(t *testing.T) {
	type args struct {
		opts      *certificates.CertOptions
		caCrtPath string
		caKeyPath string
		reqPath   string
		certOut   string
		verify    bool
	}

	// Test file paths
	caCert := "/tmp/ca_cert.pem"
	caKey := "/tmp/ca_key.pem"
	req := "/tmp/req.pem"
	certOut := "/tmp/cert_out.pem"
	badCert := "/tmp/bad_ca_cert.pem"
	badKey := "/tmp/bad_ca_key.pem"
	badReq := "/tmp/bad_req.pem"
	emptyNamesReq := "/tmp/empty_names_req.pem"

	// Helper: create a valid CertOptions
	validOpts := &certificates.CertOptions{
		CommonName: "test",
	}

	// Table of test cases
	tests := []struct {
		name       string
		args       args
		setupMocks func(o *mock_certificates.MockOser)
		setupStdin func()
		wantErr    bool
	}{
		{
			name: "Positive path (verify=true)",
			args: args{
				opts:      validOpts,
				caCrtPath: caCert,
				caKeyPath: caKey,
				reqPath:   req,
				certOut:   certOut,
				verify:    true,
			},
			setupMocks: func(o *mock_certificates.MockOser) {
				o.EXPECT().ReadFile(gomock.Eq(caCert)).Return(setupGoodCaCertificatePEMData(), nil)
				o.EXPECT().ReadFile(gomock.Eq(caKey)).Return(setupGoodCaRsaPrivateKeyPEMData(), nil)
				o.EXPECT().ReadFile(gomock.Eq(req)).Return(setupGoodCertificateRequestPEMData(), nil)
				o.EXPECT().WriteFile(gomock.Eq(certOut), gomock.Any(), gomock.Any()).Return(nil)
			},
			setupStdin: func() {},
			wantErr:    false,
		},
		{
			name: "User accepts (verify=false, input yes)",
			args: args{
				opts:      validOpts,
				caCrtPath: caCert,
				caKeyPath: caKey,
				reqPath:   req,
				certOut:   certOut,
				verify:    false,
			},
			setupMocks: func(o *mock_certificates.MockOser) {
				o.EXPECT().ReadFile(gomock.Eq(caCert)).Return(setupGoodCaCertificatePEMData(), nil)
				o.EXPECT().ReadFile(gomock.Eq(caKey)).Return(setupGoodCaRsaPrivateKeyPEMData(), nil)
				o.EXPECT().ReadFile(gomock.Eq(req)).Return(setupGoodCertificateRequestPEMData(), nil)
				o.EXPECT().WriteFile(gomock.Eq(certOut), gomock.Any(), gomock.Any()).Return(nil)
			},
			setupStdin: func() {
				// Simulate user typing "yes"
				oldStdin := os.Stdin
				r, w, _ := os.Pipe()
				w.WriteString("yes\n")
				w.Close()
				os.Stdin = r
				t.Cleanup(func() { os.Stdin = oldStdin })
			},
			wantErr: false,
		},
		{
			name: "User declines (verify=false, input no)",
			args: args{
				opts:      validOpts,
				caCrtPath: caCert,
				caKeyPath: caKey,
				reqPath:   req,
				certOut:   certOut,
				verify:    false,
			},
			setupMocks: func(o *mock_certificates.MockOser) {
				o.EXPECT().ReadFile(gomock.Eq(caCert)).Return(setupGoodCaCertificatePEMData(), nil)
				o.EXPECT().ReadFile(gomock.Eq(caKey)).Return(setupGoodCaRsaPrivateKeyPEMData(), nil)
				o.EXPECT().ReadFile(gomock.Eq(req)).Return(setupGoodCertificateRequestPEMData(), nil)
			},
			setupStdin: func() {
				// Simulate user typing "no"
				oldStdin := os.Stdin
				r, w, _ := os.Pipe()
				w.WriteString("no\n")
				w.Close()
				os.Stdin = r
				t.Cleanup(func() { os.Stdin = oldStdin })
			},
			wantErr: true,
		},
		{
			name: "CA certificate read fails",
			args: args{
				opts:      validOpts,
				caCrtPath: badCert,
				caKeyPath: caKey,
				reqPath:   req,
				certOut:   certOut,
				verify:    true,
			},
			setupMocks: func(o *mock_certificates.MockOser) {
				o.EXPECT().ReadFile(gomock.Eq(badCert)).Return(nil, fs.ErrNotExist)
			},
			setupStdin: func() {},
			wantErr:    true,
		},
		{
			name: "CA key read fails",
			args: args{
				opts:      validOpts,
				caCrtPath: caCert,
				caKeyPath: badKey,
				reqPath:   req,
				certOut:   certOut,
				verify:    true,
			},
			setupMocks: func(o *mock_certificates.MockOser) {
				o.EXPECT().ReadFile(gomock.Eq(caCert)).Return(setupGoodCaCertificatePEMData(), nil)
				o.EXPECT().ReadFile(gomock.Eq(badKey)).Return(nil, fs.ErrNotExist)
			},
			setupStdin: func() {},
			wantErr:    true,
		},
		{
			name: "Request read fails",
			args: args{
				opts:      validOpts,
				caCrtPath: caCert,
				caKeyPath: caKey,
				reqPath:   badReq,
				certOut:   certOut,
				verify:    true,
			},
			setupMocks: func(o *mock_certificates.MockOser) {
				o.EXPECT().ReadFile(gomock.Eq(caCert)).Return(setupGoodCaCertificatePEMData(), nil)
				o.EXPECT().ReadFile(gomock.Eq(caKey)).Return(setupGoodCaRsaPrivateKeyPEMData(), nil)
				o.EXPECT().ReadFile(gomock.Eq(badReq)).Return(nil, fs.ErrNotExist)
			},
			setupStdin: func() {},
			wantErr:    true,
		},
		{
			name: "Request has no names",
			args: args{
				opts:      validOpts,
				caCrtPath: caCert,
				caKeyPath: caKey,
				reqPath:   emptyNamesReq,
				certOut:   certOut,
				verify:    true,
			},
			setupMocks: func(o *mock_certificates.MockOser) {
				o.EXPECT().ReadFile(gomock.Eq(caCert)).Return(setupGoodCaCertificatePEMData(), nil)
				o.EXPECT().ReadFile(gomock.Eq(caKey)).Return(setupGoodCaRsaPrivateKeyPEMData(), nil)
				o.EXPECT().ReadFile(gomock.Eq(emptyNamesReq)).Return(setupEmptyNamesCertificateRequestPEMData(), nil)
			},
			setupStdin: func() {},
			wantErr:    true,
		},
		{
			name: "WriteFile fails",
			args: args{
				opts:      validOpts,
				caCrtPath: caCert,
				caKeyPath: caKey,
				reqPath:   req,
				certOut:   certOut,
				verify:    true,
			},
			setupMocks: func(o *mock_certificates.MockOser) {
				o.EXPECT().ReadFile(gomock.Eq(caCert)).Return(setupGoodCaCertificatePEMData(), nil)
				o.EXPECT().ReadFile(gomock.Eq(caKey)).Return(setupGoodCaRsaPrivateKeyPEMData(), nil)
				o.EXPECT().ReadFile(gomock.Eq(req)).Return(setupGoodCertificateRequestPEMData(), nil)
				o.EXPECT().WriteFile(gomock.Eq(certOut), gomock.Any(), gomock.Any()).Return(fs.ErrPermission)
			},
			setupStdin: func() {},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			o := mock_certificates.NewMockOser(ctrl)
			tt.setupMocks(o)
			tt.setupStdin()
			err := certificates.SignReq(tt.args.opts, tt.args.caCrtPath, tt.args.caKeyPath, tt.args.reqPath, tt.args.certOut, tt.args.verify, o)
			if (err != nil) != tt.wantErr {
				t.Errorf("SignReq() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
