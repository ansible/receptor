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
		{
			name: "Positive adopt without extra params",
			fields: fields{
				w: nil,
			},
			args: args{
				params: "adopt node1 unit123",
			},
			wantErr: false,
		},
		{
			name: "Positive adopt with extra params",
			fields: fields{
				w: nil,
			},
			args: args{
				params: "adopt node1 unit123 extra param data",
			},
			wantErr: false,
		},
		{
			name: "Results with negative startpos",
			fields: fields{
				w: nil,
			},
			args: args{
				params: "results unit123 -50",
			},
			wantErr: false,
		},
		{
			name: "Results with positive startpos",
			fields: fields{
				w: nil,
			},
			args: args{
				params: "results unit123 100",
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

func Test_workceptorCommandTypeInitFromString_AdoptParams(t *testing.T) {
	tests := []struct {
		name            string
		params          string
		wantNode        string
		wantUnitID      string
		wantExtraParams string
		wantErr         bool
	}{
		{
			name:            "Adopt without extra params",
			params:          "adopt node1 unit123",
			wantNode:        "node1",
			wantUnitID:      "unit123",
			wantExtraParams: "",
			wantErr:         false,
		},
		{
			name:            "Adopt with single extra param",
			params:          "adopt node1 unit123 signature=abc123",
			wantNode:        "node1",
			wantUnitID:      "unit123",
			wantExtraParams: "signature=abc123",
			wantErr:         false,
		},
		{
			name:            "Adopt with multiple extra params",
			params:          "adopt node1 unit123 param1 param2 param3",
			wantNode:        "node1",
			wantUnitID:      "unit123",
			wantExtraParams: "param1 param2 param3",
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &workceptorCommandType{w: nil}
			got, err := tr.InitFromString(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("InitFromString() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if got == nil {
				t.Fatalf("InitFromString() returned nil")
			}

			cmd := got.(*workceptorCommand)

			if node, ok := cmd.params["node"].(string); ok {
				if node != tt.wantNode {
					t.Errorf("node = %v, want %v", node, tt.wantNode)
				}
			} else {
				t.Errorf("node not found in params")
			}

			if unitID, ok := cmd.params["unitid"].(string); ok {
				if unitID != tt.wantUnitID {
					t.Errorf("unitid = %v, want %v", unitID, tt.wantUnitID)
				}
			} else {
				t.Errorf("unitid not found in params")
			}

			if tt.wantExtraParams != "" {
				if extraParams, ok := cmd.params["params"].(string); ok {
					if extraParams != tt.wantExtraParams {
						t.Errorf("params = %v, want %v", extraParams, tt.wantExtraParams)
					}
				} else {
					t.Errorf("params not found in cmd.params, wanted %v", tt.wantExtraParams)
				}
			} else {
				if _, ok := cmd.params["params"]; ok {
					t.Errorf("params should not be present when no extra params provided")
				}
			}
		})
	}
}

func Test_workceptorCommandTypeInitFromString_ResultsStartpos(t *testing.T) {
	tests := []struct {
		name         string
		params       string
		wantStartpos int64
		wantErr      bool
	}{
		{
			name:         "Results without startpos",
			params:       "results unit123",
			wantStartpos: 0,
			wantErr:      false,
		},
		{
			name:         "Results with positive startpos",
			params:       "results unit123 100",
			wantStartpos: 100,
			wantErr:      false,
		},
		{
			name:         "Results with zero startpos",
			params:       "results unit123 0",
			wantStartpos: 0,
			wantErr:      false,
		},
		{
			name:         "Results with negative startpos clamped to zero",
			params:       "results unit123 -50",
			wantStartpos: 0,
			wantErr:      false,
		},
		{
			name:         "Results with large negative startpos clamped to zero",
			params:       "results unit123 -999999",
			wantStartpos: 0,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &workceptorCommandType{w: nil}
			got, err := tr.InitFromString(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("InitFromString() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if got == nil {
				t.Fatalf("InitFromString() returned nil")
			}

			cmd := got.(*workceptorCommand)

			if startpos, ok := cmd.params["startpos"].(int64); ok {
				if startpos != tt.wantStartpos {
					t.Errorf("startpos = %v, want %v", startpos, tt.wantStartpos)
				}
			} else {
				t.Errorf("startpos not found in params or wrong type")
			}
		})
	}
}

func Test_workceptorCommandTypeInitFromJSON_AdoptParams(t *testing.T) {
	tests := []struct {
		name          string
		config        map[string]interface{}
		wantNode      string
		wantUnitID    string
		wantSignWork  string
		wantSignature string
		wantTLSClient string
		wantErr       bool
	}{
		{
			name: "Adopt with all params",
			config: map[string]interface{}{
				"subcommand": "adopt",
				"node":       "node1",
				"unitid":     "unit123",
				"signwork":   "true",
				"signature":  "abc123",
				"tlsclient":  "client1",
			},
			wantNode:      "node1",
			wantUnitID:    "unit123",
			wantSignWork:  "true",
			wantSignature: "abc123",
			wantTLSClient: "client1",
			wantErr:       false,
		},
		{
			name: "Adopt with signwork false",
			config: map[string]interface{}{
				"subcommand": "adopt",
				"node":       "node2",
				"unitid":     "unit456",
				"signwork":   "false",
			},
			wantNode:     "node2",
			wantUnitID:   "unit456",
			wantSignWork: "false",
			wantErr:      false,
		},
		{
			name: "Adopt with minimal params",
			config: map[string]interface{}{
				"subcommand": "adopt",
				"node":       "node3",
				"unitid":     "unit789",
			},
			wantNode:   "node3",
			wantUnitID: "unit789",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &workceptorCommandType{w: nil}
			got, err := tr.InitFromJSON(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("InitFromJSON() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if got == nil {
				t.Fatalf("InitFromJSON() returned nil")
			}

			cmd := got.(*workceptorCommand)

			if node, ok := cmd.params["node"].(string); ok {
				if node != tt.wantNode {
					t.Errorf("node = %v, want %v", node, tt.wantNode)
				}
			} else {
				t.Errorf("node not found in params")
			}

			if unitID, ok := cmd.params["unitid"].(string); ok {
				if unitID != tt.wantUnitID {
					t.Errorf("unitid = %v, want %v", unitID, tt.wantUnitID)
				}
			} else {
				t.Errorf("unitid not found in params")
			}

			if tt.wantSignWork != "" {
				if signWork, ok := cmd.params["signwork"].(string); ok {
					if signWork != tt.wantSignWork {
						t.Errorf("signwork = %v, want %v", signWork, tt.wantSignWork)
					}
				} else {
					t.Errorf("signwork not found in params or wrong type, wanted %v", tt.wantSignWork)
				}
			}

			if tt.wantSignature != "" {
				if signature, ok := cmd.params["signature"].(string); ok {
					if signature != tt.wantSignature {
						t.Errorf("signature = %v, want %v", signature, tt.wantSignature)
					}
				} else {
					t.Errorf("signature not found in params, wanted %v", tt.wantSignature)
				}
			}

			if tt.wantTLSClient != "" {
				if tlsClient, ok := cmd.params["tlsclient"].(string); ok {
					if tlsClient != tt.wantTLSClient {
						t.Errorf("tlsclient = %v, want %v", tlsClient, tt.wantTLSClient)
					}
				} else {
					t.Errorf("tlsclient not found in params, wanted %v", tt.wantTLSClient)
				}
			}
		})
	}
}

func Test_workceptorCommandTypeInitFromJSON_ResultsStartpos(t *testing.T) {
	tests := []struct {
		name         string
		config       map[string]interface{}
		wantStartpos int64
		wantErr      bool
	}{
		{
			name: "Results with positive startpos",
			config: map[string]interface{}{
				"subcommand": "results",
				"unitid":     "unit123",
				"startpos":   int64(100),
			},
			wantStartpos: 100,
			wantErr:      false,
		},
		{
			name: "Results with zero startpos",
			config: map[string]interface{}{
				"subcommand": "results",
				"unitid":     "unit123",
				"startpos":   int64(0),
			},
			wantStartpos: 0,
			wantErr:      false,
		},
		{
			name: "Results with negative startpos clamped to zero",
			config: map[string]interface{}{
				"subcommand": "results",
				"unitid":     "unit123",
				"startpos":   int64(-50),
			},
			wantStartpos: 0,
			wantErr:      false,
		},
		{
			name: "Results with large negative startpos clamped to zero",
			config: map[string]interface{}{
				"subcommand": "results",
				"unitid":     "unit123",
				"startpos":   int64(-999999),
			},
			wantStartpos: 0,
			wantErr:      false,
		},
		{
			name: "Results without startpos defaults to zero",
			config: map[string]interface{}{
				"subcommand": "results",
				"unitid":     "unit123",
			},
			wantStartpos: 0,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &workceptorCommandType{w: nil}
			got, err := tr.InitFromJSON(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("InitFromJSON() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if got == nil {
				t.Fatalf("InitFromJSON() returned nil")
			}

			cmd := got.(*workceptorCommand)

			if startpos, ok := cmd.params["startpos"].(int64); ok {
				if startpos != tt.wantStartpos {
					t.Errorf("startpos = %v, want %v", startpos, tt.wantStartpos)
				}
			} else {
				t.Errorf("startpos not found in params or wrong type")
			}
		})
	}
}
