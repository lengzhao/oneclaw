package memory

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/routing"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/lengzhao/oneclaw/tools"
	"github.com/openai/openai-go"
)

// maintainPathway distinguishes post-turn (near-field) vs scheduled (consolidation) distill passes.
type maintainPathway string

const (
	pathwayPostTurn  maintainPathway = "post_turn"
	pathwayScheduled maintainPathway = "scheduled"
)

// maintainPipelineMu serializes distill passes that append to MEMORY.md (and read inputs they use).
var maintainPipelineMu sync.Mutex

type distillConfig struct {
	pathway              maintainPathway
	mainChatModel        string
	maxOutputTokens      int64
	useScheduledModelEnv bool
	logDays              int
	maxCombinedBytes     int
	maxPerFile           int
	minLogBytes          int
	maxTopicFiles        int
	topicExcerptBytes    int
	topicBlockMaxTotal   int
	memoryPreviewBytes   int
	timeout              time.Duration
	turn                 *PostTurnInput // post-turn pathway only; injected into user prompt
	// incrementalInterval > 0 (scheduled only): select daily log lines by embedded timestamp since last pass; see RunScheduledMaintain opts.
	incrementalInterval time.Duration
	scheduledMaxSteps   int // scheduled: model↔tool rounds (read-only tools)
	scheduledTools      *tools.Registry
}

func logPathwaySkip(p maintainPathway, reason string, kv ...any) {
	args := append([]any{"reason", reason}, kv...)
	if p == pathwayScheduled {
		slog.Info("memory.maintain.scheduled_skip", args...)
	} else {
		slog.Debug("memory.maintain.post_turn_skip", args...)
	}
}

func scheduledDistillConfig(mainChatModel string, maxTokens int64, incrementalInterval time.Duration, toolReg *tools.Registry) distillConfig {
	return distillConfig{
		pathway:              pathwayScheduled,
		mainChatModel:        mainChatModel,
		maxOutputTokens:      maxTokens,
		useScheduledModelEnv: true,
		logDays:              maintenanceLogDays(),
		maxCombinedBytes:     maintenanceMaxCombinedLogBytes(),
		maxPerFile:           maintenanceMaxLogRead(),
		minLogBytes:          maintenanceMinLogBytes(),
		maxTopicFiles:        maintenanceMaxTopicFiles(),
		topicExcerptBytes:    maintenanceTopicExcerptBytes(),
		topicBlockMaxTotal:   24000,
		memoryPreviewBytes:   8000,
		timeout:              scheduledMaintainTimeout(),
		incrementalInterval:  incrementalInterval,
		scheduledMaxSteps:    scheduledMaintainMaxSteps(),
		scheduledTools:       toolReg,
	}
}

func postTurnDistillConfig(mainChatModel string, maxTokens int64) distillConfig {
	return distillConfig{
		pathway:              pathwayPostTurn,
		mainChatModel:        mainChatModel,
		maxOutputTokens:      maxTokens,
		useScheduledModelEnv: false,
		// Near-field does not read daily logs or project topics; minLogBytes gates formatted snapshot size.
		minLogBytes:        postTurnMaintenanceMinLogBytes(),
		memoryPreviewBytes: postTurnMaintenanceMemoryPreviewBytes(),
		timeout:            postTurnMaintainTimeout(),
	}
}

// ScheduledMaintainOpts configures scheduled distillation when the caller runs on a timer.
type ScheduledMaintainOpts struct {
	// Interval is how often the caller invokes RunScheduledMaintain (e.g. maintainloop tick, cmd/maintain -interval).
	// When Interval > 0, daily logs are collected **incrementally**: only lines whose embedded RFC3339 timestamp
	// is after the saved high-water mark (from the last successful pass), with a first-run lookback of Interval
	// and caps/overlap from ONCLAW_MAINTENANCE_INCREMENTAL_* (see docs/config.md).
	// When Interval <= 0 or opts is nil, uses legacy calendar mode: ONCLAW_MAINTENANCE_LOG_DAYS.
	Interval time.Duration
	// ToolRegistry must register read-only tools (e.g. builtin.ScheduledMaintainReadRegistry). Required for far-field runs;
	// nil skips scheduled maintenance with a log line (avoids memory importing tools/builtin).
	ToolRegistry *tools.Registry
}

