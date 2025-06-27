package services

import (
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/netceptor/mock_netceptor"
	mock_net_interface "github.com/ansible/receptor/pkg/services/interfaces/mock_interfaces"
	"github.com/ansible/receptor/pkg/services/mock_services"
	"github.com/ansible/receptor/pkg/utils"
	"go.uber.org/mock/gomock"
)

func setUpMocks(ctrl *gomock.Controller) (*mock_services.MockNetcForUDPProxy, *mock_net_interface.MockNetterUDP, *mock_net_interface.MockUDPConnInterface, *mock_netceptor.MockPacketConner) {
	mockNetceptor := mock_services.NewMockNetcForUDPProxy(ctrl)
	mockNetter := mock_net_interface.NewMockNetterUDP(ctrl)
	mockUDPConn := mock_net_interface.NewMockUDPConnInterface(ctrl)
	mockPacketCon := mock_netceptor.NewMockPacketConner(ctrl)
	logger := logger.NewReceptorLogger("")
	mockNetceptor.EXPECT().GetLogger().AnyTimes().Return(logger)

	return mockNetceptor, mockNetter, mockUDPConn, mockPacketCon
}

func TestUDPProxyServiceInbound(t *testing.T) {
	var mockNetceptor *mock_services.MockNetcForUDPProxy
	var mockNetter *mock_net_interface.MockNetterUDP
	var mockUDPConn *mock_net_interface.MockUDPConnInterface
	var mockPacketCon *mock_netceptor.MockPacketConner
	type testCase struct {
		name      string
		host      string
		port      int
		node      string
		service   string
		expectErr bool
		calls     func()
	}
	tests := []testCase{
		{
			name:      "Fail ResolveUDPAddr",
			expectErr: true,
			calls: func() {
				mockNetter.EXPECT().ResolveUDPAddr(gomock.Any(), gomock.Any()).Return(nil, errors.New("RecolveUDPAddr error"))
			},
		},
		{
			name:      "Fail ListenUDP",
			expectErr: true,
			calls: func() {
				mockNetter.EXPECT().ResolveUDPAddr(gomock.Any(), gomock.Any()).Return(nil, nil)
				mockNetter.EXPECT().ListenUDP(gomock.Any(), gomock.Any()).Return(nil, errors.New("Listen Udp Error"))
			},
		},
		{
			name: "Fail UDP Con Read From",
			calls: func() {
				mockNetter.EXPECT().ResolveUDPAddr(gomock.Any(), gomock.Any()).Return(nil, nil)
				mockNetter.EXPECT().ListenUDP(gomock.Any(), gomock.Any()).Return(mockUDPConn, nil)
				mockNetceptor.EXPECT().NewAddr(gomock.Any(), gomock.Any()).Return(netceptor.Addr{})
				mockUDPConn.EXPECT().ReadFrom(gomock.Any()).Return(0, nil, errors.New("Read From error")).AnyTimes()
			},
		},
		{
			name: "Fail Netceptor listen packet",
			calls: func() {
				mockNetter.EXPECT().ResolveUDPAddr(gomock.Any(), gomock.Any()).Return(nil, nil)
				mockNetter.EXPECT().ListenUDP(gomock.Any(), gomock.Any()).Return(mockUDPConn, nil)
				mockNetceptor.EXPECT().NewAddr(gomock.Any(), gomock.Any()).Return(netceptor.Addr{})
				mockUDPConn.EXPECT().ReadFrom(gomock.Any()).Return(0, netceptor.Addr{}, nil).AnyTimes()
				mockNetceptor.EXPECT().ListenPacket(gomock.Any()).Return(mockPacketCon, errors.New("Clean Up error")).AnyTimes()
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockNetceptor, mockNetter, mockUDPConn, mockPacketCon = setUpMocks(ctrl)
			if tc.calls != nil {
				tc.calls()
			}
			err := UDPProxyServiceInbound(mockNetceptor, tc.host, tc.port, tc.node, tc.service, mockNetter)
			if tc.expectErr {
				if err == nil {
					t.Errorf("net UDPProxyServiceInbound fail case error")
				}

				return
			} else if err != nil {
				t.Errorf("net UDPProxyServiceInbound error")
			}
		})
	}
}

func TestUDPProxyServiceOutbound(t *testing.T) {
	var mockNetceptor *mock_services.MockNetcForUDPProxy
	var mockNetter *mock_net_interface.MockNetterUDP
	var mockPacketCon *mock_netceptor.MockPacketConner
	type testCase struct {
		name      string
		service   string
		address   string
		expectErr bool
		calls     func()
	}

	tests := []testCase{
		{
			name:      "Fail ResolveUDPAddr",
			expectErr: true,
			calls: func() {
				mockNetter.EXPECT().ResolveUDPAddr(gomock.Any(), gomock.Any()).Return(nil, errors.New("RecolveUDPAddr error"))
			},
		},
		{
			name:      "Fail Listen And Advertive",
			expectErr: true,
			calls: func() {
				mockNetter.EXPECT().ResolveUDPAddr(gomock.Any(), gomock.Any()).Return(nil, nil)
				mockNetceptor.EXPECT().ListenPacketAndAdvertise(gomock.Any(), gomock.Any()).Return(nil, errors.New("Netceptor Listen Error"))
			},
		},
		{
			name: "Fail Read From",
			calls: func() {
				mockNetter.EXPECT().ResolveUDPAddr(gomock.Any(), gomock.Any()).Return(nil, nil)
				mockNetceptor.EXPECT().ListenPacketAndAdvertise(gomock.Any(), gomock.Any()).Return(mockPacketCon, nil)
				mockPacketCon.EXPECT().ReadFrom(gomock.Any()).Return(0, nil, errors.New("Read From error")).AnyTimes()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockNetceptor, mockNetter, _, mockPacketCon = setUpMocks(ctrl)
			if tc.calls != nil {
				tc.calls()
			}
			err := UDPProxyServiceOutbound(mockNetceptor, tc.service, tc.address, mockNetter)
			if tc.expectErr {
				if err == nil {
					t.Errorf("net UDPProxyServiceOutbound fail case error")
				}

				return
			} else if err != nil {
				t.Errorf("net UDPProxyServiceOutbound error")
			}
		})
	}
}

