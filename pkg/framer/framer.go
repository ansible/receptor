package framer

import (
	"encoding/binary"
	"fmt"
	"sync"
)

// Framer provides framing of discrete data entities over a stream connection.
type Framer interface {
	SendData(data []byte) []byte
	RecvData(buf []byte)
	MessageReady() bool
	GetMessage() ([]byte, error)
}

type framer struct {
	bufLock *sync.RWMutex
	buffer  []byte
}

// New returns a new framer instance.
func New() Framer {
	f := &framer{
		bufLock: &sync.RWMutex{},
		buffer:  make([]byte, 0),
	}

	return f
}

// SendData takes a data buffer and returns a framed buffer.
func (f *framer) SendData(data []byte) []byte {
	buf := make([]byte, len(data)+2)
	binary.LittleEndian.PutUint16(buf[0:2], uint16(len(data))) //nolint:gosec
	copy(buf[2:], data)

	return buf
}

// RecvData adds more data to the buffer from the network.
func (f *framer) RecvData(buf []byte) {
	f.bufLock.Lock()
	defer f.bufLock.Unlock()
	f.buffer = append(f.buffer, buf...)
}

// Caller must already hold at least a read lock on f.bufLock.
func (f *framer) messageReady() (int, bool) {
	if len(f.buffer) < 2 {
		return 0, false
	}
	msgSize := int(binary.LittleEndian.Uint16(f.buffer[:2]))

	return msgSize, len(f.buffer) >= msgSize+2
}

// MessageReady returns true if a full framed message is available to read.
func (f *framer) MessageReady() bool {
	f.bufLock.RLock()
	defer f.bufLock.RUnlock()
	_, ready := f.messageReady()

	return ready
}

// GetMessage returns a single framed message, or an error if one is not available.
func (f *framer) GetMessage() ([]byte, error) {
	f.bufLock.Lock()
	defer f.bufLock.Unlock()
	msgSize, ready := f.messageReady()
	if !ready {
		return nil, fmt.Errorf("message not ready")
	}
	data := f.buffer[2 : msgSize+2]
	f.buffer = f.buffer[msgSize+2:]

	return data, nil
}
