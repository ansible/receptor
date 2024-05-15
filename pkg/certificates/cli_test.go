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
					CommonName: "Ansible Automation Controller Nodes Mesh",
					Bits:       8192,
				},
				certOut: "receptor_cert.pem",
				keyOut:  "receptor_key.pem",
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
				certOut: "receptor_cert.pem",
				keyOut:  "receptor_key.pem",
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
