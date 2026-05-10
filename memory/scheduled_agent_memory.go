package memory

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	lzmodel "github.com/lengzhao/memory/model"
	"github.com/lengzhao/oneclaw/rtopts"
)

// scheduledMaintainRulesExcerptBytes caps MEMORY.md text embedded in scheduled extract DialogText.
const scheduledMaintainRulesExcerptBytes = 8000

// NewScheduledExtractLLM builds LLM config for scheduled/far-field extraction (timeouts + caps from maintain.* rtopts).
func NewScheduledExtractLLM(apiKey, baseURL, model string) *lzmodel.LLMConfig {
	apiKey = strings.TrimSpace(apiKey)
	model = strings.TrimSpace(model)
	if apiKey == "" || model == "" {
		return nil
	}
	timeoutSec := int(scheduledMaintainTimeout().Seconds())
	if timeoutSec <= 0 {
		timeoutSec = 1800
	}
	maxTok := 4096
	if t := rtopts.Current().MaintenanceMaxTokens; t > 0 && int(t) < maxTok {
		maxTok = int(t)
	}
	if maxTok < 512 {
		maxTok = 512
	}
	cfg := &lzmodel.LLMConfig{
		APIKey:         apiKey,
		Model:          model,
		MaxTokens:      maxTok,
		Temperature:    0.2,
		TimeoutSeconds: timeoutSec,
	}
	if b := strings.TrimSpace(baseURL); b != "" {
		cfg.BaseURL = &b
	}
	return cfg
}

// projectMemoryRulesExcerpt reads layout.Project MEMORY.md truncated to maxBytes (UTF-8 safe).
func projectMemoryRulesExcerpt(layout Layout, maxBytes int) string {
	rulesPath := layout.ProjectRulesMemoryPath()
	raw, err := os.ReadFile(rulesPath)
	if err != nil || len(raw) == 0 {
		return ""
	}
	s := string(raw)
	if maxBytes <= 0 {
		maxBytes = 8000
	}
	if len(s) > maxBytes {
		s = strings.TrimRight(utf8SafePrefix(s, maxBytes), "\n") + "\n…"
	}
	return s
}

func appendDailyLogCorpusSection(b *strings.Builder, ds string, raw []byte, perFile, totalCap int, incrementalHeading bool) bool {
	chunk := string(raw)
	if len(chunk) > perFile {
		chunk = strings.TrimRight(utf8SafePrefix(chunk, perFile), "\n") + "\n…"
	}
	var sec string
	if incrementalHeading {
		sec = fmt.Sprintf("### Daily log %s (incremental lines)\n```\n%s\n```\n\n", ds, chunk)
	} else {
		sec = fmt.Sprintf("### Daily log %s\n```\n%s\n```\n\n", ds, chunk)
	}
	if b.Len()+len(sec) > totalCap {
		return false
	}
	b.WriteString(sec)
	return true
}

