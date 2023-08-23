package netceptor_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/netceptor/mock_netceptor"
	"github.com/golang/mock/gomock"
)

func setupTest(t *testing.T) (*gomock.Controller, *mock_netceptor.MockNetcForPing, *mock_netceptor.MockPacketConner, context.Context) {
	ctrl := gomock.NewController(t)

	// Prepare mocks
	mockNetceptor := mock_netceptor.NewMockNetcForPing(ctrl)
	mockPacketConn := mock_netceptor.NewMockPacketConner(ctrl)

	// Now you can call Ping and it will use your mock Netceptor and PacketConn
	ctx := context.Background()

	mockPacketConn.EXPECT().SetHopsToLive(gomock.Any()).AnyTimes()
	mockPacketConn.EXPECT().Close().Return(nil).AnyTimes()
	mockNetceptor.EXPECT().NewAddr(gomock.Any(), gomock.Any()).Return(netceptor.Addr{}).AnyTimes()

	return ctrl, mockNetceptor, mockPacketConn, ctx
}

func teardownTest(ctrl *gomock.Controller, mockNetceptor *mock_netceptor.MockNetcForPing, mockPacketConn *mock_netceptor.MockPacketConner) {
	mockPacketConn.EXPECT().SetHopsToLive(gomock.Any()).Times(0)
	mockPacketConn.EXPECT().Close().Times(0)
	mockNetceptor.EXPECT().NewAddr(gomock.Any(), gomock.Any()).Times(0)
	mockNetceptor.EXPECT().ListenPacket(gomock.Any()).Times(0)
	mockPacketConn.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Times(0)
	mockNetceptor.EXPECT().Context().Times(0)
	mockPacketConn.EXPECT().ReadFrom(gomock.Any()).Times(0)
	mockPacketConn.EXPECT().SubscribeUnreachable(gomock.Any()).Times(0)
	ctrl.Finish()
}

func createChannel(mockPacketConn *mock_netceptor.MockPacketConner) {
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

	mockPacketConn.EXPECT().SubscribeUnreachable(gomock.Any()).Return(channel)
	go func() {
		channel <- mockUnreachableNotification
	}()
}

func checkPing(duration time.Duration, expectedDuration int, remote string, expectedRemote string, err error, expectedError string, t *testing.T) {
	if expectedError == "" && err != nil {
		t.Errorf("Expected no error, got: %v", err)
	} else if expectedError != "" && (err == nil || err.Error() != expectedError) {
		t.Errorf("Expected error: %s, got: %v", expectedError, err)
	}
}

func TestPing(t *testing.T) {
	ctrl, mockNetceptor, mockPacketConn, ctx := setupTest(t)
	defer ctx.Done()

	pingTestCases := []struct {
		name                  string
		pingTarget            string
		pingHopsToLive        byte
		returnListenPacket    netceptor.PacketConner
		returnListenPackerErr error
		returnWriteTo         int
		returnWriteToErr      error
		returnContext         context.Context
		returnReadFrom        net.Addr
		returnReadFromErr     error
		expectedDuration      int
		expectedRemote        string
		expectedError         error
	}{
		{"ListenSubscribeUnreachable Error", "target", byte(1), mockPacketConn, nil, 0, nil, context.Background(), nil, errors.New("error"), 0, "taget", errors.New("error")},
	}

	for _, testCase := range pingTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.name == "ListenSubscribeUnreachable Error" {
				createChannel(mockPacketConn)
			} else {
				mockPacketConn.EXPECT().SubscribeUnreachable(gomock.Any()).Return(make(chan netceptor.UnreachableNotification))
			}
			// Set up the mock behaviours
			mockNetceptor.EXPECT().ListenPacket(gomock.Any()).Return(testCase.returnListenPacket, testCase.returnListenPackerErr)
			mockPacketConn.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(testCase.returnWriteTo, testCase.returnWriteToErr)
			mockNetceptor.EXPECT().Context().Return(testCase.returnContext).MaxTimes(2)
			mockPacketConn.EXPECT().ReadFrom(gomock.Any()).Return(0, testCase.returnReadFrom, testCase.returnReadFromErr).Times(1)

			duration, remote, err := netceptor.CreatePing(ctx, mockNetceptor, testCase.pingTarget, testCase.pingHopsToLive)
			checkPing(duration, testCase.expectedDuration, remote, testCase.expectedRemote, err, testCase.expectedError.Error(), t)

			teardownTest(ctrl, mockNetceptor, mockPacketConn)

		})
	}
}

