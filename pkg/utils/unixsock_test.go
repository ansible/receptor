//go:build !windows
// +build !windows

package utils_test

import (
	"os"
	"testing"

	"github.com/ansible/receptor/pkg/utils"
)

func TestUnixSocketListen(t *testing.T) {
	type args struct {
		filename    string
		permissions os.FileMode
	}
	tests := []struct {
		name    string
		args    args
		want    string
		want1   *utils.FLock
		wantErr bool
	}{
		{
			name: "Good Test",
			args: args{
				filename:    "",
				permissions: 0o0600,
			},
			wantErr: false,
		},
		{
			name: "Bad Test",
			args: args{
				filename:    "/bad_file",
				permissions: 0o0600,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "Good Test" {
				f, err := os.CreateTemp("", "")
				if err != nil {
					t.Errorf("CreateTemp returned %v", err)
				}
				defer os.Remove(f.Name())

				tt.args.filename = f.Name()
			}

			_, _, err := utils.UnixSocketListen(tt.args.filename, tt.args.permissions)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnixSocketListen() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
