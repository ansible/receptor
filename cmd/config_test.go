package cmd

import (
	"reflect"
	"testing"

	"github.com/ansible/receptor/pkg/backends"
	"github.com/ansible/receptor/pkg/controlsvc"
	"github.com/ansible/receptor/pkg/workceptor"
)

func TestIsConfigEmpty(t *testing.T) {
	// Test with an empty struct
	type emptyStruct struct {
		Field1 string
		Field2 int
		Field3 bool
	}
	empty := emptyStruct{}
	if !isConfigEmpty(reflect.ValueOf(empty)) {
		t.Error("Expected empty struct to be identified as empty")
	}

	// Test with a non-empty struct
	nonEmpty := emptyStruct{Field1: "value"}
	if isConfigEmpty(reflect.ValueOf(nonEmpty)) {
		t.Error("Expected non-empty struct to be identified as non-empty")
	}
}

func TestSetBackendConfigDefaults(t *testing.T) {
	// Create a BackendConfig with no defaults set
	config := &BackendConfig{
		TCPListeners: []*backends.TCPListenerCfg{
			{
				BindAddr: "",
				Cost:     0,
			},
		},
		UDPListeners: []*backends.UDPListenerCfg{
			{
				BindAddr: "",
				Cost:     0,
			},
		},
		WSListeners: []*backends.WebsocketListenerCfg{
			{
				BindAddr: "",
				Cost:     0,
				Path:     "",
			},
		},
		TCPPeers: []*backends.TCPDialerCfg{
			{
				Cost:   0,
				Redial: false,
			},
		},
		UDPPeers: []*backends.UDPDialerCfg{
			{
				Cost:   0,
				Redial: false,
			},
		},
		WSPeers: []*backends.WebsocketDialerCfg{
			{
				Cost:   0,
				Redial: false,
			},
		},
	}

	// Apply defaults
	SetBackendConfigDefaults(config)

	// Check TCP Listener defaults
	if config.TCPListeners[0].BindAddr != "0.0.0.0" {
		t.Errorf("Expected TCP Listener BindAddr to be '0.0.0.0', got '%s'", config.TCPListeners[0].BindAddr)
	}
	if config.TCPListeners[0].Cost != 1.0 {
		t.Errorf("Expected TCP Listener Cost to be 1.0, got %f", config.TCPListeners[0].Cost)
	}

	// Check UDP Listener defaults
	if config.UDPListeners[0].BindAddr != "0.0.0.0" {
		t.Errorf("Expected UDP Listener BindAddr to be '0.0.0.0', got '%s'", config.UDPListeners[0].BindAddr)
	}
	if config.UDPListeners[0].Cost != 1.0 {
		t.Errorf("Expected UDP Listener Cost to be 1.0, got %f", config.UDPListeners[0].Cost)
	}

	// Check WS Listener defaults
	if config.WSListeners[0].BindAddr != "0.0.0.0" {
		t.Errorf("Expected WS Listener BindAddr to be '0.0.0.0', got '%s'", config.WSListeners[0].BindAddr)
	}
	if config.WSListeners[0].Cost != 1.0 {
		t.Errorf("Expected WS Listener Cost to be 1.0, got %f", config.WSListeners[0].Cost)
	}

	// Check TCP Peer defaults
	if config.TCPPeers[0].Cost != 1.0 {
		t.Errorf("Expected TCP Peer Cost to be 1.0, got %f", config.TCPPeers[0].Cost)
	}
	if !config.TCPPeers[0].Redial {
		t.Error("Expected TCP Peer Redial to be true")
	}

	// Check UDP Peer defaults
	if config.UDPPeers[0].Cost != 1.0 {
		t.Errorf("Expected UDP Peer Cost to be 1.0, got %f", config.UDPPeers[0].Cost)
	}
	if !config.UDPPeers[0].Redial {
		t.Error("Expected UDP Peer Redial to be true")
	}

	// Check WS Peer defaults
	if config.WSPeers[0].Cost != 1.0 {
		t.Errorf("Expected WS Peer Cost to be 1.0, got %f", config.WSPeers[0].Cost)
	}
	if !config.WSPeers[0].Redial {
		t.Error("Expected WS Peer Redial to be true")
	}
}

