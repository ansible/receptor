// Code generated by MockGen. DO NOT EDIT.
// Source: pkg/netceptor/netceptor.go

// Package mock_netceptor is a generated GoMock package.
package mock_netceptor

import (
	context "context"
	netceptor "github.com/ansible/receptor/pkg/netceptor"
	gomock "github.com/golang/mock/gomock"
	reflect "reflect"
	sync "sync"
	time "time"
)

// MockBackend is a mock of Backend interface
type MockBackend struct {
	ctrl     *gomock.Controller
	recorder *MockBackendMockRecorder
}

// MockBackendMockRecorder is the mock recorder for MockBackend
type MockBackendMockRecorder struct {
	mock *MockBackend
}

// NewMockBackend creates a new mock instance
func NewMockBackend(ctrl *gomock.Controller) *MockBackend {
	mock := &MockBackend{ctrl: ctrl}
	mock.recorder = &MockBackendMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockBackend) EXPECT() *MockBackendMockRecorder {
	return m.recorder
}

// Start mocks base method
func (m *MockBackend) Start(arg0 context.Context, arg1 *sync.WaitGroup) (chan netceptor.BackendSession, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Start", arg0, arg1)
	ret0, _ := ret[0].(chan netceptor.BackendSession)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Start indicates an expected call of Start
func (mr *MockBackendMockRecorder) Start(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockBackend)(nil).Start), arg0, arg1)
}

// MockBackendSession is a mock of BackendSession interface
type MockBackendSession struct {
	ctrl     *gomock.Controller
	recorder *MockBackendSessionMockRecorder
}

// MockBackendSessionMockRecorder is the mock recorder for MockBackendSession
type MockBackendSessionMockRecorder struct {
	mock *MockBackendSession
}

// NewMockBackendSession creates a new mock instance
func NewMockBackendSession(ctrl *gomock.Controller) *MockBackendSession {
	mock := &MockBackendSession{ctrl: ctrl}
	mock.recorder = &MockBackendSessionMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockBackendSession) EXPECT() *MockBackendSessionMockRecorder {
	return m.recorder
}

// Send mocks base method
func (m *MockBackendSession) Send(arg0 []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Send", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Send indicates an expected call of Send
func (mr *MockBackendSessionMockRecorder) Send(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Send", reflect.TypeOf((*MockBackendSession)(nil).Send), arg0)
}

// Recv mocks base method
func (m *MockBackendSession) Recv(arg0 time.Duration) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Recv", arg0)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Recv indicates an expected call of Recv
func (mr *MockBackendSessionMockRecorder) Recv(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Recv", reflect.TypeOf((*MockBackendSession)(nil).Recv), arg0)
}

// Close mocks base method
func (m *MockBackendSession) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close
func (mr *MockBackendSessionMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockBackendSession)(nil).Close))
}
