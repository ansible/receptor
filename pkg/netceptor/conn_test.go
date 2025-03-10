package netceptor_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/netceptor/mock_netceptor"
	"github.com/quic-go/quic-go"
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

// These tests operate on the quic Stream.
func TestRead(t *testing.T) {
	ctrl := gomock.NewController(t)
	buf := make([]byte, 1)
	// Create a mock QuicStream
	mockQs := mock_netceptor.NewMockQuicStreamForConn(ctrl)
	// both success and error
	t.Run("Returns number of bytes from successful Read", func(t *testing.T) {
		want := 1
		mockQs.EXPECT().Read(gomock.Eq(buf)).Return(want, nil).Times(1)
		conn := makeConn(t, TestConn{qs: mockQs})
		got, err := conn.Read(buf)
		if err != nil {
			t.Fatalf("Read returned unexpected error %v", err)
		}
		if got != want {
			t.Errorf("Wanted %v, got %v", want, got)
		}
	})

	t.Run("Returns error from unsuccessful Read", func(t *testing.T) {
		wantErr := errors.New("Read error")
		mockQs.EXPECT().Read(gomock.Eq(buf)).Return(0, wantErr).Times(1)
		conn := makeConn(t, TestConn{qs: mockQs})
		_, gotErr := conn.Read(buf)
		if gotErr == nil {
			t.Errorf("Read did not return expected error")
		}
		if gotErr != wantErr {
			t.Errorf("Wanted %v, got %v", wantErr, gotErr)
		}
	})
}

func TestCancelRead(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockQs := mock_netceptor.NewMockQuicStreamForConn(ctrl)
	mockQs.EXPECT().CancelRead(gomock.Eq(quic.StreamErrorCode(499))).Times(1)
	conn := makeConn(t, TestConn{qs: mockQs})
	conn.CancelRead()
}

func TestWrite(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockQs := mock_netceptor.NewMockQuicStreamForConn(ctrl)
	bytes := []byte{4, 8, 15, 16, 23, 42}
	t.Run("Returns number of bytes written in successful Write", func(t *testing.T) {
		want := 6
		mockQs.EXPECT().Write(gomock.Eq(bytes)).Return(want, nil).Times(1)
		conn := makeConn(t, TestConn{qs: mockQs})
		got, err := conn.Write(bytes)
		if err != nil {
			t.Fatalf("Write returned unexpected error %v", err)
		}
		if got != want {
			t.Errorf("Wanted %v, got %v", want, got)
		}
	})
	t.Run("Returns error from unsuccessful Write", func(t *testing.T) {
		wantErr := errors.New("Write error")
		mockQs.EXPECT().Write(gomock.Eq(bytes)).Return(0, wantErr).Times(1)
		conn := makeConn(t, TestConn{qs: mockQs})
		_, gotErr := conn.Write(bytes)
		if gotErr == nil {
			t.Errorf("Write did not return expected error")
		}
		if gotErr != wantErr {
			t.Errorf("Wanted %v, got %v", wantErr, gotErr)
		}
	})
}

func TestClose(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockQs := mock_netceptor.NewMockQuicStreamForConn(ctrl)
	mockQs.EXPECT().Close().Return(nil)
	conn := makeConn(t, TestConn{qs: mockQs})
	err := conn.Close() // This calls the doneOnce and closes the doneChan
	// would be nice to test that the doneChan is closed
	if err != nil {
		t.Fatalf("conn.Close returned error %v", err)
	}
}

// These tests operate on the quic Connection.
func TestCloseConnection(t *testing.T) {
	ctrl := gomock.NewController(t)
	// quic Connection should be closed
	mockQc := mock_netceptor.NewMockQuicConnectionForConn(ctrl)
	mockQc.EXPECT().CloseWithError(quic.ApplicationErrorCode(0), gomock.Eq("normal close")).Return(nil).Times(1)

	// PacketConner should be cancelled
	mockPc := mock_netceptor.NewMockPacketConner(ctrl)
	mockPc.EXPECT().Cancel().Times(1)

	// The CloseConnection method logs some information to the netceptor's Logger, so mock them
	mockPc.EXPECT().LocalService().Return("test-local-service").Times(1)
	mockQc.EXPECT().RemoteAddr().Return(netceptor.Addr{})

	conn := makeConn(t, TestConn{pc: mockPc, qc: mockQc})
	err := conn.CloseConnection()
	if err != nil {
		t.Fatalf("conn.CloseConnection returned error %v", err)
	}
}

