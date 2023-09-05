package controlsvc_test

import (
	"context"
	"crypto/tls"
	"errors"
	"os"
	"testing"
	"time"

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
	mockCopier := mock_controlsvc.NewMockCopier(ctrl)
	sockControl := controlsvc.NewSockControl(mock_con, mockUtil, mockCopier)

	localhost := "127.0.0.1"

	mock_con.EXPECT().RemoteAddr().Return(mock_addr)
	mock_addr.EXPECT().String().Return(localhost)
	remoteAddr := sockControl.RemoteAddr()

	if remoteAddr.String() != localhost {
		t.Errorf("expected: %s, received: %s", localhost, remoteAddr)
	}
}

func TestSockControlWriteMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCon := mock_controlsvc.NewMockConn(ctrl)
	mockUtil := mock_controlsvc.NewMockUtiler(ctrl)
	mockCopier := mock_controlsvc.NewMockCopier(ctrl)

	sockControl := controlsvc.NewSockControl(mockCon, mockUtil, mockCopier)

	writeMessageTestCases := []struct {
		name          string
		message       string
		expectedCalls func()
	}{
		{
			name:          "without message",
			message:       "",
			expectedCalls: func() {},
		},
		{
			name:    "with message",
			message: "message",
			expectedCalls: func() {
				mockCon.EXPECT().Write(gomock.Any()).Return(0, errors.New("cannot write message"))
			},
		},
	}

	for _, testCase := range writeMessageTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.expectedCalls()
			err := sockControl.WriteMessage(testCase.message)

			if testCase.message == "" && err != nil {
				t.Errorf("should be nil")
			}
			if testCase.message != "" && err.Error() != "cannot write message" {
				t.Errorf("%s %s", testCase.name, err)
			}
		})
	}
}

func TestSockControlBridgeConn(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCon := mock_controlsvc.NewMockConn(ctrl)
	mockUtil := mock_controlsvc.NewMockUtiler(ctrl)
	mockCopier := mock_controlsvc.NewMockCopier(ctrl)
	logger := logger.NewReceptorLogger("")

	sockControl := controlsvc.NewSockControl(mockCon, mockUtil, mockCopier)

	bridgeConnTestCases := []struct {
		name          string
		message       string
		expectedCalls func()
	}{
		{
			name:    "without message and no error",
			message: "",
			expectedCalls: func() {
				mockUtil.EXPECT().BridgeConns(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			},
		},
		{
			name:    "with message and error",
			message: "message",
			expectedCalls: func() {
				mockCon.EXPECT().Write(gomock.Any()).Return(0, errors.New("blargh"))
			},
		},
	}

	for _, testCase := range bridgeConnTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.expectedCalls()
			err := sockControl.BridgeConn(testCase.message, mockCon, "test", logger)

			if testCase.message == "" && err != nil {
				t.Errorf("should be nil")
			}
			if testCase.message != "" && err.Error() != "blargh" {
				t.Errorf("stuff %v", err)
			}
		})
	}
}

func TestSockControlReadFromConn(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCon := mock_controlsvc.NewMockConn(ctrl)
	mockUtil := mock_controlsvc.NewMockUtiler(ctrl)
	mockCopier := mock_controlsvc.NewMockCopier(ctrl)

	sockControl := controlsvc.NewSockControl(mockCon, mockUtil, mockCopier)

	bridgeConnTestCases := []struct {
		name          string
		message       string
		expectedCalls func()
		expectedError bool
		errorMessage  string
	}{
		{
			name:    "without message and copier error",
			message: "",
			expectedCalls: func() {
				mockCopier.EXPECT().Copy(gomock.Any(), gomock.Any()).Return(int64(0), errors.New("read from conn copy error"))
			},
			expectedError: true,
			errorMessage:  "read from conn copy error",
		},
		{
			name:    "with message and no error",
			message: "message",
			expectedCalls: func() {
				mockCon.EXPECT().Write(gomock.Any()).Return(0, errors.New("read from conn write error"))
			},
			expectedError: true,
			errorMessage:  "read from conn write error",
		},
		{
			name:    "without message and no copier error",
			message: "",
			expectedCalls: func() {
				mockCopier.EXPECT().Copy(gomock.Any(), gomock.Any()).Return(int64(0), nil)
			},
			expectedError: false,
			errorMessage:  "",
		},
	}

	for _, testCase := range bridgeConnTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.expectedCalls()
			err := sockControl.ReadFromConn(testCase.message, mockCon)

			if testCase.expectedError {
				if err == nil && err.Error() != testCase.errorMessage {
					t.Errorf("expected error: %s", err)
				}
			} else {
				if err != nil {
					t.Errorf("expected error %s", err)
				}
			}
		})
	}
}

func TestSockControlWriteToConn(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCon := mock_controlsvc.NewMockConn(ctrl)
	mockUtil := mock_controlsvc.NewMockUtiler(ctrl)
	mockCopier := mock_controlsvc.NewMockCopier(ctrl)

	sockControl := controlsvc.NewSockControl(mockCon, mockUtil, mockCopier)

	bridgeConnTestCases := []struct {
		name          string
		message       string
		input         chan []byte
		expectedCalls func()
		expectedError bool
		errorMessage  string
	}{
		{
			name:    "without message and error",
			message: "",
			expectedCalls: func() {
				mockCon.EXPECT().Write(gomock.Any()).Return(0, errors.New("write to conn chan error"))
			},
			expectedError: true,
			errorMessage:  "write to conn chan error",
		},
		{
			name:    "with message and error",
			message: "message",
			expectedCalls: func() {
				mockCon.EXPECT().Write(gomock.Any()).Return(0, errors.New("write to conn write message error"))
			},
			expectedError: true,
			errorMessage:  "write to conn write message error",
		},
		{
			name:    "without message and error",
			message: "",
			expectedCalls: func() {
				mockCon.EXPECT().Write(gomock.Any()).Return(0, nil)
			},
			expectedError: false,
			errorMessage:  "write to conn write message error",
		},
	}

	for _, testCase := range bridgeConnTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.expectedCalls()
			c := make(chan []byte)
			go func() {
				c <- []byte{7}
			}()
			if !testCase.expectedError {

				time.AfterFunc(time.Millisecond*100, func() {
					close(c)
				})
			}
			err := sockControl.WriteToConn(testCase.message, c)

			if testCase.expectedError {
				if err == nil && err.Error() != testCase.errorMessage {
					t.Errorf("expected error: %s", err)
				}
			} else {
				if err != nil {
					t.Errorf("expected error %s", err)
				}
			}
		})
	}
}

func TestSockControlClose(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCon := mock_controlsvc.NewMockConn(ctrl)
	mockUtil := mock_controlsvc.NewMockUtiler(ctrl)
	mockCopier := mock_controlsvc.NewMockCopier(ctrl)

	sockControl := controlsvc.NewSockControl(mockCon, mockUtil, mockCopier)

	errorMessage := "cannot close connection"

	mockCon.EXPECT().Close().Return(errors.New(errorMessage))

	err := sockControl.Close()
	if err == nil && err.Error() != errorMessage {
		t.Errorf("expected error: %s", errorMessage)
	}
}
