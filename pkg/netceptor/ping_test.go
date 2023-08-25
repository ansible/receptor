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

// setupTest sets up TestPing tests.
func setupTest(t *testing.T) (*gomock.Controller, *mock_netceptor.MockNetcForPing, *mock_netceptor.MockPacketConner) {
	ctrl := gomock.NewController(t)

	// Prepare mocks
	mockNetceptor := mock_netceptor.NewMockNetcForPing(ctrl)
	mockPacketConn := mock_netceptor.NewMockPacketConner(ctrl)

	return ctrl, mockNetceptor, mockPacketConn
}

// createChannel creates a channel that passes an error to errorChan inside of createPing.
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

// checkPing checks TestPing tests by comparing return values to expected values.
func checkPing(duration time.Duration, expectedDuration int, remote string, expectedRemote string, err error, expectedError error, t *testing.T) {
	if expectedError == nil && err != nil {
		t.Errorf("Expected no error, got: %v", err)
	} else if expectedError != nil && (err == nil || err.Error() != expectedError.Error()) {
		t.Errorf("Expected error: %s, got: %v", expectedError.Error(), err)
	}
	if expectedDuration != int(duration) && expectedDuration != 0 {
		t.Errorf("Expected duration to be %v, got: %v", expectedDuration, duration)
	}
	if expectedRemote != remote && expectedRemote != "" {
		t.Errorf("Expected remote to be %v, got: %v", expectedRemote, remote)
	}
}

func setupTestExpects(args ...interface{}) {
	mockNetceptor := args[0].(*mock_netceptor.MockNetcForPing)
	mockPacketConn := args[1].(*mock_netceptor.MockPacketConner)
	testCase := args[2].(pingTestCaseStruct)

	testExpects := map[string]func(){
		"ListenPacketReturn": func() {
			mockNetceptor.EXPECT().ListenPacket(gomock.Any()).Return(testCase.returnListenPacket.packetConn, testCase.returnListenPacket.err).Times(testCase.returnListenPacket.times)
		},
		"SubscribeUnreachableReturn": func() {
			mockPacketConn.EXPECT().SubscribeUnreachable(gomock.Any()).Return(make(chan netceptor.UnreachableNotification))
		},
		"WriteToReturn": func() {
			mockPacketConn.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(testCase.returnWriteTo.packetLen, testCase.returnWriteTo.err).Times(testCase.returnWriteTo.times)
		},
		"ReadFromReturn": func() {
			mockPacketConn.EXPECT().ReadFrom(gomock.Any()).Return(0, testCase.returnReadFrom.address, testCase.returnReadFrom.err).MaxTimes(testCase.returnReadFrom.times)
		},
		"ReadFromDo": func() {
			mockPacketConn.EXPECT().ReadFrom(gomock.Any()).Do(func([]byte) {
				time.Sleep(time.Second * 11)
			}).Times(testCase.returnReadFrom.times)
		},
		"ReadFromDoAndReturn": func() {
			mockPacketConn.EXPECT().ReadFrom(gomock.Any()).DoAndReturn(func([]byte) (int, net.Addr, error) {
				time.Sleep(time.Second * 2)

				return 0, testCase.returnReadFrom.address, testCase.returnReadFrom.err
			}).MaxTimes(testCase.returnReadFrom.times)
		},
		"ContextReturn": func() {
			mockNetceptor.EXPECT().Context().Return(testCase.returnContext.ctx).MaxTimes(testCase.returnContext.times)
		},
		"ContextDoAndReturn": func() {
			mockNetceptor.EXPECT().Context().DoAndReturn(func() context.Context {
				newCtx, ctxCancel := context.WithCancel(context.Background())
				ctxCancel()

				return newCtx
			}).MaxTimes(testCase.returnContext.times)
		},
		"SetHopsToLiveReturn": func() { mockPacketConn.EXPECT().SetHopsToLive(gomock.Any()).Times(testCase.returnSetHopsToLiveTimes) },
		"CloseReturn":         func() { mockPacketConn.EXPECT().Close().Return(nil).Times(testCase.returnCloseTimes) },
		"NewAddrReturn": func() {
			mockNetceptor.EXPECT().NewAddr(gomock.Any(), gomock.Any()).Return(netceptor.Addr{}).Times(testCase.returnNewAddrTimes)
		},
		"NodeID":         func() { mockNetceptor.EXPECT().NodeID().Return("nodeID") },
		"CreateChannel":  func() { createChannel(mockPacketConn) },
		"SleepOneSecond": func() { time.Sleep(time.Second * 1) },
	}

	for _, expect := range testCase.expects {
		testExpects[expect]()
	}
}

