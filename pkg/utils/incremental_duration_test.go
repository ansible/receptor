package utils

import (
	"testing"
	"time"
)

const newIncrementalDurationMessage string = "NewIncrementalDuration() = %v, want %v"

func TestNewIncrementalDuration(t *testing.T) {
	type args struct {
		duration    time.Duration
		maxDuration time.Duration
		multiplier  float64
	}

	tests := []struct {
		name string
		args args
		want time.Duration
	}{
		{
			name: "NewIncrementalDuration1",
			args: args{
				duration:    1 * time.Second,
				maxDuration: 10 * time.Second,
				multiplier:  2.0,
			},
			want: 1 * time.Second,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewIncrementalDuration(tt.args.duration, tt.args.maxDuration, tt.args.multiplier); got.duration != tt.want {
				t.Errorf(newIncrementalDurationMessage, got, tt.want)
			}
		})
	}
}

func TestIncrementalDurationReset(t *testing.T) {
	delay := NewIncrementalDuration(1*time.Second, 10*time.Second, 2.0)
	want1 := 1 * time.Second
	if delay.duration != want1 {
		t.Errorf(newIncrementalDurationMessage, delay.duration, want1)
	}
	<-delay.NextTimeout()

	want2 := 2 * time.Second
	if delay.duration != want2 {
		t.Errorf(newIncrementalDurationMessage, delay.duration, want2)
	}
	delay.Reset()
	if delay.duration != want1 {
		t.Errorf("Reset() = %v, want %v", delay.duration, want1)
	}
}

func TestIncrementalDurationincreaseDuration(t *testing.T) {
	delay := NewIncrementalDuration(1*time.Second, 10*time.Second, 2.0)
	for i := 0; i <= 10; i++ {
		delay.increaseDuration()
	}
	want10 := 10 * time.Second
	if delay.duration != want10 {
		t.Errorf("increaseDuration() = %v, want %v", delay.duration, want10)
	}
}

func TestIncrementalDurationNextTimeout(t *testing.T) {
	delay := NewIncrementalDuration(1*time.Second, 10*time.Second, 2.0)
	want1 := 1 * time.Second
	if delay.duration != want1 {
		t.Errorf(newIncrementalDurationMessage, delay.duration, want1)
	}
	<-delay.NextTimeout()

	want2 := 2 * time.Second
	if delay.duration != want2 {
		t.Errorf("NextTimeout() = %v, want %v", delay.duration, want2)
	}
}
