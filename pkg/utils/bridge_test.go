package utils_test

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/utils"
	"github.com/ansible/receptor/pkg/utils/mock_utils"
	"go.uber.org/mock/gomock"
)

func TestBridgeConnsSuccessfulBridge(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	data1 := []byte("Hello from connection 1")
	data2 := []byte("Hello from connection 2")

	// Create mocks
	connection1 := mock_utils.NewMockReadWriteCloser(ctrl)
	connection2 := mock_utils.NewMockReadWriteCloser(ctrl)

	// Track what was written to each connection
	var connection1Written bytes.Buffer
	var connection2Written bytes.Buffer
	var mu sync.Mutex

	// Setup connection1 to read data1 and write to connection1Written buffer
	reader1 := bytes.NewReader(data1)
	connection1.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
		return reader1.Read(p)
	}).Times(2) // 1 read with data + 1 EOF
	connection1.EXPECT().Write(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()

		return connection1Written.Write(p)
	}).Times(1) // Small data fits in one write
	connection1.EXPECT().Close().Return(nil).Times(1) // Closed by goroutine 2

	// Setup connection2 to read data2 and write to connection2Written buffer
	reader2 := bytes.NewReader(data2)
	connection2.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
		return reader2.Read(p)
	}).Times(2) // 1 read with data + 1 EOF
	connection2.EXPECT().Write(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()

		return connection2Written.Write(p)
	}).Times(1) // Small data fits in one write
	connection2.EXPECT().Close().Return(nil).Times(1) // Closed by goroutine 1

	log := logger.NewReceptorLogger("")

	// Run BridgeConns in a goroutine with timeout
	done := make(chan bool)
	go func() {
		utils.BridgeConns(connection1, "connection1", connection2, "connection2", log)
		done <- true
	}()

	// Wait for bridge to complete or timeout
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("BridgeConns did not complete within timeout")
	}

	// Verify data was bridged correctly (no mutex needed, goroutines have completed)
	connection1WrittenData := connection1Written.Bytes()
	connection2WrittenData := connection2Written.Bytes()

	if !bytes.Equal(connection2WrittenData, data1) {
		t.Errorf("connection2 received %q, want %q", connection2WrittenData, data1)
	}

	if !bytes.Equal(connection1WrittenData, data2) {
		t.Errorf("connection1 received %q, want %q", connection1WrittenData, data2)
	}
}

func TestBridgeConnsEmptyData(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	connection1 := mock_utils.NewMockReadWriteCloser(ctrl)
	connection2 := mock_utils.NewMockReadWriteCloser(ctrl)

	// Both connections return EOF immediately
	connection1.EXPECT().Read(gomock.Any()).Return(0, io.EOF).Times(1)
	connection2.EXPECT().Read(gomock.Any()).Return(0, io.EOF).Times(1)
	connection1.EXPECT().Close().Return(nil).Times(1) // Closed by goroutine 2
	connection2.EXPECT().Close().Return(nil).Times(1) // Closed by goroutine 1

	log := logger.NewReceptorLogger("")

	done := make(chan bool)
	go func() {
		utils.BridgeConns(connection1, "connection1", connection2, "connection2", log)
		done <- true
	}()

	select {
	case <-done:
		// Success - both should finish quickly with EOF
	case <-time.After(2 * time.Second):
		t.Fatal("BridgeConns did not complete within timeout")
	}
}

