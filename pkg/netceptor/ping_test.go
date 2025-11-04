package netceptor_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/netceptor/mock_netceptor"
	"go.uber.org/mock/gomock"
)

// Helper function to create test addresses.
func newTestAddr(node, service, network string) netceptor.Addr {
	addr := netceptor.Addr{}
	addr.SetNode(node)
	addr.SetService(service)
	addr.SetNetwork(network)

	return addr
}

func TestNetceptor_Ping(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name         string
		target       string
		hopsToLive   byte
		expectError  bool
		expectedNode string
	}{
		{
			name:         "successful ping to self",
			target:       "test-node",
			hopsToLive:   64,
			expectError:  false,
			expectedNode: "test-node",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a real Netceptor instance for testing the wrapper method
			nc := netceptor.New(context.Background(), "test-node")

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			// Test the Ping method (which is a simple wrapper around SendPing)
			duration, node, err := nc.Ping(ctx, tt.target, tt.hopsToLive)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// For successful cases, verify duration is reasonable
			if !tt.expectError && duration < 0 {
				t.Errorf("Expected positive duration, got %v", duration)
			}

			// Note: For real integration, we'd need to set up proper network backends
			// For unit testing, we focus on the wrapper behavior and SendPing separately
			_ = node // Used for successful ping validation in integration tests
		})
	}
}

func TestSendPing_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockNetc := mock_netceptor.NewMockNetcForPing(ctrl)
	mockPC := mock_netceptor.NewMockPacketConner(ctrl)

	ctx := context.Background()

	// Setup mock expectations
	mockNetc.EXPECT().ListenPacket("").Return(mockPC, nil)
	mockNetc.EXPECT().NewAddr("target-node", "ping").Return(newTestAddr("target-node", "ping", "netceptor"))
	mockNetc.EXPECT().Context().Return(ctx).AnyTimes()

	// Setup PacketConner mock expectations
	mockPC.EXPECT().SetHopsToLive(byte(64))
	mockPC.EXPECT().SubscribeUnreachable(gomock.Any()).Return(make(chan netceptor.UnreachableNotification))
	mockPC.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(0, nil)
	mockPC.EXPECT().Close().Return(nil)

	// Mock successful read
	mockPC.EXPECT().ReadFrom(gomock.Any()).DoAndReturn(func(p []byte) (int, net.Addr, error) {
		time.Sleep(10 * time.Millisecond) // Simulate network delay
		addr := newTestAddr("target-node", "ping", "")

		return 8, &addr, nil
	})

	duration, fromNode, err := netceptor.SendPing(ctx, mockNetc, "target-node", 64)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if duration <= 0 {
		t.Errorf("Expected positive duration, got: %v", duration)
	}
	if fromNode != "target-node" {
		t.Errorf("Expected fromNode 'target-node', got: %v", fromNode)
	}
}

func TestSendPing_ListenPacketError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockNetc := mock_netceptor.NewMockNetcForPing(ctrl)

	expectedErr := errors.New("listen packet failed")
	mockNetc.EXPECT().ListenPacket("").Return(nil, expectedErr)

	ctx := context.Background()
	duration, fromNode, err := netceptor.SendPing(ctx, mockNetc, "target-node", 64)

	if err != expectedErr {
		t.Errorf("Expected error %v, got: %v", expectedErr, err)
	}
	if duration != 0 {
		t.Errorf("Expected zero duration on error, got: %v", duration)
	}
	if fromNode != "" {
		t.Errorf("Expected empty fromNode on error, got: %v", fromNode)
	}
}

