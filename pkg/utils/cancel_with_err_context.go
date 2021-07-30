package utils

import (
	"context"
	"sync"
	"time"
)

// CancelWithErrFunc is like a regular context.CancelFunc, but you can specify an error to return.
type CancelWithErrFunc func(err error)

// CancelWithErrContext is a context that can be cancelled with a specific error return.
type CancelWithErrContext struct {
	parentCtx context.Context
	errChan   chan error
	doneChan  chan struct{}
	closeOnce sync.Once
	err       error
}

// ContextWithCancelWithErr returns a context and a CancelWithErrFunc. This functions like a normal
// context cancel function, except you can specify what error should be returned.
func ContextWithCancelWithErr(parent context.Context) (*CancelWithErrContext, CancelWithErrFunc) {
	cwe := &CancelWithErrContext{
		parentCtx: parent,
		errChan:   make(chan error),
		doneChan:  make(chan struct{}),
		closeOnce: sync.Once{},
	}
	go func() {
		for {
			select {
			case <-parent.Done():
				cwe.closeDoneChan()

				return
			case err := <-cwe.errChan:
				cwe.err = err
				if err != nil {
					cwe.closeDoneChan()

					return
				}
			}
		}
	}()

	return cwe, func(err error) {
		cwe.err = err
		cwe.closeDoneChan()
	}
}

func (cwe *CancelWithErrContext) closeDoneChan() {
	cwe.closeOnce.Do(func() {
		close(cwe.doneChan)
	})
}

// Done implements Context.Done().
func (cwe *CancelWithErrContext) Done() <-chan struct{} {
	return cwe.doneChan
}

// Err implements Context.Err().
func (cwe *CancelWithErrContext) Err() error {
	if cwe.err != nil {
		return cwe.err
	}

	return cwe.parentCtx.Err()
}

// Deadline implements Context.Deadline().
func (cwe *CancelWithErrContext) Deadline() (time time.Time, ok bool) {
	return cwe.parentCtx.Deadline()
}

// Value implements Context.Value().
func (cwe *CancelWithErrContext) Value(key interface{}) interface{} {
	return cwe.parentCtx.Value(key)
}
