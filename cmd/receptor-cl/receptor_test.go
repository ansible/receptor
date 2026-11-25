package main

import (
	"reflect"
	"testing"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		wantArgs []string
		wantIsV2 bool
	}{
		{
			name:     "no flags",
			input:    []string{"receptor", "--node-id", "foo"},
			wantArgs: []string{"receptor", "--node-id", "foo"},
			wantIsV2: false,
		},
		{
			name:     "config-v2 flag present",
			input:    []string{"receptor", "--config-v2", "--node-id", "foo"},
			wantArgs: []string{"receptor", "--node-id", "foo"},
			wantIsV2: true,
		},
		{
			name:     "config-v2 flag only",
			input:    []string{"receptor", "--config-v2"},
			wantArgs: []string{"receptor"},
			wantIsV2: true,
		},
		{
			name:     "multiple config-v2 flags",
			input:    []string{"receptor", "--config-v2", "--config-v2", "--node-id", "foo"},
			wantArgs: []string{"receptor", "--node-id", "foo"},
			wantIsV2: true,
		},
		{
			name:     "empty args",
			input:    []string{},
			wantArgs: []string{},
			wantIsV2: false,
		},
		{
			name:     "config-v2 not a prefix match",
			input:    []string{"receptor", "--config-v2-extra"},
			wantArgs: []string{"receptor", "--config-v2-extra"},
			wantIsV2: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotArgs, gotIsV2 := parseArgs(tt.input)
			if !reflect.DeepEqual(gotArgs, tt.wantArgs) {
				t.Errorf("args = %v, want %v", gotArgs, tt.wantArgs)
			}
			if gotIsV2 != tt.wantIsV2 {
				t.Errorf("isV2 = %v, want %v", gotIsV2, tt.wantIsV2)
			}
		})
	}
}
