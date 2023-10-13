package utils_test

import (
	"testing"

	"github.com/ansible/receptor/pkg/controlsvc/mock_controlsvc"
	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/utils"
	gomock "go.uber.org/mock/gomock"
)

func TestBridgeConns(t *testing.T) {
	type args struct {
		c1     *mock_controlsvc.MockReadWriteCloser
		c1Name string
		c2     *mock_controlsvc.MockReadWriteCloser
		c2Name string
		logger *logger.ReceptorLogger
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name string
		args args
	}{
		{
			name: "Good test",
			args: args{
				c1:     mock_controlsvc.NewMockReadWriteCloser(ctrl),
				c1Name: "channel1",
				c2:     mock_controlsvc.NewMockReadWriteCloser(ctrl),
				c2Name: "channel2",
				logger: logger.NewReceptorLogger("test"),
			},
		},
		{
			name: "Connection write error",
			args: args{
				c1:     mock_controlsvc.NewMockReadWriteCloser(ctrl),
				c1Name: "channel1",
				c2:     mock_controlsvc.NewMockReadWriteCloser(ctrl),
				c2Name: "channel2",
				logger: logger.NewReceptorLogger("test"),
			},
		},
		{
			name: "EOF test",
			args: args{
				c1:     mock_controlsvc.NewMockReadWriteCloser(ctrl),
				c1Name: "channel1",
				c2:     mock_controlsvc.NewMockReadWriteCloser(ctrl),
				c2Name: "channel2",
				logger: logger.NewReceptorLogger("test"),
			},
		},
		{
			name: "Not all bytes written",
			args: args{
				c1:     mock_controlsvc.NewMockReadWriteCloser(ctrl),
				c1Name: "channel1",
				c2:     mock_controlsvc.NewMockReadWriteCloser(ctrl),
				c2Name: "channel2",
				logger: logger.NewReceptorLogger("test"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.args.c1.EXPECT().Read( []byte("input") ).Return( 1, nil ).Times(1)
			tt.args.c2.EXPECT().Write( []byte("input") ).Return( 1, nil ).Times(1)
			utils.BridgeConns(tt.args.c1, tt.args.c1Name, tt.args.c2, tt.args.c2Name, tt.args.logger)
		})
	}
}
