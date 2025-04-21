package utils

import (
	"fmt"
	"testing"
)

func TestUnixSocketListenWindows(t *testing.T) {
	// var wantListener net.Listener = nil
	// var wantFLock *FLock = nil
	var wantErr error = fmt.Errorf("unix sockets not available on Windows")

	gotListener, gotFLock, gotErr := unixSocketListen("", 0x0000, "windows")

	if gotListener != nil {
		t.Errorf("UnixSocketListen(): gotListener=%+v wantListener=%+v", gotListener, nil)
	}

	if gotFLock != nil {
		t.Errorf("UnixSocketListen(): gotFLock=%+v wantFLock=%+v", gotFLock, nil)
	}

	if gotErr.Error() != wantErr.Error() {
		t.Errorf("UnixSocketListen(): gotErr=%+v wantErr=%+v", gotErr, wantErr)
	}
}
