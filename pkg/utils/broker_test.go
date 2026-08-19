package utils_test

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/utils"
)

// TestBrokerSubscribe tests the Subscribe method of the Broker.
func TestBrokerSubscribe(t *testing.T) {
	type testCase struct {
		name           string
		contextTimeout time.Duration
		cancelContext  bool
		expectNil      bool
	}

	tests := []testCase{
		{
			name:           "Subscribe with active context",
			contextTimeout: 1 * time.Second,
			cancelContext:  false,
			expectNil:      false,
		},
		{
			name:           "Subscribe with canceled context",
			contextTimeout: 1 * time.Second,
			cancelContext:  true,
			expectNil:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tt.contextTimeout)
			defer cancel()

			broker := utils.NewBroker(ctx, reflect.TypeOf(""))

			if tt.cancelContext {
				cancel()
				// Give some time for the cancellation to propagate
				time.Sleep(10 * time.Millisecond)
			}

			ch := broker.Subscribe()
			if (ch == nil) != tt.expectNil {
				t.Errorf("Subscribe() returned nil: %v, expected nil: %v", ch == nil, tt.expectNil)
			}
		})
	}
}

// TestBrokerUnsubscribe tests the Unsubscribe method of the Broker.
func TestBrokerUnsubscribe(t *testing.T) {
	type testCase struct {
		name           string
		contextTimeout time.Duration
		cancelContext  bool
	}

	tests := []testCase{
		{
			name:           "Unsubscribe with active context",
			contextTimeout: 1 * time.Second,
			cancelContext:  false,
		},
		{
			name:           "Unsubscribe with canceled context",
			contextTimeout: 1 * time.Second,
			cancelContext:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tt.contextTimeout)
			defer cancel()

			broker := utils.NewBroker(ctx, reflect.TypeOf(""))
			ch := broker.Subscribe()

			if tt.cancelContext {
				cancel()
				// Give some time for the cancellation to propagate
				time.Sleep(10 * time.Millisecond)
			}

			// This should not panic regardless of context state
			broker.Unsubscribe(ch)
		})
	}
}

