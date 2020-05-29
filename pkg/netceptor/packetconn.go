package netceptor

import (
	"fmt"
	"net"
	"time"
)

// PacketConn implements the net.PacketConn interface via the Receptor network
type PacketConn struct {
	s             *Netceptor
	localService  string
	recvChan      chan *messageData
	readDeadline  time.Time
	writeDeadline time.Time
}

// ListenPacket returns a datagram connection compatible with Go's net.PacketConn.
// If service is blank, generates and uses an ephemeral service name.
func (s *Netceptor) ListenPacket(service string) (*PacketConn, error) {
	s.listenerLock.Lock()
	defer s.listenerLock.Unlock()
	if service == "" {
		service = s.getEphemeralService()
	} else {
		_, ok := s.listenerRegistry[service]
		if ok {
			return nil, fmt.Errorf("service %s is already listening", service)
		}
	}
	_ = s.addNameHash(service)
	pc := &PacketConn{
		s:             s,
		localService:  service,
		recvChan:      make(chan *messageData),
	}
	s.listenerRegistry[service] = pc
	return pc, nil
}

// ReadFrom reads a packet from the network and returns its data and address.
func(nc *PacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	//TODO: respect nc.readDeadline
	m := <- nc.recvChan
	nCopied := copy(p, m.Data)
	fromAddr := Addr{
		node:    m.FromNode,
		service: m.FromService,
	}
	return nCopied, fromAddr, nil
}

// WriteTo writes a packet to an address on the network.
func(nc *PacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	//TODO: respect nc.writeDeadline
	ncaddr, ok := addr.(Addr); if !ok {
		return 0, fmt.Errorf("attempt to write to non-netceptor address")
	}
	err = nc.s.sendMessage(nc.localService, ncaddr.node, ncaddr.service, p); if err != nil {
		return 0, err
	}
	return len(p), nil
}

// LocalService returns the local service name of the connection.
func (nc *PacketConn) LocalService() string {
	return nc.localService
}

// LocalAddr returns the local address the connection is bound to.
func (nc *PacketConn) LocalAddr() net.Addr {
	return Addr{
		node:    nc.s.nodeID,
		service: nc.localService,
	}
}

// Close closes the connection.
func (nc *PacketConn) Close() error {
	nc.s.listenerLock.Lock()
	defer nc.s.listenerLock.Unlock()
	delete(nc.s.listenerRegistry, nc.localService)
	return nil
}

// SetDeadline sets both the read and write deadlines.
func (nc *PacketConn) SetDeadline(t time.Time) error {
	nc.readDeadline = t
	nc.writeDeadline = t
	return nil
}

// SetReadDeadline sets the read deadline.
func (nc *PacketConn) SetReadDeadline(t time.Time) error {
	nc.readDeadline = t
	return nil
}

// SetWriteDeadline sets the write deadline.
func (nc *PacketConn) SetWriteDeadline(t time.Time) error {
	nc.readDeadline = t
	return nil
}
