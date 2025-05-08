package cmd

import (
	"testing"

	"github.com/ansible/receptor/pkg/backends"
	"github.com/ansible/receptor/pkg/controlsvc"
	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/types"
	"github.com/ansible/receptor/pkg/workceptor"
)

func TestSetTCPListenerDefaults(t *testing.T) {
	config := &BackendConfig{
		TCPListeners: []*backends.TCPListenerCfg{
			{
				BindAddr: "",
				Cost:     0,
			},
		},
	}

	SetTCPListenerDefaults(config)

	if config.TCPListeners[0].BindAddr != "0.0.0.0" {
		t.Errorf("Expected BindAddr to be '0.0.0.0', got '%s'", config.TCPListeners[0].BindAddr)
	}
	if config.TCPListeners[0].Cost != 1.0 {
		t.Errorf("Expected Cost to be 1.0, got %f", config.TCPListeners[0].Cost)
	}
}

func TestSetUDPListenerDefaults(t *testing.T) {
	config := &BackendConfig{
		UDPListeners: []*backends.UDPListenerCfg{
			{
				BindAddr: "",
				Cost:     0,
			},
		},
	}

	SetUDPListenerDefaults(config)

	if config.UDPListeners[0].BindAddr != "0.0.0.0" {
		t.Errorf("Expected BindAddr to be '0.0.0.0', got '%s'", config.UDPListeners[0].BindAddr)
	}
	if config.UDPListeners[0].Cost != 1.0 {
		t.Errorf("Expected Cost to be 1.0, got %f", config.UDPListeners[0].Cost)
	}
}

func TestSetWSListenerDefaults(t *testing.T) {
	config := &BackendConfig{
		WSListeners: []*backends.WebsocketListenerCfg{
			{
				BindAddr: "",
				Cost:     0,
				Path:     "",
			},
		},
	}

	SetWSListenerDefaults(config)

	if config.WSListeners[0].BindAddr != "0.0.0.0" {
		t.Errorf("Expected BindAddr to be '0.0.0.0', got '%s'", config.WSListeners[0].BindAddr)
	}
	if config.WSListeners[0].Cost != 1.0 {
		t.Errorf("Expected Cost to be 1.0, got %f", config.WSListeners[0].Cost)
	}
	if config.WSListeners[0].Path != "/" {
		t.Errorf("Expected Path to be '/', got '%s'", config.WSListeners[0].Path)
	}
}

func TestSetUDPPeerDefaults(t *testing.T) {
	config := &BackendConfig{
		UDPPeers: []*backends.UDPDialerCfg{
			{
				Cost:   0,
				Redial: false,
			},
		},
	}

	SetUDPPeerDefaults(config)

	if config.UDPPeers[0].Cost != 1.0 {
		t.Errorf("Expected Cost to be 1.0, got %f", config.UDPPeers[0].Cost)
	}
	if !config.UDPPeers[0].Redial {
		t.Error("Expected Redial to be true")
	}
}

func TestSetTCPPeerDefaults(t *testing.T) {
	config := &BackendConfig{
		TCPPeers: []*backends.TCPDialerCfg{
			{
				Cost:   0,
				Redial: false,
			},
		},
	}

	SetTCPPeerDefaults(config)

	if config.TCPPeers[0].Cost != 1.0 {
		t.Errorf("Expected Cost to be 1.0, got %f", config.TCPPeers[0].Cost)
	}
	if !config.TCPPeers[0].Redial {
		t.Error("Expected Redial to be true")
	}
}

func TestSetWSPeerDefaults(t *testing.T) {
	config := &BackendConfig{
		WSPeers: []*backends.WebsocketDialerCfg{
			{
				Cost:   0,
				Redial: false,
			},
		},
	}

	SetWSPeerDefaults(config)

	if config.WSPeers[0].Cost != 1.0 {
		t.Errorf("Expected Cost to be 1.0, got %f", config.WSPeers[0].Cost)
	}
	if !config.WSPeers[0].Redial {
		t.Error("Expected Redial to be true")
	}
}