// buildScheduledMaintenanceCorpus assembles daily log text (+ optional topic excerpts) for Extract DialogText.
func buildScheduledMaintenanceCorpus(layout Layout, dateStr string, p distillConfig, incrementalStatePath string) (corpus string, probeBytes int) {
	totalCap := p.maxCombinedBytes
	if totalCap <= 0 {
		totalCap = 48_000
	}
	perFile := p.maxPerFile
	if perFile <= 0 {
		perFile = 24_000
	}
	untilUTC := time.Now().UTC()
	var b strings.Builder

	if p.incrementalInterval > 0 {
		lastWall, lineHW, err := loadScheduledState(incrementalStatePath)
		if err != nil {
			slog.Warn("memory.maintain.scheduled_state_read_failed", "path", incrementalStatePath, "err", err)
			lastWall, lineHW = nil, nil
		}
		minX := incrementalLineMinExclusive(lastWall, lineHW, p.incrementalInterval)
		probeBytes = countFilteredDailyLogBytesSince(layout.Auto, minX)
		startDay := truncateToUTCDate(minX)
		endDay := truncateToUTCDate(untilUTC)
		for d := endDay; !d.Before(startDay); d = d.AddDate(0, 0, -1) {
			ds := d.Format("2006-01-02")
			path := DailyLogPath(layout.Auto, ds)
			data, err := os.ReadFile(path)
			if err != nil || len(data) == 0 {
				continue
			}
			filtered := filterDailyLogBytesAfter(data, minX, untilUTC)
			if len(filtered) == 0 {
				continue
			}
			if !appendDailyLogCorpusSection(&b, ds, filtered, perFile, totalCap, true) {
				break
			}
		}
	} else {
		probeBytes = countRecentDailyLogBytes(layout.Auto, dateStr, p.logDays, p.minLogBytes)
		t, err := time.ParseInLocation("2006-01-02", dateStr, time.UTC)
		if err != nil {
			return "", probeBytes
		}
		for d := 0; d < p.logDays; d++ {
			day := t.AddDate(0, 0, -d)
			ds := day.Format("2006-01-02")
			path := DailyLogPath(layout.Auto, ds)
			data, err := os.ReadFile(path)
			if err != nil || len(data) < p.minLogBytes {
				continue
			}
			if !appendDailyLogCorpusSection(&b, ds, data, perFile, totalCap, false) {
				break
			}
		}
	}
	appendTopicCorpus(&b, layout, p, totalCap)
	return strings.TrimSpace(b.String()), probeBytes
}

func appendTopicCorpus(b *strings.Builder, layout Layout, p distillConfig, totalCap int) {
	if p.maxTopicFiles <= 0 {
		return
	}
	entries, err := os.ReadDir(layout.Project)
	if err != nil {
		return
	}
	var mdFiles []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.EqualFold(filepath.Ext(name), ".md") {
			continue
		}
		if strings.EqualFold(name, entrypointName) {
			continue
		}
		base := strings.TrimSuffix(name, filepath.Ext(name))
		if len(base) == 10 && base[4] == '-' && base[7] == '-' {
			continue
		}
		mdFiles = append(mdFiles, name)
	}
	sort.Strings(mdFiles)
	blockBudget := p.topicBlockMaxTotal
	if blockBudget <= 0 {
		blockBudget = 24_000
	}
	excerpt := p.topicExcerptBytes
	if excerpt <= 0 {
		excerpt = 4000
	}
	n := 0
	for _, name := range mdFiles {
		if n >= p.maxTopicFiles {
			break
		}
		path := filepath.Join(layout.Project, name)
		raw, err := os.ReadFile(path)
		if err != nil || len(raw) == 0 {
			continue
		}
		body := string(raw)
		if len(body) > excerpt {
			body = strings.TrimRight(utf8SafePrefix(body, excerpt), "\n") + "\n…"
		}
		sec := fmt.Sprintf("### Topic %s\n```\n%s\n```\n\n", name, body)
		if b.Len()+len(sec) > totalCap || len(sec) > blockBudget {
			break
		}
		b.WriteString(sec)
		n++
	}
}

func runScheduledAgentMemoryExtract(ctx context.Context, layout Layout, llm *lzmodel.LLMConfig, corpus string, rulesExcerpt string) error {
	dialog := "Scheduled / far-field memory consolidation from recent daily logs and optional topic excerpts.\n\n## Corpus\n\n" + corpus
	if ex := strings.TrimSpace(rulesExcerpt); ex != "" {
		dialog = "Project MEMORY.md rules excerpt (avoid extracting duplicates already covered as standing rules):\n```\n" + ex + "\n```\n\n" + dialog
	}
	return runAgentMemoryExtract(ctx, layout, llm, agentMemoryExtractParams{
		kind:                  agentMemoryExtractScheduled,
		isolateSessionID:      ScheduledMaintainIsolationSessionID,
		dialogText:            dialog,
		contextMemories:       nil,
		resolutionContext:     "scheduled batch maintenance",
		auditSource:           AuditSourceScheduledMaintain,
		auditExtra:            map[string]any{"pathway": "scheduled"},
		skillAugmentedExtract: true,
	})
}
