package netceptor_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/netceptor/mock_netceptor"
	"go.uber.org/mock/gomock"
)

func TestNewExternalBackend(t *testing.T) {
	t.Parallel()

	backend, err := netceptor.NewExternalBackend()
	if err != nil {
		t.Errorf("NewExternalBackend() returned unexpected error: %v", err)
	}
	if backend == nil {
		t.Error("NewExternalBackend() returned nil backend")
	}
}

func TestExternalBackendStart(t *testing.T) {
	t.Parallel()

	backend, err := netceptor.NewExternalBackend()
	if err != nil {
		t.Fatalf("Failed to create external backend: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	sessChan, err := backend.Start(ctx, &wg)
	if err != nil {
		t.Errorf("Start() returned unexpected error: %v", err)
	}
	if sessChan == nil {
		t.Error("Start() returned nil session channel")
	}
}

func TestExternalBackendNewConnection(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	backend, err := netceptor.NewExternalBackend()
	if err != nil {
		t.Fatalf("Failed to create external backend: %v", err)
	}

	ctx := context.Background()
	var wg sync.WaitGroup
	sessChan, err := backend.Start(ctx, &wg)
	if err != nil {
		t.Fatalf("Failed to start backend: %v", err)
	}

	// Create a mock MessageConn using the existing mock
	mockConn := mock_netceptor.NewMockMessageConn(ctrl)

	// Start a goroutine to receive the session
	done := make(chan bool)
	go func() {
		select {
		case sess := <-sessChan:
			if sess == nil {
				t.Error("Received nil session from channel")
			}
			done <- true
		case <-time.After(2 * time.Second):
			t.Error("Timeout waiting for session")
			done <- false
		}
	}()

	// Create a new connection
	connCtx := backend.NewConnection(mockConn, true)
	if connCtx == nil {
		t.Error("NewConnection() returned nil context")
	}

	// Wait for the session to be received
	<-done
}