// RunScheduledMaintain runs the scheduled / far-field distill: a **multi-step agent** with read-only tools
// (read_file, grep, glob_file_search, list_dir) to inspect daily logs and project memory, then emit markdown.
// Use from cmd/maintain, embedded interval workers, or jobs. Does not consult disable_auto_maintenance.
// Pass opts.Interval when the caller runs on a fixed period (incremental time-window hints + state file).
// Serialized with RunPostTurnMaintain via maintainPipelineMu.
func RunScheduledMaintain(ctx context.Context, layout Layout, client *openai.Client, mainChatModel string, maxOutputTokens int64, opts *ScheduledMaintainOpts) {
	inc := time.Duration(0)
	var toolReg *tools.Registry
	if opts != nil {
		inc = opts.Interval
		toolReg = opts.ToolRegistry
	}
	runDistill(ctx, layout, client, scheduledDistillConfig(mainChatModel, maxOutputTokens, inc, toolReg))
}

// RunPostTurnMaintain runs the post-turn distill: **current session only** (turn snapshot + MEMORY excerpt for dedupe).
// No daily logs or topic files. When turn is nil or the formatted snapshot is below min bytes, the run is a no-op.
// Tools: none. Serialized with RunScheduledMaintain via maintainPipelineMu.
func RunPostTurnMaintain(ctx context.Context, layout Layout, client *openai.Client, mainChatModel string, maxOutputTokens int64, turn *PostTurnInput) {
	c := postTurnDistillConfig(mainChatModel, maxOutputTokens)
	c.turn = turn
	runDistill(ctx, layout, client, c)
}

// MaybePostTurnMaintain runs post-turn maintenance when auto maintenance is enabled (see autoMaintenanceEnabled).
// ctx is not used for cancellation or deadlines (post-turn runs on context.Background + ONCLAW_POST_TURN_MAINTENANCE_TIMEOUT_SEC).
func MaybePostTurnMaintain(ctx context.Context, layout Layout, client *openai.Client, mainChatModel string, maxTokens int64, turn *PostTurnInput) {
	if client == nil || !autoMaintenanceEnabled() {
		return
	}
	RunPostTurnMaintain(ctx, layout, client, mainChatModel, maxTokens, turn)
}

// MaybeMaintain is a deprecated alias for MaybePostTurnMaintain without a turn snapshot.
//
// Deprecated: use MaybePostTurnMaintain.
func MaybeMaintain(ctx context.Context, layout Layout, client *openai.Client, mainChatModel string, maxTokens int64) {
	MaybePostTurnMaintain(ctx, layout, client, mainChatModel, maxTokens, nil)
}

