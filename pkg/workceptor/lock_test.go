// +build !no_workceptor

package workceptor

import (
	"context"
	"fmt"
	"io/ioutil"
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

	tmpdir, err := ioutil.TempDir(os.TempDir(), "receptor-test-*")
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
			sfd := StatusFileData{}
			err = sfd.UpdateFullStatus(statusFilename, func(status *StatusFileData) {
				time.Sleep(waitTime)
				status.State = iter
				status.StdoutSize = int64(iter)
				status.Detail = fmt.Sprintf("%d", iter)
			})
			wg.Done()
		}(i, waitTime)
	}
	ctx, cancel := context.WithCancel(context.Background())
	wg2 := sync.WaitGroup{}
	wg2.Add(numReaderThreads)
	for i := 0; i < numReaderThreads; i++ {
		go func() {
			sfd := StatusFileData{}
			fileHasExisted := false
			for {
				if ctx.Err() != nil {
					wg2.Done()
					return
				}
				err := sfd.Load(statusFilename)
				if os.IsNotExist(err) && !fileHasExisted {
					continue
				}
				fileHasExisted = true
				if err != nil {
					t.Fatal(fmt.Sprintf("Error loading status file: %s", err))
				}
				detailIter, err := strconv.Atoi(sfd.Detail)
				if err != nil {
					t.Fatal(fmt.Sprintf("Error converting status detail to int: %s", err))
				}
				if detailIter >= 0 {
					if int64(sfd.State) != sfd.StdoutSize || sfd.State != detailIter {
						t.Fatal(fmt.Sprintf("Mismatched data in struct"))
					}
				}
			}
		}()
	}
	wg.Wait()
	cancel()
	totalTime := time.Now().Sub(startTime)
	if totalTime < totalWaitTime {
		t.Fatal("File locks apparently not locking")
	}
	wg2.Wait()
}
