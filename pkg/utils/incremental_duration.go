package utils

import (
	"math"
	"time"
)

// IncrementalDuration handles a time.Duration with max limits
type IncrementalDuration struct {
	Duration        time.Duration
	InitialDuration time.Duration
	MaxDuration     time.Duration
	Multiplier      float64
}

// NewIncrementalDuration returns an IncrementalDuration object with initialized values
func NewIncrementalDuration(duration, maxDuration time.Duration, multiplier float64) *IncrementalDuration {
	return &IncrementalDuration{
		Duration:        duration,
		InitialDuration: duration,
		MaxDuration:     maxDuration,
		Multiplier:      multiplier,
	}
}

// NextDelay increases a time.Duration by a multiplier, up to a provided max
func (ID *incrementalDuration) NextDelay() {
	ID.Duration = time.Duration(math.Min(ID.Multiplier*float64(ID.Duration), float64(ID.MaxDuration)))
}

// Reset sets current duration to initial duration
func (ID *incrementalDuration) Reset() {
	ID.Duration = ID.InitialDuration
}
