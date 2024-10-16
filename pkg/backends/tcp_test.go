package backends

import (
	"crypto/tls"
	"reflect"
	"testing"

	"github.com/ansible/receptor/pkg/logger"
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
