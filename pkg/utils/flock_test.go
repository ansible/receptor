//go:build !windows
// +build !windows

package utils

import (
	"reflect"
	"testing"
)

func TestTryFLock(t *testing.T) {
	type args struct {
		filename string
	}
	tests := []struct {
		name    string
		args    args
		want    *FLock
		wantErr bool
	}{
		{
			name:		"Positive",
			args:		args{
				filename:	"",
						},
			want:		&FLock{},
			wantErr:	false,
		},
		{
			name:	"Negative",
			args:		args{
				filename:	"",
						},
			want:		&FLock{},
			wantErr:	true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TryFLock(tt.args.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("TryFLock() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TryFLock() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFLock_Unlock(t *testing.T) {
	type fields struct {
		fd int
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name:		"Positive",
			fields:		fields{
				fd: 	1,
			},
			wantErr:	false,
		},
		{
			name:		"Negative",
			fields:		fields{
				fd: 	-1,
			},
			wantErr:	true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lock := &FLock{
				fd: tt.fields.fd,
			}
			if err := lock.Unlock(); (err != nil) != tt.wantErr {
				t.Errorf("FLock.Unlock() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
