package netceptor_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/netceptor/mock_netceptor"
	"github.com/golang/mock/gomock"
)

func setupTest(t *testing.T) (*gomock.Controller, *mock_netceptor.MockNetceptorForPing, *mock_netceptor.MockPacketConner, context.Context) {
	ctrl := gomock.NewController(t)

	// Prepare mocks
	mockNetceptor := mock_netceptor.NewMockNetceptorForPing(ctrl)
	mockPacketConn := mock_netceptor.NewMockPacketConner(ctrl)

	// Now you can call Ping and it will use your mock Netceptor and PacketConn
	ctx := context.Background()

	return ctrl, mockNetceptor, mockPacketConn, ctx
}

func TestCreatePing(t *testing.T) {
	ctrl, mockNetceptor, mockPacketConn, ctx := setupTest(t)
	defer ctrl.Finish()
	defer ctx.Done()

	// Set up the mock behaviours
	mockNetceptor.EXPECT().ListenPacket(gomock.Any()).Return(mockPacketConn, nil)
	mockPacketConn.EXPECT().SetHopsToLive(gomock.Any())
	mockPacketConn.EXPECT().SubscribeUnreachable(gomock.Any()).Return(make(chan netceptor.UnreachableNotification))
	mockPacketConn.EXPECT().ReadFrom(gomock.Any()).Return(0, &netceptor.Addr{}, nil)
	mockPacketConn.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(0, nil)
	mockPacketConn.EXPECT().Close().Return(nil)
	mockNetceptor.EXPECT().NewAddr(gomock.Any(), gomock.Any()).Return(netceptor.Addr{})
	mockNetceptor.EXPECT().Context().Return(context.Background()).Times(2)

	// dur, nodeID, err := mockNetceptor.Ping(ctx, "target", 1)
	dur, nodeID, err := netceptor.CreatePing(ctx, mockNetceptor, "target", 1)
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

func TestListenPacketErr(t *testing.T) {
	ctrl, mockNetceptor, _, ctx := setupTest(t)
	defer ctrl.Finish()
	defer ctx.Done()

	// Set up the mock behaviours
	mockNetceptor.EXPECT().ListenPacket(gomock.Any()).Return(nil, errors.New("Catch ListenPacket error"))
	_, _, listenPacketError := netceptor.CreatePing(ctx, mockNetceptor, "target", 1)
	if listenPacketError == nil {
		t.Fatal("ListenPacker expected to return error but returned nil")
	}
}

func TestListenSubscribeUnreachableErr(t *testing.T) {
	ctrl, mockNetceptor, mockPacketConn, ctx := setupTest(t)
	defer ctrl.Finish()
	defer ctx.Done()

	mockUnreachableMessage := netceptor.UnreachableMessage{
		FromNode:    "",
		ToNode:      "",
		FromService: "",
		ToService:   "",
		Problem:     "test",
	}

	mockUnreachableNotification := netceptor.UnreachableNotification{
		mockUnreachableMessage,
		"test",
	}

	channel := make(chan netceptor.UnreachableNotification)

	// Set up the mock behaviours
	mockNetceptor.EXPECT().ListenPacket(gomock.Any()).Return(mockPacketConn, nil)
	mockPacketConn.EXPECT().SetHopsToLive(gomock.Any())
	mockPacketConn.EXPECT().Close().Return(nil)
	mockNetceptor.EXPECT().NewAddr(gomock.Any(), gomock.Any()).Return(netceptor.Addr{})
	mockPacketConn.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(0, nil)
	mockNetceptor.EXPECT().Context().Return(context.Background()).Times(1)
	mockPacketConn.EXPECT().ReadFrom(gomock.Any()).Return(0, &netceptor.Addr{}, nil).AnyTimes()
	mockPacketConn.EXPECT().SubscribeUnreachable(gomock.Any()).Return(channel)
	go func() {
		channel <- mockUnreachableNotification
	}()
	time.Sleep(time.Second * 2)
	_, _, subscribeUnreachableError := netceptor.CreatePing(ctx, mockNetceptor, "target", 1)
	if subscribeUnreachableError == nil {
		t.Fatal("SubscribeUnreachable expected to return error but returned nil")
	}
}

func TestReadFromErr(t *testing.T) {
	ctrl, mockNetceptor, mockPacketConn, ctx := setupTest(t)
	defer ctrl.Finish()
	defer ctx.Done()

	// Set up the mock behaviours
	mockNetceptor.EXPECT().ListenPacket(gomock.Any()).Return(mockPacketConn, nil)
	mockPacketConn.EXPECT().SetHopsToLive(gomock.Any())
	mockPacketConn.EXPECT().Close().Return(nil)
	mockNetceptor.EXPECT().NewAddr(gomock.Any(), gomock.Any()).Return(netceptor.Addr{})
	mockPacketConn.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(0, nil)
	mockNetceptor.EXPECT().Context().Return(context.Background()).Times(2)
	mockPacketConn.EXPECT().SubscribeUnreachable(gomock.Any()).Return(make(chan netceptor.UnreachableNotification))
	mockPacketConn.EXPECT().ReadFrom(gomock.Any()).Return(0, &netceptor.Addr{}, errors.New("ReadFrom error"))

	_, _, readFromError := netceptor.CreatePing(ctx, mockNetceptor, "target", 1)
	if readFromError == nil {
		t.Fatal("ReadFrom expected to return error but returned nil")
	}
}

func TestWriteToErr(t *testing.T) {
	ctrl, mockNetceptor, mockPacketConn, ctx := setupTest(t)
	defer ctrl.Finish()
	defer ctx.Done()

	// Set up the mock behaviours
	mockNetceptor.EXPECT().ListenPacket(gomock.Any()).Return(mockPacketConn, nil)
	mockPacketConn.EXPECT().SetHopsToLive(gomock.Any())
	mockPacketConn.EXPECT().Close().Return(nil)
	mockNetceptor.EXPECT().NewAddr(gomock.Any(), gomock.Any()).Return(netceptor.Addr{})
	mockPacketConn.EXPECT().SubscribeUnreachable(gomock.Any()).Return(make(chan netceptor.UnreachableNotification))
	mockNetceptor.EXPECT().NodeID().Return("nodeID")
	mockPacketConn.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(0, errors.New("WriteTo error"))

	_, _, writeToError := netceptor.CreatePing(ctx, mockNetceptor, "target", 1)
	if writeToError == nil {
		t.Fatal("ReadFrom expected to return error but returned nil")
	}
}

func TestTimeOutErr(t *testing.T) {
	ctrl, mockNetceptor, mockPacketConn, ctx := setupTest(t)
	defer ctrl.Finish()
	defer ctx.Done()

	// Set up the mock behaviours
	mockNetceptor.EXPECT().ListenPacket(gomock.Any()).Return(mockPacketConn, nil)
	mockPacketConn.EXPECT().SetHopsToLive(gomock.Any())
	mockPacketConn.EXPECT().SubscribeUnreachable(gomock.Any()).Return(make(chan netceptor.UnreachableNotification))
	mockPacketConn.EXPECT().ReadFrom(gomock.Any()).DoAndReturn(func(arg []uint8) (int, *netceptor.Addr, error) {
		time.Sleep(time.Second * 11)
		return 0, &netceptor.Addr{}, nil
	})
	mockPacketConn.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(0, nil)
	mockPacketConn.EXPECT().Close().Return(nil)
	mockNetceptor.EXPECT().NewAddr(gomock.Any(), gomock.Any()).Return(netceptor.Addr{})
	mockNetceptor.EXPECT().Context().Return(context.Background()).Times(1)

	_, _, err := netceptor.CreatePing(ctx, mockNetceptor, "target", 1)
	if err.Error() != "timeout" {
		t.Fatalf("Unexpected error: %v", err)
	}

}