// TestBrokerPublish tests the Publish method of the Broker.
func TestBrokerPublish(t *testing.T) {
	type testCase struct {
		name           string
		msgType        reflect.Type
		msg            interface{}
		expectError    bool
		contextTimeout time.Duration
		cancelContext  bool
	}

	tests := []testCase{
		{
			name:           "Publish string message with string broker",
			msgType:        reflect.TypeOf(""),
			msg:            "test message",
			expectError:    false,
			contextTimeout: 1 * time.Second,
			cancelContext:  false,
		},
		{
			name:           "Publish int message with string broker",
			msgType:        reflect.TypeOf(""),
			msg:            123,
			expectError:    true,
			contextTimeout: 1 * time.Second,
			cancelContext:  false,
		},
		{
			name:           "Publish with canceled context",
			msgType:        reflect.TypeOf(""),
			msg:            "test message",
			expectError:    false, // Publish doesn't return an error when context is canceled
			contextTimeout: 1 * time.Second,
			cancelContext:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tt.contextTimeout)
			defer cancel()

			broker := utils.NewBroker(ctx, tt.msgType)

			if tt.cancelContext {
				cancel()
				// Give some time for the cancellation to propagate
				time.Sleep(10 * time.Millisecond)
			}

			err := broker.Publish(tt.msg)
			if (err != nil) != tt.expectError {
				t.Errorf("Publish() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

// TestBrokerEndToEnd tests the full publish-subscribe workflow.
func TestBrokerEndToEnd(t *testing.T) {
	type testCase struct {
		name           string
		numSubscribers int
		numMessages    int
		contextTimeout time.Duration
	}

	tests := []testCase{
		{
			name:           "Single subscriber, single message",
			numSubscribers: 1,
			numMessages:    1,
			contextTimeout: 1 * time.Second,
		},
		{
			name:           "Multiple subscribers, single message",
			numSubscribers: 5,
			numMessages:    1,
			contextTimeout: 1 * time.Second,
		},
		{
			name:           "Single subscriber, multiple messages",
			numSubscribers: 1,
			numMessages:    5,
			contextTimeout: 1 * time.Second,
		},
		{
			name:           "Multiple subscribers, multiple messages",
			numSubscribers: 5,
			numMessages:    5,
			contextTimeout: 1 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tt.contextTimeout)
			defer cancel()

			broker := utils.NewBroker(ctx, reflect.TypeOf(""))

			// Create subscribers
			var subscribers []chan interface{}
			for i := 0; i < tt.numSubscribers; i++ {
				ch := broker.Subscribe()
				if ch == nil {
					t.Fatalf("Subscribe() returned nil")
				}
				subscribers = append(subscribers, ch)
			}

			// Create a WaitGroup to ensure all messages are received
			var wg sync.WaitGroup

			// Set up message reception tracking
			receivedMsgs := make([][]string, tt.numSubscribers)
			for i := range receivedMsgs {
				receivedMsgs[i] = make([]string, 0, tt.numMessages)
			}

			// Start goroutines to collect messages from each subscriber
			for i, ch := range subscribers {
				wg.Add(1)
				go func(idx int, ch chan interface{}) {
					defer wg.Done()
					for j := 0; j < tt.numMessages; j++ {
						select {
						case msg := <-ch:
							if msgStr, ok := msg.(string); ok {
								receivedMsgs[idx] = append(receivedMsgs[idx], msgStr)
							} else {
								t.Errorf("Subscriber %d received non-string message: %v", idx, msg)
							}
						case <-time.After(500 * time.Millisecond):
							t.Errorf("Subscriber %d timed out waiting for message %d", idx, j)

							return
						}
					}
				}(i, ch)
			}

			// Send messages
			messages := make([]string, tt.numMessages)
			for i := 0; i < tt.numMessages; i++ {
				messages[i] = "test message " + string(rune('A'+i))
				err := broker.Publish(messages[i])
				if err != nil {
					t.Fatalf("Publish() error = %v", err)
				}
				// Small delay to ensure message processing
				time.Sleep(5 * time.Millisecond)
			}

			// Wait for all subscribers to receive all messages
			wg.Wait()

			// Verify all subscribers received all messages
			for i, received := range receivedMsgs {
				if len(received) != tt.numMessages {
					t.Errorf("Subscriber %d received %d messages, expected %d", i, len(received), tt.numMessages)

					continue
				}

				// Check each message
				for j, msg := range received {
					if j < len(messages) && msg != messages[j] {
						t.Errorf("Subscriber %d received message %d = %v, want %v", i, j, msg, messages[j])
					}
				}
			}

			// Unsubscribe all
			for _, ch := range subscribers {
				broker.Unsubscribe(ch)
			}
		})
	}
}

// TestBrokerContextCancellation tests that the broker properly handles context cancellation.
func TestBrokerContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	broker := utils.NewBroker(ctx, reflect.TypeOf(""))

	// Subscribe before cancellation
	ch := broker.Subscribe()
	if ch == nil {
		t.Fatalf("Subscribe() returned nil")
	}

	// Cancel context
	cancel()
	time.Sleep(10 * time.Millisecond) // Give time for cancellation to propagate

	// Verify channel is closed
	select {
	case _, ok := <-ch:
		if ok {
			t.Errorf("Channel should be closed after context cancellation")
		}
	case <-time.After(100 * time.Millisecond):
		t.Errorf("Timed out waiting for channel to close")
	}

	// Verify Subscribe returns nil after cancellation
	ch2 := broker.Subscribe()
	if ch2 != nil {
		t.Errorf("Subscribe() should return nil after context cancellation")
	}
}

// TestBrokerConcurrency tests the broker under concurrent operations.
func TestBrokerConcurrency(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	broker := utils.NewBroker(ctx, reflect.TypeOf(""))

	const numSubscribers = 10
	const numPublishers = 5
	const messagesPerPublisher = 20

	// Use a mutex to protect the counter
	var mu sync.Mutex
	receivedMessages := make([]int, numSubscribers)

	// WaitGroup for subscribers
	var subWg sync.WaitGroup

	// WaitGroup for publishers
	var pubWg sync.WaitGroup

	// Create subscribers
	subscribers := make([]chan interface{}, numSubscribers)
	for i := 0; i < numSubscribers; i++ {
		subscribers[i] = broker.Subscribe()
		if subscribers[i] == nil {
			t.Fatalf("Subscribe() returned nil")
		}

		// Start a goroutine to read messages
		subWg.Add(1)
		go func(idx int, ch chan interface{}) {
			defer subWg.Done()
			for {
				select {
				case _, ok := <-ch:
					if !ok {
						return
					}
					mu.Lock()
					receivedMessages[idx]++
					mu.Unlock()
				case <-ctx.Done():
					return
				}
			}
		}(i, subscribers[i])
	}

	// Create publishers
	for i := 0; i < numPublishers; i++ {
		pubWg.Add(1)
		go func(idx int) {
			defer pubWg.Done()
			for j := 0; j < messagesPerPublisher; j++ {
				msg := "message from publisher " + string(rune('A'+idx)) + " #" + string(rune('0'+j)) //nolint:gosec // G115: idx and j are small loop counters, no overflow risk
				err := broker.Publish(msg)
				if err != nil {
					t.Errorf("Publish() error = %v", err)
				}
				// Small delay to simulate work
				time.Sleep(2 * time.Millisecond)
			}
		}(i)
	}

	// Wait for publishers to finish
	pubWg.Wait()

	// Give subscribers time to process all messages
	time.Sleep(100 * time.Millisecond)

	// Unsubscribe all
	for _, ch := range subscribers {
		broker.Unsubscribe(ch)
	}

	// Wait for subscribers to finish
	subWg.Wait()

	// Verify each subscriber received the expected number of messages
	expectedMessages := numPublishers * messagesPerPublisher
	for i, count := range receivedMessages {
		if count != expectedMessages {
			t.Errorf("Subscriber %d received %d messages, expected %d", i, count, expectedMessages)
		}
	}
}