type listenPacketReturn struct {
	packetConn netceptor.PacketConner
	err        error
	times      int
	returnType string
}

type writeToReturn struct {
	packetLen  int
	err        error
	times      int
	returnType string
}

type contextReturn struct {
	ctx        context.Context
	times      int
	returnType string
}

type readFromReturn struct {
	data       int
	address    net.Addr
	err        error
	times      int
	returnType string
}

type pingTestCaseStruct struct {
	name                     string
	pingTarget               string
	pingHopsToLive           byte
	returnSetHopsToLiveTimes int
	returnCloseTimes         int
	returnNewAddrTimes       int
	returnListenPacket       listenPacketReturn
	returnWriteTo            writeToReturn
	returnContext            contextReturn
	returnReadFrom           readFromReturn
	expects                  []string
	setupTestExpects         func(args ...interface{})
	expectedDuration         int
	expectedRemote           string
	expectedError            error
}

// TestCreatePing tests CreatePing inside ping.go.
func TestCreatePing(t *testing.T) {
	ctrl, mockNetceptor, mockPacketConn := setupTest(t)

	pingTestCases := []pingTestCaseStruct{
		{"NetceptorShutdown Error", "target", byte(1), 1, 1, 1, listenPacketReturn{mockPacketConn, nil, 1, "return"}, writeToReturn{0, nil, 1, "return"}, contextReturn{context.Background(), 2, "doAndReturn"}, readFromReturn{0, nil, nil, 1, "return"}, []string{"ListenPacketReturn", "SetHopsToLiveReturn", "CloseReturn", "NewAddrReturn", "SubscribeUnreachableReturn", "WriteToReturn", "ReadFromReturn", "ContextDoAndReturn", "SleepOneSecond"}, setupTestExpects, 0, "", errors.New("netceptor shutdown")},
		{"SubscribeUnreachable Error", "target", byte(1), 1, 1, 1, listenPacketReturn{mockPacketConn, nil, 1, "return"}, writeToReturn{0, nil, 1, "return"}, contextReturn{context.Background(), 2, "return"}, readFromReturn{0, nil, nil, 1, "return"}, []string{"CreateChannel", "ListenPacketReturn", "SetHopsToLiveReturn", "CloseReturn", "NewAddrReturn", "WriteToReturn", "ReadFromDoAndReturn", "ContextReturn"}, setupTestExpects, 0, "", errors.New("test")},
		{"CreatePing Success", "target", byte(1), 1, 1, 1, listenPacketReturn{mockPacketConn, nil, 1, "return"}, writeToReturn{0, nil, 1, "return"}, contextReturn{context.Background(), 2, "return"}, readFromReturn{0, &netceptor.Addr{}, nil, 1, "return"}, []string{"ListenPacketReturn", "SetHopsToLiveReturn", "CloseReturn", "NewAddrReturn", "SubscribeUnreachableReturn", "WriteToReturn", "ReadFromReturn", "ContextReturn"}, setupTestExpects, 0, ":", nil},
		{"ListenPacket Error", "target", byte(1), 1, 1, 1, listenPacketReturn{nil, errors.New("Catch ListenPacket error"), 1, "return"}, writeToReturn{0, nil, 0, "return"}, contextReturn{context.Background(), 0, "return"}, readFromReturn{0, &netceptor.Addr{}, nil, 0, "return"}, []string{"ListenPacketReturn"}, setupTestExpects, 0, "", errors.New("Catch ListenPacket error")},
		{"ReadFrom Error", "target", byte(1), 1, 1, 1, listenPacketReturn{mockPacketConn, nil, 1, "return"}, writeToReturn{0, nil, 1, "return"}, contextReturn{context.Background(), 2, "return"}, readFromReturn{0, nil, errors.New("ReadFrom error"), 1, "return"}, []string{"ListenPacketReturn", "SetHopsToLiveReturn", "CloseReturn", "NewAddrReturn", "SubscribeUnreachableReturn", "WriteToReturn", "ReadFromReturn", "ContextReturn"}, setupTestExpects, 0, "", errors.New("ReadFrom error")},
		{"WriteTo Error", "target", byte(1), 1, 1, 1, listenPacketReturn{mockPacketConn, nil, 1, "return"}, writeToReturn{0, errors.New("WriteTo error"), 1, "return"}, contextReturn{context.Background(), 2, "return"}, readFromReturn{0, nil, nil, 1, "return"}, []string{"ListenPacketReturn", "SetHopsToLiveReturn", "CloseReturn", "NewAddrReturn", "SubscribeUnreachableReturn", "WriteToReturn", "ReadFromReturn", "ContextReturn", "NodeID"}, setupTestExpects, 0, "", errors.New("WriteTo error")},
		{"Timeout Error", "target", byte(1), 1, 1, 1, listenPacketReturn{mockPacketConn, nil, 1, "return"}, writeToReturn{0, nil, 1, "return"}, contextReturn{context.Background(), 2, "return"}, readFromReturn{0, nil, nil, 1, "do"}, []string{"ListenPacketReturn", "SetHopsToLiveReturn", "CloseReturn", "NewAddrReturn", "SubscribeUnreachableReturn", "WriteToReturn", "ReadFromDo", "ContextReturn"}, setupTestExpects, 0, "", errors.New("timeout")},
		{"User Cancel Error", "target", byte(1), 1, 1, 1, listenPacketReturn{mockPacketConn, nil, 1, "return"}, writeToReturn{0, nil, 1, "return"}, contextReturn{context.Background(), 2, "return"}, readFromReturn{0, nil, nil, 1, "doAndReturn"}, []string{"ListenPacketReturn", "SetHopsToLiveReturn", "CloseReturn", "NewAddrReturn", "SubscribeUnreachableReturn", "WriteToReturn", "ReadFromDoAndReturn", "ContextReturn"}, setupTestExpects, 0, "", errors.New("user cancelled")},
	}

	for _, testCase := range pingTestCases {
		ctx := context.Background()
		t.Run(testCase.name, func(t *testing.T) {
			testCase.setupTestExpects(mockNetceptor, mockPacketConn, testCase)
			if testCase.name == "NetceptorShutdown Error" {
				time.Sleep(time.Second * 1)
			}
			if testCase.name == "User Cancel Error" {
				newCtx, ctxCancel := context.WithCancel(ctx)

				time.AfterFunc(1*time.Second, ctxCancel)

				duration, remote, err := netceptor.CreatePing(newCtx, mockNetceptor, testCase.pingTarget, testCase.pingHopsToLive)
				checkPing(duration, testCase.expectedDuration, remote, testCase.expectedRemote, err, testCase.expectedError, t)
			} else {
				duration, remote, err := netceptor.CreatePing(ctx, mockNetceptor, testCase.pingTarget, testCase.pingHopsToLive)
				checkPing(duration, testCase.expectedDuration, remote, testCase.expectedRemote, err, testCase.expectedError, t)
			}

			ctrl.Finish()
			ctx.Done()
		})
	}
}

