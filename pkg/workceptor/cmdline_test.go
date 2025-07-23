//go:build !no_workceptor
// +build !no_workceptor

package workceptor

import (
	"testing"

	"github.com/ghjm/cmdline"
)

func TestWorkersSection(t *testing.T) {
	if workersSection == nil {
		t.Fatal("workersSection should not be nil")
	}

	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{
			name:     "description",
			got:      workersSection.Description,
			expected: "Commands to configure workers that process units of work:",
		},
		{
			name:     "order",
			got:      workersSection.Order,
			expected: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, tt.got)
			}
		})
	}

	// Type check as separate subtest
	t.Run("type check", func(t *testing.T) {
		var section interface{} = workersSection
		if _, ok := section.(*cmdline.ConfigSection); !ok {
			t.Error("workersSection should be of type *cmdline.ConfigSection")
		}
	})
}
