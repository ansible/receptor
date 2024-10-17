package backends

import (
	"context"
	"crypto/tls"
	"net"
	"reflect"
	"sync"
	"testing"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
)

func TestNewTCPListener(t *testing.T) {
	type args struct {
		address string
		tls     *tls.Config
		logger  *logger.ReceptorLogger
	}
	tests := []struct {
		name    string
		args    args
		want    *TCPListener
		wantErr bool
	}{
		{
			name: "Positive",
			args: args{
				address: "127.0.0.1:9999",
				tls:     nil,
				logger:  nil,
			},
			want: &TCPListener{
				address: "127.0.0.1:9999",
				TLS:     nil,
				li:      nil,
				innerLi: nil,
				logger:  nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewTCPListener(tt.args.address, tt.args.tls, tt.args.logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewTCPListener() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewTCPListener() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestTCPListenerStart(t *testing.T) {
	type fields struct {
		address string
		TLS     *tls.Config
		li      net.Listener
		innerLi *net.TCPListener
		logger  *logger.ReceptorLogger
	}
	type args struct {
		ctx context.Context
		wg  *sync.WaitGroup
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    chan netceptor.BackendSession
		wantErr bool
	}{
		{
			name: "Positive",
			fields: fields{
				address: "127.0.0.1:9998",
				TLS:     nil,
				li:      nil,
				innerLi: nil,
				logger:  logger.NewReceptorLogger("TCPtest"),
			},
			args: args{
				ctx: context.Background(),
				wg:  &sync.WaitGroup{},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &TCPListener{
				address: tt.fields.address,
				TLS:     tt.fields.TLS,
				li:      tt.fields.li,
				innerLi: tt.fields.innerLi,
				logger:  tt.fields.logger,
			}
			got, err := b.Start(tt.args.ctx, tt.args.wg)
			if (err != nil) != tt.wantErr {
				t.Errorf("TCPListener.Start() error = %+v, wantErr %+v", err, tt.wantErr)

				return
			}
			if got == nil {
				t.Errorf("TCPListener.Start() returned nil")
			}
		})
	}
}

func TestTCPDialerStart(t *testing.T) {
	type fields struct {
		address string
		redial  bool
		tls     *tls.Config
		logger  *logger.ReceptorLogger
	}
	type args struct {
		ctx context.Context
		wg  *sync.WaitGroup
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    chan netceptor.BackendSession
		wantErr bool
	}{
		{
			name: "Positive",
			fields: fields{
				address: "127.0.0.1:9998",
				redial:  true,
				tls:     nil,
				logger:  logger.NewReceptorLogger("TCPtest"),
			},
			args: args{
				ctx: context.Background(),
				wg:  &sync.WaitGroup{},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &TCPDialer{
				address: tt.fields.address,
				redial:  tt.fields.redial,
				tls:     tt.fields.tls,
				logger:  tt.fields.logger,
			}
			got, err := b.Start(tt.args.ctx, tt.args.wg)
			if (err != nil) != tt.wantErr {
				t.Errorf("TCPDialer.Start() error = %+v, wantErr %+v", err, tt.wantErr)

				return
			}
			if got == nil {
				t.Errorf("TCPDialer.Start() got = nil")
			}
		})
	}
}
