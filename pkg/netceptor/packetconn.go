package netceptor

import (
	"fmt"
	"net"
	"time"
)

// ErrTimeout is returned for an expired deadline.
var ErrTimeout error = &TimeoutError{}

// TimeoutError is returned for an expired deadline.
type TimeoutError struct{}

// Error returns a string describing the error.
func (e *TimeoutError) Error() string { return "i/o timeout" }

// Timeout returns true if this error was a timeout.
func (e *TimeoutError) Timeout() bool { return true }

// Temporary returns true if a retry is likely a good idea.
func (e *TimeoutError) Temporary() bool { return true }

// PacketConn implements the net.PacketConn interface via the Receptor network
type PacketConn struct {
	s             *Netceptor
	localService  string
	recvChan      chan *messageData
	readDeadline  time.Time
	writeDeadline time.Time
	advertise     bool
	adTags        map[string]string
}

// ListenPacket returns a datagram connection compatible with Go's net.PacketConn.
// If service is blank, generates and uses an ephemeral service name.
func (s *Netceptor) ListenPacket(service string) (*PacketConn, error) {
	if len(service) > 8 {
		return nil, fmt.Errorf("service name %s too long", service)
	}
	if service == "" {
		service = s.getEphemeralService()
	}
	s.listenerLock.Lock()
	defer s.listenerLock.Unlock()
	_, isReserved := s.reservedServices[service]
	_, isListening := s.listenerRegistry[service]
	if isReserved || isListening {
		return nil, fmt.Errorf("service %s is already listening", service)
	}
	_ = s.addNameHash(service)
	pc := &PacketConn{
		s:            s,
		localService: service,
		recvChan:     make(chan *messageData),
		advertise:    false,
		adTags:       nil,
	}
	s.listenerRegistry[service] = pc
	return pc, nil
}

// ListenPacketAndAdvertise returns a datagram listener, and also broadcasts service
// advertisements to the Receptor network as long as the listener remains open.
func (s *Netceptor) ListenPacketAndAdvertise(service string, tags map[string]string) (*PacketConn, error) {
	pc, err := s.ListenPacket(service)
	if err != nil {
		return nil, err
	}
	pc.advertise = true
	pc.adTags = tags
	s.addLocalServiceAdvertisement(service, tags)
	return pc, nil
}

// ReadFrom reads a packet from the network and returns its data and address.
func (nc *PacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	var m *messageData
	if nc.readDeadline.IsZero() {
		m = <-nc.recvChan
	} else {
		select {
		case m = <-nc.recvChan:
		case <-time.After(time.Until(nc.readDeadline)):
			return 0, nil, ErrTimeout
		}
	}
	nCopied := copy(p, m.Data)
	fromAddr := Addr{
		node:    m.FromNode,
		service: m.FromService,
	}
	return nCopied, fromAddr, nil
}

// WriteTo writes a packet to an address on the network.
func (nc *PacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	ncaddr, ok := addr.(Addr)
	if !ok {
		return 0, fmt.Errorf("attempt to write to non-netceptor address")
	}
	err = nc.s.sendMessage(nc.localService, ncaddr.node, ncaddr.service, p)
	if err != nil {
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
	if nc.advertise {
		err := nc.s.removeLocalServiceAdvertisement(nc.localService)
		if err != nil {
			return err
		}
	}
	return nil
}

// SetDeadline sets both the read and write deadlines.
func (nc *PacketConn) SetDeadline(t time.Time) error {
	nc.readDeadline = t
	return nil
}

// SetReadDeadline sets the read deadline.
func (nc *PacketConn) SetReadDeadline(t time.Time) error {
	nc.readDeadline = t
	return nil
}

// SetWriteDeadline sets the write deadline.
func (nc *PacketConn) SetWriteDeadline(t time.Time) error {
	// Write deadline doesn't mean anything because Write() implementation is non-blocking.
	return nil
}
