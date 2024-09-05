package backends

import "testing"

func TestNullBackendCfgGetAddr(t *testing.T) {
	type fields struct {
		Local bool
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "Positive",
			fields: fields{
				Local: true,
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &NullBackendCfg{
				Local: tt.fields.Local,
			}
			if got := cfg.GetAddr(); got != tt.want {
				t.Errorf("NullBackendCfg.GetAddr() = %v, want %v", got, tt.want)
			}
		})
	}
}
