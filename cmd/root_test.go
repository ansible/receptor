package cmd

import (
	"errors"
	"os"
	"reflect"
	"testing"
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
	// Skip this test for now as it requires cobra import
	t.Skip("Skipping TestExecute as it requires cobra import")
}

func TestHandleRootCommand(t *testing.T) {
	// Skip this test for now as it calls os.Exit
	t.Skip("Skipping TestHandleRootCommand as it calls os.Exit")
}

// mockReloadService is a mock service that implements the Reloader interface.
type mockReloadService struct {
	reloadCalled bool
	reloadError  error
}

// Reload implements the Reloader interface.
func (m *mockReloadService) Reload() error {
	m.reloadCalled = true

	return m.reloadError
}

// mockNonReloadService is a mock service that does NOT implement the Reloader interface.
type mockNonReloadService struct {
	value string
}

// testReloadConfig is a test config struct with various field types.
type testReloadConfig struct {
	ReloadableService    *mockReloadService
	ReloadableServices   []*mockReloadService
	NonReloadableService *mockNonReloadService
	EmptyField           *mockReloadService
	StringField          string
}

func TestReloadItem(t *testing.T) {
	t.Run("reloads service that implements Reloader", func(t *testing.T) {
		service := &mockReloadService{}
		reloadItem(service, "TestType")

		if !service.reloadCalled {
			t.Error("Expected Reload to be called")
		}
	})

	t.Run("does nothing for service that does not implement Reloader", func(t *testing.T) {
		service := &mockNonReloadService{value: "test"}
		// Should not panic or error
		reloadItem(service, "TestType")
	})

	t.Run("handles reload error gracefully", func(t *testing.T) {
		service := &mockReloadService{
			reloadError: errors.New("reload failed"),
		}
		// Should not panic - error is logged via PrintPhaseErrorMessage
		reloadItem(service, "TestType")

		if !service.reloadCalled {
			t.Error("Expected Reload to be called even with error")
		}
	})

	t.Run("does nothing for nil interface", func(t *testing.T) {
		var service *mockReloadService
		// Should not panic
		reloadItem(service, "TestType")
	})
}

func TestReloadServices(t *testing.T) {
	t.Run("reloads single reloadable service", func(t *testing.T) {
		service := &mockReloadService{}
		config := testReloadConfig{
			ReloadableService: service,
		}

		ReloadServices(reflect.ValueOf(config))

		if !service.reloadCalled {
			t.Error("Expected single reloadable service to be reloaded")
		}
	})

	t.Run("reloads all services in a slice", func(t *testing.T) {
		service1 := &mockReloadService{}
		service2 := &mockReloadService{}
		service3 := &mockReloadService{}
		config := testReloadConfig{
			ReloadableServices: []*mockReloadService{service1, service2, service3},
		}

		ReloadServices(reflect.ValueOf(config))

		if !service1.reloadCalled {
			t.Error("Expected first service in slice to be reloaded")
		}
		if !service2.reloadCalled {
			t.Error("Expected second service in slice to be reloaded")
		}
		if !service3.reloadCalled {
			t.Error("Expected third service in slice to be reloaded")
		}
	})

	t.Run("skips non-reloadable services", func(t *testing.T) {
		reloadableService := &mockReloadService{}
		nonReloadableService := &mockNonReloadService{value: "test"}
		config := testReloadConfig{
			ReloadableService:    reloadableService,
			NonReloadableService: nonReloadableService,
		}

		ReloadServices(reflect.ValueOf(config))

		if !reloadableService.reloadCalled {
			t.Error("Expected reloadable service to be reloaded")
		}
		// NonReloadableService should be silently skipped - no panic
	})

	t.Run("skips zero/nil fields", func(t *testing.T) {
		config := testReloadConfig{
			EmptyField: nil,
		}

		// Should not panic
		ReloadServices(reflect.ValueOf(config))
	})

	t.Run("handles mixed configuration", func(t *testing.T) {
		singleService := &mockReloadService{}
		sliceService1 := &mockReloadService{}
		sliceService2 := &mockReloadService{}
		config := testReloadConfig{
			ReloadableService:    singleService,
			ReloadableServices:   []*mockReloadService{sliceService1, sliceService2},
			NonReloadableService: &mockNonReloadService{value: "test"},
			EmptyField:           nil,
			StringField:          "ignored",
		}

		ReloadServices(reflect.ValueOf(config))

		if !singleService.reloadCalled {
			t.Error("Expected single service to be reloaded")
		}
		if !sliceService1.reloadCalled {
			t.Error("Expected first slice service to be reloaded")
		}
		if !sliceService2.reloadCalled {
			t.Error("Expected second slice service to be reloaded")
		}
	})

	t.Run("continues after reload error", func(t *testing.T) {
		failingService := &mockReloadService{
			reloadError: errors.New("reload failed"),
		}
		successService := &mockReloadService{}
		config := testReloadConfig{
			ReloadableServices: []*mockReloadService{failingService, successService},
		}

		// Should not panic - errors are logged
		ReloadServices(reflect.ValueOf(config))

		if !failingService.reloadCalled {
			t.Error("Expected failing service to attempt reload")
		}
		if !successService.reloadCalled {
			t.Error("Expected success service to be reloaded after error")
		}
	})

	t.Run("handles empty slice", func(t *testing.T) {
		config := testReloadConfig{
			ReloadableServices: []*mockReloadService{},
		}

		// Should not panic
		ReloadServices(reflect.ValueOf(config))
	})
}
