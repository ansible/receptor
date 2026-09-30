//go:build !no_workceptor
// +build !no_workceptor

package workceptor

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/ansible/receptor/pkg/controlsvc"
	"github.com/ansible/receptor/pkg/controlsvc/mock_controlsvc"
	"go.uber.org/mock/gomock"
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

func assertAdoptCommandParams(t *testing.T, got controlsvc.ControlCommand, err error, wantErr bool, want map[string]interface{}) {
	t.Helper()
	if (err != nil) != wantErr {
		t.Fatalf("unexpected error state: got %v, wantErr %v", err, wantErr)
	}
	if wantErr {
		return
	}
	if got == nil {
		t.Fatal("InitFrom... returned nil")
	}
	cmd, ok := got.(*workceptorCommand)
	if !ok {
		t.Fatalf("InitFrom... returned %T, want *workceptorCommand", got)
	}
	if len(cmd.params) != len(want) {
		t.Errorf("params = %#v, want %#v", cmd.params, want)
	}
	for key, value := range want {
		if cmd.params[key] != value {
			t.Errorf("params[%q] = %#v, want %#v", key, cmd.params[key], value)
		}
	}
}

type adoptResultWorkUnit struct {
	WorkUnit
	state   int
	detail  string
	updates int
}

func (w *adoptResultWorkUnit) UpdateBasicStatus(state int, detail string, _ int64) {
	w.state = state
	w.detail = detail
	w.updates++
}

func assertAdoptRestartError(t *testing.T, err error, wantErr bool) {
	t.Helper()
	if (err != nil) != wantErr {
		t.Fatalf("error = %v, wantErr %v", err, wantErr)
	}
}

func assertAdoptRestartResult(t *testing.T, response map[string]interface{}, wantResult string) {
	t.Helper()
	if wantResult != "" && response["result"] != wantResult {
		t.Errorf("result = %#v, want %#v", response["result"], wantResult)
	}
}

func assertAdoptRestartState(t *testing.T, unit *adoptResultWorkUnit, wantState int, wantErr bool) {
	t.Helper()
	if unit.state != wantState {
		t.Errorf("state = %d, want %d", unit.state, wantState)
	}
	if !wantErr {
		return
	}
	if !strings.Contains(unit.detail, "restart failed") {
		t.Errorf("failure detail = %q, want restart failed", unit.detail)
	}
	if unit.updates != 1 {
		t.Errorf("failure status updates = %d, want 1", unit.updates)
	}
}

func TestFinalizeAdoptRestart(t *testing.T) {
	tests := []struct {
		name       string
		restartErr error
		wantResult string
		wantErr    bool
		wantState  int
	}{
		{name: "started", wantResult: "Adopted"},
		{name: "pending", restartErr: ErrPending, wantResult: "Adopt Pending"},
		{name: "failed", restartErr: fmt.Errorf("restart failed"), wantErr: true, wantState: WorkStateFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unit := &adoptResultWorkUnit{}
			response, err := finalizeAdoptRestart(unit, make(map[string]interface{}), tt.restartErr)
			assertAdoptRestartError(t, err, tt.wantErr)
			assertAdoptRestartResult(t, response, tt.wantResult)
			assertAdoptRestartState(t, unit, tt.wantState, tt.wantErr)
		})
	}
}

func TestWorkceptorControlFuncAdoptRequiresNodeAndUnitID(t *testing.T) {
	tests := []struct {
		name       string
		params     map[string]interface{}
		wantErrMsg string
	}{
		{name: "missing node", params: map[string]interface{}{"unitid": "unit"}, wantErrMsg: "field node missing"},
		{name: "missing unit ID", params: map[string]interface{}{"node": "node"}, wantErrMsg: "field unitid missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			cfo := mock_controlsvc.NewMockControlFuncOperations(ctrl)
			cfo.EXPECT().RemoteAddr().Return(&net.UnixAddr{Net: "unix"})
			cmd := &workceptorCommand{subcommand: "adopt", params: tt.params}
			_, err := cmd.ControlFunc(context.Background(), nil, cfo)
			if err == nil || err.Error() != tt.wantErrMsg {
				t.Fatalf("ControlFunc error = %v, want %q", err, tt.wantErrMsg)
			}
		})
	}
}

func Test_workceptorCommandTypeInitFromString_AdoptParams(t *testing.T) {
	tests := []struct {
		name    string
		params  string
		want    map[string]interface{}
		wantErr bool
	}{
		{name: "required parameters", params: "adopt node1 unit123", want: map[string]interface{}{"node": "node1", "unitid": "unit123"}},
		{name: "extra parameters", params: "adopt node1 unit123 signature=abc extra", want: map[string]interface{}{"node": "node1", "unitid": "unit123", "params": "signature=abc extra"}},
		{name: "missing node and unit ID", params: "adopt", wantErr: true},
		{name: "missing unit ID", params: "adopt node1", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := (&workceptorCommandType{}).InitFromString(tt.params)
			assertAdoptCommandParams(t, got, err, tt.wantErr, tt.want)
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
		name    string
		config  map[string]interface{}
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name: "all parameters",
			config: map[string]interface{}{
				"subcommand": "adopt",
				"node":       "node1",
				"unitid":     "unit123",
				"signwork":   "true",
				"signature":  "abc123",
				"tlsclient":  "client1",
			},
			want: map[string]interface{}{
				"node": "node1", "unitid": "unit123", "signwork": "true", "signature": "abc123", "tlsclient": "client1",
			},
		},
		{
			name: "false signwork",
			config: map[string]interface{}{
				"subcommand": "adopt",
				"node":       "node2",
				"unitid":     "unit456",
				"signwork":   "false",
			},
			want: map[string]interface{}{"node": "node2", "unitid": "unit456", "signwork": "false"},
		},
		{
			name: "minimal parameters",
			config: map[string]interface{}{
				"subcommand": "adopt",
				"node":       "node3",
				"unitid":     "unit789",
			},
			want: map[string]interface{}{"node": "node3", "unitid": "unit789"},
		},
		{
			name:    "missing node",
			config:  map[string]interface{}{"subcommand": "adopt", "unitid": "unit123"},
			wantErr: true,
		},
		{
			name:    "missing unit ID",
			config:  map[string]interface{}{"subcommand": "adopt", "node": "node1"},
			wantErr: true,
		},
		{
			name:    "wrong required field types",
			config:  map[string]interface{}{"subcommand": "adopt", "node": 1, "unitid": true},
			wantErr: true,
		},
		{
			name: "wrong optional field types are ignored",
			config: map[string]interface{}{
				"subcommand": "adopt", "node": "node1", "unitid": "unit123",
				"tlsclient": 1, "signwork": true, "signature": 123,
			},
			want: map[string]interface{}{"node": "node1", "unitid": "unit123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := (&workceptorCommandType{}).InitFromJSON(tt.config)
			assertAdoptCommandParams(t, got, err, tt.wantErr, tt.want)
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