func TestBridgeConnsReadError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	connection1 := mock_utils.NewMockReadWriteCloser(ctrl)
	connection2 := mock_utils.NewMockReadWriteCloser(ctrl)

	// c1 has some data
	data1 := []byte("some data")
	reader1 := bytes.NewReader(data1)
	connection1.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
		return reader1.Read(p)
	}).Times(2) // 1 read with data + 1 EOF
	connection1.EXPECT().Close().Return(nil).Times(1) // Closed by goroutine 2

	// c2 returns an error on read
	connection2.EXPECT().Read(gomock.Any()).Return(0, errors.New("read error")).Times(1)
	connection2.EXPECT().Write(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
		return len(p), nil
	}).Times(1) // Writes data1 once
	connection2.EXPECT().Close().Return(nil).Times(1) // Closed by goroutine 1

	// Capture log output
	var logBuf bytes.Buffer
	log := logger.NewReceptorLogger("")
	log.SetOutput(&logBuf)

	done := make(chan bool)
	go func() {
		utils.BridgeConns(connection1, "connection1", connection2, "connection2", log)
		done <- true
	}()

	select {
	case <-done:
		// Success - should handle error gracefully
	case <-time.After(2 * time.Second):
		t.Fatal("BridgeConns did not complete within timeout")
	}

	// Verify error was logged
	logOutput := logBuf.String()
	if !bytes.Contains([]byte(logOutput), []byte("Connection read error: read error")) {
		t.Errorf("Expected log to contain 'Connection read error: read error', got: %s", logOutput)
	}
}

func TestBridgeConnsWriteError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	connection1 := mock_utils.NewMockReadWriteCloser(ctrl)
	connection2 := mock_utils.NewMockReadWriteCloser(ctrl)

	// c1 has test data
	data1 := []byte("test data")
	reader1 := bytes.NewReader(data1)
	connection1.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
		return reader1.Read(p)
	}).Times(1) // Read once, then write fails and goroutine exits
	connection1.EXPECT().Close().Return(nil).Times(1) // Closed by goroutine 2

	// c2 returns error on write
	connection2.EXPECT().Read(gomock.Any()).Return(0, io.EOF).Times(1)
	connection2.EXPECT().Write(gomock.Any()).Return(0, errors.New("write error")).Times(1)
	connection2.EXPECT().Close().Return(nil).Times(1) // Closed by goroutine 1

	// Capture log output
	var logBuf bytes.Buffer
	log := logger.NewReceptorLogger("")
	log.SetOutput(&logBuf)

	done := make(chan bool)
	go func() {
		utils.BridgeConns(connection1, "connection1", connection2, "connection2", log)
		done <- true
	}()

	select {
	case <-done:
		// Success - should handle error gracefully
	case <-time.After(2 * time.Second):
		t.Fatal("BridgeConns did not complete within timeout")
	}

	// Verify error was logged
	logOutput := logBuf.String()
	if !bytes.Contains([]byte(logOutput), []byte("Connection write error: write error")) {
		t.Errorf("Expected log to contain 'Connection write error: write error', got: %s", logOutput)
	}
}

func TestBridgeConnsCloseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	connection1 := mock_utils.NewMockReadWriteCloser(ctrl)
	connection2 := mock_utils.NewMockReadWriteCloser(ctrl)

	// c1 has test data
	data1 := []byte("test data")
	reader1 := bytes.NewReader(data1)
	connection1.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
		return reader1.Read(p)
	}).Times(2) // 1 read with data + 1 EOF
	connection1.EXPECT().Close().Return(errors.New("close error")).Times(1) // Close fails

	// c2 returns EOF immediately
	connection2.EXPECT().Read(gomock.Any()).Return(0, io.EOF).Times(1)
	connection2.EXPECT().Write(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
		return len(p), nil
	}).Times(1) // Writes data1 once
	connection2.EXPECT().Close().Return(errors.New("close error on connection2")).Times(1) // Close also fails

	// Capture log output
	var logBuf bytes.Buffer
	log := logger.NewReceptorLogger("")
	log.SetOutput(&logBuf)

	done := make(chan bool)
	go func() {
		utils.BridgeConns(connection1, "connection1", connection2, "connection2", log)
		done <- true
	}()

	select {
	case <-done:
		// Success - should handle close errors gracefully
	case <-time.After(2 * time.Second):
		t.Fatal("BridgeConns did not complete within timeout")
	}

	// Verify both close errors were logged
	logOutput := logBuf.String()
	if !bytes.Contains([]byte(logOutput), []byte("Error closing connection1: close error")) {
		t.Errorf("Expected log to contain 'Error closing connection1: close error', got: %s", logOutput)
	}
	if !bytes.Contains([]byte(logOutput), []byte("Error closing connection2: close error on connection2")) {
		t.Errorf("Expected log to contain 'Error closing connection2: close error on connection2', got: %s", logOutput)
	}
}

