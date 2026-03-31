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

func Test_strFromMap(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]interface{}
		field   string
		want    string
		wantErr bool
	}{
		{
			name: "Valid string field",
			config: map[string]interface{}{
				"name": "test-value",
			},
			field:   "name",
			want:    "test-value",
			wantErr: false,
		},
		{
			name:    "Missing field",
			config:  map[string]interface{}{},
			field:   "name",
			want:    "",
			wantErr: true,
		},
		{
			name: "Field is not a string",
			config: map[string]interface{}{
				"name": 123,
			},
			field:   "name",
			want:    "",
			wantErr: true,
		},
		{
			name: "Field is bool not string",
			config: map[string]interface{}{
				"name": true,
			},
			field:   "name",
			want:    "",
			wantErr: true,
		},
		{
			name: "Empty string value",
			config: map[string]interface{}{
				"name": "",
			},
			field:   "name",
			want:    "",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := strFromMap(tt.config, tt.field)
			if (err != nil) != tt.wantErr {
				t.Errorf("strFromMap() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if got != tt.want {
				t.Errorf("strFromMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_intFromMap(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]interface{}
		field   string
		want    int64
		wantErr bool
	}{
		{
			name: "Valid int64 field",
			config: map[string]interface{}{
				"count": int64(42),
			},
			field:   "count",
			want:    42,
			wantErr: false,
		},
		{
			name: "Valid float64 field",
			config: map[string]interface{}{
				"count": float64(100.0),
			},
			field:   "count",
			want:    100,
			wantErr: false,
		},
		{
			name: "Valid string field",
			config: map[string]interface{}{
				"count": "256",
			},
			field:   "count",
			want:    256,
			wantErr: false,
		},
		{
			name: "Float64 with decimals",
			config: map[string]interface{}{
				"count": float64(99.7),
			},
			field:   "count",
			want:    99,
			wantErr: false,
		},
		{
			name:    "Missing field",
			config:  map[string]interface{}{},
			field:   "count",
			want:    0,
			wantErr: true,
		},
		{
			name: "Invalid string value",
			config: map[string]interface{}{
				"count": "not-a-number",
			},
			field:   "count",
			want:    0,
			wantErr: true,
		},
		{
			name: "Non-convertible type",
			config: map[string]interface{}{
				"count": true,
			},
			field:   "count",
			want:    0,
			wantErr: true,
		},
		{
			name: "Empty string value",
			config: map[string]interface{}{
				"count": "",
			},
			field:   "count",
			want:    0,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := intFromMap(tt.config, tt.field)
			if (err != nil) != tt.wantErr {
				t.Errorf("intFromMap() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if got != tt.want {
				t.Errorf("intFromMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_boolFromMap(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]interface{}
		field   string
		want    bool
		wantErr bool
	}{
		{
			name: "Valid true string",
			config: map[string]interface{}{
				"enabled": "true",
			},
			field:   "enabled",
			want:    true,
			wantErr: false,
		},
		{
			name: "Valid false string",
			config: map[string]interface{}{
				"enabled": "false",
			},
			field:   "enabled",
			want:    false,
			wantErr: false,
		},
		{
			name:    "Missing field",
			config:  map[string]interface{}{},
			field:   "enabled",
			want:    false,
			wantErr: true,
		},
		{
			name: "Field is not a string",
			config: map[string]interface{}{
				"enabled": true,
			},
			field:   "enabled",
			want:    false,
			wantErr: true,
		},
		{
			name: "Invalid bool string",
			config: map[string]interface{}{
				"enabled": "yes",
			},
			field:   "enabled",
			want:    false,
			wantErr: true,
		},
		{
			name: "Invalid bool string - numeric",
			config: map[string]interface{}{
				"enabled": "1",
			},
			field:   "enabled",
			want:    false,
			wantErr: true,
		},
		{
			name: "Empty string value",
			config: map[string]interface{}{
				"enabled": "",
			},
			field:   "enabled",
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := boolFromMap(tt.config, tt.field)
			if (err != nil) != tt.wantErr {
				t.Errorf("boolFromMap() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if got != tt.want {
				t.Errorf("boolFromMap() = %v, want %v", got, tt.want)
			}
		})
	}
}
