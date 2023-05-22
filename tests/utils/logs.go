package utils

import (
	"bytes"
	"sync"
)

// testLogWriter provides a threadsafe way of reading and writing logs to a buffer.
type TestLogWriter struct {
	Lock   *sync.RWMutex
	Buffer *bytes.Buffer
}

func (lw *TestLogWriter) Write(p []byte) (n int, err error) {
	lw.Lock.Lock()
	defer lw.Lock.Unlock()

	n, err = lw.Buffer.Write(p)

	if err != nil {
		return 0, err
	}

	return n, nil
}

func (lw *TestLogWriter) String() string {
	lw.Lock.RLock()
	defer lw.Lock.RUnlock()

	return lw.Buffer.String()
}

func NewTestLogWriter() *TestLogWriter {
	return &TestLogWriter{
		Lock:   &sync.RWMutex{},
		Buffer: &bytes.Buffer{},
	}
}
