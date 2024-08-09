package utils_test

import (
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/utils"
)

const newIncrementalDurationMessage string = "NewIncrementalDuration() = %v, want %v"

func TestNewIncrementalDuration(t *testing.T) {
	type args struct {
		Duration    time.Duration
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
				Duration:    1 * time.Second,
				maxDuration: 10 * time.Second,
				multiplier:  2.0,
			},
			want: 1 * time.Second,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := utils.NewIncrementalDuration(tt.args.Duration, tt.args.maxDuration, tt.args.multiplier); got.Duration != tt.want {
				t.Errorf(newIncrementalDurationMessage, got, tt.want)
			}
		})
	}
}

func TestIncrementalDurationReset(t *testing.T) {
	delay := utils.NewIncrementalDuration(1*time.Second, 10*time.Second, 2.0)
	want1 := 1 * time.Second
	if delay.Duration != want1 {
		t.Errorf(newIncrementalDurationMessage, delay.Duration, want1)
	}
	<-delay.NextTimeout()

	want2 := 2 * time.Second
	if delay.Duration != want2 {
		t.Errorf(newIncrementalDurationMessage, delay.Duration, want2)
	}
	delay.Reset()
	if delay.Duration != want1 {
		t.Errorf("Reset() = %v, want %v", delay.Duration, want1)
	}
}

func TestIncrementalDurationincreaseDuration(t *testing.T) {
	delay := utils.NewIncrementalDuration(1*time.Second, 10*time.Second, 2.0)
	for i := 0; i <= 10; i++ {
		delay.IncreaseDuration()
	}
	want10 := 10 * time.Second
	if delay.Duration != want10 {
		t.Errorf("increaseDuration() = %v, want %v", delay.Duration, want10)
	}
}

func TestIncrementalDurationNextTimeout(t *testing.T) {
	delay := utils.NewIncrementalDuration(1*time.Second, 10*time.Second, 2.0)
	want1 := 1 * time.Second
	if delay.Duration != want1 {
		t.Errorf(newIncrementalDurationMessage, delay.Duration, want1)
	}
	<-delay.NextTimeout()

	want2 := 2 * time.Second
	if delay.Duration != want2 {
		t.Errorf("NextTimeout() = %v, want %v", delay.Duration, want2)
	}
}
