package services

import (
	"errors"
	"testing"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/netceptor/mock_netceptor"
	mock_net_interface "github.com/ansible/receptor/pkg/services/interfaces/mock_interfaces"
	"github.com/ansible/receptor/pkg/services/mock_services"
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
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
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
			mockNetceptor, mockNetter, mockUDPConn, mockPacketCon = setUpMocks(ctrl)
			tc.calls()
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
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
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
			mockNetceptor, mockNetter, _, mockPacketCon = setUpMocks(ctrl)
			tc.calls()
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
