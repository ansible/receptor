//go:build !windows
// +build !windows

package utils_test

import (
	"os"
	"path/filepath"
	"strconv"
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
			want:    &utils.FLock{Fd: 0},
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
	f, err := os.CreateTemp("", "flock-test")
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(f.Name())
	defer f.Close()

	var maxInt uintptr
	if strconv.IntSize == 32 {
		maxInt = uintptr(1<<31 - 1)
	} else {
		maxInt = uintptr(1<<63 - 1)
	}

	fd := f.Fd()
	if fd > maxInt {
		t.Error(err)
	}

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
				Fd: int(f.Fd()), // #nosec G115
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
