//go:build !windows
// +build !windows

package utils_test

import (
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/ansible/receptor/pkg/utils"
	"golang.org/x/sys/unix"
)

func TestUnixSocketListen(t *testing.T) {
	type args struct {
		filename    string
		permissions os.FileMode
	}

	badFilename := ""

	tests := []struct {
		name    string
		args    args
		want    net.Listener
		want1   *utils.FLock
		wantErr bool
	}{
		{
			name: "Positive",
			args: args{
				filename:    filepath.Join(os.TempDir(), "good_listener"),
				permissions: 0x0400,
			},
			wantErr: false,
		},
		{
			name: "Negative",
			args: args{
				filename:    badFilename,
				permissions: 0x0000,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lockFilename := tt.args.filename + ".lock"
			defer os.Remove(lockFilename)

			_, got1, err := utils.UnixSocketListen(tt.args.filename, tt.args.permissions)
			if (err != nil) != tt.wantErr {
				t.Errorf("%s: UnixSocketListen(): error = %+v, wantErr = %+v", tt.name, err, tt.wantErr)
				return
			}

			if err == nil {
				if got1.Fd < 0 {
					t.Errorf("%s: UnixSocketListen(): Invalid got1 fd = %+v", tt.name, got1)
				}

				defer got1.Unlock()

				err = unix.Flock(got1.Fd, unix.LOCK_EX)
				if err != nil {
					t.Errorf("%s: UnixSocketListen(): Test lock error = %+v", tt.name, err)
				}

				gotFileInfo, err := os.Stat(tt.args.filename)
				if err != nil {
					t.Errorf("%s: UnixSocketListen(): Stat error = %+v", tt.name, err)
				}

				gotPermissions := gotFileInfo.Mode() & fs.ModePerm

				wantPermissions := tt.args.permissions & fs.ModePerm
				if gotPermissions != wantPermissions {
					t.Errorf("%s: UnixSocketListen(): Got permission = %d, want permissions = %d", tt.name, gotPermissions, wantPermissions)
				}
			}
		})
	}
}
