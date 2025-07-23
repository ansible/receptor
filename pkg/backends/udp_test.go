package backends

import (
	"context"
	"net"
	"sync"
	"testing"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
)

func TestNewUDPListener(t *testing.T) {
	type args struct {
		address string
		logger  *logger.ReceptorLogger
	}

	tests := []struct {
		name    string
		args    args
		want    *UDPListener
		wantErr bool
	}{
		{
			name: "Positive",
			args: args{
				address: "127.0.0.1:9997",
				logger:  logger.NewReceptorLogger("UDPtest"),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewUDPListener(tt.args.address, tt.args.logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewUDPListener() error = %+v, wantErr %+v", err, tt.wantErr)

				return
			}
			if got == nil {
				t.Errorf("NewUDPListener(): want UDP Listener, got nil")
			}
		})
	}
}

func TestUDPListenerStart(t *testing.T) {
	type fields struct {
		laddr           *net.UDPAddr
		conn            *net.UDPConn
		sessChan        chan *UDPListenerSession
		sessionRegistry map[string]*UDPListenerSession
		logger          *logger.ReceptorLogger
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
				laddr: &net.UDPAddr{
					IP:   net.IPv4(127, 0, 0, 1),
					Port: 9999,
					Zone: "",
				},
				conn:            &net.UDPConn{},
				sessChan:        make(chan *UDPListenerSession),
				sessionRegistry: make(map[string]*UDPListenerSession),
				logger:          logger.NewReceptorLogger("UDPtest"),
			},
			args: args{
				ctx: context.Background(),
				wg:  &sync.WaitGroup{},
			},
			want:    make(chan netceptor.BackendSession),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, err := net.ListenUDP("udp", tt.fields.laddr)
			if err != nil {
				t.Errorf("ListenUDP error = %+v", err)
			}

			b := &UDPListener{
				laddr:           tt.fields.laddr,
				conn:            uc,
				sessChan:        tt.fields.sessChan,
				sessRegLock:     sync.RWMutex{},
				sessionRegistry: tt.fields.sessionRegistry,
				logger:          tt.fields.logger,
			}
			got, err := b.Start(tt.args.ctx, tt.args.wg)
			if (err != nil) != tt.wantErr {
				t.Errorf("UDPListener.Start() error = %+v, wantErr %+v", err, tt.wantErr)

				return
			}
			if got == nil {
				t.Errorf("UDPListener.Start() returned nil")
			}
		})
	}
}

func TestUDPDialerStart(t *testing.T) {
	type fields struct {
		address string
		redial  bool
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
				logger:  logger.NewReceptorLogger("UDPtest"),
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
			b := &UDPDialer{
				address: tt.fields.address,
				redial:  tt.fields.redial,
				logger:  tt.fields.logger,
			}
			got, err := b.Start(tt.args.ctx, tt.args.wg)
			if (err != nil) != tt.wantErr {
				t.Errorf("UDPDialer.Start() error = %+v, wantErr %+v", err, tt.wantErr)

				return
			}
			if got == nil {
				t.Errorf("UDPDialer.Start() returned nil")
			}
		})
	}
}

func TestNewUDPDialer(t *testing.T) {
	type args struct {
		address string
		redial  bool
		logger  *logger.ReceptorLogger
	}
	tests := []struct {
		name    string
		args    args
		want    *UDPDialer
		wantErr bool
	}{
		{
			name: "Positive",
			args: args{
				address: "127.0.0.1:9995",
				redial:  true,
				logger:  logger.NewReceptorLogger("UDPtest"),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewUDPDialer(tt.args.address, tt.args.redial, tt.args.logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewUDPDialer() error = %+v, wantErr %+v", err, tt.wantErr)

				return
			}
			if got == nil {
				t.Errorf("NewUDPDialer() returned nil")
			}
		})
	}
}
