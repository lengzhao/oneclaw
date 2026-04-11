package schedule

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/lengzhao/clawbridge/bus"
)

// StartHostPollerIfEnabled runs the due-job poller and submits synthetic inbound messages (same role as legacy channel.schedule_poll).
// userDataRoot is config.UserDataRoot() (e.g. ~/.oneclaw); scheduled_jobs.json lives directly under that directory.
func StartHostPollerIfEnabled(ctx context.Context, userDataRoot, clientID string, submit func(context.Context, bus.InboundMessage) error) error {
	if Disabled() {
		return nil
	}
	if userDataRoot == "" {
		return fmt.Errorf("schedule: empty user data root for host poller")
	}
	if submit == nil {
		return fmt.Errorf("schedule: nil submit")
	}
	go runHostSchedulePoller(ctx, userDataRoot, clientID, submit)
	return nil
}

func deliverHostScheduledTurns(ctx context.Context, userDataRoot, source string, submit func(context.Context, bus.InboundMessage) error) {
	deliveries, err := CollectDue("", userDataRoot, source, time.Now())
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

func runHostSchedulePoller(ctx context.Context, userDataRoot, source string, submit func(context.Context, bus.InboundMessage) error) {
	wakeCh, wakeCancel := SubscribeWake()
	defer wakeCancel()
	minSleep := MinTimerSleep()
	idleSleep := IdleTimerSleep()

	for {
		if ctx.Err() != nil {
			return
		}
		now := time.Now()
		d, ok := NextWakeDuration("", userDataRoot, source, now)
		if ok && d <= 0 {
			deliverHostScheduledTurns(ctx, userDataRoot, source, submit)
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

		deliverHostScheduledTurns(ctx, userDataRoot, source, submit)
	}
}
