package netceptor_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/netceptor/mock_netceptor"
	"go.uber.org/mock/gomock"
)

type TestConn struct {
	pc netceptor.PacketConner
	qc netceptor.QuicConnectionForConn
	qs netceptor.QuicStreamForConn
}

func makeConn(t testing.TB, tc TestConn) *netceptor.Conn {
	t.Helper()
	conn := netceptor.NewConn(
		netceptor.New(context.TODO(), "test-node"), // netceptor
		tc.pc,                  // PacketConner
		tc.qc,                  // Connection
		tc.qs,                  // Stream
		make(chan struct{}, 1), // doneChan
		&sync.Once{},           // doneOnce
		context.TODO(),         // context
	)
	return conn
}

// These tests operate on the quic Stream
func TestRead(t *testing.T) {
	ctrl := gomock.NewController(t)
	var buf = make([]byte, 1)
	// Create a mock QuicStream
	mock_qs := mock_netceptor.NewMockQuicStreamForConn(ctrl)
	// both success and error
	t.Run("Returns number of bytes from successful Read", func(t *testing.T) {
		want := 1
		mock_qs.EXPECT().Read(gomock.Eq(buf)).Return(want, nil).Times(1)
		conn := makeConn(t, TestConn{qs: mock_qs})
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
		mock_qs.EXPECT().Read(gomock.Eq(buf)).Return(0, want_err).Times(1)
		conn := makeConn(t, TestConn{qs: mock_qs})
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
	mock_qs := mock_netceptor.NewMockQuicStreamForConn(ctrl)
	// TODO: mock that it's Canceled with 499 and not Any()
	mock_qs.EXPECT().CancelRead(gomock.Any()).Times(1)
	conn := makeConn(t, TestConn{qs: mock_qs})
	conn.CancelRead()
}

func TestWrite(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock_qs := mock_netceptor.NewMockQuicStreamForConn(ctrl)
	bytes := []byte{4, 8, 15, 16, 23, 42}
	t.Run("Returns number of bytes written in successful Write", func(t *testing.T) {
		want := 6
		mock_qs.EXPECT().Write(gomock.Eq(bytes)).Return(want, nil).Times(1)
		conn := makeConn(t, TestConn{qs: mock_qs})
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
		mock_qs.EXPECT().Write(gomock.Eq(bytes)).Return(0, want_err).Times(1)
		conn := makeConn(t, TestConn{qs: mock_qs})
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
	mock_qs := mock_netceptor.NewMockQuicStreamForConn(ctrl)
	mock_qs.EXPECT().Close().Return(nil)
	conn := makeConn(t, TestConn{qs: mock_qs})
	err := conn.Close() // This calls the doneOnce and closes the doneChan
	// would be nice to test that the doneChan is closed
	if err != nil {
		t.Fatalf("conn.Close returned error %v", err)
	}
}

// These tests operate on the quic Connection

func TestCloseConnection(t *testing.T) {
	ctrl := gomock.NewController(t)
	// quic Connection should be closed
	mock_qc := mock_netceptor.NewMockQuicConnectionForConn(ctrl)
	// TODO: mock that it's closed with 0 and not Any()
	mock_qc.EXPECT().CloseWithError(gomock.Any(), gomock.Eq("normal close")).Return(nil).Times(1)

	// PacketConner should be cancelled
	mock_pc := mock_netceptor.NewMockPacketConner(ctrl)
	mock_pc.EXPECT().Cancel().Times(1)

	// The CloseConnection method logs some information to the netceptor's Logger, so mock them
	mock_pc.EXPECT().LocalService().Return("test-local-service").Times(1)
	mock_qc.EXPECT().RemoteAddr().Return(netceptor.Addr{})

	conn := makeConn(t, TestConn{pc: mock_pc, qc: mock_qc})
	err := conn.CloseConnection()
	if err != nil {
		t.Fatalf("conn.CloseConnection returned error %v", err)
	}
}

func TestLocalAddr(t *testing.T) {
	want := netceptor.Addr{} // Could mock the net interace here rather than an empty Addr{}
	ctrl := gomock.NewController(t)
	mock_qc := mock_netceptor.NewMockQuicConnectionForConn(ctrl)
	mock_qc.EXPECT().LocalAddr().Return(want).Times(1)
	conn := makeConn(t, TestConn{qc: mock_qc})
	got := conn.LocalAddr()
	if got != want {
		t.Errorf("Wanted %v, got %v", want, got)
	}
}

func TestRemoteAddr(t *testing.T) {
	want := netceptor.Addr{} // Could mock the net interace here rather than an empty Addr{}
	ctrl := gomock.NewController(t)
	mock_qc := mock_netceptor.NewMockQuicConnectionForConn(ctrl)
	mock_qc.EXPECT().RemoteAddr().Return(want).Times(1)
	conn := makeConn(t, TestConn{qc: mock_qc})
	got := conn.RemoteAddr()
	if got != want {
		t.Errorf("Wanted %v, got %v", want, got)
	}
}

func TestSetDeadline(t *testing.T) {
	want := time.Now().Add(10 * time.Second)
	ctrl := gomock.NewController(t)
	mock_qs := mock_netceptor.NewMockQuicStreamForConn(ctrl)
	mock_qs.EXPECT().SetDeadline(gomock.Eq(want)).Return(nil)
	conn := makeConn(t, TestConn{qs: mock_qs})
	err := conn.SetDeadline(want)
	if err != nil {
		t.Fatalf("conn.TestSetDeadline returned error %v", err)
	}
}

func TestSetReadDeadline(t *testing.T) {
	want := time.Now().Add(10 * time.Second)
	ctrl := gomock.NewController(t)
	mock_qs := mock_netceptor.NewMockQuicStreamForConn(ctrl)
	mock_qs.EXPECT().SetReadDeadline(gomock.Eq(want)).Return(nil)
	conn := makeConn(t, TestConn{qs: mock_qs})
	err := conn.SetReadDeadline(want)
	if err != nil {
		t.Fatalf("conn.SetReadDeadline returned error %v", err)
	}
}

func TestSetWriteDeadline(t *testing.T) {
	want := time.Now().Add(10 * time.Second)
	ctrl := gomock.NewController(t)
	mock_qs := mock_netceptor.NewMockQuicStreamForConn(ctrl)
	mock_qs.EXPECT().SetWriteDeadline(gomock.Eq(want)).Return(nil)
	conn := makeConn(t, TestConn{qs: mock_qs})
	err := conn.SetWriteDeadline(want)
	if err != nil {
		t.Fatalf("conn.SetWriteDeadline returned error %v", err)
	}
}
