// +build no_controlsvc

// Stub package to satisfy controlsvc dependencies while providing no functionality

package controlsvc

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/project-receptor/receptor/pkg/netceptor"
	"net"
	"os"
)

// ErrNotImplemented is returned by most functions in this unit since it is a non-functional stub
var ErrNotImplemented = fmt.Errorf("not implemented")

// Server is an instance of a control service
type Server struct {
}

// New returns a new instance of a control service.
func New(stdServices bool, nc *netceptor.Netceptor) *Server {
	return &Server{}
}

// MainInstance is the global instance of the control service instantiated by the command-line main() function
var MainInstance *Server

// AddControlFunc registers a function that can be used from a control socket.
func (s *Server) AddControlFunc(name string, cType ControlCommandType) error {
	return nil
}

// RunControlSession runs the server protocol on the given connection
func (s *Server) RunControlSession(conn net.Conn) {
}

// RunControlSvc runs the main accept loop of the control service
func (s *Server) RunControlSvc(ctx context.Context, service string, tlscfg *tls.Config,
	unixSocket string, unixSocketPermissions os.FileMode) error {
	return ErrNotImplemented
}
