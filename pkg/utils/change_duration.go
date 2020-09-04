package utils

import (
	"math"
	"time"
)

// IncreaseDuration increases a time.Duration by a factor, up to a provided max
func IncreaseDuration(duration time.Duration, maxDuration time.Duration, factor float64) time.Duration {
	return time.Duration(math.Min(1.5*float64(duration), float64(maxDuration)))
}