func TestSetCmdlineUnixDefaults(t *testing.T) {
	config := &ReceptorConfig{
		ControlServices: []*controlsvc.CmdlineConfigUnix{
			{
				Service:     "",
				Permissions: 0,
			},
		},
	}

	SetCmdlineUnixDefaults(config)

	if config.ControlServices[0].Service != "control" {
		t.Errorf("Expected Service to be 'control', got '%s'", config.ControlServices[0].Service)
	}
	if config.ControlServices[0].Permissions != 0o600 {
		t.Errorf("Expected Permissions to be 0o600, got %o", config.ControlServices[0].Permissions)
	}
}

func TestSetLogLevelDefaults(t *testing.T) {
	// Test with nil LogLevel
	config := &ReceptorConfig{
		LogLevel: nil,
	}

	SetLogLevelDefaults(config)
	if config.LogLevel != nil {
		t.Error("Expected LogLevel to remain nil")
	}

	// Test with empty LogLevel
	config = &ReceptorConfig{
		LogLevel: &logger.LoglevelCfg{
			Level: "",
		},
	}

	SetLogLevelDefaults(config)
	if config.LogLevel.Level != "error" {
		t.Errorf("Expected Level to be 'error', got '%s'", config.LogLevel.Level)
	}

	// Test with non-empty LogLevel
	config = &ReceptorConfig{
		LogLevel: &logger.LoglevelCfg{
			Level: "debug",
		},
	}

	SetLogLevelDefaults(config)
	if config.LogLevel.Level != "debug" {
		t.Errorf("Expected Level to remain 'debug', got '%s'", config.LogLevel.Level)
	}
}

func TestSetNodeDefaults(t *testing.T) {
	// Test with nil Node
	config := &ReceptorConfig{
		Node: nil,
	}

	SetNodeDefaults(config)
	if config.Node == nil {
		t.Error("Expected Node to be initialized")
	} else if config.Node.DataDir != "/tmp/receptor" {
		t.Errorf("Expected DataDir to be '/tmp/receptor', got '%s'", config.Node.DataDir)
	}

	// Test with empty DataDir
	config = &ReceptorConfig{
		Node: &types.NodeCfg{
			DataDir: "",
		},
	}

	SetNodeDefaults(config)
	if config.Node.DataDir != "/tmp/receptor" {
		t.Errorf("Expected DataDir to be '/tmp/receptor', got '%s'", config.Node.DataDir)
	}

	// Test with non-empty DataDir
	config = &ReceptorConfig{
		Node: &types.NodeCfg{
			DataDir: "/custom/path",
		},
	}

	SetNodeDefaults(config)
	if config.Node.DataDir != "/custom/path" {
		t.Errorf("Expected DataDir to remain '/custom/path', got '%s'", config.Node.DataDir)
	}
}

func TestSetKubeWorkerDefaults(t *testing.T) {
	config := &ReceptorConfig{
		WorkKubernetes: []*workceptor.KubeWorkerCfg{
			{
				AuthMethod:   "",
				StreamMethod: "",
			},
		},
	}

	SetKubeWorkerDefaults(config)

	if config.WorkKubernetes[0].AuthMethod != "incluster" {
		t.Errorf("Expected AuthMethod to be 'incluster', got '%s'", config.WorkKubernetes[0].AuthMethod)
	}
	if config.WorkKubernetes[0].StreamMethod != "logger" {
		t.Errorf("Expected StreamMethod to be 'logger', got '%s'", config.WorkKubernetes[0].StreamMethod)
	}

	// Test with non-empty values
	config = &ReceptorConfig{
		WorkKubernetes: []*workceptor.KubeWorkerCfg{
			{
				AuthMethod:   "custom",
				StreamMethod: "custom",
			},
		},
	}

	SetKubeWorkerDefaults(config)

	if config.WorkKubernetes[0].AuthMethod != "custom" {
		t.Errorf("Expected AuthMethod to remain 'custom', got '%s'", config.WorkKubernetes[0].AuthMethod)
	}
	if config.WorkKubernetes[0].StreamMethod != "custom" {
		t.Errorf("Expected StreamMethod to remain 'custom', got '%s'", config.WorkKubernetes[0].StreamMethod)
	}
}
