package netceptor

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"time"

	"github.com/project-receptor/receptor/pkg/utils"
)

// PacketConn implements the net.PacketConn interface via the Receptor network
type PacketConn struct {
	s                  *Netceptor
	localService       string
	recvChan           chan *messageData
	readDeadline       time.Time
	writeDeadline      time.Time
	advertise          bool
	adTags             map[string]string
	connType           byte
	hopsToLive         byte
	unreachableMsgChan chan interface{}
	unreachableSubs    *utils.Broker
	context            context.Context
	cancel             context.CancelFunc
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
		connType:     ConnTypeDatagram,
		hopsToLive:   s.maxForwardingHops,
	}
	pc.startUnreachable()
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
	s.addLocalServiceAdvertisement(service, ConnTypeDatagram, tags)
	return pc, nil
}

// startUnreachable starts monitoring the netceptor unreachable channel and forwarding relevant messages
func (pc *PacketConn) startUnreachable() {
	pc.context, pc.cancel = context.WithCancel(pc.s.context)
	pc.unreachableSubs = utils.NewBroker(pc.context, reflect.TypeOf(UnreachableNotification{}))
	pc.unreachableMsgChan = pc.s.unreachableBroker.Subscribe()
	go func() {
		for {
			select {
			case <-pc.context.Done():
				return
			case msgIf := <-pc.unreachableMsgChan:
				msg, ok := msgIf.(UnreachableNotification)
				if !ok {
					continue
				}
				FromNode := msg.FromNode
				FromService := msg.FromService
				if FromNode == pc.s.nodeID && FromService == pc.localService {
					_ = pc.unreachableSubs.Publish(msg)
				}
			}
		}
	}()
}

// SubscribeUnreachable subscribes for unreachable messages relevant to this PacketConn
func (pc *PacketConn) SubscribeUnreachable() chan UnreachableNotification {
	iChan := pc.unreachableSubs.Subscribe()
	uChan := make(chan UnreachableNotification)
	go func() {
		for {
			select {
			case msgIf, ok := <-iChan:
				if !ok {
					close(uChan)
					return
				}
				msg, ok := msgIf.(UnreachableNotification)
				if !ok {
					continue
				}
				uChan <- msg
			case <-pc.context.Done():
				close(uChan)
				return
			}
		}
	}()
	return uChan
}

// ReadFrom reads a packet from the network and returns its data and address.
func (pc *PacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	var m *messageData
	if pc.readDeadline.IsZero() {
		m = <-pc.recvChan
	} else {
		select {
		case m = <-pc.recvChan:
		case <-time.After(time.Until(pc.readDeadline)):
			return 0, nil, ErrTimeout
		}
	}
	if m == nil {
		return 0, nil, fmt.Errorf("connection closed")
	}
	nCopied := copy(p, m.Data)
	fromAddr := Addr{
		network: pc.s.networkName,
		node:    m.FromNode,
		service: m.FromService,
	}
	return nCopied, fromAddr, nil
}

// WriteTo writes a packet to an address on the network.
func (pc *PacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	ncaddr, ok := addr.(Addr)
	if !ok {
		return 0, fmt.Errorf("attempt to write to non-netceptor address")
	}
	err = pc.s.sendMessageWithHopsToLive(pc.localService, ncaddr.node, ncaddr.service, p, pc.hopsToLive)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// SetHopsToLive sets the HopsToLive value for future outgoing packets on this connection.
func (pc *PacketConn) SetHopsToLive(hopsToLive byte) {
	pc.hopsToLive = hopsToLive
}

// LocalService returns the local service name of the connection.
func (pc *PacketConn) LocalService() string {
	return pc.localService
}

// LocalAddr returns the local address the connection is bound to.
func (pc *PacketConn) LocalAddr() net.Addr {
	return Addr{
		network: pc.s.networkName,
		node:    pc.s.nodeID,
		service: pc.localService,
	}
}

// Close closes the connection.
func (pc *PacketConn) Close() error {
	pc.s.listenerLock.Lock()
	defer pc.s.listenerLock.Unlock()
	delete(pc.s.listenerRegistry, pc.localService)
	if pc.cancel != nil {
		pc.cancel()
	}
	close(pc.recvChan)
	if pc.advertise {
		err := pc.s.removeLocalServiceAdvertisement(pc.localService)
		if err != nil {
			return err
		}
	}
	return nil
}

// SetDeadline sets both the read and write deadlines.
func (pc *PacketConn) SetDeadline(t time.Time) error {
	pc.readDeadline = t
	return nil
}

// SetReadDeadline sets the read deadline.
func (pc *PacketConn) SetReadDeadline(t time.Time) error {
	pc.readDeadline = t
	return nil
}

// SetWriteDeadline sets the write deadline.
func (pc *PacketConn) SetWriteDeadline(t time.Time) error {
	// Write deadline doesn't mean anything because Write() implementation is non-blocking.
	return nil
}
