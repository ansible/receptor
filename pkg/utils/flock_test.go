//go:build !windows
// +build !windows

package utils_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ansible/receptor/pkg/utils"
)

func TestTryFLock(t *testing.T) {
	type args struct {
		filename string
	}
	tests := []struct {
		name    string
		args    args
		want    *utils.FLock
		wantErr bool
	}{
		{
			name: "Positive",
			args: args{
				filename: filepath.Join(os.TempDir(), "good_flock_listener"),
			},
			want:    &utils.FLock{0},
			wantErr: false,
		},
		{
			name: "Negative",
			args: args{
				filename: "",
			},
			want:    &utils.FLock{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := utils.TryFLock(tt.args.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("%s: TryFLock(): error = %v, wantErr %v", tt.name, err, tt.wantErr)

				return
			}

			if err == nil {
				if got.Fd < 0 {
					t.Errorf("%s: UnixSocketListen(): Invalid got Fd = %+v", tt.name, got)
				}
			}
		})
	}
}

func TestFLock_Unlock(t *testing.T) {
	type fields struct {
		Fd int
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "Positive",
			fields: fields{
				Fd: 1,
			},
			wantErr: false,
		},
		{
			name: "Negative",
			fields: fields{
				Fd: -1,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lock := &utils.FLock{
				Fd: tt.fields.Fd,
			}
			if err := lock.Unlock(); (err != nil) != tt.wantErr {
				t.Errorf("%s: FLock.Unlock() error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}
