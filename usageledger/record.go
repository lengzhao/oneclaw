package usageledger

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/rtopts"
)

var recordMu sync.Mutex

func disabled() bool {
	return rtopts.Current().DisableUsageLedger
}

func estimateCostEnabled() bool {
	return rtopts.Current().UsageEstimateCost
}

func usageRoot(cwd string, workspaceFlat bool, instructionRoot string) string {
	if strings.TrimSpace(instructionRoot) != "" {
		return filepath.Join(filepath.Clean(instructionRoot), "usage")
	}
	_ = workspaceFlat
	return filepath.Join(cwd, "usage")
}

func hashUserFileName(userKey string) string {
	sum := sha256.Sum256([]byte(userKey))
	return hex.EncodeToString(sum[:8]) + ".json"
}

// Rollup is persisted for daily and per-user totals.
type Rollup struct {
	UserKey            string  `json:"user_key,omitempty"`
	Date               string  `json:"date,omitempty"`
	Interactions       int64   `json:"interactions"`
	PromptTokens       int64   `json:"prompt_tokens"`
	CompletionTokens   int64   `json:"completion_tokens"`
	CostUSD            float64 `json:"cost_usd,omitempty"`
	LastUpdatedRFC3339 string  `json:"last_updated"`
}

// InteractionLine is one append-only JSONL row per model completion.
// Token counts duplicate usage.* for easy grep; usage is the API usage object as returned (RawJSON).
type InteractionLine struct {
	TimeRFC3339      string          `json:"time"`
	SessionID        string          `json:"session_id"`
	CorrelationID    string          `json:"correlation_id"`
	UserKey          string          `json:"user_key"`
	Source           string          `json:"source"` // clawbridge client id (bus ClientID)
	Model            string          `json:"model"`
	Step             int             `json:"step"`
	SubagentDepth    int             `json:"subagent_depth"`
	PromptTokens     int64           `json:"prompt_tokens"`
	CompletionTokens int64           `json:"completion_tokens"`
	TotalTokens      int64           `json:"total_tokens"`
	Usage            json.RawMessage `json:"usage,omitempty"`
	CostUSD          *float64        `json:"cost_usd,omitempty"`
	CostSource       string          `json:"cost_source,omitempty"`
}

// RecordParams is one successful chat completion from loop.RunTurn.
type RecordParams struct {
	CWD              string
	SessionID        string
	Model            string
	Step             int
	SubagentDepth    int
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64
	// UsageJSON is completion.Usage.RawJSON() from the provider (source of truth for tokens).
	UsageJSON string
	Inbound   bus.InboundMessage
	// WorkspaceFlat matches toolctx.Context.WorkspaceFlat (flat session runtime vs repo cwd layout).
	WorkspaceFlat bool
	// InstructionRoot matches toolctx when IM workspace split is enabled (usage under <root>/usage).
	InstructionRoot string
}

// MaybeRecord appends an interaction line and updates daily + per-user rollups under the resolved session usage root.
func MaybeRecord(p RecordParams) {
	if disabled() || strings.TrimSpace(p.CWD) == "" {
		return
	}
	prompt := p.PromptTokens
	comp := p.CompletionTokens
	if prompt == 0 && comp == 0 {
		return
	}
	usageTrim := strings.TrimSpace(p.UsageJSON)
	var usageRM json.RawMessage
	if usageTrim != "" && usageTrim != "null" {
		usageRM = json.RawMessage(usageTrim)
	}

	var costPtr *float64
	var costSource string
	var rollupCost float64
	if c, ok := ParseCostUSDFromUsageJSON(usageTrim); ok {
		costPtr = &c
		costSource = "usage"
		rollupCost = c
	} else if estimateCostEnabled() {
		est := EstimateCostUSDFromTokens(p.Model, prompt, comp)
		costPtr = &est
		costSource = "estimated"
		rollupCost = est
	}

	userKey := UserInteractionKey(p.Inbound)
	line := InteractionLine{
		TimeRFC3339:      time.Now().UTC().Format(time.RFC3339Nano),
		SessionID:        p.SessionID,
		CorrelationID:    strings.TrimSpace(p.Inbound.MessageID),
		UserKey:          userKey,
		Source:           strings.TrimSpace(p.Inbound.ClientID),
		Model:            p.Model,
		Step:             p.Step,
		SubagentDepth:    p.SubagentDepth,
		PromptTokens:     prompt,
		CompletionTokens: comp,
		TotalTokens:      p.TotalTokens,
		Usage:            usageRM,
		CostUSD:          costPtr,
		CostSource:       costSource,
	}
	recordMu.Lock()
	defer recordMu.Unlock()
	root := usageRoot(p.CWD, p.WorkspaceFlat, p.InstructionRoot)
	if err := os.MkdirAll(root, 0o750); err != nil {
		slog.Warn("usageledger.mkdir", "path", root, "err", err)
		return
	}
	if err := os.MkdirAll(filepath.Join(root, "daily"), 0o750); err != nil {
		slog.Warn("usageledger.mkdir", "path", "daily", "err", err)
		return
	}
	if err := os.MkdirAll(filepath.Join(root, "users"), 0o750); err != nil {
		slog.Warn("usageledger.mkdir", "path", "users", "err", err)
		return
	}
	jl, err := json.Marshal(line)
	if err != nil {
		return
	}
	jlPath := filepath.Join(root, "interactions.jsonl")
	f, err := os.OpenFile(jlPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		slog.Warn("usageledger.append", "path", jlPath, "err", err)
		return
	}
	if _, err := f.Write(append(jl, '\n')); err != nil {
		slog.Warn("usageledger.write", "path", jlPath, "err", err)
	}
	_ = f.Close()

	day := time.Now().UTC().Format("2006-01-02")
	dailyPath := filepath.Join(root, "daily", day+".json")
	if err := mergeRollupFile(dailyPath, Rollup{
		Date:             day,
		Interactions:     1,
		PromptTokens:     prompt,
		CompletionTokens: comp,
		CostUSD:          rollupCost,
	}); err != nil {
		slog.Warn("usageledger.daily", "path", dailyPath, "err", err)
	}

	userPath := filepath.Join(root, "users", hashUserFileName(userKey))
	if err := mergeRollupFile(userPath, Rollup{
		UserKey:          userKey,
		Interactions:     1,
		PromptTokens:     prompt,
		CompletionTokens: comp,
		CostUSD:          rollupCost,
	}); err != nil {
		slog.Warn("usageledger.user", "path", userPath, "err", err)
	}
}

func mergeRollupFile(path string, delta Rollup) error {
	var cur Rollup
	if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
		_ = json.Unmarshal(b, &cur)
	}
	cur.Interactions += delta.Interactions
	cur.PromptTokens += delta.PromptTokens
	cur.CompletionTokens += delta.CompletionTokens
	cur.CostUSD += delta.CostUSD
	if delta.Date != "" {
		cur.Date = delta.Date
	}
	if delta.UserKey != "" {
		cur.UserKey = delta.UserKey
	}
	cur.LastUpdatedRFC3339 = time.Now().UTC().Format(time.RFC3339Nano)
	return atomicWriteJSON(path, &cur)
}

func atomicWriteJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
