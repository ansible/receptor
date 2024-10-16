package backends

import (
	"net"
	"sync"
	"testing"

	"github.com/ansible/receptor/pkg/logger"
)

func TestNewUDPListener(t *testing.T) {
	type args struct {
		address string
		logger  *logger.ReceptorLogger
	}

	goodLogger := logger.NewReceptorLogger("UDPtest")
	goodUDPListener := &UDPListener{
		laddr:           &net.UDPAddr{},
		conn:            &net.UDPConn{},
		sessChan:        make(chan *UDPListenerSession),
		sessRegLock:     sync.RWMutex{},
		sessionRegistry: make(map[string]*UDPListenerSession),
		logger:          goodLogger,
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
				address: "127.0.0.1:9998",
				logger:  goodLogger,
			},
			want:    goodUDPListener,
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
