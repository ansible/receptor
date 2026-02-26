package services

import (
	"context"
	"crypto/tls"
	"os"
	"testing"

	"github.com/ansible/receptor/pkg/netceptor"
)

func TestUnixProxyServiceInbound(t *testing.T) {
	type testCase struct {
		name        string
		filename    string
		permissions os.FileMode
		node        string
		rservice    string
		tlscfg      *tls.Config
		expecterr   bool
	}

	tests := []testCase{
		{
			name:      "Fail UnixSocketListen",
			expecterr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			s := netceptor.New(ctx, "Unix Test Node")
			err := UnixProxyServiceInbound(s, tc.filename, tc.permissions, tc.node, tc.rservice, tc.tlscfg)
			if tc.expecterr {
				if err == nil {
					t.Errorf("net UnixProxyServiceInbound fail case error")
				}

				return
			} else if err != nil {
				t.Errorf("net UnixProxyServiceInbound error")
			}
		})
	}
}

func TestUnixProxyServiceOutbound(t *testing.T) {
	type testCase struct {
		name      string
		expecterr bool
		service   string
		tlscfg    *tls.Config
		filename  string
	}

	tests := []testCase{
		{
			name: "Fail UnixSocketListen",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			s := netceptor.New(ctx, "Unix Test Node")
			err := UnixProxyServiceOutbound(s, tc.service, tc.tlscfg, tc.filename)
			if tc.expecterr {
				if err == nil {
					t.Errorf("net UnixProxyServiceInbound fail case error")
				}

				return
			} else if err != nil {
				t.Errorf("net UnixProxyServiceInbound error")
			}
		})
	}
}

func TestUnixProxyInboundCfgRun(t *testing.T) {
	type testCase struct {
		name                 string
		expectError          bool
		expectedErrorMessage string
		getConfigObj         func(sockPath string) UnixProxyInboundCfg
	}

	testCases := []testCase{
		{
			name: "Valid unix proxy inbound configuration",
			getConfigObj: func(sockPath string) UnixProxyInboundCfg {
				return UnixProxyInboundCfg{
					Filename:      sockPath,
					Permissions:   0o600,
					RemoteNode:    "node1",
					RemoteService: "service1",
				}
			},
		},
		{
			name: "Valid unix proxy inbound with custom permissions",
			getConfigObj: func(sockPath string) UnixProxyInboundCfg {
				return UnixProxyInboundCfg{
					Filename:      sockPath,
					Permissions:   0o660,
					RemoteNode:    "node2",
					RemoteService: "service2",
				}
			},
		},
		{
			name:                 "Invalid TLS configuration",
			expectError:          true,
			expectedErrorMessage: "unknown TLS config invalid-tls",
			getConfigObj: func(sockPath string) UnixProxyInboundCfg {
				return UnixProxyInboundCfg{
					Filename:      sockPath,
					Permissions:   0o600,
					RemoteNode:    "node3",
					RemoteService: "service3",
					TLS:           "invalid-tls",
				}
			},
		},
	}

	// Save original instance and create cancellable context
	originalInstance := netceptor.MainInstance
	ctx, cancel := context.WithCancel(context.Background())
	netceptor.MainInstance = netceptor.New(ctx, "test_unix_proxy_inbound_cfg_run")
	defer func() {
		cancel()
		netceptor.MainInstance = originalInstance
	}()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create unique socket path for this subtest
			tmpDir := t.TempDir()
			sockPath := tmpDir + "/test.sock"
			configObj := tc.getConfigObj(sockPath)

			// Clean up socket file if it exists before and after
			os.Remove(sockPath)
			defer os.Remove(sockPath)

			err := configObj.Run()
			if tc.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				} else if tc.expectedErrorMessage != "" && tc.expectedErrorMessage != err.Error() {
					t.Errorf("expected error message '%s', but got '%s'", tc.expectedErrorMessage, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestUnixProxyOutboundCfgRun(t *testing.T) {
	type testCase struct {
		name                 string
		expectError          bool
		expectedErrorMessage string
		getConfigObj         func(sockPath string) UnixProxyOutboundCfg
	}

	testCases := []testCase{
		{
			name: "Valid unix proxy outbound configuration",
			getConfigObj: func(sockPath string) UnixProxyOutboundCfg {
				return UnixProxyOutboundCfg{
					Service:  "unix1",
					Filename: sockPath,
				}
			},
		},
		{
			name: "Valid unix proxy outbound with different socket",
			getConfigObj: func(sockPath string) UnixProxyOutboundCfg {
				return UnixProxyOutboundCfg{
					Service:  "unix2",
					Filename: sockPath,
				}
			},
		},
		{
			name:                 "Invalid TLS configuration",
			expectError:          true,
			expectedErrorMessage: "unknown TLS config invalid-tls",
			getConfigObj: func(sockPath string) UnixProxyOutboundCfg {
				return UnixProxyOutboundCfg{
					Service:  "unix3",
					Filename: sockPath,
					TLS:      "invalid-tls",
				}
			},
		},
	}

	// Save original instance and create cancellable context
	originalInstance := netceptor.MainInstance
	ctx, cancel := context.WithCancel(context.Background())
	netceptor.MainInstance = netceptor.New(ctx, "test_unix_proxy_outbound_cfg_run")
	defer func() {
		cancel()
		netceptor.MainInstance = originalInstance
	}()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create unique socket path for this subtest
			tmpDir := t.TempDir()
			sockPath := tmpDir + "/test.sock"
			configObj := tc.getConfigObj(sockPath)

			err := configObj.Run()
			if tc.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				} else if tc.expectedErrorMessage != "" && tc.expectedErrorMessage != err.Error() {
					t.Errorf("expected error message '%s', but got '%s'", tc.expectedErrorMessage, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
