package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	lzmodel "github.com/lengzhao/memory/model"
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

// CloseAgentMemoryDBCache closes any cached agent_memory.sqlite GORM connections (e.g. tests swap temp dirs).
func CloseAgentMemoryDBCache() {
	agentMemDBCache.mu.Lock()
	defer agentMemDBCache.mu.Unlock()
	for _, db := range agentMemDBCache.m {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
	clear(agentMemDBCache.m)
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
	skillTrig := PostTurnSkillMaintainTrigger(turn)
	if len(postTurnSnap) < postTurnMaintenanceMinLogBytes() && !skillTrig {
		slog.Debug("memory.post_turn_extract.skip", "reason", "turn_snapshot_too_small",
			"snapshot_bytes", len(postTurnSnap), "min", postTurnMaintenanceMinLogBytes())
		return
	}

	mprev := postTurnMaintenanceMemoryPreviewBytes()
	if mprev <= 0 {
		mprev = 8000
	}
	prev := projectMemoryRulesExcerpt(layout, mprev)
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

	_ = runAgentMemoryExtract(ctx, layout, llm, agentMemoryExtractParams{
		kind:                  agentMemoryExtractPostTurn,
		isolateSessionID:      sessionID,
		dialogText:            dialog,
		contextMemories:       contextMemories,
		resolutionContext:     resCtx,
		auditSource:           AuditSourcePostTurnMaintain,
		skillAugmentedExtract: skillTrig,
	})
}

// DefaultRootAgentMemoryAgentID matches session.DefaultRootAgentID for isolation rows.
const DefaultRootAgentMemoryAgentID = "AGENT"
