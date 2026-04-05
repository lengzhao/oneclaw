package memory

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/routing"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/lengzhao/oneclaw/tools"
	"github.com/openai/openai-go"
)

// MaintainOptions configures a single distill pass (post-turn or scheduled CLI).
type MaintainOptions struct {
	MainChatModel   string
	MaxOutputTokens int64
	Scheduled       bool
}

func logScheduledMaintainSkip(scheduled bool, reason string, kv ...any) {
	if !scheduled {
		return
	}
	args := make([]any, 0, 2+len(kv))
	args = append(args, "reason", reason)
	args = append(args, kv...)
	slog.Info("memory.maintain.scheduled_skip", args...)
}

// RunMaintain performs one maintenance distill when preconditions match (log size, idempotency).
// Does not consult ONCLAW_DISABLE_AUTO_MAINTENANCE — use MaybeMaintain for post-turn gating.
func RunMaintain(ctx context.Context, layout Layout, client *openai.Client, opt MaintainOptions) {
	if client == nil {
		logScheduledMaintainSkip(opt.Scheduled, "nil_client")
		return
	}
	if AutoMemoryDisabled() {
		logScheduledMaintainSkip(opt.Scheduled, "auto_memory_disabled")
		return
	}
	model, override := ResolveMaintenanceModel(opt.MainChatModel, opt.Scheduled)
	if strings.TrimSpace(model) == "" {
		slog.Warn("memory.maintain.skip", "reason", "empty_model")
		return
	}

	runCtx := ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, 120*time.Second)
		defer cancel()
	}

	dateStr := time.Now().Format("2006-01-02")
	digestHeader := "## Auto-maintained (" + dateStr + ")"
	logPath := DailyLogPath(layout.Auto, dateStr)
	data, err := os.ReadFile(logPath)
	if err != nil {
		logScheduledMaintainSkip(opt.Scheduled, "daily_log_unreadable", "path", logPath, "err", err)
		return
	}
	if len(data) < maintenanceMinLogBytes() {
		logScheduledMaintainSkip(opt.Scheduled, "daily_log_too_small", "path", logPath, "bytes", len(data), "min", maintenanceMinLogBytes())
		return
	}
	excerpt := string(data)
	if len(excerpt) > maintenanceMaxLogRead() {
		excerpt = strings.TrimRight(utf8SafePrefix(excerpt, maintenanceMaxLogRead()), "\n") + "\n\n…"
	}

	memPath := filepath.Join(layout.Project, entrypointName)
	existingBytes, _ := os.ReadFile(memPath)
	existing := string(existingBytes)
	if strings.Contains(existing, digestHeader) {
		if opt.Scheduled {
			slog.Info("memory.maintain.scheduled_skip", "reason", "already_maintained", "date", dateStr, "path", memPath)
		} else {
			slog.Debug("memory.maintain.skip", "reason", "already_maintained", "date", dateStr)
		}
		return
	}

	prev := existing
	if len(prev) > 8000 {
		prev = strings.TrimRight(utf8SafePrefix(prev, 8000), "\n") + "\n…"
	}

	userPrompt := fmt.Sprintf(
		"Project MEMORY.md excerpt (may be empty):\n```\n%s\n```\n\nToday's daily log:\n```\n%s\n```\n\n"+
			"Task: Output markdown only. First line must be exactly:\n%s\n\n"+
			"Then 3–12 bullet lines of durable facts learned from the log (preferences, bugs, decisions). "+
			"If nothing durable, a single line: \"- (no durable entries)\". No other sections.",
		prev, excerpt, digestHeader,
	)

	reg := tools.NewRegistry()
	msgs := []openai.ChatCompletionMessageParamUnion{}
	mt := MaintenanceMaxOutputTokens(opt.MaxOutputTokens)
	if mt <= 0 || mt > 8192 {
		mt = 2048
	}
	cfg := loop.Config{
		Client:      client,
		Model:       model,
		System:      maintenanceSystemPrompt(),
		MaxTokens:   mt,
		MaxSteps:    1,
		Messages:    &msgs,
		Registry:    reg,
		ToolContext: toolctx.New(layout.CWD, runCtx),
	}
	slog.Info("memory.maintain.request", "model", model, "scheduled", opt.Scheduled, "dedicated_model", override)
	if err := loop.RunTurn(runCtx, cfg, routing.Inbound{Text: userPrompt}); err != nil {
		slog.Warn("memory.maintain.run_failed", "model", model, "err", err)
		return
	}
	out := strings.TrimSpace(loop.LastAssistantDisplay(msgs))
	out = stripMarkdownFences(out)
	if out == "" {
		slog.Warn("memory.maintain.empty_output", "model", model)
		return
	}
	if !strings.Contains(out, digestHeader) {
		slog.Warn("memory.maintain.missing_header", "model", model, "preview", utf8SafePrefix(out, 120))
		return
	}
	if err := appendMaintenanceSection(layout, memPath, out); err != nil {
		slog.Warn("memory.maintain.write_failed", "path", memPath, "err", err)
		return
	}
	slog.Info("memory.maintain.ok", "path", memPath, "date", dateStr, "model", model)
}

// MaybeMaintain runs post-turn maintenance when enabled by env (see autoMaintenanceEnabled).
func MaybeMaintain(ctx context.Context, layout Layout, client *openai.Client, mainChatModel string, maxTokens int64) {
	if client == nil || !autoMaintenanceEnabled() {
		return
	}
	RunMaintain(ctx, layout, client, MaintainOptions{
		MainChatModel:   mainChatModel,
		MaxOutputTokens: maxTokens,
		Scheduled:       false,
	})
}
