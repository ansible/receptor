// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/ansible/receptor/pkg/certificates (interfaces: Oser)
//
// Generated by this command:
//
//	mockgen -destination=mock_certificates/Oser.go github.com/ansible/receptor/pkg/certificates Oser
//

// Package mock_certificates is a generated GoMock package.
package mock_certificates

import (
	fs "io/fs"
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
)

// MockOser is a mock of Oser interface.
type MockOser struct {
	ctrl     *gomock.Controller
	recorder *MockOserMockRecorder
}

// MockOserMockRecorder is the mock recorder for MockOser.
type MockOserMockRecorder struct {
	mock *MockOser
}

// NewMockOser creates a new mock instance.
func NewMockOser(ctrl *gomock.Controller) *MockOser {
	mock := &MockOser{ctrl: ctrl}
	mock.recorder = &MockOserMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockOser) EXPECT() *MockOserMockRecorder {
	return m.recorder
}

// ReadFile mocks base method.
func (m *MockOser) ReadFile(arg0 string) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadFile", arg0)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadFile indicates an expected call of ReadFile.
func (mr *MockOserMockRecorder) ReadFile(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadFile", reflect.TypeOf((*MockOser)(nil).ReadFile), arg0)
}

// WriteFile mocks base method.
func (m *MockOser) WriteFile(arg0 string, arg1 []byte, arg2 fs.FileMode) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WriteFile", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// WriteFile indicates an expected call of WriteFile.
func (mr *MockOserMockRecorder) WriteFile(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WriteFile", reflect.TypeOf((*MockOser)(nil).WriteFile), arg0, arg1, arg2)
}