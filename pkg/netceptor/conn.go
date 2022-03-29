package netceptor

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/lucas-clemente/quic-go"
)

// MaxIdleTimeoutForQuicConnections for quic connections. The default is 30 which we have replicated here.
// This value is set on both Dial and Listen connections as the quic library would take the smallest of either connection.
var MaxIdleTimeoutForQuicConnections = 30 * time.Second

// KeepAliveForQuicConnections is variablized to enable testing of the timeout.
// If you are doing a heartbeat your connection wont timeout without severing the connection i.e. firewall.
// Having this variablized allows the tests to set KeepAliveForQuicConnections = False so that things will properly fail.
var KeepAliveForQuicConnections = true

type acceptResult struct {
	conn net.Conn
	err  error
}

// Listener implements the net.Listener interface via the Receptor network.
type Listener struct {
	s          *Netceptor
	pc         *PacketConn
	ql         quic.Listener
	acceptChan chan *acceptResult
	doneChan   chan struct{}
	doneOnce   *sync.Once
}

// Internal implementation of Listen and ListenAndAdvertise.
func (s *Netceptor) listen(ctx context.Context, service string, tlscfg *tls.Config, advertise bool, adTags map[string]string) (*Listener, error) {
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
	var connType byte
	if tlscfg == nil {
		connType = ConnTypeStream
		tlscfg = generateServerTLSConfig()
	} else {
		connType = ConnTypeStreamTLS
		tlscfg = tlscfg.Clone()
		tlscfg.NextProtos = []string{"netceptor"}
		if tlscfg.ClientAuth == tls.RequireAndVerifyClientCert {
			tlscfg.GetConfigForClient = func(hi *tls.ClientHelloInfo) (*tls.Config, error) {
				clientTLSCfg := tlscfg.Clone()
				remoteNode := strings.Split(hi.Conn.RemoteAddr().String(), ":")[0]
				clientTLSCfg.VerifyPeerCertificate = ReceptorVerifyFunc(tlscfg, [][]byte{}, remoteNode, ExpectedHostnameTypeReceptor, VerifyClient)

				return clientTLSCfg, nil
			}
		}
	}
	pc := &PacketConn{
		s:            s,
		localService: service,
		recvChan:     make(chan *MessageData),
		advertise:    advertise,
		adTags:       adTags,
		connType:     connType,
		hopsToLive:   s.maxForwardingHops,
	}
	pc.startUnreachable()
	s.listenerRegistry[service] = pc
	cfg := &quic.Config{
		MaxIdleTimeout: MaxIdleTimeoutForQuicConnections,
	}
	_ = os.Setenv("QUIC_GO_DISABLE_RECEIVE_BUFFER_WARNING", "1")
	ql, err := quic.Listen(pc, tlscfg, cfg)
	if err != nil {
		return nil, err
	}
	if advertise {
		s.addLocalServiceAdvertisement(service, connType, adTags)
	}
	doneChan := make(chan struct{})
	go func() {
		select {
		case <-s.context.Done():
			_ = ql.Close()
		case <-ctx.Done():
			_ = ql.Close()
		case <-doneChan:
			return
		}
	}()
	li := &Listener{
		s:          s,
		pc:         pc,
		ql:         ql,
		acceptChan: make(chan *acceptResult),
		doneChan:   doneChan,
		doneOnce:   &sync.Once{},
	}

	go li.acceptLoop()

	return li, nil
}

// Listen returns a stream listener compatible with Go's net.Listener.
// If service is blank, generates and uses an ephemeral service name.
func (s *Netceptor) Listen(service string, tlscfg *tls.Config) (*Listener, error) {
	return s.listen(context.Background(), service, tlscfg, false, nil)
}

// ListenAndAdvertise listens for stream connections on a service and also advertises it via broadcasts.
func (s *Netceptor) ListenAndAdvertise(service string, tlscfg *tls.Config, tags map[string]string) (*Listener, error) {
	return s.listen(context.Background(), service, tlscfg, true, tags)
}

// ListenContext returns a stream listener compatible with Go's net.Listener.
// If service is blank, generates and uses an ephemeral service name.
func (s *Netceptor) ListenContext(ctx context.Context, service string, tlscfg *tls.Config) (*Listener, error) {
	return s.listen(ctx, service, tlscfg, false, nil)
}

// ListenContextAndAdvertise listens for stream connections on a service and also advertises it via broadcasts.
func (s *Netceptor) ListenContextAndAdvertise(ctx context.Context, service string, tlscfg *tls.Config, tags map[string]string) (*Listener, error) {
	return s.listen(ctx, service, tlscfg, true, tags)
}

func (li *Listener) sendResult(conn net.Conn, err error) {
	select {
	case li.acceptChan <- &acceptResult{
		conn: conn,
		err:  err,
	}:
	case <-li.doneChan:
	}
}

