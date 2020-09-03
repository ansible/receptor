package utils

import (
	"bufio"
	"context"
	"fmt"
)

type readStringResult = struct {
	str string
	err error
}

// ReadStringContext calls bufio.Reader.ReadString() but uses a context.  Note that if the
// ctx.Done() fires, the ReadString() call is still active, and bufio is not re-entrant, so it is
// important for callers to error out of further use of the bufio.  Also, the goroutine will not
// exit until the bufio's underlying connection is closed.
func ReadStringContext(ctx context.Context, reader *bufio.Reader, delim byte) (string, error) {
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
	case <-ctx.Done():
		return "", fmt.Errorf("ctx timeout")
	}
}
