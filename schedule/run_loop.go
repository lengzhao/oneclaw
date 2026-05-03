package schedule

import (
	"context"
	"log/slog"
	"time"
)

// RunPollerLoop drives p.Tick using next-fire sleep + [SubscribeWake] (same idea as main branch host poller).
func RunPollerLoop(ctx context.Context, p *Poller) {
	if p == nil || p.Path == "" || p.OnDue == nil {
		return
	}
	nowFn := p.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	wakeCh, wakeCancel := SubscribeWake()
	defer wakeCancel()
	minSleep := MinTimerSleep()
	idleSleep := IdleTimerSleep()

	for {
		if ctx.Err() != nil {
			return
		}
		now := nowFn()
		d, ok := NextWakeDuration(p.Path, now)
		if ok && d <= 0 {
			if err := p.Tick(ctx); err != nil {
				slog.Error("schedule.tick", "phase", "tick", "path", p.Path, "err", err)
			}
			continue
		}

		wait := idleSleep
		if ok && d > 0 {
			wait = d
			if wait < minSleep {
				wait = minSleep
			}
		}
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			stopTimerDrain(t)
			return
		case <-wakeCh:
			stopTimerDrain(t)
			continue
		case <-t.C:
		}

		if err := p.Tick(ctx); err != nil {
			slog.Error("schedule.tick", "phase", "tick", "path", p.Path, "err", err)
		}
	}
}

func stopTimerDrain(t *time.Timer) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
}
