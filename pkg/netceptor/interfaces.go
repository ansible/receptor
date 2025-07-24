package netceptor

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/utils"
	"github.com/quic-go/quic-go"
)

// -- Netceptor Interface Segregation (Dependency Injection) -----------------------------

// These interfaces follow the Interface Segregation Principle by providing only
// the methods that specific components actually need from the main Netceptor instance,
// making the code more testable and reducing coupling.

// NetcForPacketConn defines the subset of Netceptor methods needed by PacketConn.
//
// Implemented by:
//   - Netceptor (main Netceptor struct implements this through its methods)
//
// Mock:
//   - MockNetcForPacketConn in mock_netceptor/interfaces.go
//
// Used by:
//   - PacketConn field (packetconn.go:17)
//   - NewPacketConn constructors (packetconn.go:31, 48)
//
// Tests:
//   - packetconn_test.go - TestNewPacketConn, TestPacketConn (comprehensive)
type NetcForPacketConn interface {
	GetEphemeralService() string
	AddNameHash(name string) uint64
	AddLocalServiceAdvertisement(service string, connType byte, tags map[string]string)
	SendMessageWithHopsToLive(fromService string, toNode string, toService string, data []byte, hopsToLive byte) error
	RemoveLocalServiceAdvertisement(service string) error
	GetLogger() *logger.ReceptorLogger
	NodeID() string
	GetNetworkName() string
	GetListenerLock() *sync.RWMutex
	GetListenerRegistry() map[string]*PacketConn
	GetUnreachableBroker() *utils.Broker
	MaxForwardingHops() byte
	Context() context.Context
}

// NetcForPing defines the subset of Netceptor methods needed by the SendPing function.
//
// Implemented by:
//   - Netceptor (main Netceptor struct implements this through its methods)
//
// Mock:
//   - MockNetcForPing in mock_netceptor/interfaces.go
//
// Used by:
//   - SendPing function (ping.go:16) - accepts this interface for dependency injection
//   - Netceptor.Ping() method (ping.go:11) - passes itself as this interface
//
// Tests:
//   - addr_test.go - TestNetwork uses MockNetcForPing to test address creation
type NetcForPing interface {
	ListenPacket(service string) (PacketConner, error)
	NewAddr(target string, service string) Addr
	NodeID() string
	Context() context.Context
}

// NetcForTraceroute defines the subset of Netceptor methods needed by the CreateTraceroute function.
//
// Implemented by:
//   - Netceptor (main Netceptor struct implements this through its methods)
//
// Mock:
//   - MockNetcForTraceroute in mock_netceptor/interfaces.go
//
// Used by:
//   - CreateTraceroute function (ping.go:100) - accepts this interface for dependency injection
//   - Netceptor.Traceroute() method (ping.go:96) - passes itself as this interface
//
// Tests:
//   - No direct unit tests found using MockNetcForTraceroute (potential test gap)
//   - Functional tests in tests/functional/mesh/mesh_test.go:87 (TestTraceroute)
//   - Control service tests in controlsvc/traceroute_test.go use higher-level mocks
type NetcForTraceroute interface {
	MaxForwardingHops() byte
	Ping(ctx context.Context, target string, hopsToLive byte) (time.Duration, string, error)
	Context() context.Context
}

// -- Connection Layer Interfaces (QUIC-based) -------------------------------------------

// QuicStreamForConn wraps quic.Stream to provide stream functionality for Conn instances.
//
// This interface allows for easy mocking and testing of QUIC stream operations.
// Implemented by:
//   - quic.Stream (from quic-go library)
//
// Mock:
//   - MockQuicStreamForConn in mock_netceptor/interfaces.go
//
// Used by:
//   - Conn struct field (conn.go:326)
//   - NewConn constructor (conn.go:333)
//
// Tests:
//   - conn_test.go - TestRead, TestCancelRead, TestWrite, TestClose, TestSetDeadline, TestSetReadDeadline,
//     TestSetWriteDeadline
type QuicStreamForConn interface {
	quic.Stream
}

// QuicConnectionForConn wraps quic.Connection to provide connection functionality for Conn instances.
//
// This interface allows for easy mocking and testing of QUIC connection operations.
// Implemented by:
//   - quic.Connection (from quic-go library)
//
// Mock:
//   - MockQuicConnectionForConn in mock_netceptor/interfaces.go
//
// Used by:
//   - Conn struct field (conn.go:325)
//   - NewConn constructor (conn.go:333)
//
// Tests:
//   - conn_test.go - TestCloseConnection, TestLocalAddr, TestRemoteAddr
type QuicConnectionForConn interface {
	quic.Connection
}

// QuicListenerForListener wraps quic.Listener functionality needed by the Listener struct.
//
// This interface provides the minimal set of methods required for accepting QUIC connections.
// Implemented by:
//   - quic.Listener (from quic-go library)
//
// Mock:
//   - MockQuicListenerForListener in mock_netceptor/interfaces.go
//
// Used by:
//   - Listener struct field (conn.go:42)
//   - NewListener constructor (conn.go:48)
//
// Tests:
//   - conn_test.go - TestNewListener, TestListenerClose, TestListenerCloseErrorPrecedence
type QuicListenerForListener interface {
	Accept(ctx context.Context) (quic.Connection, error)
	Addr() net.Addr
	Close() error
}

// -- Transport/Backend Layer Interfaces --------------------------------------------------

