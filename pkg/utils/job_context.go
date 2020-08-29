package utils

import (
	"context"
	"sync"
	"time"
)

// JobContext is a synchronization object that combines the functions of a Context and a WaitGroup.
// The expected lifecycle is:
//     - Caller calls JobContext.NewJob() with a parent context and a count of workers expected.
//     - Caller launches the given number of workers, passing the JobContext to them.
//     - Workers can check for termination by using the JobContext as a context.Context.
//     - Workers can cancel the overall job by calling JobContext.Cancel().
//     - Workers must call JobContext.WorkerDone() when they complete, like sync.WaitGroup.Done().
//     - The caller, or other goroutines. can call JobContext.Wait() to wait for job completion.
// A single JobContext can only run one job at a time.  If JobContext.NewJob() is called while a job
// is already running, that job will be cancelled and waited on prior to starting the new job.
type JobContext struct {
	ctx         context.Context
	cancel      context.CancelFunc
	wg          *sync.WaitGroup
	running     bool
	runningLock *sync.Mutex
}

// NewJob starts a new job with a defined number of workers.  If a prior job is running, it is cancelled.
func (mw *JobContext) NewJob(ctx context.Context, workers int, returnIfRunning bool) bool {
	if mw.runningLock == nil {
		mw.runningLock = &sync.Mutex{}
	}

	mw.runningLock.Lock()
	for mw.running {
		if returnIfRunning {
			mw.runningLock.Unlock()
			return false
		}
		mw.cancel()
		mw.runningLock.Unlock()
		mw.Wait()
		mw.runningLock.Lock()
	}

	mw.running = true
	mw.ctx, mw.cancel = context.WithCancel(ctx)
	mw.wg = &sync.WaitGroup{}
	mw.wg.Add(workers)
	mw.runningLock.Unlock()
	go func() {
		mw.wg.Wait()
		mw.runningLock.Lock()
		mw.running = false
		mw.cancel()
		mw.runningLock.Unlock()
	}()
	return true
}

// WorkerDone signals that a worker is finished, like sync.WaitGroup.Done().
func (mw *JobContext) WorkerDone() {
	mw.wg.Done()
}

// Wait waits for the current job to complete, like sync.WaitGroup.Wait().
func (mw *JobContext) Wait() {
	mw.wg.Wait()
}

// Done implements Context.Done()
func (mw *JobContext) Done() <-chan struct{} {
	return mw.ctx.Done()
}

// Err implements Context.Err()
func (mw *JobContext) Err() error {
	return mw.ctx.Err()
}

// Deadline implements Context.Deadline()
func (mw *JobContext) Deadline() (time time.Time, ok bool) {
	return mw.ctx.Deadline()
}

// Value implements Context.Value()
func (mw *JobContext) Value(key interface{}) interface{} {
	return mw.ctx.Value(key)
}

// Cancel cancels the JobContext's context
func (mw *JobContext) Cancel() {
	mw.cancel()
}

// Running returns true if a job is currently running
func (mw *JobContext) Running() bool {
	mw.runningLock.Lock()
	defer mw.runningLock.Unlock()
	return mw.running
}
