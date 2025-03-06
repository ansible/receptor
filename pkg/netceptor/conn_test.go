package netceptor_test

import (
	"context"
	"testing"

	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/netceptor/mock_netceptor"
	"go.uber.org/mock/gomock"
)

func TestRead(t *testing.T) {
	ctrl := gomock.NewController(t)

	mock_stream := mock_netceptor.NewMockQuicStreamForConn(ctrl)

	mock_stream.EXPECT().Read(gomock.Any()).Return(1, nil)
	conn := netceptor.NewConn(
		netceptor.New(context.Background(), "test-node"),
		nil,
		nil,
		mock_stream,
		nil,
		nil,
		context.Background(),
	)

	var buf = make([]byte, 1)
	num, _ := conn.Read(buf)

	if num != 1 {
		t.Errorf("Expected read to return 1, got %v", num)
	}

}