// func TestListenSubscribeUnreachableErr(t *testing.T) {
// 	ctrl, mockNetceptor, mockPacketConn, ctx := setupTest(t)
// 	defer ctx.Done()

// 	mockUnreachableMessage := netceptor.UnreachableMessage{
// 		FromNode:    "",
// 		ToNode:      "",
// 		FromService: "",
// 		ToService:   "",
// 		Problem:     "test",
// 	}

// 	mockUnreachableNotification := netceptor.UnreachableNotification{
// 		mockUnreachableMessage,
// 		"test",
// 	}

// 	channel := make(chan netceptor.UnreachableNotification)

// 	// Set up the mock behaviours
// 	mockNetceptor.EXPECT().ListenPacket(gomock.Any()).Return(mockPacketConn, nil)
// 	mockPacketConn.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(0, nil)
// 	mockNetceptor.EXPECT().Context().Return(context.Background()).MaxTimes(2)
// 	mockPacketConn.EXPECT().ReadFrom(gomock.Any()).Return(0, nil, errors.New("error")).Times(1)
// 	mockPacketConn.EXPECT().SubscribeUnreachable(gomock.Any()).Return(channel)
// 	go func() {
// 		channel <- mockUnreachableNotification
// 	}()
// 	_, _, subscribeUnreachableError := netceptor.CreatePing(ctx, mockNetceptor, "target", 1)
// 	if subscribeUnreachableError == nil {
// 		t.Fatal("SubscribeUnreachable expected to return error but returned nil")
// 	}

// 	teardownTest(ctrl, mockNetceptor, mockPacketConn)
// }

// func TestCreatePing(t *testing.T) {
// 	ctrl, mockNetceptor, mockPacketConn, ctx := setupTest(t)
// 	// defer ctrl.Finish()
// 	defer ctx.Done()

// 	// Set up the mock behaviours
// 	mockNetceptor.EXPECT().ListenPacket(gomock.Any()).Return(mockPacketConn, nil)
// 	mockPacketConn.EXPECT().SubscribeUnreachable(gomock.Any()).Return(make(chan netceptor.UnreachableNotification))
// 	mockPacketConn.EXPECT().ReadFrom(gomock.Any()).Return(0, &netceptor.Addr{}, nil).Times(1)
// 	mockPacketConn.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(0, nil)
// 	mockNetceptor.EXPECT().Context().Return(context.Background()).Times(2)

// 	// dur, nodeID, err := mockNetceptor.Ping(ctx, "target", 1)
// 	dur, nodeID, err := netceptor.CreatePing(ctx, mockNetceptor, "target", 1)
// 	if err != nil {
// 		t.Fatalf("Unexpected error: %v", err)
// 	}

// 	// assert duration and nodeID
// 	expectedDuration := time.Duration(0)
// 	if dur == expectedDuration {
// 		t.Errorf("expected duration %v, got %v", dur, expectedDuration)
// 	}

// 	expectedNodeID := ""
// 	if nodeID == expectedNodeID {
// 		t.Errorf("expected node ID %s, got %s", expectedNodeID, nodeID)
// 	}

// 	teardownTest(ctrl, mockNetceptor, mockPacketConn)
// }

// func TestListenPacketErr(t *testing.T) {
// 	ctrl, mockNetceptor, mockPacketConn, ctx := setupTest(t)
// 	// defer ctrl.Finish()
// 	defer ctx.Done()

