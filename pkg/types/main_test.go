package types

import (
	"os"
	"testing"
	"time"

	"gopkg.in/yaml.v2"
)

func TestMainInitNodeID(t *testing.T) {
	mainInitNodeIDTestCases := []struct {
		name        string
		nodeID      string
		expectedErr string
	}{
		{
			name:        "successful, no error",
			nodeID:      "t.e-s_t@1:234",
			expectedErr: "",
		},
		{
			name:        "failed, charactered not allowed",
			nodeID:      "test!#&123",
			expectedErr: "node id can only contain a-z, A-Z, 0-9 or special characters . - _ @ : but received: test!#&123",
		},
	}

	for _, testCase := range mainInitNodeIDTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := NodeCfg{
				ID: testCase.nodeID,
			}
			err := cfg.Init()
			if err == nil && testCase.expectedErr != "" {
				t.Errorf("exected error but got no error")
			} else if err != nil && err.Error() != testCase.expectedErr {
				t.Errorf("expected error to be %s, but got: %s", testCase.expectedErr, err.Error())
			}
			t.Cleanup(func() {
				cfg = NodeCfg{}
			})
		})
	}
}

func TestNodeCfgInitEmptyID(t *testing.T) {
	cfg := NodeCfg{
		ID:      "",
		DataDir: "/tmp/test-receptor",
	}

	err := cfg.Init()
	if err != nil {
		t.Errorf("Expected no error for empty ID with valid hostname, got: %v", err)
	}
}

func TestNodeCfgInitReservedID(t *testing.T) {
	cfg := NodeCfg{
		ID: "localhost",
	}

	err := cfg.Init()
	if err == nil {
		t.Errorf("Expected error for reserved localhost ID")
	}
	if err.Error() != "node ID \"localhost\" is reserved" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestNodeCfgInitMaxIdleConnectionTimeout(t *testing.T) {
	cfg := NodeCfg{
		ID:                       "test-node",
		DataDir:                  "/tmp/test-receptor",
		MaxIdleConnectionTimeout: "30s",
	}

	err := cfg.Init()
	if err != nil {
		t.Errorf("Expected no error when setting MaxIdleConnectionTimeout, got: %v", err)
	}
}

func TestNodeCfgRun(t *testing.T) {
	cfg := NodeCfg{
		ID:      "test-node",
		DataDir: "/tmp/test-receptor",
	}

	err := cfg.Init()
	if err != nil {
		t.Fatalf("Failed to initialize NodeCfg: %v", err)
	}

	err = cfg.Run()
	if err != nil {
		t.Errorf("Expected no error from Run(), got: %v", err)
	}
}

func TestReceptorPyroscopeCfgInitEmptyApplicationName(t *testing.T) {
	cfg := ReceptorPyroscopeCfg{
		ApplicationName: "",
	}

	err := cfg.Init()
	if err != nil {
		t.Errorf("Expected no error for empty ApplicationName, got: %v", err)
	}
}

func TestReceptorPyroscopeCfgInitWithConfig(t *testing.T) {
	tempDir := "/tmp/test-receptor-pyroscope"
	defer os.RemoveAll(tempDir)

	cfg := ReceptorPyroscopeCfg{
		ApplicationName: "test-app",
		ServerAddress:   "http://localhost:4040",
		UploadRate:      "30s",
		ProfileTypes:    []string{"ProfileGoroutines", "ProfileMutexCount"},
		Tags:            map[string]string{"env": "test"},
		HTTPHeaders:     map[string]string{"Authorization": "Bearer token"},
	}

	err := cfg.Init()
	if err != nil {
		t.Errorf("Expected no error for valid config, got: %v", err)
	}
}

func TestGetUploadRateDefault(t *testing.T) {
	cfg := ReceptorPyroscopeCfg{
		UploadRate: "",
	}

	rate := getUploadRate(cfg)
	expected := 15 * time.Second
	if rate != expected {
		t.Errorf("Expected default upload rate %v, got %v", expected, rate)
	}
}

func TestGetUploadRateFromConfig(t *testing.T) {
	cfg := ReceptorPyroscopeCfg{
		UploadRate: "uploadRate: 30s",
	}

	rate := getUploadRate(cfg)
	expected := 30 * time.Second
	if rate != expected {
		t.Errorf("Expected upload rate %v, got %v", expected, rate)
	}
}

func TestGetUploadRateInvalidYAML(t *testing.T) {
	cfg := ReceptorPyroscopeCfg{
		UploadRate: "invalid yaml",
	}

	rate := getUploadRate(cfg)
	if rate != 0 {
		t.Errorf("Expected zero duration for invalid YAML, got %v", rate)
	}
}

func TestGetProfileTypesDefault(t *testing.T) {
	cfg := ReceptorPyroscopeCfg{
		ProfileTypes: []string{},
	}

	types := getProfileTypes(cfg)
	if len(types) != 5 {
		t.Errorf("Expected 5 default profile types, got %d", len(types))
	}
}

func TestGetProfileTypesWithCustomTypes(t *testing.T) {
	cfg := ReceptorPyroscopeCfg{
		ProfileTypes: []string{
			"ProfileGoroutines",
			"ProfileMutexCount",
			"ProfileMutexDuration",
			"ProfileBlockCount",
			"ProfileBlockDuration",
		},
	}

	types := getProfileTypes(cfg)
	if len(types) != 10 {
		t.Errorf("Expected 10 profile types (5 default + 5 custom), got %d", len(types))
	}
}

func TestUploadRateUnmarshal(t *testing.T) {
	yamlData := "uploadRate: 45s"
	var ur UploadRate
	err := yaml.Unmarshal([]byte(yamlData), &ur)
	if err != nil {
		t.Errorf("Failed to unmarshal UploadRate: %v", err)
	}
	expected := 45 * time.Second
	if ur.UploadRate != expected {
		t.Errorf("Expected upload rate %v, got %v", expected, ur.UploadRate)
	}
}
