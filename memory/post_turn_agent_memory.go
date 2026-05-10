package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	lzmodel "github.com/lengzhao/memory/model"
	lzservice "github.com/lengzhao/memory/service"
	lzstore "github.com/lengzhao/memory/store"
	"github.com/lengzhao/oneclaw/rtopts"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const postTurnAgentMemoryFile = "agent_memory.sqlite"

var agentMemDBCache struct {
	mu sync.Mutex
	m  map[string]*gorm.DB
}

// agentMemorySQLitePath returns the per-project SQLite file for github.com/lengzhao/memory (turn-end extract).
func agentMemorySQLitePath(layout Layout) string {
	return filepath.Join(layout.Auto, postTurnAgentMemoryFile)
}

func layoutStableTenantID(l Layout) string {
	sum := sha256.Sum256([]byte(filepath.Clean(l.CWD)))
	return hex.EncodeToString(sum[:12])
}

// NewPostTurnExtractLLM builds OpenAI-compatible config for turn-end extraction from credentials and rtopts.
func NewPostTurnExtractLLM(apiKey, baseURL, model string) *lzmodel.LLMConfig {
	apiKey = strings.TrimSpace(apiKey)
	model = strings.TrimSpace(model)
	if apiKey == "" || model == "" {
		return nil
	}
	timeoutSec := int(postTurnMaintainTimeout().Seconds())
	if timeoutSec <= 0 {
		timeoutSec = 120
	}
	maxTok := 4096
	if t := rtopts.Current().PostTurnMaxTokens; t > 0 && int(t) < maxTok {
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

func resolvePostTurnExtractLLM(explicit *lzmodel.LLMConfig, maxOut int64) *lzmodel.LLMConfig {
	outTok := maintenanceEffectiveMaxTokens(maxOut, true)
	maxTok := 4096
	if outTok > 0 && int(outTok) < maxTok {
		maxTok = int(outTok)
	}
	if maxTok < 512 {
		maxTok = 512
	}
	timeoutSec := int(postTurnMaintainTimeout().Seconds())
	if timeoutSec <= 0 {
		timeoutSec = 120
	}

	if explicit == nil || strings.TrimSpace(explicit.APIKey) == "" || strings.TrimSpace(explicit.Model) == "" {
		return nil
	}
	c := *explicit
	if c.MaxTokens <= 0 {
		c.MaxTokens = maxTok
	}
	if c.TimeoutSeconds <= 0 {
		c.TimeoutSeconds = timeoutSec
	}
	if c.Temperature == 0 {
		c.Temperature = 0.2
	}
	return &c
}

func getAgentMemoryGorm(layout Layout) (*gorm.DB, error) {
	path := filepath.Clean(agentMemorySQLitePath(layout))
	agentMemDBCache.mu.Lock()
	defer agentMemDBCache.mu.Unlock()
	if agentMemDBCache.m == nil {
		agentMemDBCache.m = make(map[string]*gorm.DB)
	}
	if db, ok := agentMemDBCache.m[path]; ok {
		return db, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	cfg := lzstore.DefaultConfig()
	cfg.Path = path
	cfg.AutoMigrate = true
	cfg.LogLevel = logger.Silent
	db, err := lzstore.InitDB(cfg)
	if err != nil {
		return nil, err
	}
	agentMemDBCache.m[path] = db
	return db, nil
}

// runPostTurnAgentMemoryExtract is the post-turn github.com/lengzhao/memory integration: NewExtractor(db).Extract(...)
// into agent_memory.sqlite (same DB as recall). Caller must hold maintainPipelineMu.
// MEMORY.md under layout.Project is optional context only for duplicate avoidance.
func runPostTurnAgentMemoryExtract(ctx context.Context, layout Layout, llm *lzmodel.LLMConfig, turn *PostTurnInput) {
	if llm == nil || turn == nil {
		slog.Debug("memory.post_turn_extract.skip", "reason", "nil_llm_or_turn")
		return
	}
	postTurnSnap := strings.TrimSpace(formatMaintainTurnSnapshot(turn))
	if len(postTurnSnap) < postTurnMaintenanceMinLogBytes() {
		slog.Debug("memory.post_turn_extract.skip", "reason", "turn_snapshot_too_small",
			"snapshot_bytes", len(postTurnSnap), "min", postTurnMaintenanceMinLogBytes())
		return
	}

	db, err := getAgentMemoryGorm(layout)
	if err != nil {
		slog.Warn("memory.post_turn_extract.db_open_failed", "err", err)
		return
	}

	timeout := time.Duration(llm.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	rulesPath := filepath.Join(layout.Project, entrypointName)
	rulesBytes, _ := os.ReadFile(rulesPath)
	prev := string(rulesBytes)
	mprev := postTurnMaintenanceMemoryPreviewBytes()
	if mprev <= 0 {
		mprev = 8000
	}
	if len(prev) > mprev {
		prev = strings.TrimRight(utf8SafePrefix(prev, mprev), "\n") + "\n…"
	}
	var contextMemories []string
	if strings.TrimSpace(prev) != "" {
		contextMemories = append(contextMemories, "Project MEMORY.md rules excerpt (avoid extracting duplicates):\n"+prev)
	}

	dialog := "Current turn snapshot (current session only):\n```\n" + postTurnSnap + "\n```\n"

	sessionID := strings.TrimSpace(turn.SessionID)
	if sessionID == "" {
		sessionID = "default"
	}
	resCtx := ""
	if cid := strings.TrimSpace(turn.CorrelationID); cid != "" {
		resCtx = "correlation_id=" + cid
	}

	extCtx := lzservice.WithIsolation(runCtx, layoutStableTenantID(layout), "default", sessionID, DefaultRootAgentMemoryAgentID)

	ref := time.Now()
	extractor := lzservice.NewExtractor(db)
	req := lzservice.ExtractRequest{
		DialogText:        dialog,
		ContextMemories:   contextMemories,
		MinConfidence:     0.7,
		DryRun:            false,
		UseDecisionEngine: false,
		LLMConfig:         llm,
		ReferenceTime:     &ref,
		ResolutionContext: resCtx,
	}

	result, err := extractor.Extract(extCtx, req)
	if err != nil {
		slog.Warn("memory.post_turn_extract.failed", "err", err)
		return
	}

	sqlitePath := agentMemorySQLitePath(layout)
	auditPayload, _ := json.Marshal(map[string]any{
		"extraction_id": result.ExtractionID,
		"status":        result.Status,
		"memories":      len(result.Memories),
		"tokens":        result.TotalTokens,
	})
	AppendMemoryAudit(layout, sqlitePath, AuditSourcePostTurnMaintain, auditPayload)

	slog.Info("memory.post_turn_extract.done",
		"path", sqlitePath,
		"model", llm.Model,
		"memories", len(result.Memories),
		"status", result.Status,
		"tokens", result.TotalTokens,
	)
}

// DefaultRootAgentMemoryAgentID matches session.DefaultRootAgentID for isolation rows.
const DefaultRootAgentMemoryAgentID = "AGENT"
