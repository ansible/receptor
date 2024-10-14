package backends_test

import (
	"context"
	"crypto/tls"
	"reflect"
	"sync"
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

func TestNullBackendCfg_Start(t *testing.T) {
	type fields struct {
		Local bool
	}
	type args struct {
		in0 context.Context
		in1 *sync.WaitGroup
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		wantKind reflect.Kind
		wantType string
		wantErr  bool
	}{
		{
			name: "Positive",
			fields: fields{
				Local: true,
			},
			args: args{
				in0: nil,
				in1: nil,
			},
			wantKind: reflect.Chan,
			wantType: "netceptor.BackendSession",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := backends.NullBackendCfg{
				Local: tt.fields.Local,
			}
			got, err := cfg.Start(tt.args.in0, tt.args.in1)
			defer close(got)

			if (err != nil) != tt.wantErr {
				t.Errorf("%s: NullBackendCfg.Start() error = %+v, wantErr %+v", tt.name, err, tt.wantErr)
				return
			}

			if reflect.ValueOf(got).Kind() == tt.wantKind {
				if reflect.ValueOf(got).Type().Elem().String() != tt.wantType {
					t.Errorf("%s: NullBackendCfg.Start() gotType = %+v, wantType = %+v", tt.name, reflect.ValueOf(got).Type().Elem(), tt.wantType)
				}
			} else {
				t.Errorf("%s: NullBackendCfg.Start() gotKind = %+v, wantKind = %+v", tt.name, reflect.ValueOf(got).Kind(), tt.wantKind)
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
