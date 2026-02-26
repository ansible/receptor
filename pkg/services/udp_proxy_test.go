package services

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"
	"testing"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/netceptor/mock_netceptor"
	net_interface "github.com/ansible/receptor/pkg/services/interfaces"
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

func TestRunUDPProxyServiceInbound(t *testing.T) {
	var mockNetceptor *mock_services.MockNetcForUDPProxy
	var mockUDPConn *mock_net_interface.MockUDPConnInterface
	var mockPacketCon *mock_netceptor.MockPacketConner
	tests := []struct {
		name           string
		calls          func()
		expectContinue bool
	}{
		{
			name: "ReadFrom error - should return false",
			calls: func() {
				mockUDPConn.EXPECT().ReadFrom(gomock.Any()).Return(0, nil, fmt.Errorf("ReadFrom error")).Times(1)
			},
			expectContinue: false,
		},
		{
			name: "ListenPacket error for new connection - should return false",
			calls: func() {
				mockUDPConn.EXPECT().ReadFrom(gomock.Any()).Return(1, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}, nil).Times(1)
				mockNetceptor.EXPECT().ListenPacket("").Return(nil, fmt.Errorf("ListenPacket error")).Times(1)
			},
			expectContinue: false,
		},
		{
			name: "WriteTo error - should return true (continue)",
			calls: func() {
				mockUDPConn.EXPECT().ReadFrom(gomock.Any()).Return(1, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}, nil).Times(1)
				mockNetceptor.EXPECT().ListenPacket("").Return(mockPacketCon, nil).Times(1)
				mockNetceptor.EXPECT().NewAddr(gomock.Any(), gomock.Any()).Return(netceptor.Addr{}).AnyTimes() // For the goroutine
				mockPacketCon.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(0, fmt.Errorf("WriteTo error")).Times(1)
				// Expectations for the goroutine that might run after the test
				mockPacketCon.EXPECT().ReadFrom(gomock.Any()).Return(0, nil, fmt.Errorf("test cleanup")).AnyTimes()
				mockUDPConn.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(0, fmt.Errorf("test cleanup")).AnyTimes()
				mockPacketCon.EXPECT().GetLogger().Return(logger.NewReceptorLogger("")).AnyTimes()
			},
			expectContinue: true,
		},
		{
			name: "Partial write - should return true (continue)",
			calls: func() {
				mockUDPConn.EXPECT().ReadFrom(gomock.Any()).Return(10, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}, nil).Times(1)
				mockNetceptor.EXPECT().ListenPacket("").Return(mockPacketCon, nil).Times(1)
				mockNetceptor.EXPECT().NewAddr(gomock.Any(), gomock.Any()).Return(netceptor.Addr{}).AnyTimes() // For the goroutine
				mockPacketCon.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(5, nil).Times(1)
				// Expectations for the goroutine that might run after the test
				mockPacketCon.EXPECT().ReadFrom(gomock.Any()).Return(0, nil, fmt.Errorf("test cleanup")).AnyTimes()
				mockUDPConn.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(0, fmt.Errorf("test cleanup")).AnyTimes()
				mockPacketCon.EXPECT().GetLogger().Return(logger.NewReceptorLogger("")).AnyTimes()
			},
			expectContinue: true,
		},
		{
			name: "Successful write with new connection - should return true (continue)",
			calls: func() {
				mockUDPConn.EXPECT().ReadFrom(gomock.Any()).Return(10, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}, nil).Times(1)
				mockNetceptor.EXPECT().ListenPacket("").Return(mockPacketCon, nil).Times(1)
				mockNetceptor.EXPECT().NewAddr(gomock.Any(), gomock.Any()).Return(netceptor.Addr{}).AnyTimes() // For the goroutine
				mockPacketCon.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(10, nil).Times(1)
				// Expectations for the goroutine that might run after the test
				mockPacketCon.EXPECT().ReadFrom(gomock.Any()).Return(0, nil, fmt.Errorf("test cleanup")).AnyTimes()
				mockUDPConn.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(0, fmt.Errorf("test cleanup")).AnyTimes()
				mockPacketCon.EXPECT().GetLogger().Return(logger.NewReceptorLogger("")).AnyTimes()
			},
			expectContinue: true,
		},
		{
			name: "Successful write with existing connection - should return true (continue)",
			calls: func() {
				mockUDPConn.EXPECT().ReadFrom(gomock.Any()).Return(10, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}, nil).Times(1)
				mockPacketCon.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(10, nil).Times(1)
			},
			expectContinue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockNetceptor, _, mockUDPConn, mockPacketCon = setUpMocks(ctrl)
			if tt.calls != nil {
				tt.calls()
			}

			connMap := make(map[string]netceptor.PacketConner)
			if tt.name == "Successful write with existing connection - should return true (continue)" {
				// Pre-populate the connection map for this test
				connMap["127.0.0.1:8080"] = mockPacketCon
			}

			buffer := make([]byte, utils.NormalBufferSize)
			ncAddr := netceptor.Addr{}

			result := runUDPProxyServiceInbound(mockNetceptor, mockUDPConn, buffer, connMap, ncAddr, "testnode", "testservice")

			if result != tt.expectContinue {
				t.Errorf("Expected runUDPProxyServiceInbound to return %v, but got %v", tt.expectContinue, result)
			}
		})
	}
}