// 	// Set up the mock behaviours
// 	mockNetceptor.EXPECT().ListenPacket(gomock.Any()).Return(nil, errors.New("Catch ListenPacket error"))
// 	_, _, listenPacketError := netceptor.CreatePing(ctx, mockNetceptor, "target", 1)
// 	if listenPacketError == nil {
// 		t.Fatal("ListenPacker expected to return error but returned nil")
// 	}

// 	teardownTest(ctrl, mockNetceptor, mockPacketConn)

// }

// func TestReadFromErr(t *testing.T) {
// 	ctrl, mockNetceptor, mockPacketConn, ctx := setupTest(t)
// 	// defer ctrl.Finish()
// 	defer ctx.Done()

// 	// Set up the mock behaviours
// 	mockNetceptor.EXPECT().ListenPacket(gomock.Any()).Return(mockPacketConn, nil)
// 	mockPacketConn.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(0, nil)
// 	mockNetceptor.EXPECT().Context().Return(context.Background()).Times(2)
// 	mockPacketConn.EXPECT().SubscribeUnreachable(gomock.Any()).Return(make(chan netceptor.UnreachableNotification))
// 	mockPacketConn.EXPECT().ReadFrom(gomock.Any()).Return(0, nil, errors.New("ReadFrom error")).Times(1)

// 	_, _, readFromError := netceptor.CreatePing(ctx, mockNetceptor, "target", 1)
// 	if readFromError == nil {
// 		t.Fatal("ReadFrom expected to return error but returned nil")
// 	}

// 	teardownTest(ctrl, mockNetceptor, mockPacketConn)
// }

// func TestWriteToErr(t *testing.T) {
// 	ctrl, mockNetceptor, mockPacketConn, ctx := setupTest(t)
// 	// defer ctrl.Finish()
// 	defer ctx.Done()

// 	// Set up the mock behaviours
// 	mockNetceptor.EXPECT().ListenPacket(gomock.Any()).Return(mockPacketConn, nil)
// 	mockPacketConn.EXPECT().SubscribeUnreachable(gomock.Any()).Return(make(chan netceptor.UnreachableNotification))
// 	mockNetceptor.EXPECT().NodeID().Return("nodeID")
// 	mockPacketConn.EXPECT().ReadFrom(gomock.Any()).Return(0, nil, errors.New("ReadFrom error")).MaxTimes(1)
// 	mockNetceptor.EXPECT().Context().Return(context.Background()).MaxTimes(2)
// 	mockPacketConn.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(0, errors.New("WriteTo error"))

// 	_, _, writeToError := netceptor.CreatePing(ctx, mockNetceptor, "target", 1)
// 	if writeToError == nil {
// 		t.Fatal("ReadFrom expected to return error but returned nil")
// 	}

// 	teardownTest(ctrl, mockNetceptor, mockPacketConn)
// }

// func TestTimeOutErr(t *testing.T) {
// 	ctrl, mockNetceptor, mockPacketConn, ctx := setupTest(t)
// 	// defer ctrl.Finish()
// 	defer ctx.Done()

// 	// Set up the mock behaviours
// 	mockNetceptor.EXPECT().ListenPacket(gomock.Any()).Return(mockPacketConn, nil)
// 	mockPacketConn.EXPECT().SubscribeUnreachable(gomock.Any()).Return(make(chan netceptor.UnreachableNotification))
// 	mockPacketConn.EXPECT().ReadFrom(gomock.Any()).Do(func([]byte) {
// 		time.Sleep(time.Second * 11)
// 	}).Times(1)
// 	mockPacketConn.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(0, nil)
// 	mockNetceptor.EXPECT().Context().Return(context.Background()).MaxTimes(2)

// 	_, _, err := netceptor.CreatePing(ctx, mockNetceptor, "target", 1)
// 	if err.Error() != "timeout" {
// 		t.Fatalf("Expected error to be 'timeout' but got %v", err)
// 	}

// 	teardownTest(ctrl, mockNetceptor, mockPacketConn)
// }

// func TestUserCancel(t *testing.T) {
// 	ctrl, mockNetceptor, mockPacketConn, ctx := setupTest(t)
// 	// defer ctrl.Finish()
// 	defer ctx.Done()

