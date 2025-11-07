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
type NetcForPing interface {
	ListenPacket(service string) (PacketConner, error)
	NewAddr(target string, service string) Addr
	NodeID() string
	Context() context.Context
}

// NetcForTraceroute defines the subset of Netceptor methods needed by the CreateTraceroute function.
type NetcForTraceroute interface {
	MaxForwardingHops() byte
	Ping(ctx context.Context, target string, hopsToLive byte) (time.Duration, string, error)
	Context() context.Context
}

// -- Connection Layer Interfaces (QUIC-based) -------------------------------------------

// QuicStreamForConn defines the subset of quic.Stream methods used by Conn.
// In QUIC v0.54.0+, Stream became a struct; this interface enables mocking.
type QuicStreamForConn interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Close() error
	CancelRead(quic.StreamErrorCode)
	SetDeadline(time.Time) error
	SetReadDeadline(time.Time) error
	SetWriteDeadline(time.Time) error
}

// QuicConnectionForConn defines the subset of quic.Conn methods used by Conn.
// In QUIC v0.54.0+, Connection became *quic.Conn struct; this interface enables mocking.
type QuicConnectionForConn interface {
	AcceptStream(context.Context) (QuicStreamForConn, error)
	OpenStreamSync(context.Context) (QuicStreamForConn, error)
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
	CloseWithError(quic.ApplicationErrorCode, string) error
	Context() context.Context
}

// QuicListenerForListener defines the quic.Listener methods used by Listener.
// Accept returns QuicConnectionForConn interface for test mocking compatibility.
type QuicListenerForListener interface {
	Accept(ctx context.Context) (QuicConnectionForConn, error)
	Addr() net.Addr
	Close() error
}

// -- Transport/Backend Layer Interfaces --------------------------------------------------

// Backend is the interface for back-ends that the Receptor network can run over.
// This interface provides a pluggable transport layer allowing Receptor to work
// over various network protocols (TCP, UDP, WebSocket, etc.).
type Backend interface {
	Start(context.Context, *sync.WaitGroup) (chan BackendSession, error)
}

// BackendSession is the interface for a single session of a back-end.
// Backends must be DATAGRAM ORIENTED, meaning that Recv() must return
// whole packets sent by Send(). If the underlying protocol is stream
// oriented, then the backend must deal with any required buffering.
type BackendSession interface {
	Send([]byte) error
	Recv(time.Duration) ([]byte, error) // Must return netceptor.ErrTimeout if the timeout is exceeded
	Close() error
}

// MessageConn is an abstract connection that sends and receives whole messages (datagrams).
// This interface provides a unified way to handle message-oriented connections over various
// underlying transports (TCP, WebSocket, etc.) by ensuring message boundaries are preserved.
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
