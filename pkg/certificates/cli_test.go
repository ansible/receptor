//go:build !no_cert_auth
// +build !no_cert_auth

package certificates_test

import (
	"testing"

	"github.com/ansible/receptor/pkg/certificates"
)

func TestInitCA(t *testing.T) {
	type args struct {
		opts    *certificates.CertOptions
		certOut string
		keyOut  string
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
					CommonName: "Ansible Automation Controller Nodes Mesh",
				},
				certOut: "/tmp/receptor_cert.pem",
				keyOut:  "/tmp/receptor_key.pem",
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
				certOut: "/tmp/receptor_cert.pem",
				keyOut:  "/tmp/receptor_key.pem",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := certificates.InitCA(tt.args.opts, tt.args.certOut, tt.args.keyOut); (err != nil) != tt.wantErr {
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
				keyIn:  "/tmp/receptor_key.pem",
				keyOut: "/tmp/receptor_key_out.pem",
				reqOut: "/tmp/receptor_request_out.pem",
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
				keyIn:  "/tmp/",
				keyOut: "/tmp/receptor_key_out.pem",
				reqOut: "/tmp/receptor_request_out.pem",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := certificates.MakeReq(tt.args.opts, tt.args.keyIn, tt.args.keyOut, tt.args.reqOut); (err != nil) != tt.wantErr {
				t.Errorf("MakeReq() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
