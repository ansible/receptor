package certificates_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/ansible/receptor/pkg/certificates"
)

func TestOsWrapperReadFile(t *testing.T) {
	t.Parallel()

	// Create a temporary file for testing
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("test content")

	err := os.WriteFile(testFile, testContent, 0o600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name          string
		filename      string
		expectError   bool
		expectContent []byte
	}{
		{
			name:          "Successfully read existing file",
			filename:      testFile,
			expectError:   false,
			expectContent: testContent,
		},
		{
			name:        "Fail to read non-existent file",
			filename:    filepath.Join(tmpDir, "nonexistent.txt"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			wrapper := &certificates.OsWrapper{}
			content, err := wrapper.ReadFile(tt.filename)

			if tt.expectError {
				if err == nil {
					t.Error("Expected an error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if string(content) != string(tt.expectContent) {
					t.Errorf("Expected content %q, got %q", tt.expectContent, content)
				}
			}
		})
	}
}

func TestOsWrapperWriteFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		filename    string
		content     []byte
		perm        fs.FileMode
		expectError bool
	}{
		{
			name:        "Successfully write file",
			filename:    filepath.Join(tmpDir, "write_test.txt"),
			content:     []byte("test write content"),
			perm:        0o644,
			expectError: false,
		},
		{
			name:        "Write to invalid directory",
			filename:    "/invalid/path/that/does/not/exist/file.txt",
			content:     []byte("test content"),
			perm:        0o644,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			wrapper := &certificates.OsWrapper{}
			err := wrapper.WriteFile(tt.filename, tt.content, tt.perm)

			if tt.expectError {
				if err == nil {
					t.Error("Expected an error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				// Verify the file was written correctly
				content, readErr := os.ReadFile(tt.filename)
				if readErr != nil {
					t.Errorf("Failed to read written file: %v", readErr)
				}
				if string(content) != string(tt.content) {
					t.Errorf("Expected content %q, got %q", tt.content, content)
				}
			}
		})
	}
}
