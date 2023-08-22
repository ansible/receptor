package controlsvc

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"time"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
)

// ControlCommandType is a type of command that can be run from the control service.
type ControlCommandType interface {
	InitFromString(string) (ControlCommand, error)
	InitFromJSON(map[string]interface{}) (ControlCommand, error)
}

type NetceptorForControlCommand interface {
	GetClientTLSConfig(name string, expectedHostName string, expectedHostNameType netceptor.ExpectedHostnameType) (*tls.Config, error)
	Dial(node string, service string, tlscfg *tls.Config) (*netceptor.Conn, error)
	Ping(ctx context.Context, target string, hopsToLive byte) (time.Duration, string, error)
	MaxForwardingHops() byte
	Status() netceptor.Status
	Traceroute(ctx context.Context, target string) <-chan *netceptor.TracerouteResult
	NodeID() string
	GetLogger() *logger.ReceptorLogger
	CancelBackends()
}

// ControlCommand is an instance of a command that is being run from the control service.
type ControlCommand interface {
	ControlFunc(context.Context, NetceptorForControlCommand, ControlFuncOperations) (map[string]interface{}, error)
}

// ControlFuncOperations provides callbacks for control services to take actions.
type ControlFuncOperations interface {
	BridgeConn(message string, bc io.ReadWriteCloser, bcName string, logger *logger.ReceptorLogger) error
	ReadFromConn(message string, out io.Writer) error
	WriteToConn(message string, in chan []byte) error
	Close() error
	RemoteAddr() net.Addr
}
