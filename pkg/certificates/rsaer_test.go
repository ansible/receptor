package certificates_test

import (
	"crypto/rand"
	"testing"

	"github.com/ansible/receptor/pkg/certificates"
)

func TestRsaWrapperGenerateKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		bits        int
		expectError bool
	}{
		{
			name:        "Successfully generate 2048-bit RSA key",
			bits:        2048,
			expectError: false,
		},
		{
			name:        "Successfully generate 4096-bit RSA key",
			bits:        4096,
			expectError: false,
		},
		{
			name:        "Fail to generate key with invalid bit size",
			bits:        1,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			wrapper := &certificates.RsaWrapper{}
			key, err := wrapper.GenerateKey(rand.Reader, tt.bits)

			if tt.expectError {
				if err == nil {
					t.Error("Expected an error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if key == nil {
					t.Error("Expected a valid key but got nil")
				}
				if key != nil && key.N.BitLen() != tt.bits {
					t.Errorf("Expected key size %d bits, got %d bits", tt.bits, key.N.BitLen())
				}
			}
		})
	}
}
