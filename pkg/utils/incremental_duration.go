package utils

import (
	"math"
	"time"
)

// IncrementalDuration handles a time.Duration with max limits.
type IncrementalDuration struct {
	Duration        time.Duration
	InitialDuration time.Duration
	MaxDuration     time.Duration
	multiplier      float64
}

// NewIncrementalDuration returns an IncrementalDuration object with initialized values.
func NewIncrementalDuration(duration, maxDuration time.Duration, multiplier float64) *IncrementalDuration {
	return &IncrementalDuration{
		Duration:        duration,
		InitialDuration: duration,
		MaxDuration:     maxDuration,
		multiplier:      multiplier,
	}
}

// Reset sets current duration to initial duration.
func (id *IncrementalDuration) Reset() {
	id.Duration = id.InitialDuration
}

func (id *IncrementalDuration) IncreaseDuration() {
	id.Duration = time.Duration(math.Min(id.multiplier*float64(id.Duration), float64(id.MaxDuration)))
}

// NextTimeout returns a timeout channel based on current duration.
func (id *IncrementalDuration) NextTimeout() <-chan time.Time {
	ch := time.After(id.Duration)
	id.IncreaseDuration()

	return ch
}