func TestSendPing_WriteError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockNetc := mock_netceptor.NewMockNetcForPing(ctrl)
	mockPC := mock_netceptor.NewMockPacketConner(ctrl)

	expectedErr := errors.New("write failed")
	ctx := context.Background()
	addr := newTestAddr("target-node", "ping", "")

	mockNetc.EXPECT().ListenPacket("").Return(mockPC, nil)
	mockNetc.EXPECT().NewAddr("target-node", "ping").Return(newTestAddr("target-node", "ping", ""))
	mockNetc.EXPECT().NodeID().Return("local-node")
	mockNetc.EXPECT().Context().Return(ctx).AnyTimes()

	mockPC.EXPECT().SetHopsToLive(byte(64))
	mockPC.EXPECT().SubscribeUnreachable(gomock.Any()).Return(make(chan netceptor.UnreachableNotification))
	mockPC.EXPECT().WriteTo(gomock.Any(), gomock.Any()).Return(0, expectedErr)
	mockPC.EXPECT().ReadFrom(gomock.Any()).Return(0, addr, nil).AnyTimes()
	mockPC.EXPECT().Close().Return(nil)

	duration, fromNode, err := netceptor.SendPing(ctx, mockNetc, "target-node", 64)

	if err != expectedErr {
		t.Errorf("Expected error %v, got: %v", expectedErr, err)
	}
	if fromNode != "local-node" {
		t.Errorf("Expected fromNode 'local-node', got: %v", fromNode)
	}
	if duration <= 0 {
		t.Errorf("Expected positive duration even on write error, got: %v", duration)
	}
}

func TestNetceptor_Traceroute(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name        string
		target      string
		expectError bool
	}{
		{
			name:        "basic traceroute",
			target:      "target-node",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a real Netceptor instance for testing the wrapper method
			nc := netceptor.New(context.Background(), "test-node")

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			// Test the Traceroute method (which is a wrapper around CreateTraceroute)
			resultChan := nc.Traceroute(ctx, tt.target)

			// Read first result (should timeout since we don't have real network setup)
			select {
			case result := <-resultChan:
				if result == nil {
					t.Error("Expected non-nil traceroute result")
				}
				// In a real test environment, we'd verify the result content
			case <-time.After(200 * time.Millisecond):
				// Expected for unit test without real network setup
			}
		})
	}
}

func TestCreateTraceroute_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockNetc := mock_netceptor.NewMockNetcForTraceroute(ctrl)

	ctx := context.Background()

	// Mock successful ping responses for multiple hops
	pingResponses := []struct {
		duration time.Duration
		fromNode string
		err      error
	}{
		{10 * time.Millisecond, "hop1", fmt.Errorf(netceptor.ProblemExpiredInTransit)},
		{20 * time.Millisecond, "hop2", fmt.Errorf(netceptor.ProblemExpiredInTransit)},
		{30 * time.Millisecond, "target", nil}, // Final destination
	}

	mockNetc.EXPECT().MaxForwardingHops().Return(byte(30)).AnyTimes()
	mockNetc.EXPECT().Context().Return(ctx).AnyTimes()

	// Expect ping calls for each hop
	for i, resp := range pingResponses {
		mockNetc.EXPECT().Ping(ctx, "target", byte(i)).Return(resp.duration, resp.fromNode, resp.err)
	}

	resultChan := netceptor.CreateTraceroute(ctx, mockNetc, "target")

	// Collect results
	results := make([]*netceptor.TracerouteResult, 0, len(pingResponses))
	for result := range resultChan {
		results = append(results, result)
	}

	// Verify we got the expected number of results
	if len(results) != 3 {
		t.Errorf("Expected 3 traceroute results, got %d", len(results))
	}

	// Verify first hop (expired in transit)
	if results[0].From != "hop1" {
		t.Errorf("Expected first hop from 'hop1', got '%s'", results[0].From)
	}
	if results[0].Time != 10*time.Millisecond {
		t.Errorf("Expected first hop time 10ms, got %v", results[0].Time)
	}
	if results[0].Err != nil {
		t.Errorf("Expected no error for first hop (expired in transit should be filtered), got %v", results[0].Err)
	}

	// Verify second hop (expired in transit)
	if results[1].From != "hop2" {
		t.Errorf("Expected second hop from 'hop2', got '%s'", results[1].From)
	}
	if results[1].Time != 20*time.Millisecond {
		t.Errorf("Expected second hop time 20ms, got %v", results[1].Time)
	}
	if results[1].Err != nil {
		t.Errorf("Expected no error for second hop (expired in transit should be filtered), got %v", results[1].Err)
	}

	// Verify final hop (reached destination)
	if results[2].From != "target" {
		t.Errorf("Expected final hop from 'target', got '%s'", results[2].From)
	}
	if results[2].Time != 30*time.Millisecond {
		t.Errorf("Expected final hop time 30ms, got %v", results[2].Time)
	}
	if results[2].Err != nil {
		t.Errorf("Expected no error for final hop, got %v", results[2].Err)
	}
}

