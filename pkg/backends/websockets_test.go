package backends_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/backends"
	"github.com/ansible/receptor/pkg/backends/mock_backends"
	"github.com/ansible/receptor/pkg/logger"
	"github.com/golang/mock/gomock"
	"github.com/gorilla/websocket"
)

func setupTlsCfg(t *testing.T) tls.Certificate {
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
	return tlsCert
}

func TestNewWebsocketDialer(t *testing.T) {

	NewWebsocketDialerTestCases := []struct {
		name             string
		address          string
		redial           bool
		tlscfg           *tls.Config
		extraHeader      string
		logger           *logger.ReceptorLogger
		expectedErr      string
		failedTestString string
	}{
		{
			name:    "NewWebsocketDialer wss Success ",
			address: "wss://test.testing",
			redial:  false,
			tlscfg: &tls.Config{
				Certificates:             []tls.Certificate{setupTlsCfg(t)},
				MinVersion:               tls.VersionTLS12,
				PreferServerCipherSuites: true,
			},
			extraHeader:      "",
			logger:           logger.NewReceptorLogger("websockets_test.go>TestNewWebsocketDialer"),
			expectedErr:      "",
			failedTestString: "Expected no error, but got: %v",
		},
		{
			name:    "NewWebsocketDialer non-wss Success ",
			address: "test.testing",
			redial:  false,
			tlscfg: &tls.Config{
				Certificates:             []tls.Certificate{setupTlsCfg(t)},
				MinVersion:               tls.VersionTLS12,
				PreferServerCipherSuites: true,
			},
			extraHeader:      "",
			logger:           logger.NewReceptorLogger("websockets_test.go>TestNewWebsocketDialer"),
			expectedErr:      "",
			failedTestString: "Expected no error, but got: %v",
		},
	}

	for _, testCase := range NewWebsocketDialerTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := backends.NewWebsocketDialer(testCase.address, testCase.tlscfg, testCase.extraHeader, testCase.redial, testCase.logger)
			if testCase.expectedErr == "" && err != nil {
				t.Errorf(testCase.failedTestString, err)
			}
			if testCase.expectedErr != "" && err != nil && err.Error() != testCase.expectedErr {
				t.Errorf(testCase.failedTestString, err)
			}
			if testCase.expectedErr != "" && err == nil {
				t.Errorf(testCase.failedTestString, err)
			}
		})
	}
}

func TestWebsocketDialerStart(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWebsocketDialer := mock_backends.NewMockGorillaWebsocketDialerForDialer(ctrl)
	NewWebsocketDialerTestCases := []struct {
		name             string
		expectedCall     func(ctx context.Context, ctxCancel context.CancelFunc)
		expectedErr      string
		failedTestString string
	}{
		{
			name: "Start Success",
			expectedCall: func(ctx context.Context, ctxCancel context.CancelFunc) {
				header := make(http.Header)
				header.Add("origin", "test")
				mockWebsocketDialer.EXPECT().DialContext(ctx, "wss://test.testing", header).Return(&websocket.Conn{}, &http.Response{}, nil)
			},
			expectedErr:      "",
			failedTestString: "Expected no error, but got: %v",
		},
		{
			name: "Start Error",
			expectedCall: func(ctx context.Context, ctxCancel context.CancelFunc) {
				header := make(http.Header)
				header.Add("origin", "test")
				mockWebsocketDialer.EXPECT().DialContext(ctx, "wss://test.testing", header).Return(&websocket.Conn{}, &http.Response{}, nil)
			},
			expectedErr:      "",
			failedTestString: "Expected no error, but got: %v",
		},
	}

	for _, testCase := range NewWebsocketDialerTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, ctxCancel := context.WithCancel(context.Background())
			defer ctx.Done()
			tlscfg := &tls.Config{
				Certificates:             []tls.Certificate{setupTlsCfg(t)},
				MinVersion:               tls.VersionTLS12,
				PreferServerCipherSuites: true,
			}

			testCase.expectedCall(ctx, ctxCancel)

			wd, wdErr := backends.NewWebsocketDialer("wss://test.testing", tlscfg, "", false, logger.NewReceptorLogger("websockets_test.go>TestNewWebsocketDialer"))
			if wdErr != nil {
				t.Errorf("NewWebsockerDialer return error: %v", wdErr)
			}
			sessChan, err := wd.Start(ctx, &sync.WaitGroup{})
			go func() {
				for {
					select {
					case _, ok := <-sessChan:
						if testCase.expectedErr == "" && !ok {
							t.Errorf(testCase.failedTestString, "error")
						}
					}
				}
			}()
			if testCase.expectedErr == "" && err != nil {
				t.Errorf(testCase.failedTestString, err)
			}
			if testCase.expectedErr != "" && err != nil && err.Error() != testCase.expectedErr {
				t.Errorf(testCase.failedTestString, err)
			}
			if testCase.expectedErr != "" && err == nil {
				t.Errorf(testCase.failedTestString, err)
			}
		})
	}
}