func TestRunUDPProxyServiceOutbound(t *testing.T) {
	var mockNetceptor *mock_services.MockNetcForUDPProxy
	var mockNetter *mock_net_interface.MockNetterUDP
	var mockUDPConn *mock_net_interface.MockUDPConnInterface
	var mockPacketCon *mock_netceptor.MockPacketConner
	tests := []struct {
		name           string
		calls          func()
		expectContinue bool
	}{
		{
			name: "ReadFrom error - should return false",
			calls: func() {
				mockPacketCon.EXPECT().ReadFrom(gomock.Any()).Return(0, nil, fmt.Errorf("ReadFrom error")).Times(1)
			},
			expectContinue: false,
		},
		{
			name: "DialUDP error for new connection - should return false",
			calls: func() {
				mockPacketCon.EXPECT().ReadFrom(gomock.Any()).Return(1, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}, nil).Times(1)
				mockNetter.EXPECT().DialUDP("udp", nil, gomock.Any()).Return(nil, fmt.Errorf("DialUDP error")).Times(1)
			},
			expectContinue: false,
		},
		{
			name: "Write error - should return true (continue)",
			calls: func() {
				mockPacketCon.EXPECT().ReadFrom(gomock.Any()).Return(1, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}, nil).Times(1)
				mockNetter.EXPECT().DialUDP("udp", nil, gomock.Any()).Return(mockUDPConn, nil).Times(1)
				mockUDPConn.EXPECT().Write(gomock.Any()).Return(0, fmt.Errorf("Write error")).Times(1)
				// Expectations for the goroutine that might run after the test
				mockUDPConn.EXPECT().Read(gomock.Any()).Return(0, fmt.Errorf("test cleanup")).AnyTimes()
				mockPacketCon.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(0, fmt.Errorf("test cleanup")).AnyTimes()
				mockPacketCon.EXPECT().GetLogger().Return(logger.NewReceptorLogger("")).AnyTimes()
			},
			expectContinue: true,
		},
		{
			name: "Partial write - should return true (continue)",
			calls: func() {
				mockPacketCon.EXPECT().ReadFrom(gomock.Any()).Return(10, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}, nil).Times(1)
				mockNetter.EXPECT().DialUDP("udp", nil, gomock.Any()).Return(mockUDPConn, nil).Times(1)
				mockUDPConn.EXPECT().Write(gomock.Any()).Return(5, nil).Times(1)
				// Expectations for the goroutine that might run after the test
				mockUDPConn.EXPECT().Read(gomock.Any()).Return(0, fmt.Errorf("test cleanup")).AnyTimes()
				mockPacketCon.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(0, fmt.Errorf("test cleanup")).AnyTimes()
				mockPacketCon.EXPECT().GetLogger().Return(logger.NewReceptorLogger("")).AnyTimes()
			},
			expectContinue: true,
		},
		{
			name: "Successful write with new connection - should return true (continue)",
			calls: func() {
				mockPacketCon.EXPECT().ReadFrom(gomock.Any()).Return(10, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}, nil).Times(1)
				mockNetter.EXPECT().DialUDP("udp", nil, gomock.Any()).Return(mockUDPConn, nil).Times(1)
				mockUDPConn.EXPECT().Write(gomock.Any()).Return(10, nil).Times(1)
				// Expectations for the goroutine that might run after the test
				mockUDPConn.EXPECT().Read(gomock.Any()).Return(0, fmt.Errorf("test cleanup")).AnyTimes()
				mockPacketCon.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(0, fmt.Errorf("test cleanup")).AnyTimes()
				mockPacketCon.EXPECT().GetLogger().Return(logger.NewReceptorLogger("")).AnyTimes()
			},
			expectContinue: true,
		},
		{
			name: "Successful write with existing connection - should return true (continue)",
			calls: func() {
				mockPacketCon.EXPECT().ReadFrom(gomock.Any()).Return(10, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}, nil).Times(1)
				mockUDPConn.EXPECT().Write(gomock.Any()).Return(10, nil).Times(1)
			},
			expectContinue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockNetceptor, mockNetter, mockUDPConn, mockPacketCon = setUpMocks(ctrl)
			if tt.calls != nil {
				tt.calls()
			}

			connMap := make(map[string]net_interface.UDPConnInterface)
			if tt.name == "Successful write with existing connection - should return true (continue)" {
				// Pre-populate the connection map for this test
				connMap["127.0.0.1:8080"] = mockUDPConn
			}

			buffer := make([]byte, utils.NormalBufferSize)
			udpAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}

			result := runUDPProxyServiceOutbound(mockNetceptor, mockPacketCon, buffer, connMap, udpAddr, mockNetter)

			if result != tt.expectContinue {
				t.Errorf("Expected runUDPProxyServiceOutbound to return %v, but got %v", tt.expectContinue, result)
			}
		})
	}
}

