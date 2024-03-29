// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/ansible/receptor/pkg/controlsvc (interfaces: ControlCommandType,NetceptorForControlCommand,ControlCommand,ControlFuncOperations)

// Package mock_controlsvc is a generated GoMock package.
package mock_controlsvc

import (
	context "context"
	tls "crypto/tls"
	io "io"
	net "net"
	reflect "reflect"
	time "time"

	controlsvc "github.com/ansible/receptor/pkg/controlsvc"
	logger "github.com/ansible/receptor/pkg/logger"
	netceptor "github.com/ansible/receptor/pkg/netceptor"
	gomock "github.com/golang/mock/gomock"
)

// MockControlCommandType is a mock of ControlCommandType interface.
type MockControlCommandType struct {
	ctrl     *gomock.Controller
	recorder *MockControlCommandTypeMockRecorder
}

// MockControlCommandTypeMockRecorder is the mock recorder for MockControlCommandType.
type MockControlCommandTypeMockRecorder struct {
	mock *MockControlCommandType
}

// NewMockControlCommandType creates a new mock instance.
func NewMockControlCommandType(ctrl *gomock.Controller) *MockControlCommandType {
	mock := &MockControlCommandType{ctrl: ctrl}
	mock.recorder = &MockControlCommandTypeMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockControlCommandType) EXPECT() *MockControlCommandTypeMockRecorder {
	return m.recorder
}

// InitFromJSON mocks base method.
func (m *MockControlCommandType) InitFromJSON(arg0 map[string]interface{}) (controlsvc.ControlCommand, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "InitFromJSON", arg0)
	ret0, _ := ret[0].(controlsvc.ControlCommand)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// InitFromJSON indicates an expected call of InitFromJSON.
func (mr *MockControlCommandTypeMockRecorder) InitFromJSON(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "InitFromJSON", reflect.TypeOf((*MockControlCommandType)(nil).InitFromJSON), arg0)
}

// InitFromString mocks base method.
func (m *MockControlCommandType) InitFromString(arg0 string) (controlsvc.ControlCommand, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "InitFromString", arg0)
	ret0, _ := ret[0].(controlsvc.ControlCommand)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// InitFromString indicates an expected call of InitFromString.
func (mr *MockControlCommandTypeMockRecorder) InitFromString(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "InitFromString", reflect.TypeOf((*MockControlCommandType)(nil).InitFromString), arg0)
}

// MockNetceptorForControlCommand is a mock of NetceptorForControlCommand interface.
type MockNetceptorForControlCommand struct {
	ctrl     *gomock.Controller
	recorder *MockNetceptorForControlCommandMockRecorder
}

// MockNetceptorForControlCommandMockRecorder is the mock recorder for MockNetceptorForControlCommand.
type MockNetceptorForControlCommandMockRecorder struct {
	mock *MockNetceptorForControlCommand
}

// NewMockNetceptorForControlCommand creates a new mock instance.
func NewMockNetceptorForControlCommand(ctrl *gomock.Controller) *MockNetceptorForControlCommand {
	mock := &MockNetceptorForControlCommand{ctrl: ctrl}
	mock.recorder = &MockNetceptorForControlCommandMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockNetceptorForControlCommand) EXPECT() *MockNetceptorForControlCommandMockRecorder {
	return m.recorder
}

// CancelBackends mocks base method.
func (m *MockNetceptorForControlCommand) CancelBackends() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "CancelBackends")
}

// CancelBackends indicates an expected call of CancelBackends.
func (mr *MockNetceptorForControlCommandMockRecorder) CancelBackends() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CancelBackends", reflect.TypeOf((*MockNetceptorForControlCommand)(nil).CancelBackends))
}

