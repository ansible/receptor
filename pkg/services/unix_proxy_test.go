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
		configObj            UnixProxyInboundCfg
	}

	testCases := []testCase{
		{
			name: "Valid unix proxy inbound configuration",
			configObj: UnixProxyInboundCfg{
				Filename:      "/tmp/test-receptor-unix-inbound.sock",
				Permissions:   0600,
				RemoteNode:    "node1",
				RemoteService: "service1",
			},
		},
		{
			name: "Valid unix proxy inbound with custom permissions",
			configObj: UnixProxyInboundCfg{
				Filename:      "/tmp/test-receptor-unix-inbound2.sock",
				Permissions:   0660,
				RemoteNode:    "node2",
				RemoteService: "service2",
			},
		},
		{
			name:                 "Invalid TLS configuration",
			expectError:          true,
			expectedErrorMessage: "unknown TLS config invalid-tls",
			configObj: UnixProxyInboundCfg{
				Filename:      "/tmp/test-receptor-unix-inbound3.sock",
				Permissions:   0600,
				RemoteNode:    "node3",
				RemoteService: "service3",
				TLS:           "invalid-tls",
			},
		},
	}

	netceptor.MainInstance = netceptor.New(context.Background(), "test_unix_proxy_inbound_cfg_run")

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clean up socket file if it exists
			defer os.Remove(tc.configObj.Filename)

			err := tc.configObj.Run()
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
		configObj            UnixProxyOutboundCfg
	}

	testCases := []testCase{
		{
			name: "Valid unix proxy outbound configuration",
			configObj: UnixProxyOutboundCfg{
				Service:  "unix1",
				Filename: "/tmp/test-receptor-unix-outbound.sock",
			},
		},
		{
			name: "Valid unix proxy outbound with different socket",
			configObj: UnixProxyOutboundCfg{
				Service:  "unix2",
				Filename: "/tmp/test-receptor-unix-outbound2.sock",
			},
		},
		{
			name:                 "Invalid TLS configuration",
			expectError:          true,
			expectedErrorMessage: "unknown TLS config invalid-tls",
			configObj: UnixProxyOutboundCfg{
				Service:  "unix3",
				Filename: "/tmp/test-receptor-unix-outbound3.sock",
				TLS:      "invalid-tls",
			},
		},
	}

	netceptor.MainInstance = netceptor.New(context.Background(), "test_unix_proxy_outbound_cfg_run")

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.configObj.Run()
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
