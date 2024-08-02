package netceptor_test

import (
	"testing"

	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/netceptor/mock_netceptor"
	"github.com/golang/mock/gomock"
)

func TestNetwork(t *testing.T) {
	networkResult := "netceptor-testNode1"
	strResult := "testNode2:testService"

	ctrl := gomock.NewController(t)
	mockNetceptor := mock_netceptor.NewMockNetcForPing(ctrl)

	mockNetceptor.EXPECT().NewAddr(gomock.Any(), gomock.Any()).Return(netceptor.Addr{})

	addr := mockNetceptor.NewAddr("testNode2", "testService")
	addr.SetNetwork(networkResult)
	addr.SetNode("testNode2")
	addr.SetService("testService")

	network := addr.Network()
	str := addr.String()

	if network != networkResult {
		t.Errorf("Expected network to be %v, got %v", networkResult, network)
	}
	if str != strResult {
		t.Errorf("Expected network to be %v, got %v", strResult, str)
	}
}
