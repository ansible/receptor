package netceptor_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/netceptor/mock_netceptor"
	"github.com/ansible/receptor/pkg/utils"
	"github.com/golang/mock/gomock"
)

func setupPacketConnTest(t *testing.T) (*gomock.Controller, *mock_netceptor.MockNetcForPacketConn, *mock_netceptor.MockPacketConner) {
	ctrl := gomock.NewController(t)

	// Prepare mocks
	mockNetceptorForPacketConn := mock_netceptor.NewMockNetcForPacketConn(ctrl)
	mockPacketConn := mock_netceptor.NewMockPacketConner(ctrl)

	return ctrl, mockNetceptorForPacketConn, mockPacketConn
}

func TestPacketConn(t *testing.T) {
	ctrl, mockNetceptorForPacketConn, _ := setupPacketConnTest(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mockNetceptorForPacketConn.EXPECT().MaxForwardingHops().Return(byte(1))
	mockNetceptorForPacketConn.EXPECT().Context().Return(context.Background())
	mockNetceptorForPacketConn.EXPECT().GetUnreachableBroker().Return(utils.NewBroker(ctx, reflect.TypeOf(netceptor.UnreachableNotification{})))
	mockNetceptorForPacketConn.EXPECT().GetListenerRegistery().Return(map[string]*netceptor.PacketConn{})

	t.Run("NewPacketConn Success", func(t *testing.T) {
		pc := netceptor.NewPacketConn(mockNetceptorForPacketConn, "test", 0)
		if pc == nil {
			t.Error("Expected no error")
		}
	})
}

func TestListenPacket(t *testing.T) {
	ctrl, mockNetceptorForPacketConn, _ := setupPacketConnTest(t)
	defer ctrl.Finish()

	// mockNetceptorForPacketConn.EXPECT().GetEphemeralService().Return("test")
	mockNetceptorForPacketConn.EXPECT().GetListenerLock().Return()

	t.Run("ListenPacket Success", func(t *testing.T) {
		_, err := (*netceptor.Netceptor).ListenPacket(&netceptor.Netceptor{}, "test")
		if err != nil {
			t.Errorf("Expected no error, but got: %v", err)
		}
	})
}
