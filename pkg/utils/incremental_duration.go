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
func (ID *IncrementalDuration) Reset() {
	ID.duration = ID.initialDuration
}

func (ID *IncrementalDuration) increaseDuration() {
	ID.duration = time.Duration(math.Min(ID.multiplier*float64(ID.duration), float64(ID.maxDuration)))
}

// NextTimeout returns a timeout channel based on current duration
func (ID *IncrementalDuration) NextTimeout() <-chan time.Time {
	ch := time.After(ID.duration)
	ID.increaseDuration()
	return ch
}
