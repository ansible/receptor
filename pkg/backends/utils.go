package backends

import (
	"context"
	"github.com/project-receptor/receptor/pkg/logger"
	"github.com/project-receptor/receptor/pkg/netceptor"
	"time"
)

type dialerFunc func(context.Context, chan struct{}) (netceptor.BackendSession, error)

// dialerSession is a convenience function for backends that use dial/retry logic
func dialerSession(ctx context.Context, redial bool, redialDelay time.Duration,
	df dialerFunc) (chan netceptor.BackendSession, error) {
	sessChan := make(chan netceptor.BackendSession)
	go func() {
		defer close(sessChan)
		for {
			closeChan := make(chan struct{})
			sess, err := df(ctx, closeChan)
			if err == nil {
				select {
				case sessChan <- sess:
					// continue
				case <-ctx.Done():
					return
				}
				select {
				case <-closeChan:
					// continue
				case <-ctx.Done():
					_ = sess.Close()
					return
				}
			}
			done := false
			select {
			case <-ctx.Done():
				done = true
			default:
			}
			if redial && !done {
				if err != nil {
					logger.Warning("Backend connection failed (will retry): %s\n", err)
				} else {
					logger.Warning("Backend connection exited (will retry)\n")
				}
				select {
				case <-time.After(redialDelay):
					continue
				case <-ctx.Done():
					return
				}
			} else {
				if err != nil {
					logger.Error("Backend connection failed: %s\n", err)
				} else if !done {
					logger.Error("Backend connection exited\n")
				}
				return
			}
		}
	}()
	return sessChan, nil
}

type listenFunc func(context.Context) error
type acceptFunc func() (netceptor.BackendSession, error)
type listenerCancelFunc func()

// listenerSession is a convenience function for backends that use listen/accept logic
func listenerSession(ctx context.Context, lf listenFunc, af acceptFunc, lcf listenerCancelFunc) (chan netceptor.BackendSession, error) {
	err := lf(ctx)
	if err != nil {
		return nil, err
	}
	sessChan := make(chan netceptor.BackendSession)
	go func() {
		defer func() {
			lcf()
			close(sessChan)
		}()
		for {
			c, err := af()
			select {
			case <-ctx.Done():
				return
			default:
			}
			if err != nil {
				logger.Error("Error accepting connection: %s\n", err)
				return
			}
			sessChan <- c
		}
	}()
	return sessChan, nil
}
