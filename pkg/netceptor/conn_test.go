package netceptor_test

import (
	"context"
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
	expected := 1
	mock_stream.EXPECT().Read(gomock.Eq(buf)).Return(expected, nil).Times(1)

	conn := makeConn(t, mock_stream)

	num, _ := conn.Read(buf)
	if num != expected {
		t.Errorf("Expected read to return %v, got %v", expected, num)
	}

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
	mock_stream.EXPECT().Write(gomock.Eq(bytes)).Return(6, nil).Times(1)
	conn := makeConn(t, mock_stream)
	written, err := conn.Write(bytes)
	if written != 6 {
		t.Errorf("Expected conn.Write to return %v, got %v", len(bytes), written)
	}
	if err != nil {
		t.Fatalf("conn.Write returned error")
	}
	// TODO: Tests for handling / returning errors
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
