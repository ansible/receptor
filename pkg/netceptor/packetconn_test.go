package netceptor_test

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/netceptor/mock_netceptor"
	"github.com/ansible/receptor/pkg/utils"
	"github.com/golang/mock/gomock"
)

func setupPacketConnTest(t *testing.T) (*gomock.Controller, *mock_netceptor.MockNetcForPacketConn) {
	ctrl := gomock.NewController(t)

	// Prepare mocks
	mockNetceptorForPacketConn := mock_netceptor.NewMockNetcForPacketConn(ctrl)

	return ctrl, mockNetceptorForPacketConn
}

func TestNewPacketConn(t *testing.T) {
	ctrl, mockNetceptorForPacketConn := setupPacketConnTest(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mockNetceptorForPacketConn.EXPECT().MaxForwardingHops().Return(byte(1))
	mockNetceptorForPacketConn.EXPECT().Context().Return(context.Background())
	mockNetceptorForPacketConn.EXPECT().GetUnreachableBroker().Return(utils.NewBroker(ctx, reflect.TypeOf(netceptor.UnreachableNotification{})))
	mockNetceptorForPacketConn.EXPECT().GetListenerRegistery().Return(map[string]*netceptor.PacketConn{})

	t.Run("NewPacketConn Success", func(t *testing.T) {
		pc := netceptor.NewPacketConn(mockNetceptorForPacketConn, "test", 0)
		if pc == nil {
			t.Error("Expected new PacketConn, got nil")
		}
	})
}

func TestListenPacket(t *testing.T) {
	ctrl, _ := setupPacketConnTest(t)
	defer ctrl.Finish()

	listenPacketTestCases := []struct {
		name             string
		service          string
		expectedErr      string
		failedTestString string
	}{
		{"Success", "test", "", "Expected no error, but got: %v"},
		{"Service Too Long Error", "123456789", "service name 123456789 too long", "Expected service name too long error, but got %v"},
		{"Service Already Listening Error", "ping", "service ping is already listening", "Expected service ping is already listening, but got: %v"},
	}

	for _, testCase := range listenPacketTestCases {

		ctx := context.Background()
		netc := netceptor.New(ctx, "node")

		t.Run(testCase.name, func(t *testing.T) {
			_, err := netc.ListenPacket(testCase.service)
			if testCase.expectedErr == "" {
				if err != nil {
					t.Errorf(testCase.failedTestString, err)
				}
			} else if err != nil {
				if err.Error() != testCase.expectedErr {
					t.Errorf(testCase.failedTestString, err)
				}
			} else {
				t.Errorf(testCase.failedTestString, err)
			}
		})

	}
}

func TestListenPacketAndAdvertise(t *testing.T) {
	ctrl, _ := setupPacketConnTest(t)
	defer ctrl.Finish()

	listenPacketTestCases := []struct {
		name             string
		service          string
		tags             map[string]string
		expectedErr      string
		failedTestString string
	}{
		{"Success", "test", map[string]string{}, "", "Expected no error, but got: %v"},
		{"Success empty service", "", map[string]string{}, "", "Expected no error, but got: %v"},
		{"Service Too Long Error", "123456789", map[string]string{}, "service name 123456789 too long", "Expected service name too long error, but got %v"},
		{"Service Already Listening Error", "ping", map[string]string{}, "service ping is already listening and advertising", "Expected service ping is already listening and advertising, but got: %v"},
	}

	for _, testCase := range listenPacketTestCases {

		ctx := context.Background()
		netc := netceptor.New(ctx, "node")

		t.Run(testCase.name, func(t *testing.T) {
			_, err := netc.ListenPacketAndAdvertise(testCase.service, testCase.tags)
			if testCase.expectedErr == "" {
				if err != nil {
					t.Errorf(testCase.failedTestString, err)
				}
			} else if err != nil {
				if err.Error() != testCase.expectedErr {
					t.Errorf(testCase.failedTestString, err)
				}
			} else {
				t.Errorf(testCase.failedTestString, err)
			}
		})

	}
}

func TestPacketConn(t *testing.T) {
	ctrl, mockNetceptorForPacketConn := setupPacketConnTest(t)

	packetConnTestCases := []struct {
		name                   string
		service                string
		expectedCall           func(ctx context.Context)
		funcCall               func(pc netceptor.PacketConner) interface{}
		expectedReturnVal      interface{}
		unexpectedReturnValMsg string
	}{
		{"GetLocalService Success", "test", func(ctx context.Context) {
			mockNetceptorForPacketConn.EXPECT().Context().Return(context.Background())
			mockNetceptorForPacketConn.EXPECT().GetUnreachableBroker().Return(utils.NewBroker(ctx, reflect.TypeOf(netceptor.UnreachableNotification{})))
		}, func(pc netceptor.PacketConner) interface{} {
			return pc.LocalService()
		}, "test", "Expected GetLocalService to be test, but got %v"},
		{"GetLogger Success", "test", func(ctx context.Context) {
			mockNetceptorForPacketConn.EXPECT().Context().Return(context.Background())
			mockNetceptorForPacketConn.EXPECT().GetLogger().Return(logger.NewReceptorLogger("test"))
			mockNetceptorForPacketConn.EXPECT().GetUnreachableBroker().Return(utils.NewBroker(ctx, reflect.TypeOf(netceptor.UnreachableNotification{}))).Times(4)
		}, func(pc netceptor.PacketConner) interface{} {
			return pc.GetLogger().Logger.Prefix()
		}, "test", "Expected Logger prefix to be test, but got %v"},
		{"ReadFrom Error", "", func(ctx context.Context) {
			newCtx, ctxCancel := context.WithCancel(context.Background())
			time.AfterFunc(time.Microsecond*200, ctxCancel)
			mockNetceptorForPacketConn.EXPECT().Context().Return(newCtx)
			mockNetceptorForPacketConn.EXPECT().GetUnreachableBroker().Return(utils.NewBroker(newCtx, reflect.TypeOf(netceptor.UnreachableNotification{})))
		}, func(pc netceptor.PacketConner) interface{} {
			_, _, err := pc.ReadFrom([]byte{})
			return err.Error()
		}, "connection context closed", "Expected ReadFrom error to be connection context closed, but got %v"},
		{"SetHopsToLive Success", "test", func(ctx context.Context) {
			mockNetceptorForPacketConn.EXPECT().Context().Return(context.Background())
			mockNetceptorForPacketConn.EXPECT().GetUnreachableBroker().Return(utils.NewBroker(ctx, reflect.TypeOf(netceptor.UnreachableNotification{})))
		}, func(pc netceptor.PacketConner) interface{} {
			pc.SetHopsToLive(byte(2))
			return pc.GetHopsToLive()
		}, byte(2), "Expected hopsToLive to be 2, but got %v"},
		{"LocalAddr Success", "test", func(ctx context.Context) {
			mockNetceptorForPacketConn.EXPECT().Context().Return(context.Background())
			mockNetceptorForPacketConn.EXPECT().GetUnreachableBroker().Return(utils.NewBroker(ctx, reflect.TypeOf(netceptor.UnreachableNotification{})))
			mockNetceptorForPacketConn.EXPECT().GetNetworkName().Return("test")
			mockNetceptorForPacketConn.EXPECT().NodeID().Return("test")
		}, func(pc netceptor.PacketConner) interface{} {
			return pc.LocalAddr().Network()
		}, "test", "Expected LocalAddr Network to be test, but got %v"},
		{"Close Success", "test", func(ctx context.Context) {
			mockNetceptorForPacketConn.EXPECT().Context().Return(context.Background())
			mockNetceptorForPacketConn.EXPECT().GetUnreachableBroker().Return(utils.NewBroker(ctx, reflect.TypeOf(netceptor.UnreachableNotification{})))
			mockNetceptorForPacketConn.EXPECT().GetListenerLock().Return(&sync.RWMutex{}).Times(2)
			mockNetceptorForPacketConn.EXPECT().GetListenerRegistery().Return(map[string]*netceptor.PacketConn{})
		}, func(pc netceptor.PacketConner) interface{} {
			return pc.Close()
		}, nil, "Expected no error, but got %v"},
		{"Close Error", "test", func(ctx context.Context) {
			mockNetceptorForPacketConn.EXPECT().Context().Return(context.Background())
			mockNetceptorForPacketConn.EXPECT().RemoveLocalServiceAdvertisement("test").Return(errors.New("Close Error"))
			mockNetceptorForPacketConn.EXPECT().GetUnreachableBroker().Return(utils.NewBroker(ctx, reflect.TypeOf(netceptor.UnreachableNotification{})))
			mockNetceptorForPacketConn.EXPECT().GetListenerLock().Return(&sync.RWMutex{}).Times(2)
			mockNetceptorForPacketConn.EXPECT().GetListenerRegistery().Return(map[string]*netceptor.PacketConn{})
		}, func(pc netceptor.PacketConner) interface{} {
			return pc.Close().Error()
		}, "Close Error", "Expected error to be Close Error, but got %v"},
		{"SetDeadline Success", "test", func(ctx context.Context) {
			mockNetceptorForPacketConn.EXPECT().Context().Return(context.Background())
			mockNetceptorForPacketConn.EXPECT().GetUnreachableBroker().Return(utils.NewBroker(ctx, reflect.TypeOf(netceptor.UnreachableNotification{})))
		}, func(pc netceptor.PacketConner) interface{} {
			return pc.SetDeadline(time.Now().Add(time.Millisecond * 100))
		}, nil, "Expected no error, but got %v"},
		{"SetReadDeadline Success", "test", func(ctx context.Context) {
			mockNetceptorForPacketConn.EXPECT().Context().Return(context.Background())
			mockNetceptorForPacketConn.EXPECT().GetUnreachableBroker().Return(utils.NewBroker(ctx, reflect.TypeOf(netceptor.UnreachableNotification{})))
		}, func(pc netceptor.PacketConner) interface{} {
			return pc.SetReadDeadline(time.Now().Add(time.Millisecond * 100))
		}, nil, "Expected no error, but got %v"},
		{"SetWriteDeadline Success", "test", func(ctx context.Context) {
			mockNetceptorForPacketConn.EXPECT().Context().Return(context.Background())
			mockNetceptorForPacketConn.EXPECT().GetUnreachableBroker().Return(utils.NewBroker(ctx, reflect.TypeOf(netceptor.UnreachableNotification{})))
		}, func(pc netceptor.PacketConner) interface{} {
			return pc.SetWriteDeadline(time.Now().Add(time.Millisecond * 100))
		}, nil, "Expected no error, but got %v"},
	}

	for _, testCase := range packetConnTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := context.Background()
			mockNetceptorForPacketConn.EXPECT().MaxForwardingHops().Return(byte(1))
			mockNetceptorForPacketConn.EXPECT().GetListenerRegistery().Return(map[string]*netceptor.PacketConn{})

			testCase.expectedCall(ctx)

			var returnVal interface{}

			if testCase.name == "Close Error" {
				pc := netceptor.NewPacketConnWithConst(mockNetceptorForPacketConn, testCase.service, true, map[string]string{}, byte(0))
				returnVal = testCase.funcCall(pc)

			} else {
				pc := netceptor.NewPacketConn(mockNetceptorForPacketConn, testCase.service, 0)
				returnVal = testCase.funcCall(pc)
			}

			if returnVal != testCase.expectedReturnVal {
				t.Errorf(testCase.unexpectedReturnValMsg, returnVal)
			}

			ctx.Done()
			ctrl.Finish()
		})
	}
}
