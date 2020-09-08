package utils

import (
	"testing"
	"time"
)

func TestIncrementalDuration(t *testing.T) {
	delay := NewIncrementalDuration(1*time.Second, 10*time.Second, 2.0)
	if delay.Duration() != 1*time.Second {
		t.Fail()
	}
	select {
	case <-delay.NextTimeout():
	}
	if delay.Duration() != 2*time.Second {
		t.Fail()
	}
	delay.Reset()
	if delay.Duration() != 1*time.Second {
		t.Fail()
	}
	for i := 0; i <= 10; i++ {
		delay.IncreaseDuration()
	}
	if delay.Duration() != 10*time.Second {
		t.Fail()
	}
}
