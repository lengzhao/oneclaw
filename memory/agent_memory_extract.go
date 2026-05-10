package memory

import (
	"context"
	"encoding/json"
	"log/slog"
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
	kind              agentMemoryExtractKind
	isolateSessionID  string
	dialogText        string
	contextMemories   []string
	resolutionContext string
	auditSource       string
	auditExtra        map[string]any
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
	auditPayload, _ := json.Marshal(payload)
	AppendMemoryAudit(layout, sqlitePath, p.auditSource, auditPayload)

	slog.Info(extractDoneLogKey(p.kind),
		"path", sqlitePath,
		"model", llm.Model,
		"memories", len(result.Memories),
		"status", result.Status,
		"tokens", result.TotalTokens,
	)
	return nil
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
