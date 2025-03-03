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

func setUpTCPMocks(ctrl *gomock.Controller) (*mock_services.MockNetcForTCPProxy, *mock_services.MockNetLib, *mock_services.MockTLSLib, *mock_services.MockNetListenerTCP, *mock_services.MockUtilsLib, *mock_services.MockTCPConn) {
	mockNetceptor := mock_services.NewMockNetcForTCPProxy(ctrl)
	mockNetLib := mock_services.NewMockNetLib(ctrl)
	mockTLSLib := mock_services.NewMockTLSLib(ctrl)
	mockNetListener := mock_services.NewMockNetListenerTCP(ctrl)
	mockUtilsLib := mock_services.NewMockUtilsLib(ctrl)
	mockTCPConn := mock_services.NewMockTCPConn(ctrl)
	logger := logger.NewReceptorLogger("")
	mockNetceptor.EXPECT().GetLogger().AnyTimes().Return(logger)

	return mockNetceptor, mockNetLib, mockTLSLib, mockNetListener, mockUtilsLib, mockTCPConn
}

func TestTCPProxyServiceInbound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var mockNetceptor *mock_services.MockNetcForTCPProxy
	var mockNetLib *mock_services.MockNetLib
	var mockTLSLib *mock_services.MockTLSLib
	var mockNetListener *mock_services.MockNetListenerTCP
	var mockUtilsLib *mock_services.MockUtilsLib
	var mockTCPConn *mock_services.MockTCPConn

	type testCoverageItem struct {
		name                 string
		host                 string
		port                 int
		expectError          bool
		expectedErrorMessage string
		node                 string
		service              string
		tlsServerConfig      *tls.Config
		tlsClientConfig      *tls.Config
		calls                func()
	}
	testCases := []testCoverageItem{
		{
			name:                 "Fail to listen to input connections",
			expectError:          true,
			tlsServerConfig:      &tls.Config{},
			expectedErrorMessage: "error listening on TCP: failed to stablish a connection",
			calls: func() {
				mockNetLib.EXPECT().Listen(gomock.Any(), gomock.Any()).Return(nil, errors.New("failed to stablish a connection")).Times(1)
				mockTLSLib.EXPECT().NewListener(gomock.Any(), gomock.Any()).Return(mockNetListener).Times(1)
			},
		},
		{
			name:                 "Fail to listen to input connections with tls config set",
			expectError:          true,
			tlsServerConfig:      nil,
			expectedErrorMessage: "error listening on TCP: failed to stablish a connection",
			calls: func() {
				mockNetLib.EXPECT().Listen(gomock.Any(), gomock.Any()).Return(nil, errors.New("failed to stablish a connection")).Times(1)
			},
		},
		{
			name:            "Fail to accept incoming connections to the listener",
			tlsServerConfig: nil,
			calls: func() {
				mockNetLib.EXPECT().Listen(gomock.Any(), gomock.Any()).Return(mockNetListener, nil).Times(1)
				mockNetListener.EXPECT().Accept().Return(nil, errors.New("failed to accept incoming connection")).AnyTimes()
			},
		},
		{
			name:            "Fail to dial to the receptor network after accepting an inbound connection",
			tlsServerConfig: nil,
			calls: func() {
				mockNetLib.EXPECT().Listen(gomock.Any(), gomock.Any()).Return(mockNetListener, nil).Times(1)
				mockNetListener.EXPECT().Accept().Return(mockTCPConn, nil).AnyTimes()
				mockNetceptor.EXPECT().Dial(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("failed to connect to Receptor network")).AnyTimes()
			},
		},
		{
			name:            "Bridge connections after accepting inbound TCP connection",
			tlsServerConfig: nil,
			calls: func() {
				mockNetLib.EXPECT().Listen(gomock.Any(), gomock.Any()).Return(mockNetListener, nil).Times(1)
				mockNetListener.EXPECT().Accept().Return(mockTCPConn, nil).AnyTimes()
				mockNetceptor.EXPECT().Dial(gomock.Any(), gomock.Any(), gomock.Any()).Return(&netceptor.Conn{}, nil).AnyTimes()
				mockUtilsLib.EXPECT().BridgeConns(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockNetceptor, mockNetLib, mockTLSLib, mockNetListener, mockUtilsLib, mockTCPConn = setUpTCPMocks(ctrl)
			tc.calls()
			err := TCPProxyServiceInbound(mockNetceptor, tc.host, tc.port, tc.tlsServerConfig, tc.node, tc.service, tc.tlsClientConfig, mockNetLib, mockTLSLib, mockUtilsLib)
			// netceptor.MainInstance.GetServerTLSConfig = func(name string) (*tls.Config, error) {return nil, nil}
			if tc.expectError {
				if err == nil {
					t.Errorf("TCPProxyServiceInbound failed to raise error")
				} else if tc.expectedErrorMessage != err.Error() {
					t.Errorf("TCPProxyServiceInbound didn't return the correct error message")
				}
			} else if err != nil {
				t.Errorf("TCPProxyServiceInbound unexpected case error")
			}
		})
	}
}

