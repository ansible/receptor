package controlsvc_test

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"os"
	"testing"

	"github.com/ansible/receptor/pkg/controlsvc"
	"github.com/ansible/receptor/pkg/controlsvc/mock_controlsvc"
	"github.com/ansible/receptor/pkg/logger"
	"github.com/golang/mock/gomock"
)

const (
	writeToConnError = "write to conn write message err"
)

func printErrorMessage(t *testing.T, err error) {
	t.Errorf("expected error %s", err)
}

func TestConnectionListener(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockNetceptor := mock_controlsvc.NewMockNetceptorForControlsvc(ctrl)
	mockListener := mock_controlsvc.NewMockListener(ctrl)
	logger := logger.NewReceptorLogger("")

	connectionListenerTestCases := []struct {
		name          string
		expectedError bool
		expectedCalls func(context.CancelFunc)
	}{
		{
			name:          "error accepting connection",
			expectedError: false,
			expectedCalls: func(ctxCancel context.CancelFunc) {
				mockListener.EXPECT().Accept().DoAndReturn(func() (net.Conn, error) {
					ctxCancel()

					return nil, errors.New("terminated")
				})
				mockNetceptor.EXPECT().GetLogger().Return(logger)
			},
		},
	}

	for _, testCase := range connectionListenerTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, ctxCancel := context.WithCancel(context.Background())
			defer ctxCancel()

			testCase.expectedCalls(ctxCancel)
			s := controlsvc.New(false, mockNetceptor)
			s.ConnectionListener(ctx, mockListener)
		})
	}
}

func TestSetupConnection(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockNetceptor := mock_controlsvc.NewMockNetceptorForControlsvc(ctrl)
	mockConn := mock_controlsvc.NewMockConn(ctrl)
	logger := logger.NewReceptorLogger("")

	setupConnectionTestCases := []struct {
		name          string
		expectedCalls func()
	}{
		{
			name: "log error - setting timeout",
			expectedCalls: func() {
				mockConn.EXPECT().SetDeadline(gomock.Any()).Return(errors.New("terminated"))
				mockNetceptor.EXPECT().GetLogger().Return(logger)
				mockConn.EXPECT().Close()
			},
		},
		{
			name: "log error - tls handshake",
			expectedCalls: func() {
				mockConn.EXPECT().SetDeadline(gomock.Any()).Return(nil)
				mockNetceptor.EXPECT().GetLogger().Return(logger)
				mockConn.EXPECT().Close().AnyTimes()
			},
		},
	}

	for _, testCase := range setupConnectionTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.expectedCalls()
			s := controlsvc.New(false, mockNetceptor)
			tlsConn := tls.Client(mockConn, &tls.Config{})
			s.SetupConnection(tlsConn)
		})
	}
}

func TestRunControlSvc(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockNetceptor := mock_controlsvc.NewMockNetceptorForControlsvc(ctrl)
	mockUnix := mock_controlsvc.NewMockUtiler(ctrl)
	mockNet := mock_controlsvc.NewMockNeter(ctrl)
	mockTLS := mock_controlsvc.NewMockTlser(ctrl)
	mockListener := mock_controlsvc.NewMockListener(ctrl)

	logger := logger.NewReceptorLogger("")

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
				// empty func for testing
			},
			listeners: map[string]string{
				"service":    "",
				"unixSocket": "",
				"tcpListen":  "",
			},
		},
		{
			name:          "tcp listener set",
			expectedError: "",
			expectedCalls: func() {
				mockNet.EXPECT().Listen(gomock.Any(), gomock.Any()).Return(mockListener, nil)
				mockTLS.EXPECT().NewListener(gomock.Any(), gomock.Any()).Return(mockListener)
				mockNetceptor.EXPECT().GetLogger().Return(logger).AnyTimes()
				mockListener.EXPECT().Accept().Return(nil, errors.New("normal close")).AnyTimes()
			},
			listeners: map[string]string{
				"service":    "",
				"unixSocket": "",
				"tcpListen":  "tcp:1",
			},
		},
	}

	for _, testCase := range runControlSvcTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.expectedCalls()
			s := controlsvc.New(false, mockNetceptor)
			s.SetServerUtils(mockUnix)
			s.SetServerNet(mockNet)
			s.SetServerTLS(mockTLS)

			err := s.RunControlSvc(context.Background(), testCase.listeners["service"], &tls.Config{}, testCase.listeners["unixSocket"], os.FileMode(0o600), testCase.listeners["tcpListen"], &tls.Config{})

			if err != nil && err.Error() != testCase.expectedError {
				t.Errorf("expected error %s, got %v", testCase.expectedError, err)
			}
		})
	}
}

