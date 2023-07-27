package netceptor_test

import (
	"context"
	"errors"
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

	factoryFunc := func(netceptor *netceptor.Netceptor, service string, ConnTypeDatagram byte) (netceptor.PacketConnInterface, error) {
		return mockPacketConnInterface, nil
	}

	n.PacketConnFactoryFunc = factoryFunc

	mockPacketConnInterface.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Times(1)
	mockPacketConnInterface.EXPECT().SetHopsToLive(gomock.Any()).AnyTimes()
	mockPacketConnInterface.EXPECT().Close().AnyTimes()
	mockPacketConnInterface.EXPECT().SubscribeUnreachable(gomock.Any()).AnyTimes()
	mockPacketConnInterface.EXPECT().ReadFrom(gomock.Any()).AnyTimes()

	// Test Ping doesn't return an error
	var b byte = 10
	_, _, err := n.Ping(ctx, "test", b)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	mockPacketConnInterface.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(10, errors.New("writeTo should fail")).Times(1)

	_, _, err = n.Ping(ctx, "test", b)
	if err == nil || err.Error() != "writeTo should fail" {
		t.Errorf("Expected 'writeTo should fail', got: %v", err)
	}
}
