// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/ansible/receptor/pkg/controlsvc (interfaces: NetceptorForControlsvc,Utiler,Neter,Tlser)

// Package mock_controlsvc is a generated GoMock package.
package mock_controlsvc

import (
	context "context"
	tls "crypto/tls"
	io "io"
	fs "io/fs"
	net "net"
	reflect "reflect"
	time "time"

	logger "github.com/ansible/receptor/pkg/logger"
	netceptor "github.com/ansible/receptor/pkg/netceptor"
	utils "github.com/ansible/receptor/pkg/utils"
	gomock "github.com/golang/mock/gomock"
)

// MockNetceptorForControlsvc is a mock of NetceptorForControlsvc interface.
type MockNetceptorForControlsvc struct {
	ctrl     *gomock.Controller
	recorder *MockNetceptorForControlsvcMockRecorder
}

// MockNetceptorForControlsvcMockRecorder is the mock recorder for MockNetceptorForControlsvc.
type MockNetceptorForControlsvcMockRecorder struct {
	mock *MockNetceptorForControlsvc
}

// NewMockNetceptorForControlsvc creates a new mock instance.
func NewMockNetceptorForControlsvc(ctrl *gomock.Controller) *MockNetceptorForControlsvc {
	mock := &MockNetceptorForControlsvc{ctrl: ctrl}
	mock.recorder = &MockNetceptorForControlsvcMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockNetceptorForControlsvc) EXPECT() *MockNetceptorForControlsvcMockRecorder {
	return m.recorder
}

// CancelBackends mocks base method.
func (m *MockNetceptorForControlsvc) CancelBackends() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "CancelBackends")
}

// CancelBackends indicates an expected call of CancelBackends.
func (mr *MockNetceptorForControlsvcMockRecorder) CancelBackends() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CancelBackends", reflect.TypeOf((*MockNetceptorForControlsvc)(nil).CancelBackends))
}

// Dial mocks base method.
func (m *MockNetceptorForControlsvc) Dial(arg0, arg1 string, arg2 *tls.Config) (*netceptor.Conn, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Dial", arg0, arg1, arg2)
	ret0, _ := ret[0].(*netceptor.Conn)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Dial indicates an expected call of Dial.
func (mr *MockNetceptorForControlsvcMockRecorder) Dial(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Dial", reflect.TypeOf((*MockNetceptorForControlsvc)(nil).Dial), arg0, arg1, arg2)
}

// GetClientTLSConfig mocks base method.
func (m *MockNetceptorForControlsvc) GetClientTLSConfig(arg0, arg1 string, arg2 netceptor.ExpectedHostnameType) (*tls.Config, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetClientTLSConfig", arg0, arg1, arg2)
	ret0, _ := ret[0].(*tls.Config)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetClientTLSConfig indicates an expected call of GetClientTLSConfig.
func (mr *MockNetceptorForControlsvcMockRecorder) GetClientTLSConfig(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetClientTLSConfig", reflect.TypeOf((*MockNetceptorForControlsvc)(nil).GetClientTLSConfig), arg0, arg1, arg2)
}

// GetLogger mocks base method.
func (m *MockNetceptorForControlsvc) GetLogger() *logger.ReceptorLogger {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetLogger")
	ret0, _ := ret[0].(*logger.ReceptorLogger)
	return ret0
}

// GetLogger indicates an expected call of GetLogger.
func (mr *MockNetceptorForControlsvcMockRecorder) GetLogger() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLogger", reflect.TypeOf((*MockNetceptorForControlsvc)(nil).GetLogger))
}

// ListenAndAdvertise mocks base method.
func (m *MockNetceptorForControlsvc) ListenAndAdvertise(arg0 string, arg1 *tls.Config, arg2 map[string]string) (*netceptor.Listener, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListenAndAdvertise", arg0, arg1, arg2)
	ret0, _ := ret[0].(*netceptor.Listener)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListenAndAdvertise indicates an expected call of ListenAndAdvertise.
func (mr *MockNetceptorForControlsvcMockRecorder) ListenAndAdvertise(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListenAndAdvertise", reflect.TypeOf((*MockNetceptorForControlsvc)(nil).ListenAndAdvertise), arg0, arg1, arg2)
}

// MaxForwardingHops mocks base method.
func (m *MockNetceptorForControlsvc) MaxForwardingHops() byte {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MaxForwardingHops")
	ret0, _ := ret[0].(byte)
	return ret0
}

// MaxForwardingHops indicates an expected call of MaxForwardingHops.
func (mr *MockNetceptorForControlsvcMockRecorder) MaxForwardingHops() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MaxForwardingHops", reflect.TypeOf((*MockNetceptorForControlsvc)(nil).MaxForwardingHops))
}

// NodeID mocks base method.
func (m *MockNetceptorForControlsvc) NodeID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NodeID")
	ret0, _ := ret[0].(string)
	return ret0
}

// NodeID indicates an expected call of NodeID.
func (mr *MockNetceptorForControlsvcMockRecorder) NodeID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NodeID", reflect.TypeOf((*MockNetceptorForControlsvc)(nil).NodeID))
}

