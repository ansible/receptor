package logger

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWatchCustomConfig(t *testing.T) {
	// Create a temporary directory and file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "testconfig.yaml")

	initialContent := []byte("initial: config\n")
	if err := os.WriteFile(tmpFile, initialContent, 0o644); err != nil {
		t.Fatalf("Failed to write initial config file: %v", err)
	}

	done := make(chan struct{})

	var allContents []string
	var mu sync.Mutex

	parser := func(file string, rl *ReceptorLogger) error {
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		mu.Lock()
		allContents = append(allContents, string(data))
		mu.Unlock()

		if strings.Contains(string(data), "modified:") {
			close(done)
		}
		return nil
	}

	rl := NewReceptorLogger("")

	err := WatchCustomConfig(tmpFile, parser, rl)
	if err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	// Wait a moment for the initial parse
	time.Sleep(100 * time.Millisecond)

	// Modify the file to trigger the watcher
	newContent := []byte("modified: config\n")
	if err := os.WriteFile(tmpFile, newContent, 0o644); err != nil {
		t.Fatalf("Failed to write updated config file: %v", err)
	}

	time.Sleep(100 * time.Millisecond) // give the parser time to run

	newContent = []byte("additional: config\n")
	// Modify the file to trigger the watcher
	if err := os.WriteFile(tmpFile, newContent, 0o644); err != nil {
		t.Fatalf("Failed to write updated config file: %v", err)
	}

	// Wait for the second parse to happen (with a timeout)
	select {
	case <-done:
		found := false
		for _, c := range allContents {
			if strings.Contains(c, "modified: config") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Updated content not detected in any parse call")
		}

		found = false
		for _, c := range allContents {
			if strings.Contains(c, "additional: config") {
				found = true
				break
			}
		}

	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for parseFunc to be called after file change")
	}
}
