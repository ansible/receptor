package netceptor_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/netceptor/mock_netceptor"
	"go.uber.org/mock/gomock"
)

func makeConn(t testing.TB, stream netceptor.QuicStreamForConn) *netceptor.Conn {
	t.Helper()
	conn := netceptor.NewConn(
		netceptor.New(context.TODO(), "test-node"), // netceptor
		nil,                    // PacketConner
		nil,                    // Connection
		stream,                 // Stream
		make(chan struct{}, 1), // doneChan
		&sync.Once{},           // doneOnce
		context.TODO(),         // context
	)
	return conn
}

func TestRead(t *testing.T) {
	ctrl := gomock.NewController(t)
	var buf = make([]byte, 1)
	// Create a mock QuicStream
	mock_stream := mock_netceptor.NewMockQuicStreamForConn(ctrl)
	// both success and error
	t.Run("Returns number of bytes from successful Read", func(t *testing.T) {
		want := 1
		mock_stream.EXPECT().Read(gomock.Eq(buf)).Return(want, nil).Times(1)
		conn := makeConn(t, mock_stream)
		got, err := conn.Read(buf)
		if err != nil {
			t.Fatalf("Read returned unexpected error %v", err)
		}
		if got != want {
			t.Errorf("Wanted %v, got %v", want, got)
		}
	})

	t.Run("Returns error from unsuccessful Read", func(t *testing.T) {
		want_err := errors.New("Read error")
		mock_stream.EXPECT().Read(gomock.Eq(buf)).Return(0, want_err).Times(1)
		conn := makeConn(t, mock_stream)
		_, got_err := conn.Read(buf)
		if got_err == nil {
			t.Errorf("Read did not return expected error")
		}
		if got_err != want_err {
			t.Errorf("Wanted %v, got %v", want_err, got_err)
		}
	})
}

func TestCancelRead(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock_stream := mock_netceptor.NewMockQuicStreamForConn(ctrl)
	mock_stream.EXPECT().CancelRead(gomock.Any()).Times(1)
	conn := makeConn(t, mock_stream)
	conn.CancelRead()
}

func TestWrite(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock_stream := mock_netceptor.NewMockQuicStreamForConn(ctrl)
	bytes := []byte{4, 8, 15, 16, 23, 42}
	t.Run("Returns number of bytes written in successful Write", func(t *testing.T) {
		want := 6
		mock_stream.EXPECT().Write(gomock.Eq(bytes)).Return(want, nil).Times(1)
		conn := makeConn(t, mock_stream)
		got, err := conn.Write(bytes)
		if err != nil {
			t.Fatalf("Write returned unexpected error %v", err)
		}
		if got != want {
			t.Errorf("Wanted %v, got %v", want, got)
		}
	})
	t.Run("Returns error from unsuccessful Write", func(t *testing.T) {
		want_err := errors.New("Write error")
		mock_stream.EXPECT().Write(gomock.Eq(bytes)).Return(0, want_err).Times(1)
		conn := makeConn(t, mock_stream)
		_, got_err := conn.Write(bytes)
		if got_err == nil {
			t.Errorf("Write did not return expected error")
		}
		if got_err != want_err {
			t.Errorf("Wanted %v, got %v", want_err, got_err)
		}
	})
}

func TestClose(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock_stream := mock_netceptor.NewMockQuicStreamForConn(ctrl)
	mock_stream.EXPECT().Close().Return(nil)
	conn := makeConn(t, mock_stream)
	err := conn.Close() // This calls the doneOnce and closes the doneChan
	// would be nice to test that the doneChan is closed
	if err != nil {
		t.Fatalf("conn.Close returned error")
	}
}