func TestSetReceptorConfigDefaults(t *testing.T) {
	// Create a ReceptorConfig with no defaults set
	config := &ReceptorConfig{
		Node:     nil,
		LogLevel: nil,
		ControlServices: []*controlsvc.CmdlineConfigUnix{
			{
				Service:     "",
				Permissions: 0,
			},
		},
		WorkKubernetes: []*workceptor.KubeWorkerCfg{
			{
				AuthMethod:   "",
				StreamMethod: "",
			},
		},
	}

	// Apply defaults
	SetReceptorConfigDefaults(config)

	// Check Node defaults
	if config.Node == nil {
		t.Error("Expected Node to be initialized")
	} else if config.Node.DataDir != "/tmp/receptor" {
		t.Errorf("Expected Node DataDir to be '/tmp/receptor', got '%s'", config.Node.DataDir)
	}

	// Check ControlService defaults
	if config.ControlServices[0].Service != "control" {
		t.Errorf("Expected ControlService Service to be 'control', got '%s'", config.ControlServices[0].Service)
	}
	if config.ControlServices[0].Permissions != 0o600 {
		t.Errorf("Expected ControlService Permissions to be 0o600, got %o", config.ControlServices[0].Permissions)
	}

	// Check WorkKubernetes defaults
	if config.WorkKubernetes[0].AuthMethod != "incluster" {
		t.Errorf("Expected WorkKubernetes AuthMethod to be 'incluster', got '%s'", config.WorkKubernetes[0].AuthMethod)
	}
	if config.WorkKubernetes[0].StreamMethod != "logger" {
		t.Errorf("Expected WorkKubernetes StreamMethod to be 'logger', got '%s'", config.WorkKubernetes[0].StreamMethod)
	}
}

// mockCommand is a struct that implements the Initer, Preparer, and Runer interfaces for testing.
type mockCommand struct {
	initCalled    bool
	prepareCalled bool
	runCalled     bool
	initError     error
	prepareError  error
	runError      error
}

// Init implements the Initer interface.
func (m *mockCommand) Init() error {
	m.initCalled = true

	return m.initError
}

// Prepare implements the Preparer interface.
func (m *mockCommand) Prepare() error {
	m.prepareCalled = true

	return m.prepareError
}

// Run implements the Runer interface.
func (m *mockCommand) Run() error {
	m.runCalled = true

	return m.runError
}

func TestRunPhases(t *testing.T) {
	// Test Init phase
	mock := &mockCommand{}
	RunPhases("Init", reflect.ValueOf(mock))
	if !mock.initCalled {
		t.Error("Expected Init to be called")
	}
	if mock.prepareCalled || mock.runCalled {
		t.Error("Expected only Init to be called")
	}

	// Test Prepare phase
	mock = &mockCommand{}
	RunPhases("Prepare", reflect.ValueOf(mock))
	if !mock.prepareCalled {
		t.Error("Expected Prepare to be called")
	}
	if mock.initCalled || mock.runCalled {
		t.Error("Expected only Prepare to be called")
	}

	// Test Run phase
	mock = &mockCommand{}
	RunPhases("Run", reflect.ValueOf(mock))
	if !mock.runCalled {
		t.Error("Expected Run to be called")
	}
	if mock.initCalled || mock.prepareCalled {
		t.Error("Expected only Run to be called")
	}

	// Test with an invalid phase
	mock = &mockCommand{}
	RunPhases("InvalidPhase", reflect.ValueOf(mock))
	if mock.initCalled || mock.prepareCalled || mock.runCalled {
		t.Error("Expected no methods to be called for invalid phase")
	}
}

func TestParseReceptorConfig(t *testing.T) {
	// Skip this test for now as it requires more complex setup
	t.Skip("Skipping TestParseReceptorConfig as it requires more complex setup")
}

func TestParseBackendConfig(t *testing.T) {
	// Skip this test for now as it requires more complex setup
	t.Skip("Skipping TestParseBackendConfig as it requires more complex setup")
}

func TestParseCertificatesConfig(t *testing.T) {
	// Skip this test for now as it requires more complex setup
	t.Skip("Skipping TestParseCertificatesConfig as it requires more complex setup")
}

// testConfig is a struct with a slice of mockCommand for testing RunConfigV2.
type testConfig struct {
	Commands []*mockCommand
	Empty    string
}

func TestRunConfigV2(t *testing.T) {
	// Create a test config with some commands
	config := testConfig{
		Commands: []*mockCommand{
			{},
			{},
		},
	}

	// Run the config
	RunConfigV2(reflect.ValueOf(config))

	// Check that all commands were called for all phases
	for _, cmd := range config.Commands {
		if !cmd.initCalled {
			t.Error("Expected Init to be called")
		}
		if !cmd.prepareCalled {
			t.Error("Expected Prepare to be called")
		}
		if !cmd.runCalled {
			t.Error("Expected Run to be called")
		}
	}
}