func TestCreateTraceroute_WithError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockNetc := mock_netceptor.NewMockNetcForTraceroute(ctrl)

	ctx := context.Background()

	expectedErr := errors.New("destination unreachable")

	mockNetc.EXPECT().MaxForwardingHops().Return(byte(30)).AnyTimes()
	mockNetc.EXPECT().Context().Return(ctx).AnyTimes()

	// First hop succeeds but gets a real error (not expired in transit)
	mockNetc.EXPECT().Ping(ctx, "target", byte(0)).Return(10*time.Millisecond, "router", expectedErr)

	resultChan := netceptor.CreateTraceroute(ctx, mockNetc, "target")

	// Collect results
	numPings := 1
	results := make([]*netceptor.TracerouteResult, 0, numPings)
	for result := range resultChan {
		results = append(results, result)
	}

	// Should only get one result due to error
	if len(results) != 1 {
		t.Errorf("Expected 1 traceroute result, got %d", len(results))
	}

	// Verify the error result
	if results[0].From != "router" {
		t.Errorf("Expected hop from 'router', got '%s'", results[0].From)
	}
	if results[0].Time != 10*time.Millisecond {
		t.Errorf("Expected hop time 10ms, got %v", results[0].Time)
	}
	if results[0].Err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, results[0].Err)
	}
}

func TestCreateTraceroute_ContextCancelled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockNetc := mock_netceptor.NewMockNetcForTraceroute(ctrl)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	mockNetc.EXPECT().MaxForwardingHops().Return(byte(30))
	mockNetc.EXPECT().Context().Return(context.Background()).AnyTimes()

	// Mock a slow ping that will be cancelled
	mockNetc.EXPECT().Ping(ctx, "target", byte(0)).DoAndReturn(
		func(ctx context.Context, target string, hop byte) (time.Duration, string, error) {
			select {
			case <-ctx.Done():
				return 0, "", ctx.Err()
			case <-time.After(100 * time.Millisecond):
				return 50 * time.Millisecond, "hop1", fmt.Errorf(netceptor.ProblemExpiredInTransit)
			}
		})

	resultChan := netceptor.CreateTraceroute(ctx, mockNetc, "target")

	// Results channel should close due to context cancellation
	numPings := 1
	results := make([]*netceptor.TracerouteResult, 0, numPings)
	for result := range resultChan {
		results = append(results, result)
	}

	// Should get no results or one cancelled result
	if len(results) > 1 {
		t.Errorf("Expected at most 1 result due to cancellation, got %d", len(results))
	}
}

func TestCreateTraceroute_MaxHopsReached(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockNetc := mock_netceptor.NewMockNetcForTraceroute(ctrl)

	ctx := context.Background()

	// Set a low max hops for testing
	maxHops := byte(2)
	mockNetc.EXPECT().MaxForwardingHops().Return(maxHops).AnyTimes()
	mockNetc.EXPECT().Context().Return(ctx).AnyTimes()

	// Mock ping responses that all expire in transit (never reach destination)
	for i := 0; i <= int(maxHops); i++ {
		mockNetc.EXPECT().Ping(ctx, "target", byte(i)).Return(
			time.Duration(10*(i+1))*time.Millisecond,
			fmt.Sprintf("hop%d", i+1),
			fmt.Errorf(netceptor.ProblemExpiredInTransit),
		)
	}

	resultChan := netceptor.CreateTraceroute(ctx, mockNetc, "target")

	// Collect results
	results := make([]*netceptor.TracerouteResult, 0, maxHops+1)
	for result := range resultChan {
		results = append(results, result)
	}

	// Should get maxHops+1 results (0, 1, 2)
	expectedResults := int(maxHops) + 1
	if len(results) != expectedResults {
		t.Errorf("Expected %d traceroute results, got %d", expectedResults, len(results))
	}

	// Verify all results have no errors (expired in transit is filtered)
	for i, result := range results {
		if result.Err != nil {
			t.Errorf("Expected no error for result %d (expired in transit should be filtered), got %v", i, result.Err)
		}
		expectedFrom := fmt.Sprintf("hop%d", i+1)
		if result.From != expectedFrom {
			t.Errorf("Expected result %d from '%s', got '%s'", i, expectedFrom, result.From)
		}
	}
}
