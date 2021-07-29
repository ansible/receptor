package utils

import (
	"math"
	"time"
)

// IncrementalDuration handles a time.Duration with max limits
type IncrementalDuration struct {
	duration        time.Duration
	initialDuration time.Duration
	maxDuration     time.Duration
	multiplier      float64
}

// NewIncrementalDuration returns an IncrementalDuration object with initialized values
func NewIncrementalDuration(duration, maxDuration time.Duration, multiplier float64) *IncrementalDuration {
	return &IncrementalDuration{
		duration:        duration,
		initialDuration: duration,
		maxDuration:     maxDuration,
		multiplier:      multiplier,
	}
}

// Reset sets current duration to initial duration
func (id *IncrementalDuration) Reset() {
	id.duration = id.initialDuration
}

func (id *IncrementalDuration) increaseDuration() {
	id.duration = time.Duration(math.Min(id.multiplier*float64(id.duration), float64(id.maxDuration)))
}

// NextTimeout returns a timeout channel based on current duration
func (id *IncrementalDuration) NextTimeout() <-chan time.Time {
	ch := time.After(id.duration)
	id.increaseDuration()
	return ch
}
