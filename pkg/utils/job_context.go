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
	done        chan struct{}
	JcCancel    context.CancelFunc
	Wg          *sync.WaitGroup
	JcRunning   bool
	RunningLock *sync.Mutex
	deadlineFn  func() (time.Time, bool)
	valueFn     func(key interface{}) interface{}
	errFn       func() error
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
		prevDone := mw.done
		mw.JcCancel()
		mw.RunningLock.Unlock()
		<-prevDone // wait for cancellation to propagate; returns immediately after JcCancel
		mw.RunningLock.Lock()
	}

	done := make(chan struct{})
	mw.done = done
	var closeOnce sync.Once
	closeDone := func() {
		closeOnce.Do(func() { close(done) })
	}
	mw.JcCancel = closeDone

	// Capture parent delegation functions so Deadline/Value/Err proxy the parent.
	mw.deadlineFn = ctx.Deadline
	mw.valueFn = ctx.Value
	mw.errFn = ctx.Err

	// Propagate parent cancellation to our done channel.
	go func() {
		select {
		case <-ctx.Done():
			closeDone()
		case <-done:
		}
	}()

	mw.JcRunning = true
	mw.Wg = &sync.WaitGroup{}
	mw.Wg.Add(workers)
	mw.RunningLock.Unlock()

	// Background goroutine: mark job stopped when workers finish OR when cancelled.
	wg := mw.Wg
	go func() {
		wgDone := make(chan struct{})
		go func() { wg.Wait(); close(wgDone) }()
		select {
		case <-wgDone:
		case <-done:
		}
		mw.RunningLock.Lock()
		mw.JcRunning = false
		closeDone()
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
	return mw.done
}

// Err implements Context.Err(), delegating to the parent context's error when done.
func (mw *JobContext) Err() error {
	select {
	case <-mw.done:
		if mw.errFn != nil {
			if err := mw.errFn(); err != nil {
				return err
			}
		}

		return context.Canceled
	default:
		return nil
	}
}

// Deadline implements Context.Deadline(), delegating to the parent context.
func (mw *JobContext) Deadline() (deadline time.Time, ok bool) {
	if mw.deadlineFn != nil {
		return mw.deadlineFn()
	}

	return time.Time{}, false
}

// Value implements Context.Value(), delegating to the parent context.
func (mw *JobContext) Value(key interface{}) interface{} {
	if mw.valueFn != nil {
		return mw.valueFn(key)
	}

	return nil
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
