//go:build !no_workceptor
// +build !no_workceptor

package workceptor

import (
	"testing"

	"github.com/ansible/receptor/pkg/controlsvc"
)

func Test_workceptorCommandTypeInitFromString(t *testing.T) {
	type fields struct {
		w *Workceptor
	}
	type args struct {
		params string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    controlsvc.ControlCommand
		wantErr bool
	}{
		{
			name: "Positive cancel",
			fields: fields{
				w: nil,
			},
			args: args{
				params: "cancel u",
			},
			wantErr: false,
		},
		{
			name: "Positive force-release",
			fields: fields{
				w: nil,
			},
			args: args{
				params: "force-release u",
			},
			wantErr: false,
		},
		{
			name: "Positive list",
			fields: fields{
				w: nil,
			},
			args: args{
				params: "list",
			},
			wantErr: false,
		},
		{
			name: "Positive release",
			fields: fields{
				w: nil,
			},
			args: args{
				params: "release u",
			},
			wantErr: false,
		},
		{
			name: "Positive results",
			fields: fields{
				w: nil,
			},
			args: args{
				params: "results u",
			},
			wantErr: false,
		},
		{
			name: "Positive status",
			fields: fields{
				w: nil,
			},
			args: args{
				params: "status u",
			},
			wantErr: false,
		},
		{
			name: "Positive submit",
			fields: fields{
				w: nil,
			},
			args: args{
				params: "submit n w",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &workceptorCommandType{
				w: tt.fields.w,
			}
			got, err := tr.InitFromString(tt.args.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("workceptorCommandType.InitFromString() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if got == nil {
				t.Errorf("workceptorCommandType.InitFromString() returned nil")
			}
		})
	}
}
