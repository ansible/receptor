package utils_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/ansible/receptor/pkg/utils"
)

const goodKind string = "connection"
const goodErrorString string = "unit was already started"

var goodError error = fmt.Errorf(goodErrorString)

func TestErrorWithKind_Error(t *testing.T) {
	type fields struct {
		err  error
		kind string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "Positive",
			fields: fields{
				err:  goodError,
				kind: goodKind,
			},
			want: fmt.Sprintf("%s error: %s", goodKind, goodErrorString),
		},
		{
			name: "Negative",
			fields: fields{
				err:  nil,
				kind: goodKind,
			},
			want: fmt.Sprintf("%s error: <nil>", goodKind),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ek := utils.ErrorWithKind{
				Err:  tt.fields.err,
				Kind: tt.fields.kind,
			}
			if got := ek.Error(); got != tt.want {
				t.Errorf("ErrorWithKind.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWrapErrorWithKind(t *testing.T) {
	type args struct {
		err  error
		kind string
	}
	tests := []struct {
		name string
		args args
		want utils.ErrorWithKind
	}{
		{
			name: "Positive",
			args: args{
				err:  goodError,
				kind: goodKind,
			},
			want: utils.ErrorWithKind{
				Err:  goodError,
				Kind: goodKind,
			},
		},
		{
			name: "Negative",
			args: args{
				err:  nil,
				kind: goodKind,
			},
			want: utils.ErrorWithKind{
				Err:  nil,
				Kind: goodKind,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := utils.WrapErrorWithKind(tt.args.err, tt.args.kind); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("WrapErrorWithKind() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrorIsKind(t *testing.T) {
	type args struct {
		err  error
		kind string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Positive",
			args: args{
				err: utils.WrapErrorWithKind(
					goodError,
					goodKind,
				),
				kind: goodKind,
			},
			want: true,
		},
		{
			name: "Negative",
			args: args{
				err:  nil,
				kind: goodKind,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := utils.ErrorIsKind(tt.args.err, tt.args.kind); got != tt.want {
				t.Errorf("ErrorIsKind() = %v, want %v", got, tt.want)
			}
		})
	}
}
