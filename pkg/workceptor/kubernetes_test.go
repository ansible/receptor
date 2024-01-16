package workceptor

import (
	"testing"
)

func Test_shouldUseReconnect(t *testing.T) {
	type args struct {
		kw *kubeUnit
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldUseReconnect(tt.args.kw); got != tt.want {
				t.Errorf("shouldUseReconnect() = %v, want %v", got, tt.want)
			}
		})
	}
}
