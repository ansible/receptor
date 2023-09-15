package controlsvc_test

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/controlsvc"
	"github.com/ansible/receptor/pkg/controlsvc/mock_controlsvc"
	"github.com/ansible/receptor/pkg/logger"
	"github.com/golang/mock/gomock"
)

func TestRunControlSvc(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockNetceptor := mock_controlsvc.NewMockNetceptorForControlsvc(ctrl)
	mockUnix := mock_controlsvc.NewMockUtiler(ctrl)
	mockNet := mock_controlsvc.NewMockNeter(ctrl)
	// mockListener := mock_controlsvc.NewMockListener(ctrl)
	// logger := logger.NewReceptorLogger("")

	runControlSvcTestCases := []struct {
		name          string
		expectedError string
		expectedCalls func()
		listeners     map[string]string
	}{
		{
			name:          "unix listener error",
			expectedError: "error opening Unix socket: terminated",
			expectedCalls: func() {
				mockUnix.EXPECT().UnixSocketListen(gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("terminated"))
			},
			listeners: map[string]string{
				"service":    "",
				"unixSocket": "unix",
				"tcpListen":  "",
			},
		},
		{
			name:          "tcp listener error",
			expectedError: "error listening on TCP socket: terminated",
			expectedCalls: func() {
				mockNet.EXPECT().Listen(gomock.Any(), gomock.Any()).Return(nil, errors.New("terminated"))
			},
			listeners: map[string]string{
				"service":    "",
				"unixSocket": "",
				"tcpListen":  "tcp",
			},
		},
		{
			name:          "service listener error",
			expectedError: "error opening Unix socket: terminated",
			expectedCalls: func() {
				mockNetceptor.EXPECT().ListenAndAdvertise(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("terminated"))
			},
			listeners: map[string]string{
				"service":    "service",
				"unixSocket": "",
				"tcpListen":  "",
			},
		},
		{
			name:          "no listeners error",
			expectedError: "no listeners specified",
			expectedCalls: func() {
			},
			listeners: map[string]string{
				"service":    "",
				"unixSocket": "",
				"tcpListen":  "",
			},
		},
		// {
		// 	name:          "idk",
		// 	expectedError: "",
		// 	expectedCalls: func() {
		// 		mockNet.EXPECT().Listen(gomock.Any(), gomock.Any()).Return(mockListener, nil)
		// 		mockNetceptor.EXPECT().GetLogger().Return(logger)
		// 	},
		// 	listeners: map[string]string{
		// 		"service":    "",
		// 		"unixSocket": "",
		// 		"tcpListen":  "tcp listener",
		// 	},
		// },
	}

	for _, testCase := range runControlSvcTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.expectedCalls()
			s := controlsvc.New(false, mockNetceptor)
			s.SetServerUtils(mockUnix)
			s.SetServerNet(mockNet)

			err := s.RunControlSvc(context.Background(), testCase.listeners["service"], &tls.Config{}, testCase.listeners["unixSocket"], os.FileMode(0o600), testCase.listeners["tcpListen"], &tls.Config{})

			if err == nil || err.Error() != testCase.expectedError {
				t.Errorf("expected error %s, got %s", testCase.expectedError, err.Error())
			}
		})
	}
}

