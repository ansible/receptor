package utils

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// LogCapture is an io.Writer that captures whole log lines written to it,
// so tests can assert on the output of a ReceptorLogger.
type LogCapture struct {
	messages []string
	mutex    sync.Mutex
}

var _ io.Writer = (*LogCapture)(nil)

// NewLogCapture creates a LogCapture optionally seeded with existing lines.
func NewLogCapture(initial ...string) *LogCapture {
	return &LogCapture{messages: append([]string{}, initial...)}
}

func (lc *LogCapture) Write(p []byte) (n int, err error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()
	// Loggers write whole lines; strip the trailing newline so String()
	// joins captured lines without introducing blank lines between them.
	lc.messages = append(lc.messages, strings.TrimSuffix(string(p), "\n"))

	return len(p), nil
}

// Messages returns a copy of the captured log lines.
func (lc *LogCapture) Messages() []string {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	return append([]string{}, lc.messages...)
}

// String returns the captured log lines joined by newlines.
func (lc *LogCapture) String() string {
	return strings.Join(lc.Messages(), "\n")
}

// Contains reports whether a single captured line contains all the given
// substrings.
func (lc *LogCapture) Contains(substrings ...string) bool {
	for _, msg := range lc.Messages() {
		allFound := true
		for _, substr := range substrings {
			if !strings.Contains(msg, substr) {
				allFound = false

				break
			}
		}
		if allFound {
			return true
		}
	}

	return false
}

// LinesContaining returns a copy of the captured lines containing substr.
func (lc *LogCapture) LinesContaining(substr string) []string {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lines := make([]string, 0, len(lc.messages))
	for _, msg := range lc.messages {
		if strings.Contains(msg, substr) {
			lines = append(lines, msg)
		}
	}

	return lines
}

// WaitFor polls until a single captured line contains all the given
// substrings, failing the test on timeout.
func (lc *LogCapture) WaitFor(t *testing.T, timeout time.Duration, substrings ...string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if !CheckUntilTimeout(ctx, 20*time.Millisecond, func() bool {
		return lc.Contains(substrings...)
	}) {
		t.Fatalf("timed out waiting for log message with %v; logs:\n%s", substrings, lc.String())
	}
}
