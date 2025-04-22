package logger

import (
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// WatchCustomConfig sets up a file watcher on the given file path.
// It invokes parseFunc whenever a relevant change is detected.
// While this is a general implementation,
// It's in the logger package because the only known implementation is for changing
// v1 config changes. Based loosely on viper.WatchConfig

func WatchCustomConfig(path string, parseFunc func(string) error) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	dir := filepath.Dir(absPath)
	filename := filepath.Base(absPath)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	// watcher only watches directory.
	err = watcher.Add(dir)
	if err != nil {
		return err
	}

	var mu sync.Mutex
	var lastEventTime time.Time

	go func() {
		defer watcher.Close()

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				// filter on filename
				if filepath.Base(event.Name) != filename {
					continue
				}

				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) != 0 {
					mu.Lock()
					now := time.Now()
					// Debounce: Ignore events that occur too quickly
					if now.Sub(lastEventTime) < 200*time.Millisecond {
						mu.Unlock()

						continue
					}
					lastEventTime = now
					mu.Unlock()

					log.Printf("Config change detected: %s", event.Name)

					// Run your custom parser
					if err := parseFunc(absPath); err != nil {
						log.Printf("Error parsing config: %v", err)
					}
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("Watcher error: %v", err)
			}
		}
	}()

	// Run once initially
	if err := parseFunc(absPath); err != nil {
		log.Printf("Initial config parse failed: %v", err)
	}

	return nil
}