func (li *Listener) acceptLoop() {
	for {
		select {
		case <-li.doneChan:
			return
		default:
		}
		qc, err := li.ql.Accept(context.Background())
		select {
		case <-li.doneChan:
			return
		default:
		}
		if err != nil {
			li.sendResult(nil, err)

			continue
		}
		go func() {
			ctx, _ := context.WithTimeout(context.Background(), 60*time.Second)
			qs, err := qc.AcceptStream(ctx)
			select {
			case <-li.doneChan:
				_ = qc.CloseWithError(500, "Listener Closed")

				return
			default:
			}
			if os.IsTimeout(err) {
				_ = qc.CloseWithError(500, "Accept Timeout")

				return
			} else if err != nil {
				_ = qc.CloseWithError(500, fmt.Sprintf("AcceptStream Error: %s", err.Error()))
				li.sendResult(nil, err)

				return
			}
			buf := make([]byte, 1)
			n, err := qs.Read(buf)
			if err != nil {
				_ = qc.CloseWithError(500, fmt.Sprintf("Read Error: %s", err.Error()))
				li.sendResult(nil, err)

				return
			}
			if n != 1 || buf[0] != 0 {
				_ = qc.CloseWithError(500, "Read Data Error")
				li.sendResult(nil, fmt.Errorf("stream failed to initialize"))

				return
			}
			doneChan := make(chan struct{}, 1)
			cctx, ccancel := context.WithCancel(li.s.context)
			conn := &Conn{
				s:        li.s,
				pc:       li.pc,
				qc:       qc,
				qs:       qs,
				doneChan: doneChan,
				doneOnce: &sync.Once{},
				ctx:      cctx,
			}
			rAddr, ok := conn.RemoteAddr().(Addr)
			if ok {
				go monitorUnreachable(li.pc, doneChan, rAddr, ccancel)
			}
			go func() {
				select {
				case <-li.doneChan:
					_ = conn.Close()
				case <-cctx.Done():
					_ = conn.Close()
				case <-doneChan:
					return
				}
			}()
			li.sendResult(conn, err)
		}()
	}
}

// Accept accepts a connection via the listener.
func (li *Listener) Accept() (net.Conn, error) {
	select {
	case ar := <-li.acceptChan:
		return ar.conn, ar.err
	case <-li.doneChan:
		return nil, fmt.Errorf("listener closed")
	}
}

// Close closes the listener.
func (li *Listener) Close() error {
	li.doneOnce.Do(func() {
		close(li.doneChan)
	})
	perr := li.pc.Close()
	if qerr := li.ql.Close(); qerr != nil {
		return qerr
	}

	return perr
}

// Addr returns the local address of this listener.
func (li *Listener) Addr() net.Addr {
	return li.pc.LocalAddr()
}

// Conn implements the net.Conn interface via the Receptor network.
type Conn struct {
	s        *Netceptor
	pc       *PacketConn
	qc       quic.Session
	qs       quic.Stream
	doneChan chan struct{}
	doneOnce *sync.Once
	ctx      context.Context
}

// Dial returns a stream connection compatible with Go's net.Conn.
func (s *Netceptor) Dial(node string, service string, tlscfg *tls.Config) (*Conn, error) {
	return s.DialContext(context.Background(), node, service, tlscfg)
}

// DialContext is like Dial but uses a context to allow timeout or cancellation.
func (s *Netceptor) DialContext(ctx context.Context, node string, service string, tlscfg *tls.Config) (*Conn, error) {
	_ = s.addNameHash(node)
	_ = s.addNameHash(service)
	pc, err := s.ListenPacket("")
	if err != nil {
		return nil, err
	}
	rAddr := s.NewAddr(node, service)
	cfg := &quic.Config{
		HandshakeIdleTimeout: 15 * time.Second,
		MaxIdleTimeout:       MaxIdleTimeoutForQuicConnections,
		KeepAlive:            KeepAliveForQuicConnections,
	}
	if tlscfg == nil {
		tlscfg = generateClientTLSConfig()
	} else {
		tlscfg = tlscfg.Clone()
		tlscfg.NextProtos = []string{"netceptor"}
	}
	okChan := make(chan struct{})
	closeOnce := sync.Once{}
	pcClose := func() {
		closeOnce.Do(func() {
			_ = pc.Close()
		})
	}
	cctx, ccancel := context.WithCancel(ctx)
	go func() {
		select {
		case <-okChan:
			return
		case <-cctx.Done():
			pcClose()
		case <-s.context.Done():
			pcClose()
		}
	}()
	doneChan := make(chan struct{}, 1)
	go monitorUnreachable(pc, doneChan, rAddr, ccancel)
	_ = os.Setenv("QUIC_GO_DISABLE_RECEIVE_BUFFER_WARNING", "1")
	qc, err := quic.DialContext(cctx, pc, rAddr, s.nodeID, tlscfg, cfg)
	if err != nil {
		close(okChan)
		pcClose()
		if cctx.Err() != nil {
			return nil, cctx.Err()
		}

		return nil, err
	}
	qs, err := qc.OpenStreamSync(cctx)
	if err != nil {
		close(okChan)
		_ = qc.CloseWithError(500, err.Error())
		_ = pc.Close()
		if cctx.Err() != nil {
			return nil, cctx.Err()
		}

		return nil, err
	}
	// We need to write something to the stream to trigger the Accept() to happen
	_, err = qs.Write([]byte{0})
	if err != nil {
		close(okChan)
		_ = qs.Close()
		_ = pc.Close()
		if cctx.Err() != nil {
			return nil, cctx.Err()
		}

		return nil, err
	}
	close(okChan)
	go func() {
		select {
		case <-qc.Context().Done():
			_ = qs.Close()
			_ = pc.Close()
		case <-s.context.Done():
			_ = qs.Close()
			_ = pc.Close()
		case <-doneChan:
			return
		}
	}()
	conn := &Conn{
		s:        s,
		pc:       pc,
		qc:       qc,
		qs:       qs,
		doneChan: doneChan,
		doneOnce: &sync.Once{},
		ctx:      cctx,
	}

	return conn, nil
}

