package utils_test

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/utils"
)

func TestReadStringContextSuccessfulRead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		delimiter byte
		want      string
	}{
		{
			name:      "Read line with newline delimiter",
			input:     "hello world\n",
			delimiter: '\n',
			want:      "hello world\n",
		},
		{
			name:      "Read until custom delimiter",
			input:     "foo:bar",
			delimiter: ':',
			want:      "foo:",
		},
		{
			name:      "Read multiple lines",
			input:     "line1\nline2\n",
			delimiter: '\n',
			want:      "line1\n",
		},
		{
			name:      "Read with space delimiter",
			input:     "word1 word2",
			delimiter: ' ',
			want:      "word1 ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reader := bufio.NewReader(strings.NewReader(tt.input))
			ctx := context.Background()

			got, err := utils.ReadStringContext(ctx, reader, tt.delimiter)
			if err != nil {
				t.Errorf("ReadStringContext() unexpected error = %v", err)

				return
			}
			if got != tt.want {
				t.Errorf("ReadStringContext() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadStringContextCancellation(t *testing.T) {
	t.Parallel()

	// Create a reader that will block (empty reader that never returns)
	readPipe, writePipe := io.Pipe()
	defer writePipe.Close()
	defer readPipe.Close()

	reader := bufio.NewReader(readPipe)
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel the context immediately
	cancel()

	got, err := utils.ReadStringContext(ctx, reader, '\n')
	if err == nil {
		t.Error("ReadStringContext() expected error when context is cancelled, got nil")

		return
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("ReadStringContext() error = %v, want context.Canceled", err)
	}
	if got != "" {
		t.Errorf("ReadStringContext() = %q, want empty string", got)
	}
}

func TestReadStringContextTimeout(t *testing.T) {
	t.Parallel()

	// Create a reader that will block
	readPipe, writePipe := io.Pipe()
	defer writePipe.Close()
	defer readPipe.Close()

	reader := bufio.NewReader(readPipe)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	got, err := utils.ReadStringContext(ctx, reader, '\n')
	if err == nil {
		t.Error("ReadStringContext() expected error when context times out, got nil")

		return
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("ReadStringContext() error = %v, want context.DeadlineExceeded", err)
	}
	if got != "" {
		t.Errorf("ReadStringContext() = %q, want empty string", got)
	}
}

func TestReadStringContextReaderError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		delimiter byte
		wantStr   string
		wantErr   bool
	}{
		{
			name:      "EOF without delimiter",
			input:     "incomplete",
			delimiter: '\n',
			wantStr:   "incomplete",
			wantErr:   true,
		},
		{
			name:      "Empty input",
			input:     "",
			delimiter: '\n',
			wantStr:   "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reader := bufio.NewReader(strings.NewReader(tt.input))
			ctx := context.Background()

			got, err := utils.ReadStringContext(ctx, reader, tt.delimiter)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadStringContext() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if got != tt.wantStr {
				t.Errorf("ReadStringContext() = %q, want %q", got, tt.wantStr)
			}
			if tt.wantErr && !errors.Is(err, io.EOF) {
				t.Errorf("ReadStringContext() error = %v, want io.EOF", err)
			}
		})
	}
}

func TestReadStringContextMultipleDelimiters(t *testing.T) {
	t.Parallel()

	input := "a:b:c:d"
	reader := bufio.NewReader(strings.NewReader(input))
	ctx := context.Background()

	// First read should get "a:"
	got1, err := utils.ReadStringContext(ctx, reader, ':')
	if err != nil {
		t.Errorf("ReadStringContext() first read unexpected error = %v", err)

		return
	}
	if got1 != "a:" {
		t.Errorf("ReadStringContext() first read = %q, want %q", got1, "a:")
	}

	// Second read should get "b:"
	got2, err := utils.ReadStringContext(ctx, reader, ':')
	if err != nil {
		t.Errorf("ReadStringContext() second read unexpected error = %v", err)

		return
	}
	if got2 != "b:" {
		t.Errorf("ReadStringContext() second read = %q, want %q", got2, "b:")
	}
}

func TestReadStringContextConcurrentContextCancel(t *testing.T) {
	t.Parallel()

	// This test verifies that cancelling context during a read works correctly
	readPipe, writePipe := io.Pipe()
	defer readPipe.Close()

	reader := bufio.NewReader(readPipe)
	ctx, cancel := context.WithCancel(context.Background())

	// Start the read in a goroutine
	done := make(chan struct{})
	var gotStr string
	var gotErr error

	go func() {
		gotStr, gotErr = utils.ReadStringContext(ctx, reader, '\n')
		close(done)
	}()

	// Give the goroutine time to start
	time.Sleep(10 * time.Millisecond)

	// Cancel the context while read is in progress
	cancel()

	// Wait for completion
	select {
	case <-done:
		// Success - the function returned
	case <-time.After(1 * time.Second):
		t.Fatal("ReadStringContext() did not return after context cancellation")
	}

	// Close the writer to clean up
	writePipe.Close()

	if gotErr == nil {
		t.Error("ReadStringContext() expected error after context cancellation, got nil")
	}
	if !errors.Is(gotErr, context.Canceled) {
		t.Errorf("ReadStringContext() error = %v, want context.Canceled", gotErr)
	}
	if gotStr != "" {
		t.Errorf("ReadStringContext() = %q, want empty string", gotStr)
	}
}

func TestReadStringContextDataArrivesBeforeContextCancel(t *testing.T) {
	t.Parallel()

	// This test verifies that if data arrives before context is cancelled,
	// we get the data successfully
	input := "quick response\n"
	reader := bufio.NewReader(strings.NewReader(input))
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	got, err := utils.ReadStringContext(ctx, reader, '\n')
	if err != nil {
		t.Errorf("ReadStringContext() unexpected error = %v", err)

		return
	}
	if got != "quick response\n" {
		t.Errorf("ReadStringContext() = %q, want %q", got, "quick response\n")
	}
}
