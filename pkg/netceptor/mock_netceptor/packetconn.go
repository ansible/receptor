// Code generated by MockGen. DO NOT EDIT.
// Source: pkg/netceptor/packetconn.go

// Package mock_netceptor is a generated GoMock package.
package mock_netceptor

import (
	context "context"
	logger "github.com/ansible/receptor/pkg/logger"
	netceptor "github.com/ansible/receptor/pkg/netceptor"
	utils "github.com/ansible/receptor/pkg/utils"
	gomock "github.com/golang/mock/gomock"
	net "net"
	reflect "reflect"
	sync "sync"
	time "time"
)

// MockPacketConner is a mock of PacketConner interface
type MockPacketConner struct {
	ctrl     *gomock.Controller
	recorder *MockPacketConnerMockRecorder
}

// MockPacketConnerMockRecorder is the mock recorder for MockPacketConner
type MockPacketConnerMockRecorder struct {
	mock *MockPacketConner
}

// NewMockPacketConner creates a new mock instance
func NewMockPacketConner(ctrl *gomock.Controller) *MockPacketConner {
	mock := &MockPacketConner{ctrl: ctrl}
	mock.recorder = &MockPacketConnerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockPacketConner) EXPECT() *MockPacketConnerMockRecorder {
	return m.recorder
}

// SetHopsToLive mocks base method
func (m *MockPacketConner) SetHopsToLive(hopsToLive byte) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetHopsToLive", hopsToLive)
}

// SetHopsToLive indicates an expected call of SetHopsToLive
func (mr *MockPacketConnerMockRecorder) SetHopsToLive(hopsToLive interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetHopsToLive", reflect.TypeOf((*MockPacketConner)(nil).SetHopsToLive), hopsToLive)
}

// GetHopsToLive mocks base method
func (m *MockPacketConner) GetHopsToLive() byte {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetHopsToLive")
	ret0, _ := ret[0].(byte)
	return ret0
}

// GetHopsToLive indicates an expected call of GetHopsToLive
func (mr *MockPacketConnerMockRecorder) GetHopsToLive() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetHopsToLive", reflect.TypeOf((*MockPacketConner)(nil).GetHopsToLive))
}

// SubscribeUnreachable mocks base method
func (m *MockPacketConner) SubscribeUnreachable(doneChan chan struct{}) chan netceptor.UnreachableNotification {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubscribeUnreachable", doneChan)
	ret0, _ := ret[0].(chan netceptor.UnreachableNotification)
	return ret0
}

// SubscribeUnreachable indicates an expected call of SubscribeUnreachable
func (mr *MockPacketConnerMockRecorder) SubscribeUnreachable(doneChan interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubscribeUnreachable", reflect.TypeOf((*MockPacketConner)(nil).SubscribeUnreachable), doneChan)
}

// ReadFrom mocks base method
func (m *MockPacketConner) ReadFrom(p []byte) (int, net.Addr, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadFrom", p)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(net.Addr)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// ReadFrom indicates an expected call of ReadFrom
func (mr *MockPacketConnerMockRecorder) ReadFrom(p interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadFrom", reflect.TypeOf((*MockPacketConner)(nil).ReadFrom), p)
}

// WriteTo mocks base method
func (m *MockPacketConner) WriteTo(p []byte, addr net.Addr) (int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WriteTo", p, addr)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// WriteTo indicates an expected call of WriteTo
func (mr *MockPacketConnerMockRecorder) WriteTo(p, addr interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WriteTo", reflect.TypeOf((*MockPacketConner)(nil).WriteTo), p, addr)
}

// LocalAddr mocks base method
func (m *MockPacketConner) LocalAddr() net.Addr {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LocalAddr")
	ret0, _ := ret[0].(net.Addr)
	return ret0
}

// LocalAddr indicates an expected call of LocalAddr
func (mr *MockPacketConnerMockRecorder) LocalAddr() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LocalAddr", reflect.TypeOf((*MockPacketConner)(nil).LocalAddr))
}

// Close mocks base method
func (m *MockPacketConner) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close
func (mr *MockPacketConnerMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockPacketConner)(nil).Close))
}

// SetDeadline mocks base method
func (m *MockPacketConner) SetDeadline(t time.Time) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetDeadline", t)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetDeadline indicates an expected call of SetDeadline
func (mr *MockPacketConnerMockRecorder) SetDeadline(t interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetDeadline", reflect.TypeOf((*MockPacketConner)(nil).SetDeadline), t)
}

// SetReadDeadline mocks base method
func (m *MockPacketConner) SetReadDeadline(t time.Time) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetReadDeadline", t)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetReadDeadline indicates an expected call of SetReadDeadline
func (mr *MockPacketConnerMockRecorder) SetReadDeadline(t interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetReadDeadline", reflect.TypeOf((*MockPacketConner)(nil).SetReadDeadline), t)
}

