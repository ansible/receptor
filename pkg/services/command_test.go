package services

import (
	"crypto/tls"
	"errors"
	"testing"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/services/mock_services"
	"go.uber.org/mock/gomock"
)

func setUpCommandMocks(ctrl *gomock.Controller) (*mock_services.MockNetCForCommandService, *mock_services.MockUtilsLib) {
	mockNetceptor := mock_services.NewMockNetCForCommandService(ctrl)
	mockUtilsLib := mock_services.NewMockUtilsLib(ctrl)
	logger := logger.NewReceptorLogger("")
	mockNetceptor.EXPECT().GetLogger().AnyTimes().Return(logger)

	return mockNetceptor, mockUtilsLib
}

func TestCommandService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTCPConn := mock_services.NewMockTCPConn(ctrl)
	var mockNetceptor *mock_services.MockNetCForCommandService
	var mockUtilsLib *mock_services.MockUtilsLib

	acceptChannel := make(chan *netceptor.AcceptResult)
	doneChannel := make(chan struct{})
	neceptorListener := netceptor.Listener{AcceptChan: acceptChannel, DoneChan: doneChannel}

	type test struct {
		name         string
		service      string
		command      string
		acceptResult []*netceptor.AcceptResult
		calls        func()
	}
	tests := []test{
		{name: "No command provided"},
		{
			name:    "Fail to listen and advertise connection",
			command: "echo hello",
			calls: func() {
				mockNetceptor.EXPECT().ListenAndAdvertise(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("failed to establish a connection"))
			},
		},
		{
			name:         "Fail to accept connection",
			acceptResult: []*netceptor.AcceptResult{{Conn: nil, Err: errors.New("failed to accept connection")}},
			command:      "echo hello",
			calls: func() {
				mockNetceptor.EXPECT().ListenAndAdvertise(gomock.Any(), gomock.Any(), gomock.Any()).Return(&neceptorListener, nil)
			},
		},
		// This is a case where shlex.Split() returns an empty array
		// In order to stop execution of the CommandService() in this test we pass an error on the AcceptChannel
		{
			name:         "Malformed command",
			acceptResult: []*netceptor.AcceptResult{{Conn: mockTCPConn, Err: nil}, {Conn: nil, Err: errors.New("failed to accept connection")}},
			command:      "# nine # ten\n",
			calls: func() {
				mockNetceptor.EXPECT().ListenAndAdvertise(gomock.Any(), gomock.Any(), gomock.Any()).Return(&neceptorListener, nil)
				mockTCPConn.EXPECT().Close().AnyTimes()
			},
		},
		// Happy path
		// In order to stop execution of the CommandService() in this test we pass an error on the AcceptChannel
		{
			name:         "Test echo command",
			acceptResult: []*netceptor.AcceptResult{{Conn: mockTCPConn, Err: nil}, {Conn: mockTCPConn, Err: errors.New("failed to accept connection")}},
			command:      "echo hello",
			calls: func() {
				mockNetceptor.EXPECT().ListenAndAdvertise(gomock.Any(), gomock.Any(), gomock.Any()).Return(&neceptorListener, nil)
				mockUtilsLib.EXPECT().BridgeConns(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockNetceptor, mockUtilsLib = setUpCommandMocks(ctrl)

			if tt.calls != nil {
				tt.calls()
			}

			if tt.acceptResult != nil {
				go func() {
					for _, result := range tt.acceptResult {
						neceptorListener.AcceptChan <- result
					}
				}()
			}
			CommandService(mockNetceptor, tt.service, &tls.Config{}, tt.command, mockUtilsLib)
		})
	}
}
