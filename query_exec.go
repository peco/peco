package peco

import (
	"sync"
	"time"
)

// QueryExecState holds the state for delayed query execution,
// including the delay duration, a mutex guarding the timer,
// and the timer itself.
type QueryExecState struct {
	delay time.Duration
	mutex sync.Mutex
	timer *time.Timer
}

// Delay returns the query execution delay.
func (q *QueryExecState) Delay() time.Duration {
	return q.delay
}

// StopTimer stops and clears the pending query exec timer.
// It must be called during shutdown to prevent the timer callback
// from firing after program state is torn down.
func (q *QueryExecState) StopTimer() {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if q.timer != nil {
		q.timer.Stop()
		q.timer = nil
	}
}
