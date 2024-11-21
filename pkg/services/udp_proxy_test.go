package services

import (
	"errors"
	"testing"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/netceptor/mock_netceptor"
	"github.com/ansible/receptor/pkg/services/mock_services"

	mock_net_interface "github.com/ansible/receptor/pkg/services/interfaces/mock_interfaces"
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
	mockNetceptor, mockNetter, mockUDPConn, mockPacketCon := setUpMocks(ctrl)
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
			name: "Pass Case",
			calls: func() {
				mockNetter.EXPECT().ResolveUDPAddr(gomock.Any(), gomock.Any()).Return(nil, nil)
				mockNetter.EXPECT().ListenUDP(gomock.Any(), gomock.Any()).Return(mockUDPConn, nil)
				mockNetceptor.EXPECT().NewAddr(gomock.Any(), gomock.Any()).Return(netceptor.Addr{})
				mockUDPConn.EXPECT().ReadFrom(gomock.Any()).Return(0, netceptor.Addr{}, nil)
				mockNetceptor.EXPECT().ListenPacket(gomock.Any()).Return(mockPacketCon, nil).Times(1)
				mockNetceptor.EXPECT().NewAddr(gomock.Any(), gomock.Any()).Return(netceptor.Addr{})
				mockPacketCon.EXPECT().ReadFrom(gomock.Any()).Return(0, netceptor.Addr{}, nil).Times(1)
				mockUDPConn.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(0, nil).AnyTimes()
				mockPacketCon.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(0, nil).AnyTimes()
				mockUDPConn.EXPECT().ReadFrom(gomock.Any()).Return(0, netceptor.Addr{}, errors.New("Clean Up error"))
				mockNetceptor.EXPECT().ListenPacket(gomock.Any()).Return(mockPacketCon, errors.New("Clean Up error"))

			},
		},
		// {
		// 	name: "Fail UDP Con Read From",
		// 	calls: func() {
		// 		mockNetter.EXPECT().ResolveUDPAddr(gomock.Any(), gomock.Any()).Return(nil, nil)
		// 		mockNetter.EXPECT().ListenUDP(gomock.Any(), gomock.Any()).Return(mockUDPConn, nil)
		// 		mockNetceptor.EXPECT().NewAddr(gomock.Any(), gomock.Any()).Return(netceptor.Addr{})
		// 		mockUDPConn.EXPECT().ReadFrom(gomock.Any()).Return(0, nil, errors.New("Read From error"))
		// 	},
		// },
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.calls()
			err := UDPProxyServiceInbound(mockNetceptor, tc.host, tc.port, tc.node, tc.service, mockNetter)
			if tc.expectErr {
				if err == nil {
					t.Errorf("net UDPProxyServiceInbound fail case error")
				}
				return
			}
			if err != nil {
				t.Errorf("net UDPProxyServiceInbound error")
			}
		})
	}
}
