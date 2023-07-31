package netceptor_test

import (
	"context"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/netceptor/mock_netceptor"
	"github.com/golang/mock/gomock"
)

func TestPing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Prepare mocks
	mockNetceptor := mock_netceptor.NewMockNetceptorForPing(ctrl)
	mockPacketConn := mock_netceptor.NewMockPacketConner(ctrl)

	// Set up the mock behaviours
	mockNetceptor.EXPECT().ListenPacket(gomock.Any()).Return(mockPacketConn, nil)
	mockPacketConn.EXPECT().SetHopsToLive(gomock.Any())
	mockPacketConn.EXPECT().SubscribeUnreachable(gomock.Any()).Return(make(chan netceptor.UnreachableNotification))
	mockPacketConn.EXPECT().ReadFrom(gomock.Any()).Return(0, &netceptor.Addr{}, nil)
	mockPacketConn.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(0, nil)
	mockPacketConn.EXPECT().Close().Return(nil)
	mockNetceptor.EXPECT().NewAddr(gomock.Any(), gomock.Any()).Return(netceptor.Addr{})
	mockNetceptor.EXPECT().Context().Return(context.Background()).Times(2)

	// Now you can call Ping and it will use your mock Netceptor and PacketConn
	ctx := context.Background()
	// dur, nodeID, err := mockNetceptor.Ping(ctx, "target", 1)
	dur, nodeID, err := netceptor.Pinger(mockNetceptor, ctx, "target", 1)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// assert duration and nodeID
	expectedDuration := time.Duration(0)
	if dur == expectedDuration {
		t.Errorf("expected duration %v, got %v", dur, expectedDuration)
	}

	expectedNodeID := ""
	if nodeID == expectedNodeID {
		t.Errorf("expected node ID %s, got %s", expectedNodeID, nodeID)
	}
}
