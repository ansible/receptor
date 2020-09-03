package utils

import (
	"bufio"
	"fmt"
	"time"
)

type readStringResult = struct {
	str string
	err error
}

// ReadStringWithTimeout calls bufio.Reader.ReadString() but enforces a timeout.  Note that if the
// timeout fires, the ReadString() call is still active, and bufio is not re-entrant, so it is
// important for callers to error out of further use of the bufio.  Also, the goroutine will not
// exit until the bufio's underlying connection is closed.
func ReadStringWithTimeout(reader *bufio.Reader, delim byte, timeout time.Duration) (string, error) {
	result := make(chan *readStringResult)
	go func() {
		str, err := reader.ReadString(delim)
		result <- &readStringResult{
			str: str,
			err: err,
		}
	}()
	select {
	case res := <-result:
		return res.str, res.err
	case <-time.After(timeout):
		return "", fmt.Errorf("timeout")
	}
}