func TestProcessInboundPacket(t *testing.T) {
	expectedAddr := netceptor.Addr{}
	expectedAddr.SetNetwork("tcp")
	expectedAddr.SetNode("127.0.0.1")
	expectedAddr.SetService("2222")
	expectedUDPAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 2222}
	unexpectedUDPAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 3333}
	var mockUDPConnInterface *mock_net_interface.MockUDPConnInterface
	var mockPacketConner *mock_netceptor.MockPacketConner
	tests := []struct {
		name           string
		expectedAddr   netceptor.Addr
		calls          func()
		expectContinue bool
	}{
		{
			name:         "ReadFrom error",
			expectedAddr: expectedAddr,
			calls: func() {
				mockPacketConner.EXPECT().ReadFrom(gomock.Any()).Return(0, expectedUDPAddr, fmt.Errorf("ReadFrom error")).Times(1)
				mockPacketConner.EXPECT().GetLogger().Return(logger.NewReceptorLogger("")).Times(1)
			},
		},
		{
			name:         "Expected Address mismatch",
			expectedAddr: expectedAddr,
			calls: func() {
				mockPacketConner.EXPECT().ReadFrom(gomock.Any()).Return(0, unexpectedUDPAddr, nil).Times(1)
				mockPacketConner.EXPECT().GetLogger().Return(logger.NewReceptorLogger("")).Times(1)
			},
		},
		{
			name:         "WriteTo error",
			expectedAddr: expectedAddr,
			calls: func() {
				mockPacketConner.EXPECT().ReadFrom(gomock.Any()).Return(1, expectedUDPAddr, nil).Times(1)
				mockUDPConnInterface.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(1, fmt.Errorf("WriteTo error")).Times(1)
				mockPacketConner.EXPECT().GetLogger().Return(logger.NewReceptorLogger("")).Times(1)
			},
		},
		{
			name:         "Written bytes mismatch",
			expectedAddr: expectedAddr,
			calls: func() {
				mockPacketConner.EXPECT().ReadFrom(gomock.Any()).Return(7, expectedUDPAddr, nil).Times(1)
				mockUDPConnInterface.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(5, nil).Times(1)
				mockPacketConner.EXPECT().GetLogger().Return(logger.NewReceptorLogger("")).Times(1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			_, _, mockUDPConnInterface, mockPacketConner = setUpMocks(ctrl)
			if tt.calls != nil {
				tt.calls()
			}
			buf := make([]byte, utils.NormalBufferSize)
			processInboundPacket(mockPacketConner, mockUDPConnInterface, expectedUDPAddr, tt.expectedAddr, buf)
		})
	}
}

func TestProcessOutboundPacket(t *testing.T) {
	destinationAddr := &net.UDPAddr{IP: net.ParseIP("192.168.0.1"), Port: 2222}
	var mockPacketConner *mock_netceptor.MockPacketConner
	var mockUDPConnInterface *mock_net_interface.MockUDPConnInterface
	tests := []struct {
		name           string
		Addr           netceptor.Addr
		calls          func()
		expectContinue bool
	}{
		{
			name: "Read error",
			calls: func() {
				mockUDPConnInterface.EXPECT().Read(gomock.Any()).Return(1, fmt.Errorf("Read error")).Times(1)
				mockPacketConner.EXPECT().GetLogger().Return(logger.NewReceptorLogger("")).Times(1)
			},
		},
		{
			name: "WriteTo error",
			calls: func() {
				mockUDPConnInterface.EXPECT().Read(gomock.Any()).Return(1, nil).Times(1)
				mockPacketConner.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(1, fmt.Errorf("WriteTo error")).Times(1)
				mockPacketConner.EXPECT().GetLogger().Return(logger.NewReceptorLogger("")).Times(1)
			},
		},
		{
			name: "Written bytes mismatch",
			calls: func() {
				mockUDPConnInterface.EXPECT().Read(gomock.Any()).Return(7, nil).Times(1)
				mockPacketConner.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(5, nil).Times(1)
				mockPacketConner.EXPECT().GetLogger().Return(logger.NewReceptorLogger("")).Times(1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			_, _, mockUDPConnInterface, mockPacketConner = setUpMocks(ctrl)
			if tt.calls != nil {
				tt.calls()
			}
			buf := make([]byte, utils.NormalBufferSize)
			processOutboundPacket(mockUDPConnInterface, mockPacketConner, destinationAddr, buf)
		})
	}
}
