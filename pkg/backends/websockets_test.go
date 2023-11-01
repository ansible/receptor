package backends_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/backends"
	"github.com/ansible/receptor/pkg/backends/mock_backends"
	"github.com/ansible/receptor/pkg/logger"
	"github.com/golang/mock/gomock"
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
			_, err := backends.NewWebsocketDialer(testCase.address, testCase.tlscfg, testCase.extraHeader, testCase.redial, testCase.logger, nil)
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
	mockWebsocketConner := mock_backends.NewMockConner(ctrl)
	ctx := context.Background()
	defer ctx.Done()

	wd, wdErr := backends.NewWebsocketDialer("wss://test.testing", &tls.Config{}, "", false, logger.NewReceptorLogger("websockets_test.go>TestNewWebsocketDialer"), mockWebsocketDialer)
	if wdErr != nil {
		t.Errorf("NewWebsockerDialer return error: %v", wdErr)
	}
	resp := &http.Response{
		Body: io.NopCloser(bytes.NewBufferString("Hello World")),
	}
	mockWebsocketDialer.EXPECT().DialContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockWebsocketConner, resp, nil)
	mockWebsocketConner.EXPECT().ReadMessage().Return(0, []byte{}, nil).AnyTimes()
	sess, err := wd.Start(ctx, &sync.WaitGroup{})
	if err != nil {
		t.Errorf(err.Error())
	}
	s := <-sess

	if s == nil {
		t.Errorf("session should not be nil")
	}
}

func TestWebsocketDialerStartError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockWebsocketDialer := mock_backends.NewMockGorillaWebsocketDialerForDialer(ctrl)
	mockWebsocketConner := mock_backends.NewMockConner(ctrl)
	ctx := context.Background()
	defer ctx.Done()

	wd, wdErr := backends.NewWebsocketDialer("wss://test.testing", &tls.Config{}, "", false, logger.NewReceptorLogger("websockets_test.go>TestNewWebsocketDialer"), mockWebsocketDialer)
	if wdErr != nil {
		t.Errorf("NewWebsockerDialer return error: %v", wdErr)
	}
	resp := &http.Response{
		Body: io.NopCloser(bytes.NewBufferString("Hello World")),
	}
	mockWebsocketDialer.EXPECT().DialContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockWebsocketConner, resp, errors.New("Websocket Start error"))
	mockWebsocketConner.EXPECT().ReadMessage().Return(0, []byte{}, nil).AnyTimes()
	sess, err := wd.Start(ctx, &sync.WaitGroup{})
	if err != nil {
		t.Errorf(err.Error())
	}
	s := <-sess

	if s != nil {
		t.Errorf("session should be nil")
	}
}