// GetReadDeadline mocks base method
func (m *MockPacketConner) GetReadDeadline() time.Time {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetReadDeadline")
	ret0, _ := ret[0].(time.Time)
	return ret0
}

// GetReadDeadline indicates an expected call of GetReadDeadline
func (mr *MockPacketConnerMockRecorder) GetReadDeadline() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetReadDeadline", reflect.TypeOf((*MockPacketConner)(nil).GetReadDeadline))
}

// SetWriteDeadline mocks base method
func (m *MockPacketConner) SetWriteDeadline(t time.Time) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetWriteDeadline", t)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetWriteDeadline indicates an expected call of SetWriteDeadline
func (mr *MockPacketConnerMockRecorder) SetWriteDeadline(t interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetWriteDeadline", reflect.TypeOf((*MockPacketConner)(nil).SetWriteDeadline), t)
}

// Cancel mocks base method
func (m *MockPacketConner) Cancel() *context.CancelFunc {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Cancel")
	ret0, _ := ret[0].(*context.CancelFunc)
	return ret0
}

// Cancel indicates an expected call of Cancel
func (mr *MockPacketConnerMockRecorder) Cancel() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Cancel", reflect.TypeOf((*MockPacketConner)(nil).Cancel))
}

// LocalService mocks base method
func (m *MockPacketConner) LocalService() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LocalService")
	ret0, _ := ret[0].(string)
	return ret0
}

// LocalService indicates an expected call of LocalService
func (mr *MockPacketConnerMockRecorder) LocalService() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LocalService", reflect.TypeOf((*MockPacketConner)(nil).LocalService))
}

// GetLogger mocks base method
func (m *MockPacketConner) GetLogger() *logger.ReceptorLogger {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetLogger")
	ret0, _ := ret[0].(*logger.ReceptorLogger)
	return ret0
}

// GetLogger indicates an expected call of GetLogger
func (mr *MockPacketConnerMockRecorder) GetLogger() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLogger", reflect.TypeOf((*MockPacketConner)(nil).GetLogger))
}

// StartUnreachable mocks base method
func (m *MockPacketConner) StartUnreachable() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "StartUnreachable")
}

// StartUnreachable indicates an expected call of StartUnreachable
func (mr *MockPacketConnerMockRecorder) StartUnreachable() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StartUnreachable", reflect.TypeOf((*MockPacketConner)(nil).StartUnreachable))
}

// MockNetcForPacketConn is a mock of NetcForPacketConn interface
type MockNetcForPacketConn struct {
	ctrl     *gomock.Controller
	recorder *MockNetcForPacketConnMockRecorder
}

// MockNetcForPacketConnMockRecorder is the mock recorder for MockNetcForPacketConn
type MockNetcForPacketConnMockRecorder struct {
	mock *MockNetcForPacketConn
}

// NewMockNetcForPacketConn creates a new mock instance
func NewMockNetcForPacketConn(ctrl *gomock.Controller) *MockNetcForPacketConn {
	mock := &MockNetcForPacketConn{ctrl: ctrl}
	mock.recorder = &MockNetcForPacketConnMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockNetcForPacketConn) EXPECT() *MockNetcForPacketConnMockRecorder {
	return m.recorder
}

// GetEphemeralService mocks base method
func (m *MockNetcForPacketConn) GetEphemeralService() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetEphemeralService")
	ret0, _ := ret[0].(string)
	return ret0
}

// GetEphemeralService indicates an expected call of GetEphemeralService
func (mr *MockNetcForPacketConnMockRecorder) GetEphemeralService() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetEphemeralService", reflect.TypeOf((*MockNetcForPacketConn)(nil).GetEphemeralService))
}

// AddNameHash mocks base method
func (m *MockNetcForPacketConn) AddNameHash(name string) uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddNameHash", name)
	ret0, _ := ret[0].(uint64)
	return ret0
}

// AddNameHash indicates an expected call of AddNameHash
func (mr *MockNetcForPacketConnMockRecorder) AddNameHash(name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddNameHash", reflect.TypeOf((*MockNetcForPacketConn)(nil).AddNameHash), name)
}

// AddLocalServiceAdvertisement mocks base method
func (m *MockNetcForPacketConn) AddLocalServiceAdvertisement(service string, connType byte, tags map[string]string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddLocalServiceAdvertisement", service, connType, tags)
}

// AddLocalServiceAdvertisement indicates an expected call of AddLocalServiceAdvertisement
func (mr *MockNetcForPacketConnMockRecorder) AddLocalServiceAdvertisement(service, connType, tags interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddLocalServiceAdvertisement", reflect.TypeOf((*MockNetcForPacketConn)(nil).AddLocalServiceAdvertisement), service, connType, tags)
}

