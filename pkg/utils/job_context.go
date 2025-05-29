package utils

import (
	"context"
	"sync"
	"time"
)

// JobContext is a synchronization object that combines the functions of a Context and a WaitGroup.
// The expected lifecycle is:
//   - Caller calls JobContext.NewJob() with a parent context and a count of workers expected.
//   - Caller launches the given number of workers, passing the JobContext to them.
//   - Workers can check for termination by using the JobContext as a context.Context.
//   - Workers can cancel the overall job by calling JobContext.Cancel().
//   - Workers must call JobContext.WorkerDone() when they complete, like sync.WaitGroup.Done().
//   - The caller, or other goroutines. can call JobContext.Wait() to wait for job completion.
//
// A single JobContext can only run one job at a time.  If JobContext.NewJob() is called while a job
// is already running, that job will be cancelled and waited on prior to starting the new job.
type JobContext struct {
	Ctx         context.Context
	JcCancel    context.CancelFunc
	Wg          *sync.WaitGroup
	JcRunning   bool
	RunningLock *sync.Mutex
}

// NewJob starts a new job with a defined number of workers.  If a prior job is running, it is cancelled.
func (mw *JobContext) NewJob(ctx context.Context, workers int, returnIfRunning bool) bool {
	if mw.RunningLock == nil {
		mw.RunningLock = &sync.Mutex{}
	}

	mw.RunningLock.Lock()
	for mw.JcRunning {
		if returnIfRunning {
			mw.RunningLock.Unlock()

			return false
		}
		mw.JcCancel()
		mw.RunningLock.Unlock()
		mw.Wait()
		mw.RunningLock.Lock()
	}

	mw.JcRunning = true
	mw.Ctx, mw.JcCancel = context.WithCancel(ctx)
	mw.Wg = &sync.WaitGroup{}
	mw.Wg.Add(workers)
	mw.RunningLock.Unlock()
	go func() {
		mw.Wg.Wait()
		mw.RunningLock.Lock()
		mw.JcRunning = false
		mw.JcCancel()
		mw.RunningLock.Unlock()
	}()

	return true
}

// WorkerDone signals that a worker is finished, like sync.WaitGroup.Done().
func (mw *JobContext) WorkerDone() {
	mw.Wg.Done()
}

// Wait waits for the current job to complete, like sync.WaitGroup.Wait().
// If no job has been started, always just returns.
func (mw *JobContext) Wait() {
	if mw.Wg != nil {
		mw.Wg.Wait()
	}
}

// Done implements Context.Done().
func (mw *JobContext) Done() <-chan struct{} {
	return mw.Ctx.Done()
}

// Err implements Context.Err().
func (mw *JobContext) Err() error {
	return mw.Ctx.Err()
}

// Deadline implements Context.Deadline().
func (mw *JobContext) Deadline() (time time.Time, ok bool) {
	return mw.Ctx.Deadline()
}

// Value implements Context.Value().
func (mw *JobContext) Value(key interface{}) interface{} {
	return mw.Ctx.Value(key)
}

// Cancel cancels the JobContext's context.  If no job has been started, this does nothing.
func (mw *JobContext) Cancel() {
	if mw.JcCancel != nil {
		mw.JcCancel()
	}
}

// Running returns true if a job is currently running.
func (mw *JobContext) Running() bool {
	mw.RunningLock.Lock()
	defer mw.RunningLock.Unlock()

	return mw.JcRunning
}
