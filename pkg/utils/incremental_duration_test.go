package utils

import (
	"testing"
)

func TestIncrementalDuration(t *testing.T) {
	delay := NewIncrementalDuration(10, 100, 2.0)
	if delay.Duration != 10 {
		t.Fail()
	}
	delay.NextDelay()
	if delay.Duration != 20 {
		t.Fail()
	}
	delay.Reset()
	if delay.Duration != 10 {
		t.Fail()
	}
	for i := 0; i <= 10; i++ {
		delay.NextDelay()
	}
	if delay.Duration != 100 {
		t.Fail()
	}
}