type pingReturn struct {
	duration time.Duration
	remote   string
	err      error
}

type expectedResult struct {
	from string
	time time.Duration
	err  error
}

// TestCreateTraceroute tests CreateTraceroute inside ping.go.
func TestCreateTraceroute(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockNetceptor := mock_netceptor.NewMockNetcForTraceroute(ctrl)
	ctx := context.Background()
	defer ctx.Done()

	createTracerouteTestCases := []struct {
		name                   string
		createTracerouteTarget string
		returnPing             pingReturn
		expectedResult         expectedResult
	}{
		{"CreateTraceroute Success", "target", pingReturn{time.Since(time.Now()), "target", nil}, expectedResult{":", time.Since(time.Now()), nil}},
		{"CreateTraceroute Error", "target", pingReturn{time.Since(time.Now()), "target", errors.New("traceroute error")}, expectedResult{":", time.Since(time.Now()), errors.New("traceroute error")}},
	}

	for _, testCase := range createTracerouteTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			mockNetceptor.EXPECT().Context().Return(context.Background())
			mockNetceptor.EXPECT().MaxForwardingHops().Return(byte(1))
			mockNetceptor.EXPECT().Ping(ctx, testCase.createTracerouteTarget, byte(0)).Return(testCase.returnPing.duration, testCase.returnPing.remote, testCase.returnPing.err)

			result := netceptor.CreateTraceroute(ctx, mockNetceptor, testCase.createTracerouteTarget)
			for res := range result {
				if testCase.expectedResult.err == nil && res.Err != nil {
					t.Errorf("Expected no error, got: %v", res.Err.Error())
				} else if testCase.expectedResult.err != nil && (res.Err == nil || res.Err.Error() != testCase.expectedResult.err.Error()) {
					t.Errorf("Expected error: %s, got: %v", testCase.expectedResult.err.Error(), res.Err)
				}
			}

			ctrl.Finish()
		})
	}
}
