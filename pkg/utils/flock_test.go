//go:build !windows
// +build !windows

package utils_test

import (
	"testing"

	"github.com/ansible/receptor/pkg/utils"
	"github.com/ansible/receptor/pkg/utils/mock_utils"
	"go.uber.org/mock/gomock"
)

func TestTryFLock(t *testing.T) {
	const goodTest = "Good Test"
	const closeErrorTest = "Close Error"
	const flockErrorTest = "Flock Error"
	const openErrorTest = "Open Error"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

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
			name: goodTest,
			args: args{
				filename: "",
			},
			wantErr: false,
		},
		{
			name: openErrorTest,
			args: args{
				filename: "",
			},
			wantErr: true,
		},
		{
			name: flockErrorTest,
			args: args{
				filename: "",
			},
			wantErr: true,
		},
		{
			name: closeErrorTest,
			args: args{
				filename: "",
			},
			wantErr: true,
		},
	}
	t.Parallel()

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockSys := mock_utils.NewMockSyscaller(ctrl)

			_, err := utils.TryFLock(utils.SyscallImpl{}, tt.args.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("TryFLock() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
		})
	}
}

func TestFLockUnlock(t *testing.T) {
	const goodTest = "Good Test"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name             string
		wantErr          bool
		wantErrorMessage error
	}{
		{
			name:             goodTest,
			wantErr:          false,
			wantErrorMessage: nil,
		},
	}

	t.Parallel()

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockSys := mock_utils.NewMockSyscaller(ctrl)
			mockSys.EXPECT().Close(gomock.Any()).Return(tt.wantErrorMessage)
			if err := utils.Unlock(mockSys); (err != nil) != tt.wantErr {
				t.Errorf("Unlock() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
