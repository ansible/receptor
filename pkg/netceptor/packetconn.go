package netceptor

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"sync"
	"time"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/utils"
)

type PacketConner interface {
	SetHopsToLive(hopsToLive byte)
	SubscribeUnreachable(doneChan chan struct{}) chan UnreachableNotification
	ReadFrom(p []byte) (int, net.Addr, error)
	WriteTo(p []byte, addr net.Addr) (n int, err error)
	LocalAddr() net.Addr
	Close() error
	SetDeadline(t time.Time) error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
	Cancel() *context.CancelFunc
	GetLocalService() string
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
	GetListenerRegistery() map[string]*PacketConn
	GetUnreachableBroker() *utils.Broker
	MaxForwardingHops() byte
	Context() context.Context
}

// PacketConn implements the net.PacketConn interface via the Receptor network.
type PacketConn struct {
	s               NetcForPacketConn
	localService    string
	recvChan        chan *MessageData
	readDeadline    time.Time
	advertise       bool
	adTags          map[string]string
	connType        byte
	hopsToLive      byte
	unreachableSubs *utils.Broker
	context         context.Context
	cancel          context.CancelFunc
}

func NewPacketConnWithConst(s NetcForPacketConn, service string, advertise bool, adtags map[string]string, connTypeDatagram byte) PacketConner {
	npc := &PacketConn{
		s:            s,
		localService: service,
		recvChan:     make(chan *MessageData),
		advertise:    advertise,
		adTags:       adtags,
		connType:     connTypeDatagram,
		hopsToLive:   s.MaxForwardingHops(),
	}

	npc.StartUnreachable()
	s.GetListenerRegistery()[service] = npc

	return npc
}

func NewPacketConn(s NetcForPacketConn, service string, connTypeDatagram byte) PacketConner {
	return NewPacketConnWithConst(s, service, false, nil, connTypeDatagram)
}

// ListenPacket returns a datagram connection compatible with Go's net.PacketConn.
// If service is blank, generates and uses an ephemeral service name.
func (s *Netceptor) ListenPacket(service string) (PacketConner, error) {
	if len(service) > 8 {
		return nil, fmt.Errorf("service name %s too long", service)
	}
	if service == "" {
		service = s.GetEphemeralService()
	}
	s.GetListenerLock().Lock()
	defer s.GetListenerLock().Unlock()
	_, isReserved := s.reservedServices[service]
	_, isListening := s.listenerRegistry[service]
	if isReserved || isListening {
		return nil, fmt.Errorf("service %s is already listening", service)
	}
	_ = s.AddNameHash(service)
	pc := NewPacketConn(s, service, ConnTypeDatagram)

	return pc, nil
}

// ListenPacketAndAdvertise returns a datagram listener, and also broadcasts service
// advertisements to the Receptor network as long as the listener remains open.
func (s *Netceptor) ListenPacketAndAdvertise(service string, tags map[string]string) (PacketConner, error) {
	if len(service) > 8 {
		return nil, fmt.Errorf("service name %s too long", service)
	}
	if service == "" {
		service = s.GetEphemeralService()
	}
	s.listenerLock.Lock()
	defer s.listenerLock.Unlock()
	_, isReserved := s.reservedServices[service]
	_, isListening := s.listenerRegistry[service]
	if isReserved || isListening {
		return nil, fmt.Errorf("service %s is already listening and advertising", service)
	}
	pc := NewPacketConnWithConst(s, service, true, tags, ConnTypeDatagram)

	s.AddLocalServiceAdvertisement(service, ConnTypeDatagram, tags)

	return pc, nil
}

func (pc *PacketConn) Cancel() *context.CancelFunc {
	return &pc.cancel
}

func (pc *PacketConn) GetLocalService() string {
	return pc.localService
}

func (pc *PacketConn) GetLogger() *logger.ReceptorLogger {
	return pc.s.GetLogger()
}

// startUnreachable starts monitoring the netceptor unreachable channel and forwarding relevant messages.
func (pc *PacketConn) StartUnreachable() {
	pc.context, pc.cancel = context.WithCancel(pc.s.Context())
	pc.unreachableSubs = utils.NewBroker(pc.context, reflect.TypeOf(UnreachableNotification{}))
	iChan := pc.s.GetUnreachableBroker().Subscribe()
	go func() {
		<-pc.context.Done()
		pc.s.GetUnreachableBroker().Unsubscribe(iChan)
	}()
	go func() {
		for msgIf := range iChan {
			msg, ok := msgIf.(UnreachableNotification)
			if !ok {
				continue
			}
			FromNode := msg.FromNode
			FromService := msg.FromService
			if FromNode == pc.s.NodeID() && FromService == pc.localService {
				_ = pc.unreachableSubs.Publish(msg)
			}
		}
	}()
}

// SubscribeUnreachable subscribes for unreachable messages relevant to this PacketConn.
func (pc *PacketConn) SubscribeUnreachable(doneChan chan struct{}) chan UnreachableNotification {
	iChan := pc.unreachableSubs.Subscribe()
	if iChan == nil {
		return nil
	}
	uChan := make(chan UnreachableNotification)
	// goroutine 1
	// if doneChan is selected, this will unsubscribe the channel, which should
	// eventually close out the go routine 2
	go func() {
		select {
		case <-doneChan:
			pc.unreachableSubs.Unsubscribe(iChan)
		case <-pc.context.Done():
		}
	}()
	// goroutine 2
	// this will exit when either the broker closes iChan, or the broker
	// returns via pc.context.Done()
	go func() {
		for {
			msgIf, ok := <-iChan
			if !ok {
				close(uChan)

				return
			}
			msg, ok := msgIf.(UnreachableNotification)
			if !ok {
				continue
			}
			uChan <- msg
		}
	}()

	return uChan
}

// ReadFrom reads a packet from the network and returns its data and address.
func (pc *PacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	var m *MessageData
	if pc.readDeadline.IsZero() {
		select {
		case m = <-pc.recvChan:
		case <-pc.context.Done():
			return 0, nil, fmt.Errorf("connection context closed")
		}
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
		network: pc.s.GetNetworkName(),
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
	err = pc.s.SendMessageWithHopsToLive(pc.localService, ncaddr.node, ncaddr.service, p, pc.hopsToLive)
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
		network: pc.s.GetNetworkName(),
		node:    pc.s.NodeID(),
		service: pc.localService,
	}
}

// Close closes the connection.
func (pc *PacketConn) Close() error {
	pc.s.GetListenerLock().Lock()
	defer pc.s.GetListenerLock().Unlock()
	delete(pc.s.GetListenerRegistery(), pc.localService)
	if pc.cancel != nil {
		pc.cancel()
	}
	if pc.advertise {
		err := pc.s.RemoveLocalServiceAdvertisement(pc.localService)
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
func (pc *PacketConn) SetWriteDeadline(_ time.Time) error {
	// Write deadline doesn't mean anything because Write() implementation is non-blocking.
	return nil
}
