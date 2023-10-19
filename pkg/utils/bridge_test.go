package utils_test

import (
	"errors"
	"testing"

	"github.com/ansible/receptor/pkg/controlsvc/mock_controlsvc"
	logger "github.com/ansible/receptor/pkg/logger"
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

	type supplied struct {
		closeError error

		readError  error
		readValues []byte

		writeError  error
		writeValues []byte
	}
	ctrl := gomock.NewController(t)
	t.Cleanup(func() { ctrl.Finish() })

	tests := []struct {
		name     string
		args     args
		supplied supplied
	}{
		{
			name: "Good Test",
			args: args{
				c1:     mock_controlsvc.NewMockReadWriteCloser(ctrl),
				c1Name: "channel1",

				c2:     mock_controlsvc.NewMockReadWriteCloser(ctrl),
				c2Name: "channel2",

				logger: logger.NewReceptorLogger("test"),
			},
			supplied: supplied{
				closeError: nil,

				readError:  errors.New("EOF"),
				readValues: make([]byte, 5),

				writeError:  nil,
				writeValues: make([]byte, 5),
			},
		},
		{
			name: "Bad Test",
			args: args{
				c1:     mock_controlsvc.NewMockReadWriteCloser(ctrl),
				c1Name: "channel1",

				c2:     mock_controlsvc.NewMockReadWriteCloser(ctrl),
				c2Name: "channel2",

				logger: logger.NewReceptorLogger("test"),
			},
			supplied: supplied{
				closeError: nil,

				readError:  errors.New("EOF"),
				readValues: make([]byte, utils.NormalBufferSize),

				writeError:  errors.New("EOF"),
				writeValues: make([]byte, utils.NormalBufferSize),
			},
		},
	}

	t.Parallel()

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.args.c1.EXPECT().Read(gomock.Any()).Return(len(tt.supplied.readValues), tt.supplied.readError).AnyTimes()
			tt.args.c2.EXPECT().Read(gomock.Any()).Return(len(tt.supplied.readValues), tt.supplied.readError).AnyTimes()

			tt.args.c1.EXPECT().Close().Return(tt.supplied.closeError).AnyTimes()
			tt.args.c2.EXPECT().Close().Return(tt.supplied.closeError).AnyTimes()

			tt.args.c1.EXPECT().Write(gomock.Any()).Return(len(tt.supplied.writeValues), tt.supplied.writeError).AnyTimes()
			tt.args.c2.EXPECT().Write(gomock.Any()).Return(len(tt.supplied.writeValues), tt.supplied.writeError).AnyTimes()

			utils.BridgeConns(tt.args.c1, tt.args.c1Name, tt.args.c2, tt.args.c2Name, tt.args.logger)
		})
	}
}
