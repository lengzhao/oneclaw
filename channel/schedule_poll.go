package channel

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/lengzhao/oneclaw/schedule"
)

func deliverScheduledTurns(ctx context.Context, cwd, source string, inCh chan<- InboundTurn) {
	deliveries, err := schedule.CollectDue(cwd, source, time.Now())
	if err != nil {
		slog.Warn("schedule.collect_due", "err", err)
		return
	}
	for _, d := range deliveries {
		turnCtx := context.Background()
		done := make(chan error, 1)
		select {
		case <-ctx.Done():
			return
		case inCh <- InboundTurn{
			Ctx:           turnCtx,
			Text:          d.Text,
			SessionKey:    d.SessionKey,
			UserID:        d.UserID,
			TenantID:      d.TenantID,
			CorrelationID: d.CorrelationID,
			Done:          done,
		}:
		}
		select {
		case err := <-done:
			if err != nil {
				slog.Warn("schedule.turn_failed", "correlation_id", d.CorrelationID, "err", err)
			} else {
				slog.Info("schedule.turn_ok", "correlation_id", d.CorrelationID)
			}
		case <-ctx.Done():
			return
		}
	}
}

func runSchedulePoller(ctx context.Context, cwd, source string, inCh chan<- InboundTurn) {
	wakeCh, wakeCancel := schedule.SubscribeWake()
	defer wakeCancel()
	minSleep := schedule.MinTimerSleep()
	idleSleep := schedule.IdleTimerSleep()

	for {
		if ctx.Err() != nil {
			return
		}
		now := time.Now()
		d, ok := schedule.NextWakeDuration(cwd, source, now)
		if ok && d <= 0 {
			deliverScheduledTurns(ctx, cwd, source, inCh)
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
			if !t.Stop() {
				select {
				case <-t.C:
				default:
				}
			}
			return
		case <-wakeCh:
			if !t.Stop() {
				select {
				case <-t.C:
				default:
				}
			}
			continue
		case <-t.C:
		}

		deliverScheduledTurns(ctx, cwd, source, inCh)
	}
}

func startSchedulePollerIfEnabled(ctx context.Context, cwd, source string, inCh chan<- InboundTurn) error {
	if schedule.Disabled() {
		return nil
	}
	if cwd == "" {
		return fmt.Errorf("channel: empty cwd for schedule poller")
	}
	go runSchedulePoller(ctx, cwd, source, inCh)
	return nil
}
