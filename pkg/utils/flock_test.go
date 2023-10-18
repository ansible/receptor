//go:build !windows
// +build !windows

package utils_test

import (
	"os"
	//"reflect"
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
			name: "Good Test",
			args: args{
				filename: "",
			},
			wantErr: false,
		},
		{
			name: "Bad Test",
			args: args{
				filename: "",
			},
			// want:    nil,
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

			_, err := utils.TryFLock(tt.args.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("TryFLock() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
		})
	}
}

func TestFLockUnlock(t *testing.T) {
	type fields struct {
		fd int
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name:    "Good Test",
			wantErr: false,
		},
		{
			name: "bad",

			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.CreateTemp("", "")
			if err != nil {
				t.Errorf("CreateTemp returned %v", err)
			}
			defer os.Remove(f.Name())

			flock, _ := utils.TryFLock(f.Name())
		
			if tt.name == "Good Test " {
				if err := flock.Unlock(); (err != nil) != tt.wantErr {
					t.Errorf("Unlock() error = %v, wantErr %v", err, tt.wantErr)
				}
			} else if tt.name == "Bad Test " {
				_ = flock.Unlock()
				if err = flock.Unlock(); (err != nil) != tt.wantErr {
					t.Errorf("Unlock() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}