func TestLocalAddr(t *testing.T) {
	want := netceptor.Addr{} // Could mock the net interacted here rather than an empty Addr{}
	ctrl := gomock.NewController(t)
	mockQc := mock_netceptor.NewMockQuicConnectionForConn(ctrl)
	mockQc.EXPECT().LocalAddr().Return(want).Times(1)
	conn := makeConn(t, TestConn{qc: mockQc})
	got := conn.LocalAddr()
	if got != want {
		t.Errorf("Wanted %v, got %v", want, got)
	}
}

func TestRemoteAddr(t *testing.T) {
	want := netceptor.Addr{} // Could mock the net interacted here rather than an empty Addr{}
	ctrl := gomock.NewController(t)
	mockQc := mock_netceptor.NewMockQuicConnectionForConn(ctrl)
	mockQc.EXPECT().RemoteAddr().Return(want).Times(1)
	conn := makeConn(t, TestConn{qc: mockQc})
	got := conn.RemoteAddr()
	if got != want {
		t.Errorf("Wanted %v, got %v", want, got)
	}
}

func TestSetDeadline(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockQs := mock_netceptor.NewMockQuicStreamForConn(ctrl)
	want := time.Now().Add(10 * time.Second)
	t.Run("Returns no error after successful SetDeadline", func(t *testing.T) {
		mockQs.EXPECT().SetDeadline(gomock.Eq(want)).Return(nil)
		conn := makeConn(t, TestConn{qs: mockQs})
		err := conn.SetDeadline(want)
		if err != nil {
			t.Fatalf("conn.TestSetDeadline returned error %v", err)
		}
	})
	t.Run("Returns error from unsuccessful SetDeadline", func(t *testing.T) {
		wantErr := errors.New("SetDeadline error")
		mockQs.EXPECT().SetDeadline(gomock.Eq(want)).Return(wantErr)
		conn := makeConn(t, TestConn{qs: mockQs})
		gotErr := conn.SetDeadline(want)
		if gotErr != wantErr {
			t.Errorf("Wanted %v, got %v", wantErr, gotErr)
		}
	})
}

func TestSetReadDeadline(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockQs := mock_netceptor.NewMockQuicStreamForConn(ctrl)
	want := time.Now().Add(10 * time.Second)
	t.Run("Returns no error after successful SetReadDeadline", func(t *testing.T) {
		mockQs.EXPECT().SetReadDeadline(gomock.Eq(want)).Return(nil)
		conn := makeConn(t, TestConn{qs: mockQs})
		err := conn.SetReadDeadline(want)
		if err != nil {
			t.Fatalf("conn.SetReadDeadline returned error %v", err)
		}
	})
	t.Run("Returns error from unsuccessful SetReadDeadline", func(t *testing.T) {
		wantErr := errors.New("SetReadDeadline error")
		mockQs.EXPECT().SetReadDeadline(gomock.Eq(want)).Return(wantErr)
		conn := makeConn(t, TestConn{qs: mockQs})
		gotErr := conn.SetReadDeadline(want)
		if gotErr != wantErr {
			t.Errorf("Wanted %v, got %v", wantErr, gotErr)
		}
	})
}

func TestSetWriteDeadline(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockQs := mock_netceptor.NewMockQuicStreamForConn(ctrl)
	want := time.Now().Add(10 * time.Second)
	t.Run("Returns no error after successful SetWriteDeadline", func(t *testing.T) {
		mockQs.EXPECT().SetWriteDeadline(gomock.Eq(want)).Return(nil)
		conn := makeConn(t, TestConn{qs: mockQs})
		err := conn.SetWriteDeadline(want)
		if err != nil {
			t.Fatalf("conn.SetWriteDeadline returned error %v", err)
		}
	})
	t.Run("Returns error from unsuccessful SetWriteDeadline", func(t *testing.T) {
		wantErr := errors.New("SetWriteDeadline error")
		mockQs.EXPECT().SetWriteDeadline(gomock.Eq(want)).Return(wantErr)
		conn := makeConn(t, TestConn{qs: mockQs})
		gotErr := conn.SetWriteDeadline(want)
		if gotErr != wantErr {
			t.Errorf("Wanted %v, got %v", wantErr, gotErr)
		}
	})
}
