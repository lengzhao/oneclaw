// Package maintainloop runs RunScheduledMaintain on an interval inside the oneclaw process.
package maintainloop

import (
	"context"
	"log/slog"
	"time"

	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/tools/builtin"
	"github.com/openai/openai-go"
)

// Params configures the background scheduled maintenance goroutine.
type Params struct {
	Interval          time.Duration
	Layout            memory.Layout
	Client            *openai.Client
	MainModel         string
	MaxMaintainTokens int64
}

// Start launches a goroutine that runs RunScheduledMaintain once immediately, then every Interval,
// until ctx is done. No-op when Interval <= 0 or scheduled maintenance is globally disabled.
func Start(ctx context.Context, p Params) {
	if p.Interval <= 0 || p.Client == nil {
		return
	}
	if memory.ScheduledMaintenanceBackgroundDisabled() {
		slog.Info("maintainloop.skip", "reason", "scheduled_maintenance_disabled")
		return
	}
	mt := p.MaxMaintainTokens
	if mt <= 0 {
		mt = 8192
	}
	go run(ctx, p.Layout, p.Client, p.MainModel, mt, p.Interval)
}

func run(ctx context.Context, layout memory.Layout, client *openai.Client, mainModel string, maxTok int64, every time.Duration) {
	opts := &memory.ScheduledMaintainOpts{
		Interval:     every,
		ToolRegistry: builtin.ScheduledMaintainReadRegistry(),
	}
	slog.Info("maintainloop.start", "interval", every.String(), "cwd", layout.CWD, "project", layout.Project)
	runPass := func(reason string) {
		slog.Info("maintainloop.scheduled_pass", "reason", reason, "interval", every.String(), "cwd", layout.CWD)
		memory.RunScheduledMaintain(ctx, layout, client, mainModel, maxTok, opts)
	}
	runPass("immediate")
	t := time.NewTimer(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			runPass("interval")
			if !t.Stop() {
				select {
				case <-t.C:
				default:
				}
			}
			t.Reset(every)
		}
	}
}