// 	// Set up the mock behaviours
// 	mockNetceptor.EXPECT().ListenPacket(gomock.Any()).Return(mockPacketConn, nil)
// 	mockPacketConn.EXPECT().SubscribeUnreachable(gomock.Any()).Return(make(chan netceptor.UnreachableNotification))
// 	mockPacketConn.EXPECT().ReadFrom(gomock.Any()).Return(0, nil, nil).Times(1)
// 	mockPacketConn.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(0, nil)
// 	mockNetceptor.EXPECT().Context().DoAndReturn(func() context.Context {
// 		time.Sleep(time.Second * 2)

// 		return context.Background()
// 	}).Times(2)

// 	newCtx, ctxCancel := context.WithCancel(ctx)

// 	time.AfterFunc(1*time.Second, ctxCancel)

// 	_, _, err := netceptor.CreatePing(newCtx, mockNetceptor, "target", 1)
// 	if err.Error() != "user cancelled" {
// 		t.Fatalf("Expected error to be 'user cancelled' but got %v", err)
// 	}

// 	teardownTest(ctrl, mockNetceptor, mockPacketConn)
// }

// func TestNetceptorShutdown(t *testing.T) {
// 	ctrl, mockNetceptor, mockPacketConn, ctx := setupTest(t)
// 	// defer ctrl.Finish()
// 	defer ctx.Done()

// 	// Set up the mock behaviours
// 	mockNetceptor.EXPECT().ListenPacket(gomock.Any()).Return(mockPacketConn, nil)
// 	mockPacketConn.EXPECT().SubscribeUnreachable(gomock.Any()).Return(make(chan netceptor.UnreachableNotification))
// 	mockPacketConn.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(0, nil)
// 	mockPacketConn.EXPECT().ReadFrom(gomock.Any()).Return(0, nil, nil).MaxTimes(1)
// 	mockNetceptor.EXPECT().Context().DoAndReturn(func() context.Context {
// 		newCtx, ctxCancel := context.WithCancel(context.Background())
// 		ctxCancel()

// 		return newCtx
// 	}).Times(2)
// 	time.Sleep(time.Second * 1)

// 	_, _, err := netceptor.CreatePing(ctx, mockNetceptor, "target", 1)
// 	if err.Error() != "netceptor shutdown" {
// 		t.Fatalf("Expected error to be 'netceptor shutdown' but got %v", err)
// 	}

// 	teardownTest(ctrl, mockNetceptor, mockPacketConn)
// }

func TestCreateTraceroute(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockNetceptor := mock_netceptor.NewMockNetcForTraceroute(ctrl)
	ctx := context.Background()
	defer ctx.Done()

	mockNetceptor.EXPECT().Context().Return(context.Background())
	mockNetceptor.EXPECT().MaxForwardingHops().Return(byte(1))
	mockNetceptor.EXPECT().Ping(ctx, "target", byte(0)).Return(time.Since(time.Now()), "target", nil)

	result := netceptor.CreateTraceroute(ctx, mockNetceptor, "target")
	for res := range result {
		if res.Err != nil {
			t.Fatalf("Unexpected error %v", res.Err)
		}
	}

	ctrl.Finish()
}

func TestCreateTracerouteError(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockNetceptor := mock_netceptor.NewMockNetcForTraceroute(ctrl)
	ctx := context.Background()
	defer ctx.Done()

	mockNetceptor.EXPECT().Context().Return(context.Background())
	mockNetceptor.EXPECT().MaxForwardingHops().Return(byte(1))
	mockNetceptor.EXPECT().Ping(ctx, "target", byte(0)).Return(time.Since(time.Now()), "target", errors.New("traceroute error"))

	result := netceptor.CreateTraceroute(ctx, mockNetceptor, "target")
	for res := range result {
		if res.Err.Error() != "traceroute error" {
			t.Fatalf("Expected error to be 'traceroute error' but got: %v", res.Err)
		}
	}

	ctrl.Finish()
}
