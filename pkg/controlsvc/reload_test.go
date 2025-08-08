package controlsvc

import (
	"os"
	"path/filepath"
	"testing"
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

// TestInitReload tests the InitReload function with various scenarios.
func TestInitReload(t *testing.T) {
	// Save original values to restore later
	originalConfigPath := configPath
	originalReloadParseAndRun := reloadParseAndRun
	originalCfgNotReloadable := make(map[string]bool)
	for k, v := range cfgNotReloadable {
		originalCfgNotReloadable[k] = v
	}

	defer func() {
		// Restore original values
		configPath = originalConfigPath
		reloadParseAndRun = originalReloadParseAndRun
		cfgNotReloadable = originalCfgNotReloadable
	}()

	t.Run("successful initialization", func(t *testing.T) {
		// Reset state
		configPath = ""
		cfgNotReloadable = make(map[string]bool)

		mockParseAndRun := func(toRun []string) error {
			return nil
		}

		err := InitReload("reload_test_yml/init.yml", mockParseAndRun)
		if err != nil {
			t.Errorf("InitReload failed: %v", err)
		}

		if configPath != "reload_test_yml/init.yml" {
			t.Errorf("configPath not set correctly, got %s", configPath)
		}

		if len(cfgNotReloadable) == 0 {
			t.Error("cfgNotReloadable should be populated after InitReload")
		}
	})

	t.Run("file not found", func(t *testing.T) {
		// Reset state
		configPath = ""
		cfgNotReloadable = make(map[string]bool)

		mockParseAndRun := func(toRun []string) error {
			return nil
		}

		err := InitReload("nonexistent.yml", mockParseAndRun)
		if err == nil {
			t.Error("InitReload should fail with non-existent file")
		}
	})

	t.Run("invalid yaml file", func(t *testing.T) {
		// Reset state
		configPath = ""
		cfgNotReloadable = make(map[string]bool)

		// Create a temporary invalid YAML file
		tmpDir, err := os.MkdirTemp("", "reload_test")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir)

		invalidYAMLFile := filepath.Join(tmpDir, "invalid.yml")
		err = os.WriteFile(invalidYAMLFile, []byte("invalid: yaml: content: ["), 0o644)
		if err != nil {
			t.Fatal(err)
		}

		mockParseAndRun := func(toRun []string) error {
			return nil
		}

		err = InitReload(invalidYAMLFile, mockParseAndRun)
		if err == nil {
			t.Error("InitReload should fail with invalid YAML")
		}
	})
}

// TestCheckReload tests the checkReload function.
func TestCheckReload(t *testing.T) {
	// Save original values to restore later
	originalConfigPath := configPath
	originalCfgNotReloadable := make(map[string]bool)
	for k, v := range cfgNotReloadable {
		originalCfgNotReloadable[k] = v
	}

	defer func() {
		// Restore original values
		configPath = originalConfigPath
		cfgNotReloadable = originalCfgNotReloadable
	}()

	t.Run("successful check with valid config", func(t *testing.T) {
		// Initialize with a valid config
		configPath = "reload_test_yml/init.yml"
		cfgNotReloadable = make(map[string]bool)
		err := parseConfigForReload(configPath, false)
		if err != nil {
			t.Fatal(err)
		}

		// Now test checkReload with the same config
		err = checkReload()
		if err != nil {
			t.Errorf("checkReload failed: %v", err)
		}
	})

	t.Run("check with modified non-reloadable config", func(t *testing.T) {
		// Initialize with initial config
		configPath = "reload_test_yml/init.yml"
		cfgNotReloadable = make(map[string]bool)
		err := parseConfigForReload(configPath, false)
		if err != nil {
			t.Fatal(err)
		}

		// Change to config with added non-reloadable items
		configPath = "reload_test_yml/add_cfg.yml"
		err = checkReload()
		if err == nil {
			t.Error("checkReload should fail when non-reloadable config is added")
		}
	})

	t.Run("check with nonexistent config file", func(t *testing.T) {
		configPath = "nonexistent.yml"
		cfgNotReloadable = make(map[string]bool)

		err := checkReload()
		if err == nil {
			t.Error("checkReload should fail with non-existent file")
		}
	})

	t.Run("check with invalid yaml", func(t *testing.T) {
		// Create a temporary invalid YAML file
		tmpDir, err := os.MkdirTemp("", "reload_test")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir)

		invalidYAMLFile := filepath.Join(tmpDir, "invalid.yml")
		err = os.WriteFile(invalidYAMLFile, []byte("invalid: yaml: content: ["), 0o644)
		if err != nil {
			t.Fatal(err)
		}

		configPath = invalidYAMLFile
		cfgNotReloadable = make(map[string]bool)

		err = checkReload()
		if err == nil {
			t.Error("checkReload should fail with invalid YAML")
		}
	})
}

// TestInitFromString tests the InitFromString method of ReloadCommandType.
func TestInitFromString(t *testing.T) {
	reloadCommandType := &ReloadCommandType{}

	t.Run("successful initialization from empty string", func(t *testing.T) {
		cmd, err := reloadCommandType.InitFromString("")
		if err != nil {
			t.Errorf("InitFromString failed: %v", err)
		}

		if cmd == nil {
			t.Error("InitFromString should return a non-nil command")
		}

		_, ok := cmd.(*ReloadCommand)
		if !ok {
			t.Error("InitFromString should return a ReloadCommand")
		}
	})

	t.Run("initialization from non-empty string", func(t *testing.T) {
		cmd, err := reloadCommandType.InitFromString("some config string")
		if err != nil {
			t.Errorf("InitFromString failed: %v", err)
		}

		if cmd == nil {
			t.Error("InitFromString should return a non-nil command")
		}

		_, ok := cmd.(*ReloadCommand)
		if !ok {
			t.Error("InitFromString should return a ReloadCommand")
		}
	})

	t.Run("returns valid ReloadCommand instances", func(t *testing.T) {
		cmd1, err1 := reloadCommandType.InitFromString("test1")
		cmd2, err2 := reloadCommandType.InitFromString("test2")

		if err1 != nil || err2 != nil {
			t.Errorf("InitFromString failed: %v, %v", err1, err2)
		}

		// Verify both are ReloadCommand instances
		reloadCmd1, ok1 := cmd1.(*ReloadCommand)
		reloadCmd2, ok2 := cmd2.(*ReloadCommand)
		if !ok1 || !ok2 {
			t.Error("Both commands should be ReloadCommand instances")
		}

		// Note: ReloadCommand is an empty struct, so Go may optimize by reusing
		// the same memory location. This is expected behavior and doesn't affect
		// functionality since each call still returns a valid ReloadCommand.
		if reloadCmd1 == nil || reloadCmd2 == nil {
			t.Error("InitFromString should return non-nil ReloadCommand instances")
		}
	})
}