// Dial mocks base method.
func (m *MockNetceptorForControlCommand) Dial(arg0, arg1 string, arg2 *tls.Config) (*netceptor.Conn, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Dial", arg0, arg1, arg2)
	ret0, _ := ret[0].(*netceptor.Conn)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Dial indicates an expected call of Dial.
func (mr *MockNetceptorForControlCommandMockRecorder) Dial(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Dial", reflect.TypeOf((*MockNetceptorForControlCommand)(nil).Dial), arg0, arg1, arg2)
}

// GetClientTLSConfig mocks base method.
func (m *MockNetceptorForControlCommand) GetClientTLSConfig(arg0, arg1 string, arg2 netceptor.ExpectedHostnameType) (*tls.Config, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetClientTLSConfig", arg0, arg1, arg2)
	ret0, _ := ret[0].(*tls.Config)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetClientTLSConfig indicates an expected call of GetClientTLSConfig.
func (mr *MockNetceptorForControlCommandMockRecorder) GetClientTLSConfig(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetClientTLSConfig", reflect.TypeOf((*MockNetceptorForControlCommand)(nil).GetClientTLSConfig), arg0, arg1, arg2)
}

// GetLogger mocks base method.
func (m *MockNetceptorForControlCommand) GetLogger() *logger.ReceptorLogger {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetLogger")
	ret0, _ := ret[0].(*logger.ReceptorLogger)
	return ret0
}

// GetLogger indicates an expected call of GetLogger.
func (mr *MockNetceptorForControlCommandMockRecorder) GetLogger() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLogger", reflect.TypeOf((*MockNetceptorForControlCommand)(nil).GetLogger))
}

// MaxForwardingHops mocks base method.
func (m *MockNetceptorForControlCommand) MaxForwardingHops() byte {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MaxForwardingHops")
	ret0, _ := ret[0].(byte)
	return ret0
}

// MaxForwardingHops indicates an expected call of MaxForwardingHops.
func (mr *MockNetceptorForControlCommandMockRecorder) MaxForwardingHops() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MaxForwardingHops", reflect.TypeOf((*MockNetceptorForControlCommand)(nil).MaxForwardingHops))
}

// NodeID mocks base method.
func (m *MockNetceptorForControlCommand) NodeID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NodeID")
	ret0, _ := ret[0].(string)
	return ret0
}

// NodeID indicates an expected call of NodeID.
func (mr *MockNetceptorForControlCommandMockRecorder) NodeID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NodeID", reflect.TypeOf((*MockNetceptorForControlCommand)(nil).NodeID))
}

// Ping mocks base method.
func (m *MockNetceptorForControlCommand) Ping(arg0 context.Context, arg1 string, arg2 byte) (time.Duration, string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Ping", arg0, arg1, arg2)
	ret0, _ := ret[0].(time.Duration)
	ret1, _ := ret[1].(string)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// Ping indicates an expected call of Ping.
func (mr *MockNetceptorForControlCommandMockRecorder) Ping(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Ping", reflect.TypeOf((*MockNetceptorForControlCommand)(nil).Ping), arg0, arg1, arg2)
}

// Status mocks base method.
func (m *MockNetceptorForControlCommand) Status() netceptor.Status {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Status")
	ret0, _ := ret[0].(netceptor.Status)
	return ret0
}

// Status indicates an expected call of Status.
func (mr *MockNetceptorForControlCommandMockRecorder) Status() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Status", reflect.TypeOf((*MockNetceptorForControlCommand)(nil).Status))
}

// Traceroute mocks base method.
func (m *MockNetceptorForControlCommand) Traceroute(arg0 context.Context, arg1 string) <-chan *netceptor.TracerouteResult {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Traceroute", arg0, arg1)
	ret0, _ := ret[0].(<-chan *netceptor.TracerouteResult)
	return ret0
}

// Traceroute indicates an expected call of Traceroute.
func (mr *MockNetceptorForControlCommandMockRecorder) Traceroute(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Traceroute", reflect.TypeOf((*MockNetceptorForControlCommand)(nil).Traceroute), arg0, arg1)
}

// MockControlCommand is a mock of ControlCommand interface.
type MockControlCommand struct {
	ctrl     *gomock.Controller
	recorder *MockControlCommandMockRecorder
}

// MockControlCommandMockRecorder is the mock recorder for MockControlCommand.
type MockControlCommandMockRecorder struct {
	mock *MockControlCommand
}

