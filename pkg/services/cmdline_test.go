package services

import (
	"testing"

	"github.com/ghjm/cmdline"
)

func TestServicesSection(t *testing.T) {
	if servicesSection == nil {
		t.Fatal("servicesSection should not be nil")
	}

	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{
			name:     "description",
			got:      servicesSection.Description,
			expected: "Commands to configure services that run on top of the Receptor mesh:",
		},
		{
			name:     "order",
			got:      servicesSection.Order,
			expected: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, tt.got)
			}
		})
	}

	// Type check as separate subtest.
	t.Run("type check", func(t *testing.T) {
		var section interface{} = servicesSection
		if _, ok := section.(*cmdline.ConfigSection); !ok {
			t.Error("servicesSection should be of type *cmdline.ConfigSection")
		}
	})
}