func TestSockControlRemoteAddr(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockCon := mock_controlsvc.NewMockConn(ctrl)
	mockAddr := mock_controlsvc.NewMockAddr(ctrl)
	sockControl := controlsvc.NewSockControl(mockCon)

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
	sockControl := controlsvc.NewSockControl(mockCon)

	writeMessageTestCases := []struct {
		name          string
		message       string
		expectedError bool
		expectedCalls func()
	}{
		{
			name:          "pass without message",
			message:       "",
			expectedError: false,
			expectedCalls: func() {
				// empty func for testing
			},
		},
		{
			name:          "fail with message",
			message:       "message",
			expectedError: true,
			expectedCalls: func() {
				mockCon.EXPECT().Write(gomock.Any()).Return(0, errors.New("cannot write message"))
			},
		},
		{
			name:          "pass with message",
			message:       "message",
			expectedError: false,
			expectedCalls: func() {
				mockCon.EXPECT().Write(gomock.Any()).Return(0, nil)
			},
		},
	}

	for _, testCase := range writeMessageTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.expectedCalls()
			err := sockControl.WriteMessage(testCase.message)

			if !testCase.expectedError && err != nil {
				t.Errorf("write message ran unsuccessfully %s", err)
			}

			if testCase.expectedError && err.Error() != "cannot write message" {
				printErrorMessage(t, err)
			}
		})
	}
}

func TestSockControlBridgeConn(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCon := mock_controlsvc.NewMockConn(ctrl)
	mockUtil := mock_controlsvc.NewMockUtiler(ctrl)
	logger := logger.NewReceptorLogger("")

	sockControl := controlsvc.NewSockControl(mockCon)

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
				mockCon.EXPECT().Write(gomock.Any()).Return(0, errors.New("terminated"))
			},
		},
	}

	for _, testCase := range bridgeConnTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.expectedCalls()
			err := sockControl.BridgeConn(testCase.message, mockCon, "test", logger, mockUtil)

			if testCase.message == "" && err != nil {
				t.Errorf("bridge conn ran unsuccessfully")
			}
			if testCase.message != "" && err.Error() != "terminated" {
				t.Errorf("write message error for bridge conn %v", err)
			}
		})
	}
}

func TestSockControlReadFromConn(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCon := mock_controlsvc.NewMockConn(ctrl)
	mockCopier := mock_controlsvc.NewMockCopier(ctrl)

	sockControl := controlsvc.NewSockControl(mockCon)

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
			err := sockControl.ReadFromConn(testCase.message, mockCon, mockCopier)

			if testCase.expectedError && err.Error() != testCase.errorMessage {
				printErrorMessage(t, err)
			}

			if !testCase.expectedError && err != nil {
				printErrorMessage(t, err)
			}
		})
	}
}

func TestSockControlWriteToConn(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCon := mock_controlsvc.NewMockConn(ctrl)
	sockControl := controlsvc.NewSockControl(mockCon)

	bridgeConnTestCases := []struct {
		name          string
		message       string
		input         chan []byte
		expectedCalls func()
		expectedError bool
		errorMessage  string
	}{
		{
			name:    "without message and with error",
			message: "",
			expectedCalls: func() {
				mockCon.EXPECT().Write(gomock.Any()).Return(0, errors.New("write to conn chan error"))
			},
			expectedError: true,
			errorMessage:  "write to conn chan error",
		},
		{
			name:    "with message and with error",
			message: "message",
			expectedCalls: func() {
				mockCon.EXPECT().Write(gomock.Any()).Return(0, errors.New(writeToConnError))
			},
			expectedError: true,
			errorMessage:  writeToConnError,
		},
		{
			name:    "without message and error",
			message: "",
			expectedCalls: func() {
				mockCon.EXPECT().Write(gomock.Any()).Return(0, nil)
			},
			expectedError: false,
			errorMessage:  writeToConnError,
		},
	}

	for _, testCase := range bridgeConnTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.expectedCalls()
			c := make(chan []byte)
			go func(c chan []byte) {
				c <- []byte{7}
				defer close(c)
			}(c)

			err := sockControl.WriteToConn(testCase.message, c)

			if testCase.expectedError && err.Error() != testCase.errorMessage {
				printErrorMessage(t, err)
			}

			if !testCase.expectedError && err != nil {
				printErrorMessage(t, err)
			}
		})
	}
}

