// Code generated by MockGen. DO NOT EDIT.
// Source: pkg/workceptor/stdio_utils.go

// Package mock_workceptor is a generated GoMock package.
package mock_workceptor

import (
	gomock "go.uber.org/mock/gomock"
	os "os"
	reflect "reflect"
)

// MockFileSystemer is a mock of FileSystemer interface
type MockFileSystemer struct {
	ctrl     *gomock.Controller
	recorder *MockFileSystemerMockRecorder
}

// MockFileSystemerMockRecorder is the mock recorder for MockFileSystemer
type MockFileSystemerMockRecorder struct {
	mock *MockFileSystemer
}

// NewMockFileSystemer creates a new mock instance
func NewMockFileSystemer(ctrl *gomock.Controller) *MockFileSystemer {
	mock := &MockFileSystemer{ctrl: ctrl}
	mock.recorder = &MockFileSystemerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockFileSystemer) EXPECT() *MockFileSystemerMockRecorder {
	return m.recorder
}

// OpenFile mocks base method
func (m *MockFileSystemer) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "OpenFile", name, flag, perm)
	ret0, _ := ret[0].(*os.File)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// OpenFile indicates an expected call of OpenFile
func (mr *MockFileSystemerMockRecorder) OpenFile(name, flag, perm interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "OpenFile", reflect.TypeOf((*MockFileSystemer)(nil).OpenFile), name, flag, perm)
}

// Stat mocks base method
func (m *MockFileSystemer) Stat(name string) (os.FileInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stat", name)
	ret0, _ := ret[0].(os.FileInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Stat indicates an expected call of Stat
func (mr *MockFileSystemerMockRecorder) Stat(name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stat", reflect.TypeOf((*MockFileSystemer)(nil).Stat), name)
}

// Open mocks base method
func (m *MockFileSystemer) Open(name string) (*os.File, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Open", name)
	ret0, _ := ret[0].(*os.File)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Open indicates an expected call of Open
func (mr *MockFileSystemerMockRecorder) Open(name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Open", reflect.TypeOf((*MockFileSystemer)(nil).Open), name)
}

// RemoveAll mocks base method
func (m *MockFileSystemer) RemoveAll(path string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemoveAll", path)
	ret0, _ := ret[0].(error)
	return ret0
}

// RemoveAll indicates an expected call of RemoveAll
func (mr *MockFileSystemerMockRecorder) RemoveAll(path interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveAll", reflect.TypeOf((*MockFileSystemer)(nil).RemoveAll), path)
}

// MockFileWriteCloser is a mock of FileWriteCloser interface
type MockFileWriteCloser struct {
	ctrl     *gomock.Controller
	recorder *MockFileWriteCloserMockRecorder
}

// MockFileWriteCloserMockRecorder is the mock recorder for MockFileWriteCloser
type MockFileWriteCloserMockRecorder struct {
	mock *MockFileWriteCloser
}

// NewMockFileWriteCloser creates a new mock instance
func NewMockFileWriteCloser(ctrl *gomock.Controller) *MockFileWriteCloser {
	mock := &MockFileWriteCloser{ctrl: ctrl}
	mock.recorder = &MockFileWriteCloserMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockFileWriteCloser) EXPECT() *MockFileWriteCloserMockRecorder {
	return m.recorder
}

// Write mocks base method
func (m *MockFileWriteCloser) Write(p []byte) (int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Write", p)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Write indicates an expected call of Write
func (mr *MockFileWriteCloserMockRecorder) Write(p interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Write", reflect.TypeOf((*MockFileWriteCloser)(nil).Write), p)
}

// Close mocks base method
func (m *MockFileWriteCloser) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close
func (mr *MockFileWriteCloserMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockFileWriteCloser)(nil).Close))
}

// MockFileReadCloser is a mock of FileReadCloser interface
type MockFileReadCloser struct {
	ctrl     *gomock.Controller
	recorder *MockFileReadCloserMockRecorder
}

// MockFileReadCloserMockRecorder is the mock recorder for MockFileReadCloser
type MockFileReadCloserMockRecorder struct {
	mock *MockFileReadCloser
}

// NewMockFileReadCloser creates a new mock instance
func NewMockFileReadCloser(ctrl *gomock.Controller) *MockFileReadCloser {
	mock := &MockFileReadCloser{ctrl: ctrl}
	mock.recorder = &MockFileReadCloserMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockFileReadCloser) EXPECT() *MockFileReadCloserMockRecorder {
	return m.recorder
}

// Read mocks base method
func (m *MockFileReadCloser) Read(p []byte) (int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Read", p)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Read indicates an expected call of Read
func (mr *MockFileReadCloserMockRecorder) Read(p interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Read", reflect.TypeOf((*MockFileReadCloser)(nil).Read), p)
}

// Close mocks base method
func (m *MockFileReadCloser) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close
func (mr *MockFileReadCloserMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockFileReadCloser)(nil).Close))
}
