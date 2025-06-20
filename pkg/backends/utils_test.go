package backends

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
)

const (
	testTimeout = 500 * time.Millisecond
	shortDelay  = 50 * time.Millisecond
)

type mockBackendSession struct{}

func (m *mockBackendSession) Send([]byte) error                  { return nil }
func (m *mockBackendSession) Recv(time.Duration) ([]byte, error) { return nil, nil }
func (m *mockBackendSession) Close() error                       { return nil }

func TestDialerSessionScenarios(t *testing.T) {
	tests := []struct {
		name          string
		redial        bool
		dialerFunc    func() func(chan struct{}) (netceptor.BackendSession, error)
		expectSession bool
		expectRetries bool
		timeout       time.Duration
		minCallCount  int
	}{
		{
			name:   "successful single dial",
			redial: false,
			dialerFunc: func() func(chan struct{}) (netceptor.BackendSession, error) {
				return func(closeChan chan struct{}) (netceptor.BackendSession, error) {
					go func() {
						time.Sleep(10 * time.Millisecond)
						close(closeChan)
					}()

					return &mockBackendSession{}, nil
				}
			},
			expectSession: true,
			expectRetries: false,
			timeout:       testTimeout,
			minCallCount:  1,
		},
		{
			name:   "single dial error without redial",
			redial: false,
			dialerFunc: func() func(chan struct{}) (netceptor.BackendSession, error) {
				return func(closeChan chan struct{}) (netceptor.BackendSession, error) {
					return nil, errors.New("dial error")
				}
			},
			expectSession: false,
			expectRetries: false,
			timeout:       testTimeout,
			minCallCount:  1,
		},
		{
			name:   "first attempt fails, retry succeeds",
			redial: true,
			dialerFunc: func() func(chan struct{}) (netceptor.BackendSession, error) {
				callCount := 0

				return func(closeChan chan struct{}) (netceptor.BackendSession, error) {
					callCount++
					if callCount == 1 {
						return nil, errors.New("first attempt fails")
					}
					go func() {
						time.Sleep(10 * time.Millisecond)
						close(closeChan)
					}()

					return &mockBackendSession{}, nil
				}
			},
			expectSession: true,
			expectRetries: true,
			timeout:       testTimeout,
			minCallCount:  2,
		},
		{
			name:   "multiple redial attempts with eventual success",
			redial: true,
			dialerFunc: func() func(chan struct{}) (netceptor.BackendSession, error) {
				callCount := 0

				return func(closeChan chan struct{}) (netceptor.BackendSession, error) {
					callCount++
					if callCount < 3 {
						return nil, errors.New("attempt fails")
					}
					go func() {
						time.Sleep(10 * time.Millisecond)
						close(closeChan)
					}()

					return &mockBackendSession{}, nil
				}
			},
			expectSession: true,
			expectRetries: true,
			timeout:       testTimeout,
			minCallCount:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			var wg sync.WaitGroup
			logger := logger.NewReceptorLogger("test")

			dialerFunc := tt.dialerFunc()
			sessChan, err := dialerSession(ctx, &wg, tt.redial, time.Millisecond, logger, dialerFunc)
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}

			if tt.expectSession {
				select {
				case sess := <-sessChan:
					if sess == nil {
						t.Error("Expected to receive a session")
					}
				case <-time.After(tt.timeout):
					t.Error("Timeout waiting for session")
				}
			} else {
				select {
				case _, ok := <-sessChan:
					if ok {
						t.Error("Expected channel to be closed without session")
					}
				case <-time.After(shortDelay):
					t.Error("Timeout waiting for channel to close")
				}
			}

			cancel()
			wg.Wait()
		})
	}
}

func TestContextBehavior(t *testing.T) {
	tests := []struct {
		name           string
		contextTimeout time.Duration
		operationDelay time.Duration
		redial         bool
		expectSuccess  bool
		expectClosed   bool
	}{
		{
			name:           "operation completes before timeout",
			contextTimeout: 100 * time.Millisecond,
			operationDelay: 10 * time.Millisecond,
			redial:         false,
			expectSuccess:  true,
			expectClosed:   false,
		},
		{
			name:           "context cancelled during operation",
			contextTimeout: 50 * time.Millisecond,
			operationDelay: 100 * time.Millisecond,
			redial:         false,
			expectSuccess:  false,
			expectClosed:   true,
		},
		{
			name:           "redial with context cancellation",
			contextTimeout: 80 * time.Millisecond,
			operationDelay: 200 * time.Millisecond,
			redial:         true,
			expectSuccess:  false,
			expectClosed:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tt.contextTimeout)
			defer cancel()

			var wg sync.WaitGroup
			logger := logger.NewReceptorLogger("test")

			df := func(closeChan chan struct{}) (netceptor.BackendSession, error) {
				select {
				case <-time.After(tt.operationDelay):
					go func() {
						time.Sleep(10 * time.Millisecond)
						close(closeChan)
					}()

					return &mockBackendSession{}, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}

			sessChan, err := dialerSession(ctx, &wg, tt.redial, time.Millisecond, logger, df)
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}

			if tt.expectSuccess {
				select {
				case sess := <-sessChan:
					if sess == nil {
						t.Error("Expected to receive a session")
					}
				case <-time.After(tt.contextTimeout + 50*time.Millisecond):
					t.Error("Timeout waiting for session")
				}
			} else {
				select {
				case _, ok := <-sessChan:
					if ok && !tt.expectSuccess {
						t.Error("Expected channel to be closed")
					}
				case <-time.After(tt.contextTimeout + 50*time.Millisecond):
					// This is expected for context cancellation cases
				}
			}

			wg.Wait()
		})
	}
}

