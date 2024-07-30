package utils_test

import (
	"crypto/x509/pkix"
	"encoding/asn1"
	"net"
	"reflect"
	"testing"

	"github.com/ansible/receptor/pkg/utils"
	"github.com/google/go-cmp/cmp"
)

func TestReceptorNames(t *testing.T) {
	type args struct {
		extensions []pkix.Extension
	}

	rawValues := []asn1.RawValue{
		{
			Tag:   2,
			Class: 2,
			Bytes: []byte(`hybrid_node1`),
		},
	}

	goodValue, err := asn1.Marshal(rawValues)
	if err != nil {
		t.Errorf("asn1.Marshal(): %s", err)
	}

	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "Positive",
			args: args{
				extensions: []pkix.Extension{
					{
						Id:       utils.OIDSubjectAltName,
						Critical: false,
						Value:    goodValue,
					},
				},
			},
			want:    []string{},
			wantErr: false,
		},
		{
			name: "Negative",
			args: args{
				extensions: []pkix.Extension{
					{
						Id:       utils.OIDSubjectAltName,
						Critical: false,
						Value:    nil,
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := utils.ReceptorNames(tt.args.extensions)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReceptorNames() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("ReceptorNames() mismatch(-want +got)\n%s", diff)
			}
		})
	}
}

func TestMakeReceptorSAN(t *testing.T) {
	type args struct {
		dnsNames    []string
		ipAddresses []net.IP
		nodeIDs     []string
	}
	tests := []struct {
		name    string
		args    args
		want    *pkix.Extension
		wantErr bool
	}{
		{
			name: "Positive",
			args: args{
				dnsNames: []string{
					"hybrid_node1",
				},
				ipAddresses: []net.IP{
					net.IPv4(192, 168, 4, 1),
				},
				nodeIDs: []string{
					"hybrid_node1",
				},
			},
			want: &pkix.Extension{
				Id:       utils.OIDSubjectAltName,
				Critical: false,
				Value: []byte{
					48, 49, 130, 12, 104, 121, 98, 114, 105, 100,
					95, 110, 111, 100, 101, 49, 135, 4, 192, 168,
					4, 1, 160, 27, 6, 9, 43, 6, 1, 4,
					1, 146, 8, 19, 1, 160, 14, 12, 12, 104,
					121, 98, 114, 105, 100, 95, 110, 111, 100, 101,
					49,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := utils.MakeReceptorSAN(tt.args.dnsNames, tt.args.ipAddresses, tt.args.nodeIDs)
			if (err != nil) != tt.wantErr {
				t.Errorf("MakeReceptorSAN() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MakeReceptorSAN() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