func TestWebsocketListener(t *testing.T) {
	tlscfg := &tls.Config{
		Certificates:             []tls.Certificate{setupTlsCfg(t)},
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
	}
	NewWebsocketDialerTestCases := []struct {
		name             string
		path             string
		address          string
		tlscfg           *tls.Config
		logger           *logger.ReceptorLogger
		callMeathod      func(string, string, backends.WebsocketListener)
		expectedErr      string
		failedTestString string
	}{
		{
			name:             "NewWebsocketListener wss Success ",
			path:             "",
			address:          "wss://test.testing",
			tlscfg:           tlscfg,
			logger:           logger.NewReceptorLogger("websockets_test.go>TestNewWebsocketDialer"),
			expectedErr:      "",
			failedTestString: "Expected no error, but got: %v",
		},
		{
			name:    "WebsocketDialer GetAddr",
			path:    "",
			address: "wss://test.testing",
			tlscfg:  tlscfg,
			logger:  logger.NewReceptorLogger("websockets_test.go>TestNewWebsocketDialer"),
			callMeathod: func(address string, path string, wl backends.WebsocketListener) {
				_, err := wl.Start(context.Background(), &sync.WaitGroup{})
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
				addr := wl.Addr()
				if addr.String() != address {
					t.Errorf("Expected WebsocketListener.GetAddr to be %v, got %v", address, addr)
				}
			},
			expectedErr:      "",
			failedTestString: "Expected no error, but got: %v",
		},
		{
			name:    "WebsocketDialer GetTLS",
			path:    "",
			address: "wss://test.testing",
			tlscfg:  tlscfg,
			logger:  logger.NewReceptorLogger("websockets_test.go>TestNewWebsocketDialer"),
			callMeathod: func(address string, path string, wl backends.WebsocketListener) {
				tlsConfig := wl.GetTLS()
				if tlsConfig != tlscfg {
					t.Errorf("Expected WebsocketListener.GetTLS to be %v, got %v", tlscfg, tlsConfig)
				}
			},
			expectedErr:      "",
			failedTestString: "Expected no error, but got: %v",
		},
		{
			name:    "WebsocketDialer Setpath",
			path:    "test",
			address: "wss://test.testing",
			tlscfg:  tlscfg,
			logger:  logger.NewReceptorLogger("websockets_test.go>TestNewWebsocketDialer"),
			callMeathod: func(address string, path string, wl backends.WebsocketListener) {
				wl.SetPath(path)
				if wl.Path() != path {
					t.Errorf("Expected WebsocketListener.path to be %v, got %v", path, wl.Path())
				}
			},
			expectedErr:      "",
			failedTestString: "Expected no error, but got: %v",
		},
	}

	for _, testCase := range NewWebsocketDialerTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			wl, err := backends.NewWebsocketListener(testCase.address, testCase.tlscfg, testCase.logger)
			if testCase.expectedErr == "" && err != nil {
				t.Errorf(testCase.failedTestString, err)
			}
			if testCase.expectedErr != "" && err != nil && err.Error() != testCase.expectedErr {
				t.Errorf(testCase.failedTestString, err)
			}
			if testCase.expectedErr != "" && err == nil {
				t.Errorf(testCase.failedTestString, err)
			}
			if testCase.callMeathod != nil {
				testCase.callMeathod(testCase.address, testCase.path, *wl)
			}
		})
	}
}