func runDistill(ctx context.Context, layout Layout, client *openai.Client, p distillConfig) {
	if client == nil {
		logPathwaySkip(p.pathway, "nil_client")
		return
	}
	if AutoMemoryDisabled() {
		logPathwaySkip(p.pathway, "auto_memory_disabled")
		return
	}
	EnsureDefaultAgentMd(layout)
	model, override := ResolveMaintenanceModel(p.mainChatModel, p.useScheduledModelEnv)
	if strings.TrimSpace(model) == "" {
		slog.Warn("memory.maintain.skip", "reason", "empty_model", "pathway", p.pathway)
		return
	}

	// Post-turn maintenance must not inherit the user/session context (HTTP disconnect, CLI cancel,
	// upstream deadlines). Use Background + maintain timeout only. Scheduled runs keep caller ctx so
	// shutdown can cancel long far-field passes.
	baseCtx := ctx
	if p.pathway == pathwayPostTurn {
		baseCtx = context.Background()
	}

	runCtx := baseCtx
	var cancel context.CancelFunc
	if p.timeout > 0 {
		runCtx, cancel = context.WithTimeout(baseCtx, p.timeout)
	} else if p.pathway == pathwayPostTurn {
		runCtx, cancel = context.WithTimeout(baseCtx, 120*time.Second)
	} else if _, ok := ctx.Deadline(); !ok {
		runCtx, cancel = context.WithTimeout(ctx, 120*time.Second)
	}
	if cancel != nil {
		defer cancel()
	}

	maintainPipelineMu.Lock()
	defer maintainPipelineMu.Unlock()

	runTS := time.Now().UTC().Format(time.RFC3339)
	dateStr := time.Now().Format("2006-01-02")
	digestHeader := "## Auto-maintained (" + dateStr + ")"
	memPath := filepath.Join(layout.Project, entrypointName)

	existingBytes, _ := os.ReadFile(memPath)
	existing := string(existingBytes)
	spanStart, spanEnd, hadTodayDigest := findSameDayAutoMaintainedSpan(existing, dateStr)

	incrementalStatePath := ""
	if p.pathway == pathwayScheduled {
		migrateScheduledMaintainState(layout)
		incrementalStatePath = scheduledMaintainStatePath(layout)
	}

	var userPrompt string
	if p.pathway == pathwayPostTurn {
		prev := existing
		mprev := p.memoryPreviewBytes
		if mprev <= 0 {
			mprev = 8000
		}
		if len(prev) > mprev {
			prev = strings.TrimRight(utf8SafePrefix(prev, mprev), "\n") + "\n…"
		}
		if p.turn == nil {
			logPathwaySkip(p.pathway, "nil_turn_snapshot")
			return
		}
		postTurnSnap := strings.TrimSpace(formatMaintainTurnSnapshot(p.turn))
		if len(postTurnSnap) < p.minLogBytes {
			logPathwaySkip(p.pathway, "turn_snapshot_too_small", "snapshot_bytes", len(postTurnSnap), "min", p.minLogBytes)
			return
		}
		scopeHint := "This is a **post-turn / near-field** pass: **only** the **Current turn snapshot** below plus the MEMORY excerpt (for dedupe). " +
			"Facts, rules, cautions, tool usage, **repeated tool calls** (reasons only if visible in this turn). " +
			"No daily logs or project topics are included. "
		turnBlock := "Current turn snapshot (current session only):\n```\n" + postTurnSnap + "\n```\n\n"
		taskBody := "Then **3–8** short bullet lines (one sentence each; **no** long paragraphs or redundant absolute paths) of **new** durable information from **this turn only** (facts, rules, cautions, tool-usage preferences, repeated tool calls and why **only** if stated or clearly implied in the snapshot). " +
			"Skip anything already in the MEMORY.md excerpt (paraphrases count as duplicates). " +
			"If nothing new remains for this turn, a single line: \"- (no durable entries)\". No other sections."
		sameDayNote := ""
		if hadTodayDigest {
			sameDayNote = "Note: **" + digestHeader + "** already exists for today; your bullets will be **merged** into that section (deduped). Use the **exact** same first line, then only **new** durable lines from this turn.\n\n"
		}
		userPrompt = fmt.Sprintf(
			"%s%s%sProject MEMORY.md excerpt (may be empty; use only to avoid duplicating existing bullets):\n```\n%s\n```\n\n"+
				"Task: Output markdown only. First line must be exactly:\n%s\n\n%s",
			scopeHint, sameDayNote, turnBlock, prev, digestHeader, taskBody,
		)
	} else {
		lastWall, lineHW, err := loadScheduledState(incrementalStatePath)
		if err != nil {
			slog.Warn("memory.maintain.scheduled_state_read_failed", "path", incrementalStatePath, "err", err)
			lastWall, lineHW = nil, nil
		}
		var rawIncluded int
		if p.incrementalInterval > 0 {
			minX := incrementalLineMinExclusive(lastWall, lineHW, p.incrementalInterval)
			rawIncluded = countFilteredDailyLogBytesSince(layout.Auto, minX)
			slog.Info("memory.maintain.scheduled_probe", "mode", "incremental", "interval", p.incrementalInterval.String(),
				"raw_bytes", rawIncluded, "min", p.minLogBytes)
		} else {
			rawIncluded = countRecentDailyLogBytes(layout.Auto, dateStr, p.logDays, p.minLogBytes)
			slog.Debug("memory.maintain.scheduled_probe", "mode", "log_days", "days", p.logDays, "raw_bytes", rawIncluded)
		}
		if rawIncluded < p.minLogBytes {
			logPathwaySkip(p.pathway, "daily_logs_too_small", "days", p.logDays, "raw_bytes", rawIncluded, "min", p.minLogBytes)
			return
		}
		userPrompt = buildScheduledToolUserPrompt(layout, memPath, p, incrementalStatePath, digestHeader, dateStr, hadTodayDigest)
	}

	tctx := toolctx.New(layout.CWD, runCtx)
	if home, herr := os.UserHomeDir(); herr == nil {
		tctx.HomeDir = home
	}
	var reg *tools.Registry
	maxSteps := 1
	if p.pathway == pathwayScheduled {
		if p.scheduledTools == nil {
			slog.Warn("memory.maintain.scheduled_skip", "reason", "nil_scheduled_tool_registry")
			return
		}
		tctx.MemoryWriteRoots = layout.WriteRoots()
		reg = p.scheduledTools
		maxSteps = p.scheduledMaxSteps
		if maxSteps <= 0 {
			maxSteps = 24
		}
	} else {
		reg = tools.NewRegistry()
	}

	msgs := []openai.ChatCompletionMessageParamUnion{}
	mt := maintenanceEffectiveMaxTokens(p.maxOutputTokens, p.pathway == pathwayPostTurn)
	if mt <= 0 || mt > 8192 {
		mt = 2048
	}
	cfg := loop.Config{
		Client:      client,
		Model:       model,
		System:      maintenanceSystemPromptForPathway(p.pathway, layout.CWD, memPath, dateStr, runTS),
		MaxTokens:   mt,
		MaxSteps:    maxSteps,
		Messages:    &msgs,
		Registry:    reg,
		ToolContext: tctx,
	}
	slog.Info("memory.maintain.request", "model", model, "pathway", p.pathway, "dedicated_model", override)
	if err := loop.RunTurn(runCtx, cfg, routing.Inbound{Text: userPrompt}); err != nil {
		slog.Warn("memory.maintain.run_failed", "model", model, "pathway", p.pathway, "err", err)
		return
	}
	out := strings.TrimSpace(loop.LastAssistantDisplay(msgs))
	out = stripMarkdownFences(out)
	if out == "" {
		slog.Warn("memory.maintain.empty_output", "model", model, "pathway", p.pathway)
		return
	}
	if !strings.Contains(out, digestHeader) {
		slog.Warn("memory.maintain.missing_header", "model", model, "pathway", p.pathway, "preview", utf8SafePrefix(out, 120))
		return
	}
	withoutToday := existing
	if hadTodayDigest {
		withoutToday = existing[:spanStart] + existing[spanEnd:]
	}
	out = dedupeMaintenanceBullets(out, withoutToday)
	if maintenanceSectionOnlyNoDurable(out) {
		persistScheduledMaintainSuccess(incrementalStatePath, p)
		slog.Info("memory.maintain.skip", "reason", "no_new_facts_after_dedupe", "date", dateStr, "pathway", p.pathway,
			"kept_existing_same_day", hadTodayDigest,
			"section_bytes", len(out), "bullets", countMaintenanceBullets(out), "preview", utf8SafePrefix(out, 1024))
		return
	}
	merged := mergeSameDayAutoMaintainedBlocks(existing[spanStart:spanEnd], out, digestHeader, withoutToday)
	if maintenanceSectionOnlyNoDurable(merged) {
		persistScheduledMaintainSuccess(incrementalStatePath, p)
		slog.Info("memory.maintain.skip", "reason", "merge_no_net_bullets", "date", dateStr, "pathway", p.pathway)
		return
	}
	if hadTodayDigest && strings.TrimSpace(merged) == strings.TrimSpace(existing[spanStart:spanEnd]) {
		persistScheduledMaintainSuccess(incrementalStatePath, p)
		slog.Info("memory.maintain.unchanged", "path", memPath, "date", dateStr, "pathway", p.pathway)
		return
	}
	auditSrc := AuditSourceScheduledMaintain
	if p.pathway == pathwayPostTurn {
		auditSrc = AuditSourcePostTurnMaintain
	}
	if err := writeMergedOrAppendMaintenanceSection(layout, memPath, hadTodayDigest, spanStart, spanEnd, existing, merged, auditSrc); err != nil {
		slog.Warn("memory.maintain.write_failed", "path", memPath, "pathway", p.pathway, "err", err)
		return
	}
	persistScheduledMaintainSuccess(incrementalStatePath, p)
	logMsg := "memory.maintain.wrote"
	if hadTodayDigest {
		logMsg = "memory.maintain.merged"
	}
	slog.Info(logMsg,
		"path", memPath,
		"date", dateStr,
		"model", model,
		"pathway", p.pathway,
		"audit_source", auditSrc,
		"section_bytes", len(merged),
		"bullets", countMaintenanceBullets(merged),
		"preview", utf8SafePrefix(merged, 1536),
	)
}
