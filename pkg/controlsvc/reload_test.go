package controlsvc

import (
	"os"
	"testing"

	"github.com/ansible/receptor/pkg/certificates/mock_certificates"
	"go.uber.org/mock/gomock"
)

func TestReload(t *testing.T) {
	type yamltest struct {
		filename    string
		modifyError bool
		absentError bool
	}

	scenarios := []yamltest{
		{filename: "reload_test_yml/init.yml", modifyError: false, absentError: false},
		{filename: "reload_test_yml/add_cfg.yml", modifyError: true, absentError: false},
		{filename: "reload_test_yml/drop_cfg.yml", modifyError: false, absentError: true},
		{filename: "reload_test_yml/modify_cfg.yml", modifyError: true, absentError: true},
		{filename: "reload_test_yml/syntax_error.yml", modifyError: true, absentError: true},
		{filename: "reload_test_yml/successful_reload.yml", modifyError: false, absentError: false},
	}
	err := parseConfigForReload("reload_test_yml/init.yml", false)
	if err != nil {
		t.Errorf("parseConfigForReload %s: Unexpected err: %v", "init.yml", err)
	}

	if len(cfgNotReloadable) != 5 {
		t.Errorf("cfNotReloadable length expected %d, got %d", 5, len(cfgNotReloadable))
	}

	for _, s := range scenarios {
		t.Logf("%s", s.filename)
		err = parseConfigForReload(s.filename, true)
		if s.modifyError {
			if err == nil {
				t.Errorf("parseConfigForReload %s %s: Expected err, got %v", s.filename, "modifyError", err)
			}
		} else {
			if err != nil {
				t.Errorf("parseConfigForReload %s %s: Unexpected err: %v", s.filename, "modifyError", err)
			}
		}
		err = cfgAbsent()
		if s.absentError {
			if err == nil {
				t.Errorf("parseConfigForReload %s %s: Expected err, got %v", s.filename, "absentError", err)
			}
		} else {
			if err != nil {
				t.Errorf("parseConfigForReload %s %s: Unexpected err: %v", s.filename, "absentError", err)
			}
		}
	}
}

// setupReloadTest saves original state and returns cleanup function.
func setupReloadTest(t *testing.T) func() {
	t.Helper()
	originalConfigPath := configPath
	originalReloadParseAndRun := reloadParseAndRun
	originalCfgNotReloadable := make(map[string]bool)
	for k, v := range cfgNotReloadable {
		originalCfgNotReloadable[k] = v
	}

	return func() {
		configPath = originalConfigPath
		reloadParseAndRun = originalReloadParseAndRun
		cfgNotReloadable = originalCfgNotReloadable
	}
}

// resetReloadState clears global state for clean test start.
func resetReloadState() {
	configPath = ""
	cfgNotReloadable = make(map[string]bool)
}

// setupMockFileReader creates a mock file reader for testing.
func setupMockFileReader(t *testing.T) (*gomock.Controller, *mock_certificates.MockOser) {
	t.Helper()
	ctrl := gomock.NewController(t)
	mockReader := mock_certificates.NewMockOser(ctrl)

	return ctrl, mockReader
}

// TestParseConfigForReloadWithReader tests the core parsing logic with mocks.
func TestParseConfigForReloadWithReader(t *testing.T) {
	cleanup := setupReloadTest(t)
	defer cleanup()

	t.Run("valid yaml", func(t *testing.T) {
		ctrl, mockReader := setupMockFileReader(t)
		defer ctrl.Finish()

		resetReloadState()
		validYAML := []byte(`
---
- node:
    id: test
- tcp-peer:
    address: localhost:8001
`)
		mockReader.EXPECT().ReadFile("config.yml").Return(validYAML, nil).Times(1)

		err := parseConfigForReloadWithReader("config.yml", false, mockReader)
		if err != nil {
			t.Fatalf("parseConfigForReload failed: %v", err)
		}
		if len(cfgNotReloadable) == 0 {
			t.Error("cfgNotReloadable should be populated")
		}
	})

	t.Run("file not found", func(t *testing.T) {
		ctrl, mockReader := setupMockFileReader(t)
		defer ctrl.Finish()

		resetReloadState()
		mockReader.EXPECT().ReadFile("missing.yml").Return(nil, os.ErrNotExist).Times(1)

		err := parseConfigForReloadWithReader("missing.yml", false, mockReader)
		if err != os.ErrNotExist {
			t.Errorf("expected ErrNotExist, got %v", err)
		}
	})

	t.Run("invalid yaml", func(t *testing.T) {
		ctrl, mockReader := setupMockFileReader(t)
		defer ctrl.Finish()

		resetReloadState()
		invalidYAML := []byte("invalid: yaml: content: [")
		mockReader.EXPECT().ReadFile("invalid.yml").Return(invalidYAML, nil).Times(1)

		err := parseConfigForReloadWithReader("invalid.yml", false, mockReader)
		if err == nil {
			t.Error("should fail with invalid YAML")
		}
	})
}

// TestInitReload tests the InitReload function with real files.
func TestInitReload(t *testing.T) {
	cleanup := setupReloadTest(t)
	defer cleanup()

	mockParseAndRun := func(toRun []string) error { return nil }

	t.Run("successful initialization", func(t *testing.T) {
		resetReloadState()
		err := InitReload("reload_test_yml/init.yml", mockParseAndRun)
		if err != nil {
			t.Fatalf("InitReload failed: %v", err)
		}
		if configPath != "reload_test_yml/init.yml" {
			t.Errorf("configPath = %q, want %q", configPath, "reload_test_yml/init.yml")
		}
		if len(cfgNotReloadable) == 0 {
			t.Error("cfgNotReloadable should be populated")
		}
	})
}

// TestCheckReload tests the checkReload function with real files.
func TestCheckReload(t *testing.T) {
	cleanup := setupReloadTest(t)
	defer cleanup()

	t.Run("valid config", func(t *testing.T) {
		configPath = "reload_test_yml/init.yml"
		cfgNotReloadable = make(map[string]bool)
		err := parseConfigForReload(configPath, false)
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		err = checkReload()
		if err != nil {
			t.Errorf("checkReload failed: %v", err)
		}
	})

	t.Run("modified non-reloadable config", func(t *testing.T) {
		configPath = "reload_test_yml/init.yml"
		cfgNotReloadable = make(map[string]bool)
		err := parseConfigForReload(configPath, false)
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		configPath = "reload_test_yml/add_cfg.yml"
		err = checkReload()
		if err == nil {
			t.Error("checkReload should fail when non-reloadable config is added")
		}
	})
}

// TestInitFromString tests the InitFromString method of ReloadCommandType.
func TestInitFromString(t *testing.T) {
	reloadCommandType := &ReloadCommandType{}

	t.Run("creates valid ReloadCommand", func(t *testing.T) {
		cmd, err := reloadCommandType.InitFromString("ignored parameter")
		if err != nil {
			t.Fatalf("InitFromString failed: %v", err)
		}
		if cmd == nil {
			t.Fatal("InitFromString returned nil command")
		}
		if _, ok := cmd.(*ReloadCommand); !ok {
			t.Error("InitFromString should return a ReloadCommand")
		}
	})
}