func TestBridgeConnsPartialWrite(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	connection1 := mock_utils.NewMockReadWriteCloser(ctrl)
	connection2 := mock_utils.NewMockReadWriteCloser(ctrl)

	// c1 has test data
	data1 := []byte("test data for partial write")
	reader1 := bytes.NewReader(data1)
	connection1.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
		return reader1.Read(p)
	}).Times(1) // Read once, then partial write error causes exit
	connection1.EXPECT().Close().Return(nil).Times(1) // Closed by goroutine 2

	// c2 does partial writes (only writes half the data)
	connection2.EXPECT().Read(gomock.Any()).Return(0, io.EOF).Times(1)
	connection2.EXPECT().Write(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
		if len(p) > 1 {
			return len(p) / 2, nil // Partial write
		}

		return len(p), nil
	}).Times(1) // Partial write detected, goroutine exits
	connection2.EXPECT().Close().Return(nil).Times(1) // Closed by goroutine 1

	// Capture log output
	var logBuf bytes.Buffer
	log := logger.NewReceptorLogger("")
	log.SetOutput(&logBuf)

	done := make(chan bool)
	go func() {
		utils.BridgeConns(connection1, "connection1", connection2, "connection2", log)
		done <- true
	}()

	select {
	case <-done:
		// Success - should detect partial write and close
	case <-time.After(2 * time.Second):
		t.Fatal("BridgeConns did not complete within timeout")
	}

	// Verify partial write error was logged
	logOutput := logBuf.String()
	if !bytes.Contains([]byte(logOutput), []byte("Not all bytes written")) {
		t.Errorf("Expected log to contain 'Not all bytes written', got: %s", logOutput)
	}
}

func TestBridgeConnsLargeData(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create data larger than NormalBufferSize to test multiple reads/writes
	largeData := make([]byte, utils.NormalBufferSize*2+1000)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	connection1 := mock_utils.NewMockReadWriteCloser(ctrl)
	connection2 := mock_utils.NewMockReadWriteCloser(ctrl)

	var connection2Written bytes.Buffer
	var mu sync.Mutex

	// Setup connection1 to read large data (no writes expected since connection2 returns EOF immediately)
	reader1 := bytes.NewReader(largeData)
	connection1.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
		return reader1.Read(p)
	}).Times(4) // 132072 bytes = 2 full buffers (65536*2) + 1000 bytes + EOF = 4 reads
	connection1.EXPECT().Close().Return(nil).Times(1) // Closed by goroutine 2

	// Setup connection2 to write to buffer
	connection2.EXPECT().Read(gomock.Any()).Return(0, io.EOF).Times(1)
	connection2.EXPECT().Write(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()

		return connection2Written.Write(p)
	}).Times(3) // 3 writes for the 3 data chunks (65536 + 65536 + 1000 bytes)
	connection2.EXPECT().Close().Return(nil).Times(1) // Closed by goroutine 1

	log := logger.NewReceptorLogger("")

	done := make(chan bool)
	go func() {
		utils.BridgeConns(connection1, "connection1", connection2, "connection2", log)
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("BridgeConns did not complete within timeout")
	}

	// Verify all data was transferred (no mutex needed, goroutines have completed)
	connection2WrittenData := connection2Written.Bytes()

	if len(connection2WrittenData) != len(largeData) {
		t.Errorf("connection2 received %d bytes, want %d bytes", len(connection2WrittenData), len(largeData))
	}

	if !bytes.Equal(connection2WrittenData, largeData) {
		t.Error("connection2 received corrupted data")
	}
}

