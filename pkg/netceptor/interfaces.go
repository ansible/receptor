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

// -- conn.go ------------------------------------------------------------------------------

type QuicStreamForConn interface {
	quic.Stream
}

type QuicConnectionForConn interface {
	quic.Connection
}

type QuicListenerForListener interface {
	Accept(ctx context.Context) (quic.Connection, error)
	Addr() net.Addr
	Close() error
}

// -- external_backend.go ------------------------------------------------------------------

// MessageConn is an abstract connection that sends and receives whole messages (datagrams).
type MessageConn interface {
	WriteMessage(ctx context.Context, data []byte) error
	ReadMessage(ctx context.Context, timeout time.Duration) ([]byte, error)
	SetReadDeadline(t time.Time) error
	Close() error
}

// Backend is the interface for back-ends that the Receptor network can run over.
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

// -- packetconn.go ------------------------------------------------------------------------

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

// -- ping.go ------------------------------------------------------------------------------

// NetcForPing should include all methods of Netceptor needed by the Ping function.
type NetcForPing interface {
	ListenPacket(service string) (PacketConner, error)
	NewAddr(target string, service string) Addr
	NodeID() string
	Context() context.Context
}

type NetcForTraceroute interface {
	MaxForwardingHops() byte
	Ping(ctx context.Context, target string, hopsToLive byte) (time.Duration, string, error)
	Context() context.Context
}
