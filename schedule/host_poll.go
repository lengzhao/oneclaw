package schedule

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/memory"
)

// StartHostPollerIfEnabled runs the due-job poller and submits synthetic inbound messages (same role as legacy channel.schedule_poll).
// userDataRoot is config.UserDataRoot() (e.g. ~/.oneclaw). When sessionIsolateWorkspace is false, jobs are read from
// <userDataRoot>/scheduled_jobs.json. When true, each session <userDataRoot>/sessions/<id>/scheduled_jobs.json
// is polled (matches cron tool persistence under sessions.isolate_workspace).
func StartHostPollerIfEnabled(ctx context.Context, userDataRoot string, sessionIsolateWorkspace bool, clientID string, submit func(context.Context, bus.InboundMessage) error) error {
	if Disabled() {
		return nil
	}
	if userDataRoot == "" {
		return fmt.Errorf("schedule: empty user data root for host poller")
	}
	if submit == nil {
		return fmt.Errorf("schedule: nil submit")
	}
	go runHostSchedulePoller(ctx, userDataRoot, sessionIsolateWorkspace, clientID, submit)
	return nil
}

func deliverHostScheduledTurns(ctx context.Context, userDataRoot string, sessionIsolate bool, source string, submit func(context.Context, bus.InboundMessage) error) {
	if !sessionIsolate {
		deliveries, err := CollectDue(userDataRoot, userDataRoot, true, "", source, time.Now())
		if err != nil {
			slog.Warn("schedule.collect_due", "err", err)
			return
		}
		submitScheduleDeliveries(ctx, deliveries, source, submit)
		return
	}
	dir := filepath.Join(userDataRoot, "sessions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("schedule.collect_due.isolated", "dir", dir, "err", err)
		}
		return
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		sessionRoot := filepath.Join(userDataRoot, "sessions", name)
		sessionWorkspace := filepath.Join(sessionRoot, memory.IMWorkspaceDirName)
		if st, err := os.Stat(sessionRoot); err != nil || !st.IsDir() {
			continue
		}
		deliveries, err := CollectDue(sessionWorkspace, "", true, sessionRoot, source, time.Now())
		if err != nil {
			slog.Warn("schedule.collect_due", "session_workspace", sessionWorkspace, "err", err)
			continue
		}
		submitScheduleDeliveries(ctx, deliveries, source, submit)
	}
}

func submitScheduleDeliveries(ctx context.Context, deliveries []TurnDelivery, source string, submit func(context.Context, bus.InboundMessage) error) {
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

func nextWakeHost(userDataRoot string, sessionIsolate bool, targetSource string, now time.Time) (d time.Duration, ok bool) {
	if !sessionIsolate {
		return NextWakeDuration(userDataRoot, userDataRoot, true, "", targetSource, now)
	}
	dir := filepath.Join(userDataRoot, "sessions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, false
	}
	var minD time.Duration
	found := false
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		sessionRoot := filepath.Join(userDataRoot, "sessions", e.Name())
		sessionWorkspace := filepath.Join(sessionRoot, memory.IMWorkspaceDirName)
		if st, err := os.Stat(sessionRoot); err != nil || !st.IsDir() {
			continue
		}
		di, oi := NextWakeDuration(sessionWorkspace, "", true, sessionRoot, targetSource, now)
		if !oi {
			continue
		}
		if !found || di < minD {
			minD = di
			found = true
		}
	}
	return minD, found
}

func runHostSchedulePoller(ctx context.Context, userDataRoot string, sessionIsolate bool, source string, submit func(context.Context, bus.InboundMessage) error) {
	wakeCh, wakeCancel := SubscribeWake()
	defer wakeCancel()
	minSleep := MinTimerSleep()
	idleSleep := IdleTimerSleep()

	for {
		if ctx.Err() != nil {
			return
		}
		now := time.Now()
		d, ok := nextWakeHost(userDataRoot, sessionIsolate, source, now)
		if ok && d <= 0 {
			deliverHostScheduledTurns(ctx, userDataRoot, sessionIsolate, source, submit)
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

		deliverHostScheduledTurns(ctx, userDataRoot, sessionIsolate, source, submit)
	}
}
