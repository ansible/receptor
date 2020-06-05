package netceptor

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/lucas-clemente/quic-go"
	"math/big"
	"net"
	"time"
)

// Listener implements the net.Listener interface via the Receptor network
type Listener struct {
	s  *Netceptor
	pc *PacketConn
	ql quic.Listener
}

// Internal implementation of Listen and ListenAndAdvertise
func (s *Netceptor) listen(service string, advertise bool, adTags map[string]string) (*Listener, error) {
	s.listenerLock.Lock()
	defer s.listenerLock.Unlock()
	if service == "" {
		service = s.getEphemeralService()
	} else if len(service) > 8 {
		return nil, fmt.Errorf("service name %s too long", service)
	} else {
		_, isReserved := s.reservedServices[service]
		_, isListening := s.listenerRegistry[service]
		if isReserved || isListening {
			return nil, fmt.Errorf("service %s is already listening", service)
		}
	}
	_ = s.addNameHash(service)
	pc := &PacketConn{
		s:            s,
		localService: service,
		recvChan:     make(chan *messageData),
		advertise:    advertise,
		adTags:       adTags,
	}
	s.listenerRegistry[service] = pc
	ql, err := quic.Listen(pc, generateServerTLSConfig(), nil)
	if err != nil {
		return nil, err
	}
	if advertise {
		s.addLocalServiceAdvertisement(service, adTags)
	}
	return &Listener{
		s:  s,
		pc: pc,
		ql: ql,
	}, nil
}

// Listen returns a stream listener compatible with Go's net.Listener.
// If service is blank, generates and uses an ephemeral service name.
func (s *Netceptor) Listen(service string) (*Listener, error) {
	return s.listen(service, false, nil)
}

// ListenAndAdvertise listens for stream connections on a service and also advertises it via broadcasts.
func (s *Netceptor) ListenAndAdvertise(service string, tags map[string]string) (*Listener, error) {
	return s.listen(service, true, tags)
}

// Accept accepts a connection via the listener
func (li *Listener) Accept() (net.Conn, error) {
	qc, err := li.ql.Accept(context.Background())
	if err != nil {
		return nil, err
	}
	qs, err := qc.AcceptStream(context.Background())
	if err != nil {
		return nil, err
	}
	return &Conn{
		s:  li.s,
		pc: li.pc,
		qc: qc,
		qs: qs,
	}, nil
}

// Close closes the listener
func (li *Listener) Close() error {
	qerr := li.ql.Close()
	perr := li.pc.Close()
	if qerr != nil {
		return qerr
	}
	return perr
}

// Addr returns the local address of this listener
func (li *Listener) Addr() net.Addr {
	return li.pc.LocalAddr()
}

// Conn implements the net.Conn interface via the Receptor network
type Conn struct {
	s             *Netceptor
	pc            *PacketConn
	qc            quic.Session
	qs            quic.Stream
	readDeadline  time.Time
	writeDeadline time.Time
}

// Dial returns a stream connection compatible with Go's net.Conn.
func (s *Netceptor) Dial(node string, service string) (*Conn, error) {
	_ = s.addNameHash(node)
	_ = s.addNameHash(service)
	lservice := s.getEphemeralService()
	pc, err := s.ListenPacket(lservice)
	if err != nil {
		return nil, err
	}
	rAddr := NewAddr(node, service)
	cfg := &quic.Config{
		KeepAlive: true,
	}
	qc, err := quic.Dial(pc, rAddr, s.nodeID, generateClientTLSConfig(), cfg)
	if err != nil {
		return nil, err
	}
	qs, err := qc.OpenStream()
	if err != nil {
		return nil, err
	}
	return &Conn{
		s:  s,
		pc: pc,
		qc: qc,
		qs: qs,
	}, nil
}

// Read reads data from the connection
func (c *Conn) Read(b []byte) (n int, err error) {
	//TODO: respect nc.readDeadline
	return c.qs.Read(b)
}

// Write writes data to the connection
func (c *Conn) Write(b []byte) (n int, err error) {
	//TODO: respect nc.writeDeadline
	return c.qs.Write(b)
}

// Close closes the writer side of the connection
func (c *Conn) Close() error {
	return c.qs.Close()
}

// LocalAddr returns the local address of this connection
func (c *Conn) LocalAddr() net.Addr {
	return c.qc.LocalAddr()
}

// RemoteAddr returns the remote address of this connection
func (c *Conn) RemoteAddr() net.Addr {
	return c.qc.RemoteAddr()
}

// SetDeadline sets both read and write deadlines
func (c *Conn) SetDeadline(t time.Time) error {
	c.readDeadline = t
	c.writeDeadline = t
	return nil
}

// SetReadDeadline sets the read deadline
func (c *Conn) SetReadDeadline(t time.Time) error {
	c.readDeadline = t
	return nil
}

// SetWriteDeadline sets the write deadline
func (c *Conn) SetWriteDeadline(t time.Time) error {
	c.writeDeadline = t
	return nil
}

//TODO: Remove this and do real TLS with properly configured certs etc
func generateServerTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"netceptor"},
	}
}

//TODO: Remove this and do real TLS with properly configured certs etc
func generateClientTLSConfig() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"netceptor"},
	}
}
