package backends

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
)

func TestdialerSession(t *testing.T) {
	type args struct {
		ctx         context.Context
		wg          *sync.WaitGroup
		redial      bool
		redialDelay time.Duration
		logger      *logger.ReceptorLogger
		df          dialerFunc
	}
	tests := []struct {
		name    string
		args    args
		want    chan netceptor.BackendSession
		wantErr bool
	}{
		{
			name:		"Positive",
			args:		args{
				ctx:			nil,
				wg:				nil,
				redial:			true,
				redialDelay:	1 * time.Second,
				logger:			nil,
				df:				nil,
			},
			want:		nil,
			wantErr:	false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := dialerSession(tt.args.ctx, tt.args.wg, tt.args.redial, tt.args.redialDelay, tt.args.logger, tt.args.df)
			if (err != nil) != tt.wantErr {
				t.Errorf("dialerSession() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("dialerSession() = %v, want %v", got, tt.want)
			}
		})
	}
}
