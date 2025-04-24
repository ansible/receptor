package cmd

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
)

func TestInitConfig(t *testing.T) {
	// Save the original cfgFile value
	originalCfgFile := cfgFile
	defer func() {
		cfgFile = originalCfgFile
	}()

	// Test with a specific config file
	tmpfile, err := os.CreateTemp("", "receptor-config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	configContent := `
node:
  id: test-node
  data-dir: /tmp/test-receptor
`
	if _, err := tmpfile.Write([]byte(configContent)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Set the config file
	cfgFile = tmpfile.Name()

	// Call initConfig
	initConfig()

	// Test with no config file (should use default)
	cfgFile = ""
	initConfig()
}

func TestExecute(t *testing.T) {
	// Save the original rootCmd
	originalRootCmd := rootCmd
	defer func() {
		rootCmd = originalRootCmd
	}()

	// Create a mock rootCmd that doesn't exit on error
	rootCmd = &cobra.Command{
		Use:   "test",
		Short: "Test command",
		Run: func(cmd *cobra.Command, args []string) {
			// Do nothing
		},
	}

	// Call Execute
	Execute()
}

func TestHandleRootCommand(t *testing.T) {
	// Skip this test for now as it calls os.Exit
	t.Skip("Skipping TestHandleRootCommand as it calls os.Exit")
}

func TestReloadServices(t *testing.T) {
	// Skip this test for now as it requires more complex setup
	t.Skip("Skipping TestReloadServices as it requires more complex setup")
}