// Backend is the interface for back-ends that the Receptor network can run over.
// This interface provides a pluggable transport layer allowing Receptor to work
// over various network protocols (TCP, UDP, WebSocket, etc.).
//
// Implemented by:
//   - TCPDialer, TCPListener (backends/tcp.go)
//   - UDPDialer, UDPListener (backends/udp.go)
//   - WebsocketDialer, WebsocketListener (backends/websockets.go)
//   - ExternalBackend (external_backend.go:17)
//   - NullBackendCfg (backends/null.go:15)
//
// Mock:
//   - MockBackend in mock_netceptor/interfaces.go (currently unused in tests)
//
// Used by:
//   - Netceptor.AddBackend() (netceptor.go:505)
//   - Backend management in netceptor.go
//
// Tests:
//   - No test usage
type Backend interface {
	Start(context.Context, *sync.WaitGroup) (chan BackendSession, error)
}

// BackendSession is the interface for a single session of a back-end.
// Backends must be DATAGRAM ORIENTED, meaning that Recv() must return
// whole packets sent by Send(). If the underlying protocol is stream
// oriented, then the backend must deal with any required buffering.
//
// Implemented by:
//   - TCPSession (backends/tcp.go:166)
//   - UDPDialerSession, UDPListenerSession (backends/udp.go)
//   - WebsocketSession (backends/websockets.go:277)
//   - ExternalSession (external_backend.go:153)
//   - mockBackendSession (various test implementations)
//
// Mock:
//   - MockBackendSession in mock_netceptor/interfaces.go (currently unused in tests)
//
// Used by:
//   - Protocol handlers: protoReader, protoWriter (netceptor.go:1736, 1762)
//   - runProtocol (netceptor.go:1857)
//
// Tests:
//   - backends/utils_test.go - TestDialerSessionScenarios, TestContextBehavior, TestListenerSessionScenarios,
//     TestListenerSessionContextCancellation, TestMaxRedialDelayConstant, TestDialerSessionConnectionCloseImmediate,
//     TestDialerSessionRedialDelayReset
//   - netceptor/netceptor_test.go - TestRunProtocol* functions with mockBackendSession
//   - backends/tcp_test.go - TestTCPListenerStart, TestTCPDialerStart
//   - backends/udp_test.go - TestUDPListenerStart, TestUDPDialerStart
//   - backends/websockets_test.go - TestWebsocketDialerStart, TestWebsocketListenerStart
//   - backends/websocket_interop_test.go - TestWebsocketExternalInterop (integration)
//   - backends/null_test.go - TestNullBackendCfg_Start
type BackendSession interface {
	Send([]byte) error
	Recv(time.Duration) ([]byte, error) // Must return netceptor.ErrTimeout if the timeout is exceeded
	Close() error
}

// MessageConn is an abstract connection that sends and receives whole messages (datagrams).
// This interface provides a unified way to handle message-oriented connections over various
// underlying transports (TCP, WebSocket, etc.) by ensuring message boundaries are preserved.
//
// Implemented by:
//   - netMessageConn (external_backend.go:23) - wraps net.Conn
//   - websocketMessageConn (external_backend.go:97) - wraps websocket.Conn
//
// Mock:
//   - MockMessageConn in mock_netceptor/interfaces.go (currently unused in tests)
//
// Used by:
//   - ExternalBackend field (external_backend.go:156)
//   - Factory functions: MessageConnFromNetConn, MessageConnFromWebsocketConn
//
// Tests:
//   - No test usage
type MessageConn interface {
	WriteMessage(ctx context.Context, data []byte) error
	ReadMessage(ctx context.Context, timeout time.Duration) ([]byte, error)
	SetReadDeadline(t time.Time) error
	Close() error
}

// PacketConner provides packet-based network communication functionality.
// This interface abstracts packet-oriented network operations, allowing for
// datagram-style communication with features like hop counting and unreachable notifications.
// Part of the transport layer as it provides another form of network communication abstraction.
//
// Implemented by:
//   - PacketConn (packetconn.go:16) - the main implementation
//
// Mock:
//   - MockPacketConner in mock_netceptor/interfaces.go
//
// Used by:
//   - Netceptor.ListenPacket() (packetconn.go:54)
//   - Netceptor.ListenPacketAndAdvertise() (packetconn.go:76)
//   - Conn struct field (conn.go:324)
//   - Listener struct field (conn.go:41)
//   - Various network services (UDP proxy, IP router, etc.)
//
// Tests:
//   - conn_test.go - TestCloseConnection, TestNewListener, TestListenerAddr, TestListenerAccept,
//     TestListenerAcceptEdgeCases, TestListenerAcceptWithContextCancellation, TestListenerClose,
//     TestListenerCloseErrorPrecedence
//   - udp_proxy_test.go - TestUDPProxyServiceInbound, TestUDPProxyServiceOutbound, TestProcessInboundPacket,
//     TestProcessOutboundPacket, TestRunUDPProxyServiceInbound, TestRunUDPProxyServiceOutbound
type PacketConner interface {
	SetHopsToLive(hopsToLive byte)
	GetHopsToLive() byte
	SubscribeUnreachable(doneChan chan struct{}) chan UnreachableNotification
	ReadFrom(p []byte) (int, net.Addr, error)
	WriteTo(p []byte, addr net.Addr) (n int, err error)
	LocalAddr() net.Addr
	Close() error
	SetDeadline(t time.Time) error
	SetReadDeadline(t time.Time) error
	GetReadDeadline() time.Time
	SetWriteDeadline(t time.Time) error
	Cancel() *context.CancelFunc
	LocalService() string
	GetLogger() *logger.ReceptorLogger
	StartUnreachable()
}