func TestRunControlSvcOld(t *testing.T) {
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

	mockCon := mock_controlsvc.NewMockConn(ctrl)
	mockAddr := mock_controlsvc.NewMockAddr(ctrl)
	mockUtil := mock_controlsvc.NewMockUtiler(ctrl)
	mockCopier := mock_controlsvc.NewMockCopier(ctrl)
	sockControl := controlsvc.NewSockControl(mockCon, mockUtil, mockCopier)

	localhost := "127.0.0.1"

	mockCon.EXPECT().RemoteAddr().Return(mockAddr)
	mockAddr.EXPECT().String().Return(localhost)
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

func TestAddControlFunc(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtrlCmd := mock_controlsvc.NewMockControlCommandType(ctrl)
	mock_netceptor := mock_controlsvc.NewMockNetceptorForControlsvc(ctrl)
	controlFuncTestsCases := []struct {
		name          string
		input         string
		expectedError bool
		errorMessage  string
		testCase      func(msg string, err error)
	}{
		{
			name:          "ping command",
			input:         "ping",
			expectedError: true,
			errorMessage:  "control function named ping already exists",
			testCase: func(msg string, err error) {
				if msg != err.Error() {
					t.Errorf("expected error: %s, received: %s", msg, err)
				}
			},
		},
		{
			name:          "obliterate command",
			input:         "obliterate",
			expectedError: false,
			testCase: func(msg string, err error) {
				if err != nil {
					t.Errorf("error should be nil. received %s", err)
				}
			},
		},
	}

	for _, testCase := range controlFuncTestsCases {
		t.Run(testCase.name, func(t *testing.T) {
			s := controlsvc.New(true, mock_netceptor)
			err := s.AddControlFunc(testCase.input, mockCtrlCmd)
			testCase.testCase(testCase.errorMessage, err)
		})
	}
}

func TestRunControlSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCon := mock_controlsvc.NewMockConn(ctrl)
	mockAddr := mock_controlsvc.NewMockAddr(ctrl)
	mockNetceptor := mock_controlsvc.NewMockNetceptorForControlsvc(ctrl)
	logger := logger.NewReceptorLogger("")

	mandatoryExpects := func() {
		mockNetceptor.EXPECT().GetLogger().Return(logger).Times(3)
		mockCon.EXPECT().RemoteAddr().Return(mockAddr).Times(2)
		mockAddr.EXPECT().String().Times(2)
		mockNetceptor.EXPECT().NodeID()
	}

	runControlSessionTestCases := []struct {
		name          string
		message       string
		input         chan []byte
		expectedCalls func()
		expectedError bool
		errorMessage  string
	}{
		{
			name: "logger warning - could not close connection",
			expectedCalls: func() {
				mockCon.EXPECT().Write(gomock.Any()).Return(0, nil)
				// meh
				mockCon.EXPECT().Read(make([]byte, 1)).Return(0, io.EOF)
				mockCon.EXPECT().Close().Return(errors.New("test"))
				mockNetceptor.EXPECT().GetLogger().Return(logger)
			},
		},
		{
			name: "logger error",
			expectedCalls: func() {
				mockCon.EXPECT().Write(gomock.Any()).Return(0, errors.New("test"))
				mockCon.EXPECT().Close()
			},
			errorMessage: "Could not write in control service: test",
		},
		{
			name: "logger debug - control service closed",
			expectedCalls: func() {
				mockCon.EXPECT().Write(gomock.Any()).Return(0, nil)
				mockCon.EXPECT().Read(make([]byte, 1)).Return(0, io.EOF)
				mockCon.EXPECT().Close()
			},
		},
	}

	for _, testCase := range runControlSessionTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			mandatoryExpects()
			testCase.expectedCalls()
			s := controlsvc.New(false, mockNetceptor)
			s.RunControlSession(mockCon)
		})
	}
}

func TestRunControlSessionTwo(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCon := mock_controlsvc.NewMockConn(ctrl)
	mockNetceptor := mock_controlsvc.NewMockNetceptorForControlsvc(ctrl)
	logger := logger.NewReceptorLogger("")

	runControlSessionTestCases := []struct {
		name          string
		message       string
		input         chan []byte
		expectedCalls func()
		expectedError bool
		errorMessage  string
		commandByte   []byte
	}{
		{
			name: "command must be a string",
			expectedCalls: func() {
				mockNetceptor.EXPECT().NodeID()
				mockCon.EXPECT().Write(gomock.Any()).Return(0, nil).AnyTimes() // don't know why
				mockNetceptor.EXPECT().GetLogger().Return(logger).Times(4)
			},
			commandByte: []byte("{\"command\": 0}"),
		},
		{
			name: "JSON did not contain a command",
			expectedCalls: func() {
				mockNetceptor.EXPECT().NodeID()
				mockCon.EXPECT().Write(gomock.Any()).Return(0, nil).AnyTimes()
				mockNetceptor.EXPECT().GetLogger().Return(logger).Times(4)
			},
			commandByte: []byte("{}"),
		},
		{
			name: "command must be a string",
			expectedCalls: func() {
				mockNetceptor.EXPECT().NodeID()
				mockCon.EXPECT().Write(gomock.Any()).Return(0, nil).AnyTimes() // don't know why
				mockNetceptor.EXPECT().GetLogger().Return(logger).Times(4)
			},
			commandByte: []byte("{\"command\": \"echo\"}"),
		},
		{
			name: "tokens",
			expectedCalls: func() {
				mockNetceptor.EXPECT().NodeID()
				mockCon.EXPECT().Write(gomock.Any()).Return(0, nil).AnyTimes() // don't know why
				mockNetceptor.EXPECT().GetLogger().Return(logger).Times(4)
			},
			commandByte: []byte("a b"),
		},
		{
			name: "control types - reload",
			expectedCalls: func() {
				mockNetceptor.EXPECT().NodeID()
				mockCon.EXPECT().Write(gomock.Any()).Return(0, nil).AnyTimes() // don't know why
				mockNetceptor.EXPECT().GetLogger().Return(logger).Times(6)
			},
			commandByte: []byte("{\"command\": \"reload\"}"),
		},
		{
			name: "control types - no ping target",
			expectedCalls: func() {
				mockNetceptor.EXPECT().NodeID()
				mockCon.EXPECT().Write(gomock.Any()).Return(0, nil).AnyTimes() // don't know why
				mockNetceptor.EXPECT().GetLogger().Return(logger).Times(5)
			},
			commandByte: []byte("{\"command\": \"ping\"}"),
		},
	}

	for _, testCase := range runControlSessionTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.expectedCalls()
			s := controlsvc.New(true, mockNetceptor)
			pipeA, pipeB := net.Pipe()

			go func() {
				pipeA.Write(testCase.commandByte)
				pipeA.Close()

			}()
			go func() {
				io.ReadAll(pipeA)
			}()

			s.RunControlSession(pipeB)
		})
	}
}
