package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	lzmodel "github.com/lengzhao/memory/model"
	lzservice "github.com/lengzhao/memory/service"
)

type agentMemoryExtractKind string

const (
	agentMemoryExtractPostTurn    agentMemoryExtractKind = "post_turn"
	agentMemoryExtractScheduled   agentMemoryExtractKind = "scheduled"
)

type agentMemoryExtractParams struct {
	kind                  agentMemoryExtractKind
	isolateSessionID      string
	dialogText            string
	contextMemories       []string
	resolutionContext     string
	auditSource           string
	auditExtra            map[string]any
	skillAugmentedExtract bool
}

func runAgentMemoryExtract(ctx context.Context, layout Layout, llm *lzmodel.LLMConfig, p agentMemoryExtractParams) error {
	if llm == nil {
		slog.Debug("memory.agent_extract.skip", "reason", "nil_llm", "pathway", string(p.kind))
		return nil
	}
	db, err := getAgentMemoryGorm(layout)
	if err != nil {
		slog.Warn(dbOpenLogKey(p.kind), "err", err)
		return err
	}
	timeout := time.Duration(llm.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		if p.kind == agentMemoryExtractPostTurn {
			timeout = 120 * time.Second
		} else {
			timeout = 1800 * time.Second
		}
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	extCtx := lzservice.WithIsolation(runCtx, layoutStableTenantID(layout), "default", p.isolateSessionID, DefaultRootAgentMemoryAgentID)
	ref := time.Now()
	extractor := lzservice.NewExtractor(db)
	req := lzservice.ExtractRequest{
		DialogText:        p.dialogText,
		ContextMemories:   p.contextMemories,
		MinConfidence:     0.7,
		DryRun:            false,
		UseDecisionEngine: false,
		LLMConfig:         llm,
		ReferenceTime:     &ref,
		ResolutionContext: p.resolutionContext,
	}
	if p.skillAugmentedExtract {
		ep := skillAugmentedExtractionPrompt()
		req.ExtractionPrompt = ep
		req.PostExtractHook = skillExtractPostHook(layout)
	}
	result, err := extractor.Extract(extCtx, req)
	if err != nil {
		slog.Warn(extractFailLogKey(p.kind), "err", err)
		return err
	}

	sqlitePath := agentMemorySQLitePath(layout)
	payload := map[string]any{
		"extraction_id": result.ExtractionID,
		"status":        result.Status,
		"memories":      len(result.Memories),
		"tokens":        result.TotalTokens,
	}
	for k, v := range p.auditExtra {
		payload[k] = v
	}
	if p.skillAugmentedExtract {
		payload["skill_augmented"] = true
	}
	auditPayload, _ := json.Marshal(payload)
	AppendMemoryAudit(layout, sqlitePath, p.auditSource, auditPayload)

	appendExtractSyncProjectMarkdown(layout, ref, p.kind, result.Memories)

	slog.Info(extractDoneLogKey(p.kind),
		"path", sqlitePath,
		"model", llm.Model,
		"memories", len(result.Memories),
		"status", result.Status,
		"tokens", result.TotalTokens,
	)
	return nil
}

func appendExtractSyncProjectMarkdown(layout Layout, ref time.Time, kind agentMemoryExtractKind, memories []lzservice.ExtractedMemory) {
	if len(memories) == 0 || strings.TrimSpace(layout.Project) == "" {
		return
	}
	body := formatExtractedMemoriesMarkdown(kind, ref, memories)
	if strings.TrimSpace(body) == "" {
		return
	}
	dateStr := ref.UTC().Format("2006-01-02")
	path := ProjectExtractDailyMarkdownPath(layout.Project, dateStr)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		slog.Warn("memory.extract_sync_md.mkdir", "path", path, "err", err)
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		slog.Warn("memory.extract_sync_md.open", "path", path, "err", err)
		return
	}
	defer f.Close()
	if _, err := f.WriteString(body); err != nil {
		slog.Warn("memory.extract_sync_md.write", "path", path, "err", err)
		return
	}
	AppendMemoryAudit(layout, path, "extract_sync_md", []byte(body))
}

func formatExtractedMemoriesMarkdown(kind agentMemoryExtractKind, ref time.Time, memories []lzservice.ExtractedMemory) string {
	var b strings.Builder
	b.WriteString("\n## Structured extract (")
	b.WriteString(string(kind))
	b.WriteString(") ")
	b.WriteString(ref.UTC().Format(time.RFC3339))
	b.WriteString("\n\n")
	for _, m := range memories {
		title := strings.TrimSpace(m.Title)
		if title == "" {
			title = "(untitled)"
		}
		ns := strings.TrimSpace(string(m.Namespace))
		if ns == "" {
			ns = "unknown"
		}
		fmt.Fprintf(&b, "### %s [%s]\n\n", title, ns)
		if s := strings.TrimSpace(m.Summary); s != "" {
			fmt.Fprintf(&b, "**Summary:** %s\n\n", s)
		}
		if s := strings.TrimSpace(m.Content); s != "" {
			b.WriteString(s)
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "_confidence %.2f · importance %d_", m.Confidence, m.Importance)
		if r := strings.TrimSpace(m.Reasoning); r != "" {
			fmt.Fprintf(&b, " · _%s_", r)
		}
		b.WriteString("\n\n---\n\n")
	}
	return b.String()
}

func dbOpenLogKey(kind agentMemoryExtractKind) string {
	if kind == agentMemoryExtractScheduled {
		return "memory.scheduled_extract.db_open_failed"
	}
	return "memory.post_turn_extract.db_open_failed"
}

func extractFailLogKey(kind agentMemoryExtractKind) string {
	if kind == agentMemoryExtractScheduled {
		return "memory.scheduled_extract.failed"
	}
	return "memory.post_turn_extract.failed"
}

func extractDoneLogKey(kind agentMemoryExtractKind) string {
	if kind == agentMemoryExtractScheduled {
		return "memory.scheduled_extract.done"
	}
	return "memory.post_turn_extract.done"
}