// monitorUnreachable receives unreachable messages from the underlying PacketConn, and ends the connection
// if the remote service has gone away.
func monitorUnreachable(pc *PacketConn, doneChan chan struct{}, remoteAddr Addr, cancel context.CancelFunc) {
	msgCh := pc.SubscribeUnreachable(doneChan)
	if msgCh == nil {
		cancel()

		return
	}
	// read from channel until closed
	for msg := range msgCh {
		if msg.Problem == ProblemServiceUnknown && msg.ToNode == remoteAddr.node && msg.ToService == remoteAddr.service {
			logger.Warning("remote service %s to node %s is unreachable", msg.ToService, msg.ToNode)
			cancel()
		}
	}
}

// Read reads data from the connection.
func (c *Conn) Read(b []byte) (n int, err error) {
	return c.qs.Read(b)
}

// CancelRead cancels a pending read operation.
func (c *Conn) CancelRead() {
	c.qs.CancelRead(499)
}

// Write writes data to the connection.
func (c *Conn) Write(b []byte) (n int, err error) {
	return c.qs.Write(b)
}

// Close closes the writer side of the connection.
func (c *Conn) Close() error {
	c.doneOnce.Do(func() {
		close(c.doneChan)
	})

	return c.qs.Close()
}

func (c *Conn) CloseConnection() error {
	c.pc.cancel()
	c.doneOnce.Do(func() {
		close(c.doneChan)
	})
	logger.Debug("closing connection from service %s to %s", c.pc.localService, c.RemoteAddr().String())

	return c.qc.CloseWithError(0, "normal close")
}

// LocalAddr returns the local address of this connection.
func (c *Conn) LocalAddr() net.Addr {
	return c.qc.LocalAddr()
}

// RemoteAddr returns the remote address of this connection.
func (c *Conn) RemoteAddr() net.Addr {
	return c.qc.RemoteAddr()
}

// SetDeadline sets both read and write deadlines.
func (c *Conn) SetDeadline(t time.Time) error {
	return c.qs.SetDeadline(t)
}

// SetReadDeadline sets the read deadline.
func (c *Conn) SetReadDeadline(t time.Time) error {
	return c.qs.SetReadDeadline(t)
}

// SetWriteDeadline sets the write deadline.
func (c *Conn) SetWriteDeadline(t time.Time) error {
	return c.qs.SetWriteDeadline(t)
}

const insecureCommonName = "netceptor-insecure-common-name"

func generateServerTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: insecureCommonName,
		},
		NotBefore: time.Now().Add(-1 * time.Minute),
		NotAfter:  time.Now().Add(24 * time.Hour),
	}
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
		Certificates:             []tls.Certificate{tlsCert},
		NextProtos:               []string{"netceptor"},
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
	}
}

func verifyServerCertificate(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	for i := 0; i < len(rawCerts); i++ {
		cert, err := x509.ParseCertificate(rawCerts[i])
		if err != nil {
			continue
		}
		if cert.Subject.CommonName == insecureCommonName {
			return nil
		}
	}

	return fmt.Errorf("insecure connection to secure service")
}

func generateClientTLSConfig() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify:    true,
		VerifyPeerCertificate: verifyServerCertificate,
		NextProtos:            []string{"netceptor"},
		ServerName:            insecureCommonName,
		MinVersion:            tls.VersionTLS12,
	}
}
