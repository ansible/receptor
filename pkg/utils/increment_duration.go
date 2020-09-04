package utils

import (
	"math"
	"time"
)

// IncrementalDuration increases a time.Duration instance up to MaxDuration
type IncrementalDuration struct {
	Duration    time.Duration
	MaxDuration time.Duration
	Multiplier  float64
}

// NextDelay increases a time.Duration by a multiplier, up to a provided max
func (ID *IncrementalDuration) NextDelay() {
	ID.Duration = time.Duration(math.Min(ID.Multiplier*float64(ID.Duration), float64(ID.MaxDuration)))
}
