//go:build !no_cert_auth
// +build !no_cert_auth

package certificates_test

import (
	"fmt"
	"io/fs"
	"os"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/certificates"
	"github.com/ansible/receptor/pkg/certificates/mock_certificates"
	"go.uber.org/mock/gomock"
)

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

func TestInitCAConfigRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	o := mock_certificates.NewMockOser(ctrl)

	tests := []struct {
		name        string
		CAConfig    certificates.InitCAConfig
		expectError bool
	}{
		{
			name: "successful run with minimal configuration",
			CAConfig: certificates.InitCAConfig{
				CommonName: "Test CA",
				Bits:       2048,
				OutCert:    "test.crt",
				OutKey:     "test.key",
				Osw:        o,
			},
			expectError: false,
		},
		{
			name: "successful run with full configuration",
			CAConfig: certificates.InitCAConfig{
				CommonName: "Test CA",
				Bits:       2048,
				NotBefore:  "2023-01-01T00:00:00Z",
				NotAfter:   "2024-01-01T00:00:00Z",
				OutCert:    "test.crt",
				OutKey:     "test.key",
				Osw:        o,
			},
			expectError: false,
		},
		{
			name: "invalid NotBefore date",
			CAConfig: certificates.InitCAConfig{
				CommonName: "Test CA",
				Bits:       2048,
				NotBefore:  "invalid date",
				NotAfter:   "2024-01-01T00:00:00Z",
				OutCert:    "test.crt",
				OutKey:     "test.key",
				Osw:        o,
			},
			expectError: true,
		},
		{
			name: "invalid NotAfter date",
			CAConfig: certificates.InitCAConfig{
				CommonName: "Test CA",
				Bits:       2048,
				NotBefore:  "2023-01-01T00:00:00Z",
				NotAfter:   "invalid date",
				OutCert:    "test.crt",
				OutKey:     "test.key",
				Osw:        o,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o.EXPECT().
				WriteFile(gomock.Eq(tt.CAConfig.OutCert), gomock.Any(), gomock.Any()).
				Return(nil).
				MinTimes(0).
				MaxTimes(1)

			// Mock WriteFile for private key if OutKey is set
			if tt.CAConfig.OutKey != "" {
				o.EXPECT().
					WriteFile(gomock.Eq(tt.CAConfig.OutKey), gomock.Any(), gomock.Any()).
					Return(nil).
					MinTimes(0).
					MaxTimes(1)
			}
			err := tt.CAConfig.Run()
			if (err != nil) != tt.expectError {
				t.Errorf("InitCAConfig.Run() error = %v, expectError %v", err, tt.expectError)
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

	positiveKeyIn := "/tmp/receptor_key.pem"
	positiveKeyOut := "/tmp/receptor_key_out.pem"
	positiveReqOut := "/tmp/receptor_request_out.pem"

	negativeKeyIn := "/tmp"
	duplicateKeyIn := "/tmp/receptor_key_multiple.pem"
	emptyKeyIn := "/tmp/receptor_key_empty.pem"

	tests := []struct {
		name          string
		args          args
		wantErr       bool
		wantErrString string
	}{
		{
			name: "Positive test",
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
		},
		{
			name: "Negative test",
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
		},
		{
			name: "multiple private keys in keyIn",
			args: args{
				opts: &certificates.CertOptions{
					Bits:       8192,
					CommonName: "Ansible Automation Controller Nodes Mesh",
				},
				keyIn:  duplicateKeyIn,
				keyOut: positiveKeyOut,
				reqOut: positiveReqOut,
			},
			wantErr:       true,
			wantErrString: "multiple keys in file /tmp/receptor_key_multiple.pem",
		},
		{
			name: "empty keyIn",
			args: args{
				opts: &certificates.CertOptions{
					Bits:       8192,
					CommonName: "Ansible Automation Controller Nodes Mesh",
				},
				keyIn:  emptyKeyIn,
				keyOut: positiveKeyOut,
				reqOut: positiveReqOut,
			},
			wantErr:       true,
			wantErrString: "no keys in file /tmp/receptor_key_empty.pem",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			o := mock_certificates.NewMockOser(ctrl)

			switch tt.args.keyIn {
			case negativeKeyIn:
				o.
					EXPECT().
					ReadFile(gomock.Eq(negativeKeyIn)).
					Return(nil, fs.ErrInvalid).
					Times(1)

			case positiveKeyIn:
				o.
					EXPECT().
					ReadFile(gomock.Eq(positiveKeyIn)).
					Return(setupGoodPrivateKeyPEMData(), nil).
					Times(1)

			case duplicateKeyIn:
				o.
					EXPECT().
					ReadFile(gomock.Eq(duplicateKeyIn)).
					Return(setupDuplicateKeyPEMData(), nil).
					Times(1)

			case emptyKeyIn:
				o.
					EXPECT().
					ReadFile(gomock.Eq(emptyKeyIn)).
					Return([]byte{}, nil).
					Times(1)

			default:
				t.Errorf("Unexpected keyIn filename: %s", tt.args.keyIn)
			}

			switch tt.args.keyOut {
			case positiveKeyOut:
				o.
					EXPECT().
					WriteFile(gomock.Eq(positiveKeyOut), gomock.Any(), gomock.Any()).
					Return(nil).
					MinTimes(0).
					MaxTimes(1)

			default:
				t.Errorf("Unexpected keyOut filename: %s", tt.args.keyOut)
			}

			switch tt.args.reqOut {
			case positiveReqOut:
				o.
					EXPECT().
					WriteFile(gomock.Eq(positiveReqOut), gomock.Any(), gomock.Any()).
					Return(nil).
					MinTimes(0).
					MaxTimes(1)

			default:
				t.Errorf("Unexpected reqOut filename: %s", tt.args.reqOut)
			}

			err := certificates.MakeReq(tt.args.opts, tt.args.keyIn, tt.args.keyOut, tt.args.reqOut, o)

			if (err != nil) != tt.wantErr {
				t.Errorf("MakeReq() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErrString != "" {
				if err.Error() != tt.wantErrString {
					t.Errorf("MakeReq() error = %v, wantErrString %v", err, tt.wantErrString)
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

	// Define reusable path constants
	const (
		positiveCaCrtPath = "/tmp/receptor_ca_cert.pem"
		positiveCaKeyPath = "/tmp/receptor_ca_key.pem"
		positiveCertOut   = "/tmp/receptor_cert_out.pem"
		positiveReqPath   = "/tmp/receptor_request.pem"
		negativeReqPath   = "/tmp/receptor_request_bad.pem"
		invalidPath       = "invalid_path"
	)

	positiveCertOptions, _, err := setupGoodCertRequest()
	if err != nil {
		t.Errorf("Invalid good Certificate Request: %+v", err)
	}

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
			name: "Error in CA Path",
			args: args{
				opts:      &positiveCertOptions,
				caCrtPath: invalidPath,
				caKeyPath: positiveCaKeyPath,
				reqPath:   positiveReqPath,
				certOut:   positiveCertOut,
				verify:    true,
			},
			wantErr: true,
		},
		{
			name: "Error in Key Path",
			args: args{
				opts:      &positiveCertOptions,
				caCrtPath: positiveCaCrtPath,
				caKeyPath: invalidPath,
				reqPath:   positiveReqPath,
				certOut:   positiveCertOut,
				verify:    true,
			},
			wantErr: true,
		},
		{
			name: "Error in Req Path",
			args: args{
				opts:      &positiveCertOptions,
				caCrtPath: positiveCaCrtPath,
				caKeyPath: positiveCaKeyPath,
				reqPath:   invalidPath,
				certOut:   positiveCertOut,
				verify:    true,
			},
			wantErr: true,
		},
		{
			name: "No Verify",
			args: args{
				opts:      &positiveCertOptions,
				caCrtPath: positiveCaCrtPath,
				caKeyPath: positiveCaKeyPath,
				reqPath:   positiveReqPath,
				certOut:   positiveCertOut,
				verify:    false,
			},
			wantErr: true,
		},
		{
			name: "Malformed Req file",
			args: args{
				opts:      &positiveCertOptions,
				caCrtPath: positiveCaCrtPath,
				caKeyPath: positiveCaKeyPath,
				reqPath:   negativeReqPath,
				certOut:   positiveCertOut,
				verify:    false,
			},
			wantErr: true,
		},
		{
			name: "No names in Req file",
			args: args{
				opts:      &positiveCertOptions,
				caCrtPath: positiveCaCrtPath,
				caKeyPath: positiveCaKeyPath,
				reqPath:   negativeReqPath,
				certOut:   positiveCertOut,
				verify:    false,
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
					AnyTimes()
				o.EXPECT(). // I can't see this as best practice,
					// but it is what the original code expected because it didn't used to mock WriteFile()
					WriteFile(gomock.Eq(positiveCertOut), gomock.Any(), gomock.Any()).
					Return(nil).
					MinTimes(0).
					MaxTimes(1)

			default:
				o.
					EXPECT().
					ReadFile(gomock.Eq(invalidPath)).
					Return(nil, fmt.Errorf("Unexpected filename: %s", tt.args.caCrtPath)).
					Times(1)
			}

			switch tt.args.caKeyPath {
			case positiveCaKeyPath:
				o.
					EXPECT().
					ReadFile(gomock.Eq(positiveCaKeyPath)).
					Return(setupGoodCaRsaPrivateKeyPEMData(), nil).
					AnyTimes()

			default:
				o.
					EXPECT().
					ReadFile(gomock.Eq(invalidPath)).
					Return(nil, fmt.Errorf("Unexpected filename: %s", tt.args.caKeyPath)).
					Times(1)
			}

			switch tt.args.reqPath {
			case positiveReqPath:
				o.
					EXPECT().
					ReadFile(gomock.Eq(positiveReqPath)).
					Return(setupGoodCertificateRequestPEMData(), nil).
					AnyTimes()
			case negativeReqPath:
				switch tt.name {
				case "Malformed Req file":
					o.
						EXPECT().
						ReadFile(gomock.Eq(negativeReqPath)).
						Return([]byte{}, nil).
						AnyTimes()
				case "No names in Req file":
					o.
						EXPECT().
						ReadFile(gomock.Eq(negativeReqPath)).
						Return(setupBadCertificateRequestPEMData(), nil).
						AnyTimes()
				}
			default:
				o.
					EXPECT().
					ReadFile(gomock.Eq(invalidPath)).
					Return(nil, fmt.Errorf("Unexpected filename: %s", tt.args.reqPath)).
					Times(1)
			}

			signReqImpl := certificates.SignerReqImpl{}
			if err := signReqImpl.SignReq(tt.args.opts, tt.args.caCrtPath, tt.args.caKeyPath, tt.args.reqPath, tt.args.certOut, tt.args.verify, o); (err != nil) != tt.wantErr {
				t.Errorf("SignReq() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSignReqConfigValidateAndSign(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSigner := mock_certificates.NewMockSignReqFunc(ctrl)

	tests := []struct {
		name        string
		config      certificates.SignReqConfig
		expectError bool
	}{
		{
			name: "successful run with minimal configuration",
			config: certificates.SignReqConfig{
				Req:     "req.pem",
				CACert:  "ca.pem",
				CAKey:   "ca.key",
				OutCert: "out.pem",
				Verify:  true,
			},
			expectError: false,
		},
		{
			name: "successful run with full configuration",
			config: certificates.SignReqConfig{
				Req:       "req.pem",
				CACert:    "ca.pem",
				CAKey:     "ca.key",
				NotBefore: "2023-01-01T00:00:00Z",
				NotAfter:  "2024-01-01T00:00:00Z",
				OutCert:   "out.pem",
				Verify:    true,
			},
			expectError: false,
		},
		{
			name: "Invalid NotBefore date",
			config: certificates.SignReqConfig{
				Req:       "req.pem",
				CACert:    "ca.pem",
				CAKey:     "ca.key",
				NotBefore: "invalid-date",
				NotAfter:  "2024-01-01T00:00:00Z",
				OutCert:   "out.pem",
				Verify:    true,
			},
			expectError: true,
		},
		{
			name: "Invalid NotAfter date",
			config: certificates.SignReqConfig{
				Req:       "req.pem",
				CACert:    "ca.pem",
				CAKey:     "ca.key",
				NotBefore: "2024-01-01T00:00:00Z",
				NotAfter:  "invalid-date",
				OutCert:   "out.pem",
				Verify:    true,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Only mock when we expect Run() to reach the SignReq call
			if !tt.expectError {
				mockSigner.EXPECT().SignReq(gomock.Any(), tt.config.CACert, tt.config.CAKey, tt.config.Req, tt.config.OutCert, tt.config.Verify, gomock.Any()).Return(nil)
			}

			err := tt.config.ValidateAndSign(mockSigner)
			if (err != nil) != tt.expectError {
				t.Errorf("SignReq.Run() error = %v, expectError %v", err, tt.expectError)
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

func TestMakeReqConfigRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOsw := mock_certificates.NewMockOser(ctrl)

	tests := []struct {
		name        string
		config      certificates.MakeReqConfig
		setupMocks  func()
		wantErr     bool
		expectedErr string
	}{
		{
			name: "successful run with valid IP",
			config: certificates.MakeReqConfig{
				CommonName: "test.example.com",
				Bits:       2048,
				DNSName:    []string{"dns.example.com"},
				NodeID:     []string{"node123"},
				IPAddress:  []string{"192.168.1.1"},
				OutReq:     "request.pem",
				OutKey:     "key.pem",
				Osw:        mockOsw,
			},
			setupMocks: func() {
				// Mock successful file operations
				mockOsw.EXPECT().WriteFile("request.pem", gomock.Any(), gomock.Any()).Return(nil)
				mockOsw.EXPECT().WriteFile("key.pem", gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "invalid IP address",
			config: certificates.MakeReqConfig{
				CommonName: "test.example.com",
				Bits:       2048,
				IPAddress:  []string{"invalid-ip"},
				OutReq:     "request.pem",
				OutKey:     "key.pem",
				Osw:        mockOsw,
			},
			setupMocks:  func() {},
			wantErr:     true,
			expectedErr: "invalid IP address: invalid-ip",
		},
		{
			name: "file write error",
			config: certificates.MakeReqConfig{
				CommonName: "test.example.com",
				Bits:       2048,
				OutReq:     "request.pem",
				OutKey:     "key.pem",
				Osw:        mockOsw,
			},
			setupMocks: func() {
				// Simulate file write failure
				mockOsw.EXPECT().WriteFile("request.pem", gomock.Any(), gomock.Any()).Return(os.ErrPermission)
			},
			wantErr:     true,
			expectedErr: "permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks()
			err := tt.config.Run()

			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && err != nil && err.Error() != tt.expectedErr {
				t.Errorf("Expected error '%s', got '%s'", tt.expectedErr, err.Error())
			}
		})
	}
}
