package backends_test

import (
	"crypto/tls"
	"reflect"
	"testing"

	"github.com/ansible/receptor/pkg/backends"
)

func TestNullBackendCfgGetAddr(t *testing.T) {
	type fields struct {
		Local bool
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "Positive",
			fields: fields{
				Local: true,
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &backends.NullBackendCfg{
				Local: tt.fields.Local,
			}
			if got := cfg.GetAddr(); got != tt.want {
				t.Errorf("NullBackendCfg.GetAddr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNullBackendCfgGetTLS(t *testing.T) {
	type fields struct {
		Local bool
	}
	tests := []struct {
		name   string
		fields fields
		want   *tls.Config
	}{
		{
			name: "Positive",
			fields: fields{
				Local: true,
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &backends.NullBackendCfg{
				Local: tt.fields.Local,
			}
			if got := cfg.GetTLS(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NullBackendCfg.GetTLS() = %v, want %v", got, tt.want)
			}
		})
	}
}
