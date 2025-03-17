package tickrunner

import (
	"context"
	"sync"
	"testing"
	"time"
)

// SafeCounter is safe to use concurrently.
type SafeCounter struct {
	mu sync.Mutex
	v  int
}

// Inc increments the counter for the given key.
func (c *SafeCounter) Inc() {
	c.mu.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	c.v++
	c.mu.Unlock()
}

// Value returns the current value of the counter for the given key.
func (c *SafeCounter) Value() int {
	c.mu.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	defer c.mu.Unlock()

	return c.v
}

func TestRun(t *testing.T) {
	type testCase struct {
		name             string
		requestTime      int
		requestCount     int
		periodicInterval time.Duration
		defaultReqDelay  time.Duration
		expectedRunCount int
		/* This value must take into account the function's algorithm
		and the other two time.Duration values */
		waitForBeforeChecks time.Duration
	}

	tests := []testCase{
		{
			name:             "Run only with default periodicInterval once",
			periodicInterval: time.Duration(2) * time.Second,
			// Run can only execute one request with the set periodicInterval.
			expectedRunCount:    1,
			waitForBeforeChecks: time.Duration(3) * time.Second,
		},
		{
			name:             "Run only with default periodicInterval more than once",
			periodicInterval: time.Duration(2) * time.Second,
			// Run can only execute one request with the set periodicInterval.
			expectedRunCount:    3,
			waitForBeforeChecks: time.Duration(7) * time.Second,
		},
		{
			name:         "Run request inmediately",
			requestCount: 1,
			requestTime:  0,
			/* Setting this to a high value so it doesn't run at all
			with the default periodicInterval. We only want to test
			for the requests sent. */
			periodicInterval: time.Duration(300) * time.Second,
			defaultReqDelay:  time.Duration(1) * time.Second,
			// Run can only execute one request with the set periodicInterval.
			expectedRunCount:    1,
			waitForBeforeChecks: time.Duration(2) * time.Second,
		},
		{
			name:         "Run sending some requests overrides default periodicInterval",
			requestCount: 3,
			requestTime:  2,
			/* Setting this to a high value so it doesn't run at all
			with the default periodicInterval. We only want to test
			for the requests sent. */
			periodicInterval: time.Duration(300) * time.Second,
			defaultReqDelay:  time.Duration(1) * time.Second,
			/* Due to the design of the test itself, the requests get sent
			into the channel back to back with no time in between them
			This expectedRunCount is the correct value since only the
			first one will be run. */
			expectedRunCount:    1,
			waitForBeforeChecks: time.Duration(3) * time.Second,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, ctxCancel := context.WithCancel(context.Background())
			defer ctxCancel()

			/* Create a counter value and increment function to keep track of
			inner function calls count. */
			runCounter := SafeCounter{}
			runFunction := func() { runCounter.Inc() }
			runChan := Run(ctx, runFunction, tc.periodicInterval, tc.defaultReqDelay)

			/* Send the required requests through the returned channel
			These are inmediate back-to-back requests
			Following the function's logic, if there are too many requests coming in
			simultaneously, these should be batched since only the oldest one is taken
			into account before the set time passes. */
			for i := 0; i < tc.requestCount; i++ {
				runChan <- time.Duration(tc.requestTime) * time.Second
			}

			/* Since this function is time-based, lets wait for a calculated time
			before asserting any value so we avoid race conditions with
			non-blocking code. */
			time.Sleep(tc.waitForBeforeChecks)
			if runCounter.Value() != tc.expectedRunCount {
				t.Errorf("Run count: %d, Expected number of runs: %d", runCounter.Value(), tc.expectedRunCount)
			}
		})
	}
}