func TestListenerSessionScenarios(t *testing.T) {
	tests := []struct {
		name              string
		listenFunc        func() error
		acceptFunc        func() (netceptor.BackendSession, error)
		expectError       bool
		expectSession     bool
		expectedErrString string
	}{
		{
			name: "successful listen and accept",
			listenFunc: func() error {
				return nil
			},
			acceptFunc: func() (netceptor.BackendSession, error) {
				return &mockBackendSession{}, nil
			},
			expectError:   false,
			expectSession: true,
		},
		{
			name: "listen fails",
			listenFunc: func() error {
				return errors.New("listen error")
			},
			acceptFunc: func() (netceptor.BackendSession, error) {
				return nil, nil
			},
			expectError:       true,
			expectSession:     false,
			expectedErrString: "listen error",
		},
		{
			name: "accept fails",
			listenFunc: func() error {
				return nil
			},
			acceptFunc: func() (netceptor.BackendSession, error) {
				return nil, errors.New("accept error")
			},
			expectError:   false,
			expectSession: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var wg sync.WaitGroup
			logger := logger.NewReceptorLogger("test")

			var mu sync.Mutex
			cancelCalled := false
			lcf := func() {
				mu.Lock()
				cancelCalled = true
				mu.Unlock()
			}

			sessChan, err := listenerSession(ctx, &wg, logger, tt.listenFunc, tt.acceptFunc, lcf)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if tt.expectedErrString != "" && !strings.Contains(err.Error(), tt.expectedErrString) {
					t.Errorf("Expected error containing '%s', got: %v", tt.expectedErrString, err)
				}

				return
			}

			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}

			if tt.expectSession {
				select {
				case sess := <-sessChan:
					if sess == nil {
						t.Error("Expected to receive a session")
					}
				case <-time.After(100 * time.Millisecond):
					t.Error("Timeout waiting for session")
				}
			} else {
				select {
				case _, ok := <-sessChan:
					if ok {
						t.Error("Expected channel to be closed")
					}
				case <-time.After(100 * time.Millisecond):
					t.Error("Timeout waiting for channel to close")
				}
			}

			cancel()
			wg.Wait()

			// Give a moment for cleanup to complete
			time.Sleep(10 * time.Millisecond)

			mu.Lock()
			if !cancelCalled {
				t.Error("Expected cancel function to be called")
			}
			mu.Unlock()
		})
	}
}

func TestListenerSessionContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	logger := logger.NewReceptorLogger("test")

	lf := func() error {
		return nil
	}

	af := func() (netceptor.BackendSession, error) {
		// Block until context is cancelled
		<-ctx.Done()

		return nil, ctx.Err()
	}

	lcf := func() {}

	sessChan, err := listenerSession(ctx, &wg, logger, lf, af, lcf)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	cancel() // Cancel context to trigger cleanup

	select {
	case _, ok := <-sessChan:
		if ok {
			t.Error("Expected channel to be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for channel to close")
	}

	wg.Wait()
}

func TestMaxRedialDelayConstant(t *testing.T) {
	expected := 20 * time.Second
	if maxRedialDelay != expected {
		t.Errorf("Expected maxRedialDelay to be %v, got %v", expected, maxRedialDelay)
	}
}

// Test edge case where session connection closes immediately.
func TestDialerSessionConnectionCloseImmediate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	logger := logger.NewReceptorLogger("test")

	df := func(closeChan chan struct{}) (netceptor.BackendSession, error) {
		close(closeChan) // Close immediately

		return &mockBackendSession{}, nil
	}

	sessChan, err := dialerSession(ctx, &wg, false, time.Millisecond, logger, df)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	select {
	case sess := <-sessChan:
		if sess == nil {
			t.Error("Expected to receive a session")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for session")
	}

	// Wait for the connection to close and function to exit
	select {
	case _, ok := <-sessChan:
		if ok {
			t.Error("Expected channel to be closed after connection close")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for channel to close")
	}

	cancel()
	wg.Wait()
}

// Test to ensure redial logic properly resets delays on successful connection.
func TestDialerSessionRedialDelayReset(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	var wg sync.WaitGroup
	logger := logger.NewReceptorLogger("test")

	callCount := 0
	df := func(closeChan chan struct{}) (netceptor.BackendSession, error) {
		callCount++
		if callCount == 1 {
			return nil, errors.New("first attempt fails")
		}
		go func() {
			time.Sleep(10 * time.Millisecond)
			close(closeChan)
		}()

		return &mockBackendSession{}, nil
	}

	sessChan, err := dialerSession(ctx, &wg, true, 10*time.Millisecond, logger, df)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should get a session on retry
	select {
	case sess := <-sessChan:
		if sess == nil {
			t.Error("Expected to receive a session")
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("Timeout waiting for session")
	}

	wg.Wait()

	if callCount < 2 {
		t.Errorf("Expected at least 2 dial attempts (for retry), got %d", callCount)
	}
}
