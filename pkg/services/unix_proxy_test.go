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
