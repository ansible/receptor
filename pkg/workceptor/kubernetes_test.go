package workceptor

import (
	"os"
	"reflect"
	"testing"
	"time"
)

func Test_shouldUseReconnect(t *testing.T) {
	const envVariable string = "RECEPTOR_KUBE_SUPPORT_RECONNECT"

	tests := []struct {
		name     string
		envValue string
		want     bool
	}{
		{
			name:     "Positive (undefined) test",
			envValue: "",
			want:     true,
		},
		{
			name:     "Enabled test",
			envValue: "enabled",
			want:     true,
		},
		{
			name:     "Disabled test",
			envValue: "disabled",
			want:     false,
		},
		{
			name:     "Auto test",
			envValue: "auto",
			want:     true,
		},
		{
			name:     "Default test",
			envValue: "default",
			want:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(envVariable, tt.envValue)
				defer os.Unsetenv(envVariable)
			} else {
				os.Unsetenv(envVariable)
			}

			if got := shouldUseReconnect(); got != tt.want {
				t.Errorf("shouldUseReconnect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseTime(t *testing.T) {
	type args struct {
		s string
	}

	desiredTimeString := "2024-01-17T00:00:00Z"
	desiredTime, _ := time.Parse(time.RFC3339, desiredTimeString)

	tests := []struct {
		name    string
		args    args
		want    *time.Time
		wantErr bool
	}{
		{
			name: "Positive test",
			args: args{
				s: desiredTimeString,
			},
			want: &desiredTime,
		},
		{
			name: "Error test",
			args: args{
				s: "Invalid time",
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseTime(tt.args.s); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseTime() = %v, want %v", got, tt.want)
			}
		})
	}
}
