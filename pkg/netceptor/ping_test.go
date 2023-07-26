package netceptor_test

import (
	"context"
	"testing"

	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/netceptor/mock_netceptor"
	"github.com/golang/mock/gomock"
)

func TestPing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()

	mockPacketConnInterface := mock_netceptor.NewMockPacketConnInterface(ctrl)
	n := netceptor.New(ctx, "test")

	// Test Ping doesn't return an error
	var b byte = 10
	_, _, err := n.Ping(ctx, "test", b)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	mockPacketConnInterface.EXPECT().SetHopsToLive(gomock.Any()).AnyTimes()
	mockPacketConnInterface.EXPECT().LocalAddr().Return(mockPacketConnInterface.LocalAddr()).Times(1)
}
