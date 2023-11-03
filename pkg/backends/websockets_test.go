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
	"strings"
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

func TestWebsocketDialerGetAddr(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockWebsocketDialer := mock_backends.NewMockGorillaWebsocketDialerForDialer(ctrl)

	address := "wss://test.testing"
	wd, wdErr := backends.NewWebsocketDialer(address, &tls.Config{}, "", false, logger.NewReceptorLogger("websockets_test.go>TestNewWebsocketDialer"), mockWebsocketDialer)
	if wdErr != nil {
		t.Errorf("NewWebsockerDialer return error: %v", wdErr)
	}
	add := wd.GetAddr()
	if add != address {
		t.Errorf("Expected Dialer Address to be %s, got %s instead", address, add)
	}
}

func TestWebsocketDialerGetTLS(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockWebsocketDialer := mock_backends.NewMockGorillaWebsocketDialerForDialer(ctrl)

	blankTLS := &tls.Config{}
	wd, wdErr := backends.NewWebsocketDialer("wss://test.testing", blankTLS, "", false, logger.NewReceptorLogger("websockets_test.go>TestNewWebsocketDialer"), mockWebsocketDialer)
	if wdErr != nil {
		t.Errorf("NewWebsockerDialer return error: %v", wdErr)
	}
	TLS := wd.GetTLS()
	if TLS != blankTLS {
		t.Errorf("Expected Dialer TLS to be %v, got %v instead", blankTLS, TLS)
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

func TestNewWebsocketListener(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockWebsocketUpgrader := mock_backends.NewMockGorillaWebsocketUpgraderForListener(ctrl)
	mockServer := mock_backends.NewMockHttpServerForListener(ctrl)

	wi, err := backends.NewWebsocketListener("address", &tls.Config{}, logger.NewReceptorLogger("test"), mockWebsocketUpgrader, mockServer)
	if err != nil {
		t.Errorf(err.Error())
	}

	if wi == nil {
		t.Errorf("Websocket listener expected, nil returned")
	}
}

func TestWebsocketListenerSetandGetPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockWebsocketUpgrader := mock_backends.NewMockGorillaWebsocketUpgraderForListener(ctrl)
	mockServer := mock_backends.NewMockHttpServerForListener(ctrl)

	wi, err := backends.NewWebsocketListener("address", &tls.Config{}, logger.NewReceptorLogger("test"), mockWebsocketUpgrader, mockServer)
	if err != nil {
		t.Errorf(err.Error())
	}

	if wi == nil {
		t.Errorf("Websocket listener expected, nil returned")
	}
	pathName := "Test Path"

	wi.SetPath(pathName)
	p := wi.Path()
	if p != pathName {
		t.Errorf("Expected path to be %s got %s instead", pathName, p)
	}
}

func TestWebsocketListenerStart(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()
	defer ctx.Done()

	mockWebsocketUpgrader := mock_backends.NewMockGorillaWebsocketUpgraderForListener(ctrl)
	mockServer := mock_backends.NewMockHttpServerForListener(ctrl)
	mockWebsocketConner := mock_backends.NewMockConner(ctrl)

	wi, err := backends.NewWebsocketListener("localhost:21700", &tls.Config{}, logger.NewReceptorLogger("test"), mockWebsocketUpgrader, mockServer)
	if err != nil {
		t.Errorf(err.Error())
	}
	if wi == nil {
		t.Errorf("Websocket listener expected, nil returned")
	}

	mockWebsocketUpgrader.EXPECT().Upgrade(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockWebsocketConner, nil).AnyTimes()
	mockServer.EXPECT().SetHandeler(gomock.Any())
	mockServer.EXPECT().SetTLSConfig(gomock.Any()).AnyTimes()
	mockServer.EXPECT().ServeTLS(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	bs, err := wi.Start(ctx, &sync.WaitGroup{})
	if err != nil {
		t.Errorf(err.Error())
	}
	if bs == nil {
		t.Errorf("Expected Websocket Listener, got nil")
	}
}

func TestWebsocketListenerStartUpgradeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()
	defer ctx.Done()

	mockWebsocketUpgrader := mock_backends.NewMockGorillaWebsocketUpgraderForListener(ctrl)
	mockServer := mock_backends.NewMockHttpServerForListener(ctrl)

	wi, err := backends.NewWebsocketListener("localhost:21701", &tls.Config{}, logger.NewReceptorLogger("test"), mockWebsocketUpgrader, mockServer)
	if err != nil {
		t.Errorf(err.Error())
	}
	if wi == nil {
		t.Errorf("Websocket listener expected, nil returned")
	}
	returnError := errors.New("Upgrade Error")

	mockWebsocketUpgrader.EXPECT().Upgrade(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, returnError).AnyTimes()
	mockServer.EXPECT().SetHandeler(gomock.Any())
	mockServer.EXPECT().SetTLSConfig(gomock.Any()).AnyTimes()
	mockServer.EXPECT().ServeTLS(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	bs, err := wi.Start(ctx, &sync.WaitGroup{})
	if err != nil {
		t.Errorf("Expected error %v, got %v instead", nil, err)
	}
	if bs == nil {
		t.Errorf("Expected Websocket Listener, got nil")
	}
}

func TestWebsocketListenerStartNetError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()
	defer ctx.Done()

	mockWebsocketUpgrader := mock_backends.NewMockGorillaWebsocketUpgraderForListener(ctrl)
	mockServer := mock_backends.NewMockHttpServerForListener(ctrl)
	mockWebsocketConner := mock_backends.NewMockConner(ctrl)

	badAddress := "127.0.0.1:80"
	wi, err := backends.NewWebsocketListener(badAddress, &tls.Config{}, logger.NewReceptorLogger("test"), mockWebsocketUpgrader, mockServer)
	if err != nil {
		t.Errorf(err.Error())
	}
	if wi == nil {
		t.Errorf("Websocket listener expected, nil returned")
	}

	mockWebsocketUpgrader.EXPECT().Upgrade(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockWebsocketConner, nil).AnyTimes()

	bs, err := wi.Start(ctx, &sync.WaitGroup{})
	if !strings.Contains(err.Error(), "listen tcp 127.0.0.1:80: bind: permission denied") {
		t.Errorf(err.Error())
	}
	if bs != nil {
		t.Errorf("Expected Websocket Listener to be nil")
	}
}

func TestWebsocketListenerStartTLSNil(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()
	defer ctx.Done()

	mockWebsocketUpgrader := mock_backends.NewMockGorillaWebsocketUpgraderForListener(ctrl)
	mockServer := mock_backends.NewMockHttpServerForListener(ctrl)
	mockWebsocketConner := mock_backends.NewMockConner(ctrl)

	wi, err := backends.NewWebsocketListener("localhost:21702", nil, logger.NewReceptorLogger("test"), mockWebsocketUpgrader, mockServer)
	if err != nil {
		t.Errorf(err.Error())
	}
	if wi == nil {
		t.Errorf("Websocket listener expected, nil returned")
	}

	mockWebsocketUpgrader.EXPECT().Upgrade(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockWebsocketConner, nil).AnyTimes()
	mockServer.EXPECT().SetHandeler(gomock.Any())
	mockServer.EXPECT().Serve(gomock.Any()).Return(errors.New("Test Error")).AnyTimes()

	bs, err := wi.Start(ctx, &sync.WaitGroup{})
	if err != nil {
		t.Errorf(err.Error())
	}
	if bs == nil {
		t.Errorf("Expected Websocket Listener not be nil")
	}
}

func TestWebsocketListenerGetAddr(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()
	defer ctx.Done()

	mockWebsocketUpgrader := mock_backends.NewMockGorillaWebsocketUpgraderForListener(ctrl)
	mockServer := mock_backends.NewMockHttpServerForListener(ctrl)
	mockWebsocketConner := mock_backends.NewMockConner(ctrl)
	address := "127.0.0.1:21703"

	wi, err := backends.NewWebsocketListener(address, &tls.Config{}, logger.NewReceptorLogger("test"), mockWebsocketUpgrader, mockServer)
	if err != nil {
		t.Errorf(err.Error())
	}
	if wi == nil {
		t.Errorf("Websocket listener expected, nil returned")
	}

	mockWebsocketUpgrader.EXPECT().Upgrade(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockWebsocketConner, nil).AnyTimes()
	mockServer.EXPECT().SetHandeler(gomock.Any())
	mockServer.EXPECT().SetTLSConfig(gomock.Any()).AnyTimes()
	mockServer.EXPECT().ServeTLS(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	bs, err := wi.Start(ctx, &sync.WaitGroup{})
	if err != nil {
		t.Errorf(err.Error())
	}
	if bs == nil {
		t.Errorf("Expected Websocket Listener, got nil")
	}

	add := wi.GetAddr()
	if add != address {
		t.Errorf("Expected Listener Address to be %s, got %s instead", address, add)
	}
}

func TestWebsocketListenerGetTLS(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()
	defer ctx.Done()

	mockWebsocketUpgrader := mock_backends.NewMockGorillaWebsocketUpgraderForListener(ctrl)
	mockServer := mock_backends.NewMockHttpServerForListener(ctrl)

	blankTLS := &tls.Config{}
	wi, err := backends.NewWebsocketListener("127.0.0.1:21704", blankTLS, logger.NewReceptorLogger("test"), mockWebsocketUpgrader, mockServer)
	if err != nil {
		t.Errorf(err.Error())
	}
	if wi == nil {
		t.Errorf("Websocket listener expected, nil returned")
	}

	TLS := wi.GetTLS()
	if TLS != blankTLS {
		t.Errorf("Expected Dialer TLS to be %v, got %v instead", blankTLS, TLS)
	}
}

func TestWebsocketListenerCfg(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	baseBindAddr := "127.0.0.1"
	basePort := 21710
	basePath := ""
	baseTLS := ""
	baseCost := 1.0

	wlc := backends.WebsocketListenerCfg{
		BindAddr: baseBindAddr,
		Port:     basePort,
		Path:     basePath,
		TLS:      baseTLS,
		Cost:     baseCost,
	}

	c := wlc.GetCost()
	if c != baseCost {
		t.Errorf("Expected %v, got %v", baseCost, c)
	}

	a := wlc.GetAddr()
	if a != baseBindAddr {
		t.Errorf("Expected %s, got %s", baseBindAddr, a)
	}

	tls := wlc.GetTLS()
	if tls != baseTLS {
		t.Errorf("Expected %s, got %s", baseTLS, tls)
	}

	err := wlc.Prepare()
	if err != nil {
		t.Errorf(err.Error())
	}
}
