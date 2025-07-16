package netceptor

import (
	"context"
	"crypto/tls"
	"sync"
	"time"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/utils"
)

type NetceptorInterface interface {
	AddBackend(backend Backend, modifiers ...func(*BackendInfo)) error
	AddFirewallRules(rules []FirewallRuleFunc, clearExisting bool) error
	AddLocalServiceAdvertisement(service string, connType byte, tags map[string]string)
	AddNameHash(name string) uint64
	AddWorkCommand(command string, secure bool) error
	BackendCount() int
	BackendDone()
	BackendWait()
	CancelBackends()
	Context() context.Context
	Dial(node string, service string, tlscfg *tls.Config) (*Conn, error)
	DialContext(ctx context.Context, node string, service string, tlscfg *tls.Config) (*Conn, error)
	GetClientTLSConfig(name string, expectedHostName string, expectedHostNameType ExpectedHostnameType) (*tls.Config, error)
	GetEphemeralService() string
	GetListenerLock() *sync.RWMutex
	GetListenerRegistry() map[string]*PacketConn
	GetLogger() *logger.ReceptorLogger
	GetNameFromHash(namehash uint64) (string, error)
	GetNetworkName() string
	GetServerTLSConfig(name string) (*tls.Config, error)
	GetServiceInfo(nodeID string, service string) (*ServiceAdvertisement, bool)
	GetUnreachableBroker() *utils.Broker
	Listen(service string, tlscfg *tls.Config) (*Listener, error)
	ListenAndAdvertise(service string, tlscfg *tls.Config, tags map[string]string) (*Listener, error)
	ListenPacket(service string) (PacketConner, error)
	ListenPacketAndAdvertise(service string, tags map[string]string) (PacketConner, error)
	MTU() int
	MaxConnectionIdleTime() time.Duration
	MaxForwardingHops() byte
	NetceptorDone() <-chan struct{}
	NewAddr(node string, service string) Addr
	NodeID() string
	PathCost(nodeID string) (float64, error)
	Ping(ctx context.Context, target string, hopsToLive byte) (time.Duration, string, error)
	RemoveLocalServiceAdvertisement(service string) error
	RouteUpdateTime() time.Duration
	SeenUpdateExpireTime() time.Duration
	SendMessageWithHopsToLive(fromService string, toNode string, toService string, data []byte, hopsToLive byte) error
	ServiceAdTime() time.Duration
	SetClientTLSConfig(name string, config *tls.Config, pinnedFingerprints [][]byte) error
	SetMaxConnectionIdleTime(userDefinedMaxIdleConnectionTimeout string) error
	SetServerTLSConfig(name string, config *tls.Config) error
	Shutdown()
	Status() Status
	SubscribeRoutingUpdates() chan map[string]string
	Traceroute(ctx context.Context, target string) <-chan *TracerouteResult
}