// SendMessageWithHopsToLive mocks base method
func (m *MockNetcForPacketConn) SendMessageWithHopsToLive(fromService, toNode, toService string, data []byte, hopsToLive byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendMessageWithHopsToLive", fromService, toNode, toService, data, hopsToLive)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendMessageWithHopsToLive indicates an expected call of SendMessageWithHopsToLive
func (mr *MockNetcForPacketConnMockRecorder) SendMessageWithHopsToLive(fromService, toNode, toService, data, hopsToLive interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendMessageWithHopsToLive", reflect.TypeOf((*MockNetcForPacketConn)(nil).SendMessageWithHopsToLive), fromService, toNode, toService, data, hopsToLive)
}

// RemoveLocalServiceAdvertisement mocks base method
func (m *MockNetcForPacketConn) RemoveLocalServiceAdvertisement(service string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemoveLocalServiceAdvertisement", service)
	ret0, _ := ret[0].(error)
	return ret0
}

// RemoveLocalServiceAdvertisement indicates an expected call of RemoveLocalServiceAdvertisement
func (mr *MockNetcForPacketConnMockRecorder) RemoveLocalServiceAdvertisement(service interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveLocalServiceAdvertisement", reflect.TypeOf((*MockNetcForPacketConn)(nil).RemoveLocalServiceAdvertisement), service)
}

// GetLogger mocks base method
func (m *MockNetcForPacketConn) GetLogger() *logger.ReceptorLogger {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetLogger")
	ret0, _ := ret[0].(*logger.ReceptorLogger)
	return ret0
}

// GetLogger indicates an expected call of GetLogger
func (mr *MockNetcForPacketConnMockRecorder) GetLogger() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLogger", reflect.TypeOf((*MockNetcForPacketConn)(nil).GetLogger))
}

// NodeID mocks base method
func (m *MockNetcForPacketConn) NodeID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NodeID")
	ret0, _ := ret[0].(string)
	return ret0
}

// NodeID indicates an expected call of NodeID
func (mr *MockNetcForPacketConnMockRecorder) NodeID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NodeID", reflect.TypeOf((*MockNetcForPacketConn)(nil).NodeID))
}

// GetNetworkName mocks base method
func (m *MockNetcForPacketConn) GetNetworkName() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetNetworkName")
	ret0, _ := ret[0].(string)
	return ret0
}

// GetNetworkName indicates an expected call of GetNetworkName
func (mr *MockNetcForPacketConnMockRecorder) GetNetworkName() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetNetworkName", reflect.TypeOf((*MockNetcForPacketConn)(nil).GetNetworkName))
}

// GetListenerLock mocks base method
func (m *MockNetcForPacketConn) GetListenerLock() *sync.RWMutex {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetListenerLock")
	ret0, _ := ret[0].(*sync.RWMutex)
	return ret0
}

// GetListenerLock indicates an expected call of GetListenerLock
func (mr *MockNetcForPacketConnMockRecorder) GetListenerLock() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetListenerLock", reflect.TypeOf((*MockNetcForPacketConn)(nil).GetListenerLock))
}

// GetListenerRegistry mocks base method
func (m *MockNetcForPacketConn) GetListenerRegistry() map[string]*netceptor.PacketConn {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetListenerRegistry")
	ret0, _ := ret[0].(map[string]*netceptor.PacketConn)
	return ret0
}

// GetListenerRegistry indicates an expected call of GetListenerRegistry
func (mr *MockNetcForPacketConnMockRecorder) GetListenerRegistry() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetListenerRegistry", reflect.TypeOf((*MockNetcForPacketConn)(nil).GetListenerRegistry))
}

// GetUnreachableBroker mocks base method
func (m *MockNetcForPacketConn) GetUnreachableBroker() *utils.Broker {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetUnreachableBroker")
	ret0, _ := ret[0].(*utils.Broker)
	return ret0
}

// GetUnreachableBroker indicates an expected call of GetUnreachableBroker
func (mr *MockNetcForPacketConnMockRecorder) GetUnreachableBroker() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetUnreachableBroker", reflect.TypeOf((*MockNetcForPacketConn)(nil).GetUnreachableBroker))
}

// MaxForwardingHops mocks base method
func (m *MockNetcForPacketConn) MaxForwardingHops() byte {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MaxForwardingHops")
	ret0, _ := ret[0].(byte)
	return ret0
}

// MaxForwardingHops indicates an expected call of MaxForwardingHops
func (mr *MockNetcForPacketConnMockRecorder) MaxForwardingHops() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MaxForwardingHops", reflect.TypeOf((*MockNetcForPacketConn)(nil).MaxForwardingHops))
}

// Context mocks base method
func (m *MockNetcForPacketConn) Context() context.Context {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Context")
	ret0, _ := ret[0].(context.Context)
	return ret0
}

// Context indicates an expected call of Context
func (mr *MockNetcForPacketConnMockRecorder) Context() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Context", reflect.TypeOf((*MockNetcForPacketConn)(nil).Context))
}
