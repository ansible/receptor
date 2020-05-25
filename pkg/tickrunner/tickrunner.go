package tickrunner

import (
	"time"
)

func Run(f func(), periodicInterval time.Duration, defaultReqDelay time.Duration,
	shutdownChan chan bool) chan time.Duration {
	runChan := make(chan time.Duration)
	go func() {
		nextRunTime := time.Now().Add(periodicInterval)
		for {
			select {
			case <-time.After(time.Until(nextRunTime)):
				nextRunTime = time.Now().Add(periodicInterval)
				f()
			case req := <- runChan:
				proposedTime := time.Now()
				if req == 0 {
					proposedTime = proposedTime.Add(defaultReqDelay)
				} else {
					proposedTime = proposedTime.Add(req)
				}
				if proposedTime.Before(nextRunTime) {
					nextRunTime = proposedTime
				}
			case <- shutdownChan:
				return
			}
		}
	}()
	return runChan
}