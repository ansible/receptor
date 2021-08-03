package backends

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/project-receptor/receptor/pkg/netceptor"
)

// This test verifies that a websockets backend client can connect to an
// external backend running the websockets protocol.
func TestWebsocketExternalInterop(t *testing.T) {
	// Create a Netceptor with an external backend
	n1 := netceptor.New(context.Background(), "node1", nil)
	b1, err := netceptor.NewExternalBackend()
	if err != nil {
		t.Fatal(err)
	}
	err = n1.AddBackend(b1, 1.0, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a server TLS certificate for "localhost"
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "localhost",
		},
		DNSNames:  []string{"localhost"},
		NotBefore: time.Now().Add(-1 * time.Minute),
		NotAfter:  time.Now().Add(24 * time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}

	// Create a websocket server
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		extra := r.Header.Get("X-Extra-Data")
		if extra != "SomeData" {
			t.Fatal("Extra header not passed through")
		}
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Error upgrading websocket connection: %s", err)

			return
		}
		b1.NewConnection(netceptor.MessageConnFromWebsocketConn(conn), true)
	})
	li, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Error listening for TCP: %s", err)
	}
	server := &http.Server{
		Addr:    li.Addr().String(),
		Handler: mux,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
		},
	}
	go func() {
		err := server.ServeTLS(li, "", "")
		if err != nil {
			t.Fatalf("Error in web server: %s", err)
		}
	}()

	// Create a Netceptor websocket client that connects to our server
	n2 := netceptor.New(context.Background(), "node2", nil)
	CAcerts := x509.NewCertPool()
	CAcerts.AppendCertsFromPEM(certPEM)
	tls2 := &tls.Config{
		RootCAs:    CAcerts,
		ServerName: "localhost",
	}
	b2, err := NewWebsocketDialer("wss://"+li.Addr().String(), tls2, "X-Extra-Data: SomeData", true)
	if err != nil {
		t.Fatal(err)
	}
	err = n2.AddBackend(b2, 1.0, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for the nodes to establish routing to each other
	timeout, _ := context.WithTimeout(context.Background(), 2*time.Second)
	for {
		if timeout.Err() != nil {
			t.Fatal(timeout.Err())
		}
		_, ok := n1.Status().RoutingTable["node2"]
		if ok {
			_, ok := n2.Status().RoutingTable["node1"]
			if ok {
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Send a packet between nodes
	pc1, err := n1.ListenPacket("test")
	if err != nil {
		t.Fatal(err)
	}
	pc2, err := n2.ListenPacket("test")
	if err != nil {
		t.Fatal(err)
	}
	_, err = pc1.WriteTo([]byte("hello"), n1.NewAddr("node2", "test"))
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 16)
	n, _, err := pc2.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf[:n]) != "hello" {
		t.Fatal("Wrong message received")
	}

	// Shut down the nodes
	n1.Shutdown()
	n2.Shutdown()
	n1.BackendWait()
	n2.BackendWait()
}
