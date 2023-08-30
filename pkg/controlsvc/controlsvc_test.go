package controlsvc_test

import (
	"context"
	"crypto/tls"
	"errors"
	"os"
	"testing"

	"github.com/ansible/receptor/pkg/controlsvc"
	"github.com/ansible/receptor/pkg/controlsvc/mock_controlsvc"
	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/golang/mock/gomock"
)

func TestAddControlFunc(t *testing.T) {

}

func TestOne(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mock_netceptor := mock_controlsvc.NewMockNetceptorForControlsvc(ctrl)
	s := controlsvc.New(false, mock_netceptor)

	mock_netceptor.EXPECT().ListenAndAdvertise(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("the world blew up"))

	err := s.RunControlSvc(context.Background(), "test", &tls.Config{}, "", os.FileMode(0o600), "", &tls.Config{})

	errorString := "error opening Unix socket: the world blew up"
	if err == nil || err.Error() != errorString {
		t.Errorf("expected error %s, got %s", errorString, err.Error())
	}

}

func TestTwo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mock_netceptor := mock_controlsvc.NewMockNetceptorForControlsvc(ctrl)
	s := controlsvc.New(false, mock_netceptor)

	mock_netceptor.EXPECT().ListenAndAdvertise(gomock.Any(), gomock.Any(), gomock.Any()).Return(&netceptor.Listener{}, nil)

	mock_netceptor.EXPECT().GetLogger().Return(logger.NewReceptorLogger("test"))
	err := s.RunControlSvc(context.Background(), "test", &tls.Config{}, "", os.FileMode(0o600), "", &tls.Config{})

	if err != nil {
		t.Errorf("it blew up the second time")
	}
}

func TestThree(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mock_netceptor := mock_controlsvc.NewMockNetceptorForControlsvc(ctrl)
	s := controlsvc.New(false, mock_netceptor)

	err := s.RunControlSvc(context.Background(), "", &tls.Config{}, "", os.FileMode(0o600), "", &tls.Config{})

	errorString := "no listeners specified"
	if err == nil || err.Error() != errorString {
		t.Errorf("expected error: %+v, got: %+v", errorString, err.Error())
	}
}

func TestFour(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mock_netceptor := mock_controlsvc.NewMockNetceptorForControlsvc(ctrl)
	s := controlsvc.New(false, mock_netceptor)
	mock_unix := mock_controlsvc.NewMockUtiler(ctrl)
	s.SetServerUtils(mock_unix)

	mock_unix.EXPECT().UnixSocketListen(gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("unix blargh"))

	err := s.RunControlSvc(context.Background(), "", &tls.Config{}, "unixSocket", os.FileMode(0o600), "", &tls.Config{})

	errorString := "error opening Unix socket: unix blargh"
	if err == nil || err.Error() != errorString {
		t.Errorf("expected error: %+v, got: %+v", errorString, err.Error())
	}
}

func TestFive(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mock_netceptor := mock_controlsvc.NewMockNetceptorForControlsvc(ctrl)
	s := controlsvc.New(false, mock_netceptor)
	mock_net := mock_controlsvc.NewMockNeter(ctrl)
	s.SetServerNet(mock_net)

	mock_net.EXPECT().Listen(gomock.Any(), gomock.Any()).Return(nil, errors.New("net blargh"))

	err := s.RunControlSvc(context.Background(), "", &tls.Config{}, "", os.FileMode(0o600), "tcpListen", &tls.Config{})

	errorString := "error listening on TCP socket: net blargh"
	if err == nil || err.Error() != errorString {
		t.Errorf("expected error: %+v, got: %+v", errorString, err.Error())
	}
}

func TestSix(t *testing.T) {
}

func TestSeven(t *testing.T) {

}
func TestRunControlSvc(t *testing.T) {
	// ctrl := gomock.NewController(t)
	// defer ctrl.Finish()

	// mock_netceptor := mock_controlsvc.NewMockNetceptorForControlsvc(ctrl)
	// s := controlsvc.New(false, mock_netceptor)
	// mock_unix := mock_controlsvc.NewMockUtiler(ctrl)
	// s.SetServerUtils(mock_unix)

	// mock_net_listener := mock_controlsvc.NewMockListener(ctrl)
	// mock_unix.EXPECT().UnixSocketListen(gomock.Any(), gomock.Any()).Return(mock_net_listener, nil, nil)

	// newCtx, ctxCancel := context.WithTimeout(context.Background(), time.Millisecond*1)
	// defer ctxCancel()

	// logger := logger.NewReceptorLogger("test")
	// mock_net_listener.EXPECT().Accept().Return(nil, errors.New("blargh"))
	// // mock_net_listener.EXPECT().Close()
	// mock_netceptor.EXPECT().GetLogger().Return(logger)
	// err := s.RunControlSvc(newCtx, "", &tls.Config{}, "unixSocket", os.FileMode(0o600), "", &tls.Config{})
	// errorString := "Error accepting connection: blargh"
	// fmt.Println(err, errorString)
	// if err == nil || err.Error() != errorString {
	// 	t.Errorf("expected error: %+v, got: %+v", errorString, err.Error())
	// }

}

func TestSockControlRemoteAddr(t *testing.T) {
	ctrl := gomock.NewController(t)

	mock_con := mock_controlsvc.NewMockConn(ctrl)
	mock_addr := mock_controlsvc.NewMockAddr(ctrl)
	mockUtil := mock_controlsvc.NewMockUtiler(ctrl)
	sockControl := controlsvc.NewSockControl(mock_con, mockUtil)
	localhost := "127.0.0.1"

	mock_con.EXPECT().RemoteAddr().Return(mock_addr)
	mock_addr.EXPECT().String().Return(localhost)
	remoteAddr := sockControl.RemoteAddr()

	if remoteAddr.String() != localhost {
		t.Errorf("expected: %s, received: %s", localhost, remoteAddr)
	}
}

func TestSockControlBridgeConn(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCon := mock_controlsvc.NewMockConn(ctrl)
	mockUtil := mock_controlsvc.NewMockUtiler(ctrl)
	mockRWCloser := mock_controlsvc.NewMockReadWriteCloser(ctrl)
	sockControl := controlsvc.NewSockControl(mockCon, mockUtil)
	logger := logger.NewReceptorLogger("")

	bridgeConnTestCases := []struct {
		name          string
		message       string
		expectedCalls func()
	}{
		{
			name:    "without message",
			message: "",
			expectedCalls: func() {
				mockUtil.EXPECT().BridgeConns(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			},
		},
		{
			name:    "with message",
			message: "message",
			expectedCalls: func() {
				mockCon.EXPECT().Write(gomock.Any()).Return(0, errors.New("blargh"))
			},
		},
	}

	for _, testCase := range bridgeConnTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.expectedCalls()
			err := sockControl.BridgeConn(testCase.message, mockRWCloser, "test", logger)

			if testCase.message == "" {
				if err != nil {
					t.Errorf("should be nil")
				}
			} else {
				if err.Error() == "" {
					t.Errorf("should be nil")
				}
			}
		})

	}

}