func TestSockControlClose(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCon := mock_controlsvc.NewMockConn(ctrl)
	sockControl := controlsvc.NewSockControl(mockCon)

	errorMessage := "cannot close connection"

	mockCon.EXPECT().Close().Return(errors.New(errorMessage))

	err := sockControl.Close()
	if err == nil && err.Error() != errorMessage {
		printErrorMessage(t, err)
	}
}

func TestAddControlFunc(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtrlCmd := mock_controlsvc.NewMockControlCommandType(ctrl)
	mockNetceptor := mock_controlsvc.NewMockNetceptorForControlsvc(ctrl)
	controlFuncTestsCases := []struct {
		name         string
		input        string
		errorMessage string
		testCase     func(msg string, err error)
	}{
		{
			name:         "ping command",
			input:        "ping",
			errorMessage: "control function named ping already exists",
			testCase: func(msg string, err error) {
				if msg != err.Error() {
					t.Errorf("expected error: %s, received: %s", msg, err)
				}
			},
		},
		{
			name:  "obliterate command",
			input: "obliterate",
			testCase: func(msg string, err error) {
				if err != nil {
					t.Errorf("error should be nil. received %s", err)
				}
			},
		},
	}

	for _, testCase := range controlFuncTestsCases {
		t.Run(testCase.name, func(t *testing.T) {
			s := controlsvc.New(true, mockNetceptor)
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
		expectedCalls func()
	}{
		{
			name: "logger warning - could not close connection",
			expectedCalls: func() {
				mockCon.EXPECT().Write(gomock.Any()).Return(0, nil)
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
		},
		{
			name: "logger debug - control service closed",
			expectedCalls: func() {
				mockCon.EXPECT().Write(gomock.Any()).Return(0, nil)
				mockCon.EXPECT().Read(make([]byte, 1)).Return(0, io.EOF)
				mockCon.EXPECT().Close()
			},
		},
		{
			name: "logger warning - could not read in control service",
			expectedCalls: func() {
				mockCon.EXPECT().Write(gomock.Any()).Return(0, nil)
				mockCon.EXPECT().Read(make([]byte, 1)).Return(0, errors.New("terminated"))
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
	mockNetceptor := mock_controlsvc.NewMockNetceptorForControlsvc(ctrl)
	logger := logger.NewReceptorLogger("")

	runControlSessionTestCases := []struct {
		name          string
		expectedCalls func()
		commandByte   []byte
	}{
		{
			name: "command must be a string",
			expectedCalls: func() {
				mockNetceptor.EXPECT().NodeID()
				mockNetceptor.EXPECT().GetLogger().Return(logger).Times(4)
			},
			commandByte: []byte("{\"command\": 0}"),
		},
		{
			name: "JSON did not contain a command",
			expectedCalls: func() {
				mockNetceptor.EXPECT().NodeID()
				mockNetceptor.EXPECT().GetLogger().Return(logger).Times(4)
			},
			commandByte: []byte("{}"),
		},
		{
			name: "command must be a string",
			expectedCalls: func() {
				mockNetceptor.EXPECT().NodeID()
				mockNetceptor.EXPECT().GetLogger().Return(logger).Times(4)
			},
			commandByte: []byte("{\"command\": \"echo\"}"),
		},
		{
			name: "tokens",
			expectedCalls: func() {
				mockNetceptor.EXPECT().NodeID()
				mockNetceptor.EXPECT().GetLogger().Return(logger).Times(4)
			},
			commandByte: []byte("a b"),
		},
		{
			name: "control types - reload",
			expectedCalls: func() {
				mockNetceptor.EXPECT().NodeID()
				mockNetceptor.EXPECT().GetLogger().Return(logger).Times(6)
			},
			commandByte: []byte("{\"command\": \"reload\"}"),
		},
		{
			name: "control types - no ping target",
			expectedCalls: func() {
				mockNetceptor.EXPECT().NodeID()
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
