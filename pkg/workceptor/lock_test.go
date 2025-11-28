//go:build !no_workceptor
// +build !no_workceptor

package workceptor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestStatusFileLock(t *testing.T) {
	numWriterThreads := 8
	numReaderThreads := 8
	baseWaitTime := 200 * time.Millisecond

	tmpdir, err := os.MkdirTemp(os.TempDir(), "receptor-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	statusFilename := path.Join(tmpdir, "status")
	startTime := time.Now()
	var totalWaitTime time.Duration
	wg := sync.WaitGroup{}
	wg.Add(numWriterThreads)
	for i := 0; i < numWriterThreads; i++ {
		waitTime := time.Duration(i) * baseWaitTime
		totalWaitTime += waitTime
		go func(iter int, waitTime time.Duration) {
			defer wg.Done()
			sfd := StatusFileData{}
			sfd.UpdateFullStatus(statusFilename, func(status *StatusFileData) {
				time.Sleep(waitTime)
				status.State = iter
				status.StdoutSize = int64(iter)
				status.Detail = fmt.Sprintf("%d", iter)
			})
		}(i, waitTime)
	}
	ctx, cancel := context.WithCancel(context.Background())
	wg2 := sync.WaitGroup{}
	wg2.Add(numReaderThreads)
	errChan := make(chan error, numReaderThreads)
	for i := 0; i < numReaderThreads; i++ {
		go func() {
			defer wg2.Done()
			sfd := StatusFileData{}
			fileHasExisted := false
			for {
				if ctx.Err() != nil {
					return
				}
				err := sfd.Load(statusFilename)
				if os.IsNotExist(err) && !fileHasExisted {
					continue
				}
				fileHasExisted = true
				if err != nil {
					errChan <- fmt.Errorf("Error loading status file: %w", err)
					cancel()

					return
				}
				detailIter, err := strconv.Atoi(sfd.Detail)
				if err != nil {
					errChan <- fmt.Errorf("Error converting status detail to int: %w", err)
					cancel()

					return
				}
				if detailIter >= 0 {
					if int64(sfd.State) != sfd.StdoutSize || sfd.State != detailIter {
						errChan <- errors.New("Mismatched data in struct")
						cancel()

						return
					}
				}
			}
		}()
	}
	wg.Wait()
	cancel()
	totalTime := time.Since(startTime)
	if totalTime < totalWaitTime {
		t.Fatal("File locks apparently not locking")
	}
	wg2.Wait()
	close(errChan)
	// Collect errors
	errs := make([]error, 0, numReaderThreads)
	for err := range errChan {
		errs = append(errs, err)
	}
	if err := errors.Join(errs...); err != nil {
		t.Fatal(err)
	}
}
