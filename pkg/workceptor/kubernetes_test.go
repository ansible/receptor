package workceptor

import (
	"testing"
)

func Test_isCompatibleOCP(t *testing.T) {
	type args struct {
		version      string
		isCompatible bool
	}

	kw := &kubeUnit{}

	tests := []args{
		// OCP compatible versions
		{version: "v4.12.0", isCompatible: true},
		{version: "v4.11.16", isCompatible: true},
		{version: "v4.10.42", isCompatible: true},

		// OCP Z stream >
		{version: "v4.12.99", isCompatible: true},
		{version: "v4.11.99", isCompatible: true},
		{version: "v4.10.99", isCompatible: true},

		// OCP Z stream <
		{version: "v4.11.15", isCompatible: false},
		{version: "v4.10.41", isCompatible: false},

		// OCP X stream >
		{version: "v5.12.0", isCompatible: true},
		{version: "v5.11.16", isCompatible: true},
		{version: "v5.10.42", isCompatible: true},

		// OCP X stream <
		{version: "v3.12.0", isCompatible: false},
		{version: "v3.11.16", isCompatible: false},
		{version: "v3.10.42", isCompatible: false},

		// Other versions
		{version: "yoloswag", isCompatible: false},
		{version: "v4.10.43+sadfasdf", isCompatible: true},
		{version: "v4.10.42-asdfasdf+12131", isCompatible: false},
		{version: "v4.10.43-asdfasdf+12131", isCompatible: true},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			if got := isCompatibleOCP(kw, tt.version); got != tt.isCompatible {
				t.Errorf("isCompatibleOCP() = %v, want %v", got, tt.isCompatible)
			}
		})
	}
}

func Test_isCompatibleK8S(t *testing.T) {
	type args struct {
		version      string
		isCompatible bool
	}

	kw := &kubeUnit{}

	tests := []args{
		// K8S compatible versions
		{version: "v1.24.8", isCompatible: true},
		{version: "v1.25.4", isCompatible: true},
		{version: "v1.23.14", isCompatible: true},

		// K8S Z stream >
		{version: "v1.24.99", isCompatible: true},
		{version: "v1.25.99", isCompatible: true},
		{version: "v1.23.99", isCompatible: true},

		// K8S Z stream <
		{version: "v1.24.7", isCompatible: false},
		{version: "v1.25.3", isCompatible: false},
		{version: "v1.23.13", isCompatible: false},

		// K8S X stream >
		{version: "v2.24.8", isCompatible: true},
		{version: "v2.25.4", isCompatible: true},
		{version: "v2.23.14", isCompatible: true},

		// K8S X stream <
		{version: "v0.24.8", isCompatible: false},
		{version: "v0.25.4", isCompatible: false},
		{version: "v0.23.14", isCompatible: false},

		// Other versions
		{version: "yoloswag", isCompatible: false},
		{version: "v1.23.14+sadfasdf", isCompatible: true},
		{version: "v1.23.14-asdfasdf+12131", isCompatible: false},
		{version: "v1.23.15-asdfasdf+12131", isCompatible: true},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			if got := isCompatibleK8S(kw, tt.version); got != tt.isCompatible {
				t.Errorf("isCompatibleK8S() = %v, want %v", got, tt.isCompatible)
			}
		})
	}
}