// Ping mocks base method.
func (m *MockNetceptorForControlsvc) Ping(arg0 context.Context, arg1 string, arg2 byte) (time.Duration, string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Ping", arg0, arg1, arg2)
	ret0, _ := ret[0].(time.Duration)
	ret1, _ := ret[1].(string)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// Ping indicates an expected call of Ping.
func (mr *MockNetceptorForControlsvcMockRecorder) Ping(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Ping", reflect.TypeOf((*MockNetceptorForControlsvc)(nil).Ping), arg0, arg1, arg2)
}

// Status mocks base method.
func (m *MockNetceptorForControlsvc) Status() netceptor.Status {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Status")
	ret0, _ := ret[0].(netceptor.Status)
	return ret0
}

// Status indicates an expected call of Status.
func (mr *MockNetceptorForControlsvcMockRecorder) Status() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Status", reflect.TypeOf((*MockNetceptorForControlsvc)(nil).Status))
}

// Traceroute mocks base method.
func (m *MockNetceptorForControlsvc) Traceroute(arg0 context.Context, arg1 string) <-chan *netceptor.TracerouteResult {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Traceroute", arg0, arg1)
	ret0, _ := ret[0].(<-chan *netceptor.TracerouteResult)
	return ret0
}

// Traceroute indicates an expected call of Traceroute.
func (mr *MockNetceptorForControlsvcMockRecorder) Traceroute(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Traceroute", reflect.TypeOf((*MockNetceptorForControlsvc)(nil).Traceroute), arg0, arg1)
}

// MockUtiler is a mock of Utiler interface.
type MockUtiler struct {
	ctrl     *gomock.Controller
	recorder *MockUtilerMockRecorder
}

// MockUtilerMockRecorder is the mock recorder for MockUtiler.
type MockUtilerMockRecorder struct {
	mock *MockUtiler
}

// NewMockUtiler creates a new mock instance.
func NewMockUtiler(ctrl *gomock.Controller) *MockUtiler {
	mock := &MockUtiler{ctrl: ctrl}
	mock.recorder = &MockUtilerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockUtiler) EXPECT() *MockUtilerMockRecorder {
	return m.recorder
}

// BridgeConns mocks base method.
func (m *MockUtiler) BridgeConns(arg0 io.ReadWriteCloser, arg1 string, arg2 io.ReadWriteCloser, arg3 string, arg4 *logger.ReceptorLogger) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "BridgeConns", arg0, arg1, arg2, arg3, arg4)
}

// BridgeConns indicates an expected call of BridgeConns.
func (mr *MockUtilerMockRecorder) BridgeConns(arg0, arg1, arg2, arg3, arg4 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BridgeConns", reflect.TypeOf((*MockUtiler)(nil).BridgeConns), arg0, arg1, arg2, arg3, arg4)
}

// UnixSocketListen mocks base method.
func (m *MockUtiler) UnixSocketListen(arg0 string, arg1 fs.FileMode) (net.Listener, *utils.FLock, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UnixSocketListen", arg0, arg1)
	ret0, _ := ret[0].(net.Listener)
	ret1, _ := ret[1].(*utils.FLock)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// UnixSocketListen indicates an expected call of UnixSocketListen.
func (mr *MockUtilerMockRecorder) UnixSocketListen(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UnixSocketListen", reflect.TypeOf((*MockUtiler)(nil).UnixSocketListen), arg0, arg1)
}

// MockNeter is a mock of Neter interface.
type MockNeter struct {
	ctrl     *gomock.Controller
	recorder *MockNeterMockRecorder
}

// MockNeterMockRecorder is the mock recorder for MockNeter.
type MockNeterMockRecorder struct {
	mock *MockNeter
}

// NewMockNeter creates a new mock instance.
func NewMockNeter(ctrl *gomock.Controller) *MockNeter {
	mock := &MockNeter{ctrl: ctrl}
	mock.recorder = &MockNeterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockNeter) EXPECT() *MockNeterMockRecorder {
	return m.recorder
}

// Listen mocks base method.
func (m *MockNeter) Listen(arg0, arg1 string) (net.Listener, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Listen", arg0, arg1)
	ret0, _ := ret[0].(net.Listener)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Listen indicates an expected call of Listen.
func (mr *MockNeterMockRecorder) Listen(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Listen", reflect.TypeOf((*MockNeter)(nil).Listen), arg0, arg1)
}

// MockTlser is a mock of Tlser interface.
type MockTlser struct {
	ctrl     *gomock.Controller
	recorder *MockTlserMockRecorder
}

// MockTlserMockRecorder is the mock recorder for MockTlser.
type MockTlserMockRecorder struct {
	mock *MockTlser
}

// NewMockTlser creates a new mock instance.
func NewMockTlser(ctrl *gomock.Controller) *MockTlser {
	mock := &MockTlser{ctrl: ctrl}
	mock.recorder = &MockTlserMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockTlser) EXPECT() *MockTlserMockRecorder {
	return m.recorder
}

// NewListener mocks base method.
func (m *MockTlser) NewListener(arg0 net.Listener, arg1 *tls.Config) net.Listener {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NewListener", arg0, arg1)
	ret0, _ := ret[0].(net.Listener)
	return ret0
}

// NewListener indicates an expected call of NewListener.
func (mr *MockTlserMockRecorder) NewListener(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewListener", reflect.TypeOf((*MockTlser)(nil).NewListener), arg0, arg1)
}
