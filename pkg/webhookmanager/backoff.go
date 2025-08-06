package webhookmanager

import (
	"time"

	"github.com/jpillora/backoff"
	"k8s.io/utils/clock"
)

type backoffTimer struct {
	timer   clock.Timer
	backoff backoff.Backoff
}

func newBackoffTimer(clk clock.Clock, minDuration, maxDuration time.Duration) *backoffTimer {
	t := &backoffTimer{
		backoff: backoff.Backoff{
			Min: minDuration,
			Max: maxDuration,
		},
	}
	t.timer = clk.NewTimer(t.backoff.Duration())
	return t
}

func (t *backoffTimer) C() <-chan time.Time {
	return t.timer.C()
}

func (t *backoffTimer) Stop() bool {
	return t.timer.Stop()
}

func (t *backoffTimer) Reset() {
	t.backoff.Reset()
	t.BackOff()
}

func (t *backoffTimer) BackOff() {
	t.timer.Reset(t.backoff.Duration())
}
