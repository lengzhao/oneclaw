package memory

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	lzmodel "github.com/lengzhao/memory/model"
	"github.com/openai/openai-go"
)

// maintainPipelineMu serializes scheduled extraction writes with turn-end agent memory extraction SQLite writes.
var maintainPipelineMu sync.Mutex

type distillConfig struct {
	logDays             int
	maxCombinedBytes    int
	maxPerFile          int
	minLogBytes         int
	maxTopicFiles       int
	topicExcerptBytes   int
	topicBlockMaxTotal  int
	incrementalInterval time.Duration
}

func logScheduledSkip(reason string, kv ...any) {
	args := append([]any{"reason", reason}, kv...)
	slog.Info("memory.maintain.scheduled_skip", args...)
}

func scheduledExtractConfig(incrementalInterval time.Duration) distillConfig {
	return distillConfig{
		logDays:             maintenanceLogDays(),
		maxCombinedBytes:    maintenanceMaxCombinedLogBytes(),
		maxPerFile:          maintenanceMaxLogRead(),
		minLogBytes:         maintenanceMinLogBytes(),
		maxTopicFiles:       maintenanceMaxTopicFiles(),
		topicExcerptBytes:   maintenanceTopicExcerptBytes(),
		topicBlockMaxTotal:  24000,
		incrementalInterval: incrementalInterval,
	}
}

// ScheduledMaintainOpts configures scheduled extraction when the caller runs on a timer.
type ScheduledMaintainOpts struct {
	// Interval is how often the caller invokes RunScheduledMaintain (e.g. external cron / cmd wrapper with fixed tick).
	// When Interval > 0, daily logs are collected **incrementally**: only lines whose embedded RFC3339 timestamp
	// is after the saved high-water mark (from the last successful pass), with a first-run lookback of Interval
	// and caps/overlap from YAML maintain.incremental_* (see docs/config.md).
	// When Interval <= 0 or opts is nil, uses legacy calendar mode: maintain.log_days (rtopts).
	Interval time.Duration
}

// RunScheduledMaintain runs scheduled / far-field memory extraction via github.com/lengzhao/memory into agent_memory.sqlite.
// client is ignored (kept for API stability). extractLLM must be non-nil with API key and model (same pattern as post-turn).
// Does not write episodic digest markdown. Serialized with MaybePostTurnMaintain on maintainPipelineMu.
func RunScheduledMaintain(ctx context.Context, layout Layout, client *openai.Client, mainChatModel string, maxOutputTokens int64, opts *ScheduledMaintainOpts, extractLLM *lzmodel.LLMConfig) {
	_ = client
	inc := time.Duration(0)
	if opts != nil {
		inc = opts.Interval
	}
	runScheduledAgentMemoryMaintain(ctx, layout, mainChatModel, maxOutputTokens, inc, extractLLM)
}

// MaybePostTurnMaintain runs github.com/lengzhao/memory/service.Extractor.Extract once for this turn (writes agent_memory.sqlite).
// Skips when features.disable_auto_maintenance is set, or when [MemoryExtractEnabled] is false (same effective gates as [PostTurn] daily logging).
// session.Engine calls this from a goroutine after each successful turn so inbound channels are not blocked.
func MaybePostTurnMaintain(ctx context.Context, layout Layout, maxTokens int64, turn *PostTurnInput, extractLLM *lzmodel.LLMConfig) {
	if !autoMaintenanceEnabled() || !MemoryExtractEnabled() {
		return
	}
	llm := resolvePostTurnExtractLLM(extractLLM, maxTokens)
	if llm == nil {
		slog.Debug("memory.post_turn_extract.skip", "reason", "no_llm_config")
		return
	}
	maintainPipelineMu.Lock()
	defer maintainPipelineMu.Unlock()
	runPostTurnAgentMemoryExtract(ctx, layout, llm, turn)
}

func runScheduledAgentMemoryMaintain(ctx context.Context, layout Layout, mainChatModel string, maxOutputTokens int64, incrementalInterval time.Duration, extractLLM *lzmodel.LLMConfig) {
	if AutoMemoryDisabled() {
		logScheduledSkip("auto_memory_disabled")
		return
	}
	llm := resolveScheduledExtractLLM(extractLLM, maxOutputTokens)
	if llm == nil {
		slog.Debug("memory.scheduled_extract.skip", "reason", "no_llm_config")
		return
	}
	if m, _ := ResolveMaintenanceModel(mainChatModel, true); strings.TrimSpace(m) != "" {
		llm.Model = strings.TrimSpace(m)
	}

	migrateScheduledMaintainState(layout)
	incrementalStatePath := scheduledMaintainStatePath(layout)

	dateStr := time.Now().Format("2006-01-02")
	p := scheduledExtractConfig(incrementalInterval)

	corpus, probeBytes := buildScheduledMaintenanceCorpus(layout, dateStr, p, incrementalStatePath)
	if p.incrementalInterval > 0 {
		slog.Info("memory.maintain.scheduled_probe", "mode", "incremental", "interval", p.incrementalInterval.String(),
			"raw_bytes", probeBytes, "min", p.minLogBytes)
	} else {
		slog.Debug("memory.maintain.scheduled_probe", "mode", "log_days", "days", p.logDays, "raw_bytes", probeBytes)
	}
	if probeBytes < p.minLogBytes {
		logScheduledSkip("daily_logs_too_small", "days", p.logDays, "raw_bytes", probeBytes, "min", p.minLogBytes)
		return
	}
	if strings.TrimSpace(corpus) == "" {
		logScheduledSkip("empty_corpus", "raw_bytes", probeBytes)
		return
	}

	maintainPipelineMu.Lock()
	defer maintainPipelineMu.Unlock()

	rulesExcerpt := rulesExcerptForScheduledMaintain(layout, 8000)
	if err := runScheduledAgentMemoryExtract(ctx, layout, llm, corpus, rulesExcerpt); err != nil {
		return
	}
	persistScheduledMaintainSuccess(incrementalStatePath)
}