func TestNetUDPWrapper_ResolveUDPAddr(t *testing.T) {
	wrapper := &NetUDPWrapper{}

	t.Run("Valid address resolution", func(t *testing.T) {
		addr, err := wrapper.ResolveUDPAddr("udp", "localhost:8080")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if addr == nil {
			t.Fatal("expected non-nil address")
		}
		if addr.Port != 8080 {
			t.Errorf("expected port 8080, got %d", addr.Port)
		}
	})

	t.Run("Invalid address resolution", func(t *testing.T) {
		_, err := wrapper.ResolveUDPAddr("udp", "invalid::address::format")
		if err == nil {
			t.Fatal("expected error for invalid address, got nil")
		}
	})

	t.Run("IPv4 address resolution", func(t *testing.T) {
		addr, err := wrapper.ResolveUDPAddr("udp", "127.0.0.1:9999")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if addr == nil {
			t.Fatal("expected non-nil address")
		}
		if addr.IP.String() != "127.0.0.1" {
			t.Errorf("expected IP 127.0.0.1, got %s", addr.IP.String())
		}
	})
}

func TestNetUDPWrapper_ListenUDP(t *testing.T) {
	wrapper := &NetUDPWrapper{}

	t.Run("Successful listen on random port", func(t *testing.T) {
		addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
		conn, err := wrapper.ListenUDP("udp", addr)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if conn == nil {
			t.Fatal("expected non-nil connection")
		}
		defer conn.Close()

		localAddr := conn.LocalAddr()
		if localAddr == nil {
			t.Fatal("expected non-nil local address")
		}
	})

	t.Run("Listen on IPv6", func(t *testing.T) {
		addr := &net.UDPAddr{IP: net.IPv6loopback, Port: 0}
		conn, err := wrapper.ListenUDP("udp", addr)
		if err != nil {
			// Skip if IPv6 is not available (check for specific syscall errors)
			if opErr, ok := err.(*net.OpError); ok {
				if sysErr, ok := opErr.Err.(*os.SyscallError); ok {
					if errno, ok := sysErr.Err.(syscall.Errno); ok {
						if errno == syscall.EAFNOSUPPORT || errno == syscall.EADDRNOTAVAIL {
							t.Skipf("IPv6 not available: %v", err)
						}
					}
				}
			}
			t.Fatalf("expected no error, got %v", err)
		}
		if conn == nil {
			t.Fatal("expected non-nil connection")
		}
		defer conn.Close()
	})
}

