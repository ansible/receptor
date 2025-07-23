package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateVersion(t *testing.T) {
	type test struct {
		name     string
		input    string
		expected string
	}

	for _, tt := range []*test{
		{
			name:     "version is empty string",
			input:    "",
			expected: "Version unknown",
		},
		{
			name:     "version is zero",
			input:    "0",
			expected: "0",
		},
		{
			name:     "version is one",
			input:    "1",
			expected: "1",
		},
	} {
		Version = tt.input
		output := validateVersion()
		assert.Equal(t, tt.expected, output)
	}
}
