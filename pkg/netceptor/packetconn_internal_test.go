package netceptor

import "testing"

func TestPacketConnIsDone(t *testing.T) {
	pc := &PacketConn{}
	if pc.isDone() {
		t.Fatal("expected isDone to be false before the connection is cancelled")
	}

	done := make(chan struct{})
	close(done)
	pc.done = done

	if !pc.isDone() {
		t.Fatal("expected isDone to be true after the connection is cancelled")
	}
}
