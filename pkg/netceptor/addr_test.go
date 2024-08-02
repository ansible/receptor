package netceptor_test

import (
	"testing"

	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/netceptor/mock_netceptor"
	"github.com/golang/mock/gomock"
)

// func setupTests() *netceptor.Netceptor {
// 	ctx := context.Background()
// 	defer ctx.Done()

// 	n1 := netceptor.New(ctx, "testNode1")
// 	return n1
// }

func TestNetwork(t *testing.T) {
	networkResult := "netceptor-testNode1"
	strResult := "testNode2:testService"

	ctrl := gomock.NewController(t)
	mockNetceptor := mock_netceptor.NewMockNetcForPing(ctrl)

	mockNetceptor.EXPECT().NewAddr(gomock.Any(), gomock.Any()).Return(netceptor.Addr{
		NetworkStr: "netceptor-testNode1",
		Node:       "testNode2",
		Service:    "testService",
	})

	addr := mockNetceptor.NewAddr("testNode2", "testService")
	network := addr.Network()
	str := addr.String()

	if network != networkResult {
		t.Errorf("Expected network to be %v, got %v", networkResult, network)
	}
	if str != strResult {
		t.Errorf("Expected network to be %v, got %v", strResult, str)
	}
}
