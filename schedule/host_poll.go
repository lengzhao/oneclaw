package schedule

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/lengzhao/clawbridge/bus"
)

// StartHostPollerIfEnabled runs the due-job poller and submits synthetic inbound messages (same role as legacy channel.schedule_poll).
func StartHostPollerIfEnabled(ctx context.Context, cwd, clientID string, submit func(context.Context, bus.InboundMessage) error) error {
	if Disabled() {
		return nil
	}
	if cwd == "" {
		return fmt.Errorf("schedule: empty cwd for host poller")
	}
	if submit == nil {
		return fmt.Errorf("schedule: nil submit")
	}
	go runHostSchedulePoller(ctx, cwd, clientID, submit)
	return nil
}

func deliverHostScheduledTurns(ctx context.Context, cwd, source string, submit func(context.Context, bus.InboundMessage) error) {
	deliveries, err := CollectDue(cwd, source, time.Now())
	if err != nil {
		slog.Warn("schedule.collect_due", "err", err)
		return
	}
	for _, d := range deliveries {
		turnCtx := context.Background()
		msg := syntheticInboundFromDelivery(source, d)
		done := make(chan error, 1)
		go func() {
			done <- submit(turnCtx, msg)
		}()
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

func runHostSchedulePoller(ctx context.Context, cwd, source string, submit func(context.Context, bus.InboundMessage) error) {
	wakeCh, wakeCancel := SubscribeWake()
	defer wakeCancel()
	minSleep := MinTimerSleep()
	idleSleep := IdleTimerSleep()

	for {
		if ctx.Err() != nil {
			return
		}
		now := time.Now()
		d, ok := NextWakeDuration(cwd, source, now)
		if ok && d <= 0 {
			deliverHostScheduledTurns(ctx, cwd, source, submit)
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

		deliverHostScheduledTurns(ctx, cwd, source, submit)
	}
}