func TestNetUDPWrapper_DialUDP(t *testing.T) {
	wrapper := &NetUDPWrapper{}

	t.Run("Successful dial", func(t *testing.T) {
		raddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9999}
		conn, err := wrapper.DialUDP("udp", nil, raddr)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if conn == nil {
			t.Fatal("expected non-nil connection")
		}
		defer conn.Close()

		remoteAddr := conn.RemoteAddr()
		if remoteAddr == nil {
			t.Fatal("expected non-nil remote address")
		}
	})

	t.Run("Dial with local address", func(t *testing.T) {
		laddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
		raddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9998}
		conn, err := wrapper.DialUDP("udp", laddr, raddr)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if conn == nil {
			t.Fatal("expected non-nil connection")
		}
		defer conn.Close()
	})
}

func TestUDPProxyInboundCfgRun(t *testing.T) {
	type testCase struct {
		name        string
		expectError bool
		configObj   UDPProxyInboundCfg
	}

	testCases := []testCase{
		{
			name: "Valid UDP proxy inbound configuration",
			configObj: UDPProxyInboundCfg{
				Port:          0, // Use ephemeral port
				BindAddr:      "127.0.0.1",
				RemoteNode:    "node1",
				RemoteService: "service1",
			},
		},
		{
			name: "Valid UDP proxy inbound with default bind address",
			configObj: UDPProxyInboundCfg{
				Port:          0, // Use ephemeral port
				BindAddr:      "0.0.0.0",
				RemoteNode:    "node2",
				RemoteService: "service2",
			},
		},
	}

	// Save original instance and create cancellable context
	originalInstance := netceptor.MainInstance
	ctx, cancel := context.WithCancel(context.Background())
	netceptor.MainInstance = netceptor.New(ctx, "test_udp_proxy_inbound_cfg_run")
	defer func() {
		cancel()
		netceptor.MainInstance = originalInstance
	}()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.configObj.Run()
			if tc.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestUDPProxyOutboundCfgRun(t *testing.T) {
	type testCase struct {
		name        string
		expectError bool
		configObj   UDPProxyOutboundCfg
	}

	testCases := []testCase{
		{
			name: "Valid UDP proxy outbound configuration",
			configObj: UDPProxyOutboundCfg{
				Service: "udp1",
				Address: "127.0.0.1:9090",
			},
		},
		{
			name: "Valid UDP proxy outbound with different address",
			configObj: UDPProxyOutboundCfg{
				Service: "udp2",
				Address: "localhost:9091",
			},
		},
	}

	// Save original instance and create cancellable context
	originalInstance := netceptor.MainInstance
	ctx, cancel := context.WithCancel(context.Background())
	netceptor.MainInstance = netceptor.New(ctx, "test_udp_proxy_outbound_cfg_run")
	defer func() {
		cancel()
		netceptor.MainInstance = originalInstance
	}()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.configObj.Run()
			if tc.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
