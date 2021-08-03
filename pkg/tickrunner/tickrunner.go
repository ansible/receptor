package tickrunner

import (
	"context"
	"time"
)

// Run runs a task at a given periodic interval, or as requested over a channel.
// If many requests come in close to the same time, only run the task once.
// Callers can ask for the task to be run within a given amount of time, which
// overrides defaultReqDelay. Sending a zero to the channel runs it immediately.
func Run(ctx context.Context, f func(), periodicInterval time.Duration, defaultReqDelay time.Duration) chan time.Duration {
	runChan := make(chan time.Duration)
	go func() {
		nextRunTime := time.Now().Add(periodicInterval)
		for {
			select {
			case <-time.After(time.Until(nextRunTime)):
				nextRunTime = time.Now().Add(periodicInterval)
				f()
			case req := <-runChan:
				proposedTime := time.Now()
				if req == 0 {
					proposedTime = proposedTime.Add(defaultReqDelay)
				} else {
					proposedTime = proposedTime.Add(req)
				}
				if proposedTime.Before(nextRunTime) {
					nextRunTime = proposedTime
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return runChan
}