func TestTCPProxyServiceOutbound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var mockNetceptor *mock_services.MockNetcForTCPProxy
	var mockNetLib *mock_services.MockNetLib
	var mockTLSLib *mock_services.MockTLSLib
	var mockNetListener *mock_services.MockNetListenerTCP
	var mockUtilsLib *mock_services.MockUtilsLib

	type testCoverageItem struct {
		name                 string
		expectError          bool
		expectedErrorMessage string
		service              string
		address              string
		tlsClientConfig      *tls.Config
		calls                func()
	}
	testCases := []testCoverageItem{
		{
			name:                 "Fail to listen and advertise connection",
			expectError:          true,
			expectedErrorMessage: "error listening on Receptor network: failed to stablish a connection",
			calls: func() {
				mockNetceptor.EXPECT().ListenAndAdvertise(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("failed to stablish a connection")).Times(1)
			},
		},
		{
			name: "Fail to accept input connections",
			calls: func() {
				mockNetceptor.EXPECT().ListenAndAdvertise(gomock.Any(), gomock.Any(), gomock.Any()).Return(&netceptor.Listener{}, nil).Times(1)
				mockNetListener.EXPECT().Accept().Return(nil, errors.New("connection acceptance failed")).AnyTimes()
			},
		},
		{
			name:            "Fail to dial through non-TLS TCP connection",
			tlsClientConfig: nil,
			calls: func() {
				mockNetceptor.EXPECT().ListenAndAdvertise(gomock.Any(), gomock.Any(), gomock.Any()).Return(&netceptor.Listener{}, nil).Times(1)
				mockNetListener.EXPECT().Accept().Return(&netceptor.Conn{}, nil).AnyTimes()
				mockNetLib.EXPECT().Dial(gomock.Any(), gomock.Any()).Return(nil, errors.New("non-TLS TCP dial failed")).AnyTimes()
			},
		},
		{
			name:            "Fail to dial through TLS TCP connection",
			tlsClientConfig: &tls.Config{},
			calls: func() {
				mockNetceptor.EXPECT().ListenAndAdvertise(gomock.Any(), gomock.Any(), gomock.Any()).Return(&netceptor.Listener{}, nil).Times(1)
				mockNetListener.EXPECT().Accept().Return(&netceptor.Conn{}, nil).AnyTimes()
				mockTLSLib.EXPECT().Dial(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("TLS TCP dial failed")).AnyTimes()
			},
		},
		{
			name: "Complete connection bridge after successful non-TLS connection",
			calls: func() {
				mockNetceptor.EXPECT().ListenAndAdvertise(gomock.Any(), gomock.Any(), gomock.Any()).Return(&netceptor.Listener{}, nil).Times(1)
				mockNetListener.EXPECT().Accept().Return(&netceptor.Conn{}, nil).AnyTimes()
				mockTLSLib.EXPECT().Dial(gomock.Any(), gomock.Any(), gomock.Any()).Return(&tls.Conn{}, nil).AnyTimes()
				mockUtilsLib.EXPECT().BridgeConns(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockNetceptor, mockNetLib, mockTLSLib, mockNetListener, mockUtilsLib, _ = setUpTCPMocks(ctrl)
			tc.calls()
			err := TCPProxyServiceOutbound(mockNetceptor, tc.service, &tls.Config{}, tc.address, tc.tlsClientConfig, mockNetLib, mockTLSLib, mockUtilsLib)
			if tc.expectError {
				if err == nil {
					t.Errorf("TCPProxyServiceOutbound case failed to raise error")
				} else if tc.expectedErrorMessage != err.Error() {
					t.Errorf("TCPProxyServiceOutbound didn't return the correct error message")
				}
			} else if err != nil {
				t.Errorf("TCPProxyServiceOutbound unexpected case error")
			}
		})
	}
}