// NewMockControlCommand creates a new mock instance.
func NewMockControlCommand(ctrl *gomock.Controller) *MockControlCommand {
	mock := &MockControlCommand{ctrl: ctrl}
	mock.recorder = &MockControlCommandMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockControlCommand) EXPECT() *MockControlCommandMockRecorder {
	return m.recorder
}

// ControlFunc mocks base method.
func (m *MockControlCommand) ControlFunc(arg0 context.Context, arg1 controlsvc.NetceptorForControlCommand, arg2 controlsvc.ControlFuncOperations) (map[string]interface{}, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ControlFunc", arg0, arg1, arg2)
	ret0, _ := ret[0].(map[string]interface{})
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ControlFunc indicates an expected call of ControlFunc.
func (mr *MockControlCommandMockRecorder) ControlFunc(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ControlFunc", reflect.TypeOf((*MockControlCommand)(nil).ControlFunc), arg0, arg1, arg2)
}

// MockControlFuncOperations is a mock of ControlFuncOperations interface.
type MockControlFuncOperations struct {
	ctrl     *gomock.Controller
	recorder *MockControlFuncOperationsMockRecorder
}

// MockControlFuncOperationsMockRecorder is the mock recorder for MockControlFuncOperations.
type MockControlFuncOperationsMockRecorder struct {
	mock *MockControlFuncOperations
}

// NewMockControlFuncOperations creates a new mock instance.
func NewMockControlFuncOperations(ctrl *gomock.Controller) *MockControlFuncOperations {
	mock := &MockControlFuncOperations{ctrl: ctrl}
	mock.recorder = &MockControlFuncOperationsMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockControlFuncOperations) EXPECT() *MockControlFuncOperationsMockRecorder {
	return m.recorder
}

// BridgeConn mocks base method.
func (m *MockControlFuncOperations) BridgeConn(arg0 string, arg1 io.ReadWriteCloser, arg2 string, arg3 *logger.ReceptorLogger, arg4 controlsvc.Utiler) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BridgeConn", arg0, arg1, arg2, arg3, arg4)
	ret0, _ := ret[0].(error)
	return ret0
}

// BridgeConn indicates an expected call of BridgeConn.
func (mr *MockControlFuncOperationsMockRecorder) BridgeConn(arg0, arg1, arg2, arg3, arg4 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BridgeConn", reflect.TypeOf((*MockControlFuncOperations)(nil).BridgeConn), arg0, arg1, arg2, arg3, arg4)
}

// Close mocks base method.
func (m *MockControlFuncOperations) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockControlFuncOperationsMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockControlFuncOperations)(nil).Close))
}

// ReadFromConn mocks base method.
func (m *MockControlFuncOperations) ReadFromConn(arg0 string, arg1 io.Writer, arg2 controlsvc.Copier) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadFromConn", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// ReadFromConn indicates an expected call of ReadFromConn.
func (mr *MockControlFuncOperationsMockRecorder) ReadFromConn(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadFromConn", reflect.TypeOf((*MockControlFuncOperations)(nil).ReadFromConn), arg0, arg1, arg2)
}

// RemoteAddr mocks base method.
func (m *MockControlFuncOperations) RemoteAddr() net.Addr {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemoteAddr")
	ret0, _ := ret[0].(net.Addr)
	return ret0
}

// RemoteAddr indicates an expected call of RemoteAddr.
func (mr *MockControlFuncOperationsMockRecorder) RemoteAddr() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoteAddr", reflect.TypeOf((*MockControlFuncOperations)(nil).RemoteAddr))
}

// WriteToConn mocks base method.
func (m *MockControlFuncOperations) WriteToConn(arg0 string, arg1 chan []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WriteToConn", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// WriteToConn indicates an expected call of WriteToConn.
func (mr *MockControlFuncOperationsMockRecorder) WriteToConn(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WriteToConn", reflect.TypeOf((*MockControlFuncOperations)(nil).WriteToConn), arg0, arg1)
}
