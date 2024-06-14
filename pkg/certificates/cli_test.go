//go:build !no_cert_auth
// +build !no_cert_auth

package certificates_test

import (
	"io/fs"
	"net"
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

			if err := certificates.MakeReq(tt.args.opts, tt.args.keyIn, tt.args.keyOut, tt.args.reqOut, o); (err != nil) != tt.wantErr {
				t.Errorf("MakeReq() error = %v, wantErr %v", err, tt.wantErr)
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