func TestBridgeConnsUnidirectionalData(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Only c1 has data, c2 is empty
	data := []byte("one way data transfer")

	connection1 := mock_utils.NewMockReadWriteCloser(ctrl)
	connection2 := mock_utils.NewMockReadWriteCloser(ctrl)

	var connection2Written bytes.Buffer
	var mu sync.Mutex

	// connection1 reads data (no writes expected since connection2 returns EOF immediately)
	reader1 := bytes.NewReader(data)
	connection1.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
		return reader1.Read(p)
	}).Times(2) // 1 read with data + 1 EOF
	connection1.EXPECT().Close().Return(nil).Times(1) // Closed by goroutine 2

	// connection2 has no data (EOF) but can write
	connection2.EXPECT().Read(gomock.Any()).Return(0, io.EOF).Times(1)
	connection2.EXPECT().Write(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()

		return connection2Written.Write(p)
	}).Times(1) // Small data fits in one write
	connection2.EXPECT().Close().Return(nil).Times(1) // Closed by goroutine 1

	log := logger.NewReceptorLogger("")

	done := make(chan bool)
	go func() {
		utils.BridgeConns(connection1, "connection1", connection2, "connection2", log)
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("BridgeConns did not complete within timeout")
	}

	// Verify data went from connection1 to connection2 (no mutex needed, goroutines have completed)
	connection2WrittenData := connection2Written.Bytes()

	if !bytes.Equal(connection2WrittenData, data) {
		t.Errorf("connection2 received %q, want %q", connection2WrittenData, data)
	}
}

func TestBridgeConnsConcurrentData(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Test with data flowing in both directions simultaneously
	data1 := []byte("concurrent data from c1")
	data2 := []byte("concurrent data from c2")

	connection1 := mock_utils.NewMockReadWriteCloser(ctrl)
	connection2 := mock_utils.NewMockReadWriteCloser(ctrl)

	var connection1Written bytes.Buffer
	var connection2Written bytes.Buffer
	var mu sync.Mutex

	// Setup connection1
	reader1 := bytes.NewReader(data1)
	connection1.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
		return reader1.Read(p)
	}).Times(2) // 1 read with data + 1 EOF
	connection1.EXPECT().Write(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()

		return connection1Written.Write(p)
	}).Times(1) // Small data fits in one write
	connection1.EXPECT().Close().Return(nil).Times(1) // Closed by goroutine 2

	// Setup connection2
	reader2 := bytes.NewReader(data2)
	connection2.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
		return reader2.Read(p)
	}).Times(2) // 1 read with data + 1 EOF
	connection2.EXPECT().Write(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()

		return connection2Written.Write(p)
	}).Times(1) // Small data fits in one write
	connection2.EXPECT().Close().Return(nil).Times(1) // Closed by goroutine 1

	log := logger.NewReceptorLogger("")

	done := make(chan bool)
	go func() {
		utils.BridgeConns(connection1, "connection1", connection2, "connection2", log)
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("BridgeConns did not complete within timeout")
	}

	// Both sides should receive data (no mutex needed, goroutines have completed)
	connection1WrittenData := connection1Written.Bytes()
	connection2WrittenData := connection2Written.Bytes()

	if !bytes.Equal(connection2WrittenData, data1) {
		t.Errorf("connection2 received %q, want %q", connection2WrittenData, data1)
	}

	if !bytes.Equal(connection1WrittenData, data2) {
		t.Errorf("connection1 received %q, want %q", connection1WrittenData, data2)
	}
}

func TestNormalBufferSizeConstant(t *testing.T) {
	t.Parallel()
	// Verify the buffer size constant has the expected value
	expectedSize := 65536
	if utils.NormalBufferSize != expectedSize {
		t.Errorf("NormalBufferSize = %d, want %d", utils.NormalBufferSize, expectedSize)
	}
}
