package subagent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/lengzhao/oneclaw/budget"
	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/routing"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/lengzhao/oneclaw/tools"
	"github.com/openai/openai-go"
)

// Host carries parent-session knobs for nested loops (TS: cache-safe / tool-use shell).
type Host struct {
	Client         *openai.Client
	Model          string
	MaxTokens      int64
	MaxSteps       int
	Registry       *tools.Registry
	CanUseTool     tools.CanUseTool
	CWD            string
	SessionID      string
	Catalog        *Catalog
	ParentSystem   string
	ParentMessages *[]openai.ChatCompletionMessageParamUnion
	// MaxInheritedMessages caps fork / inheritContext parent tail (0 → default 32).
	MaxInheritedMessages int
	// HistoryBudget trims nested transcript each step (usually same as main session).
	HistoryBudget budget.Global
}

func (h *Host) maxInherited() int {
	if h == nil || h.MaxInheritedMessages <= 0 {
		return 32
	}
	return h.MaxInheritedMessages
}

func (h *Host) maxStepsForDef(def Definition) int {
	if def.MaxTurns > 0 {
		return def.MaxTurns
	}
	if h == nil || h.MaxSteps <= 0 {
		return 32
	}
	return h.MaxSteps
}

// RunAgent starts an isolated sub-agent with its own transcript slice (sidechain).
func RunAgent(ctx context.Context, h *Host, parent *toolctx.Context, agentType, task string, inheritContext bool) (string, error) {
	if h == nil || h.Client == nil || h.Registry == nil {
		return "", fmt.Errorf("subagent: incomplete host")
	}
	if parent == nil {
		return "", fmt.Errorf("subagent: nil parent tool context")
	}
	if parent.MaxSubagentDepth <= 0 {
		parent.MaxSubagentDepth = 3
	}
	if parent.SubagentDepth >= parent.MaxSubagentDepth {
		return "subagent nesting depth limit reached", nil
	}
	def, ok := h.Catalog.Get(strings.TrimSpace(agentType))
	if !ok {
		return "", fmt.Errorf("subagent: unknown agent_type %q", agentType)
	}

	reg, err := FilterRegistry(h.Registry, def.Tools)
	if err != nil {
		return "", err
	}
	if parent.SubagentDepth >= 1 {
		reg, err = WithoutMetaTools(reg, "run_agent", "fork_context")
		if err != nil {
			return "", err
		}
	}

	child := parent.ChildContext()
	agentID := newSubAgentID()
	slog.Info("subagent.run_agent", "agent_id", agentID, "agent_type", def.AgentType, "depth", child.SubagentDepth, "inherit", inheritContext)

	msgs := make([]openai.ChatCompletionMessageParamUnion, 0)
	if inheritContext && h.ParentMessages != nil {
		msgs = append(msgs, trimMessages(*h.ParentMessages, h.maxInherited())...)
	}

	sys := buildSubagentSystem(h.CWD, def.SystemPrompt)
	cfg := loop.Config{
		Client:        h.Client,
		Model:         h.Model,
		System:        sys,
		MaxTokens:     h.MaxTokens,
		MaxSteps:      h.maxStepsForDef(def),
		Messages:      &msgs,
		Registry:      reg,
		ToolContext:   child,
		CanUseTool:    h.CanUseTool,
		Outbound:      nil,
		MemoryAgentMd: "",
		MemoryRecall:  "",
		Budget:        h.HistoryBudget,
	}
	if err := loop.RunTurn(ctx, cfg, routing.Inbound{Text: task}); err != nil {
		return "", err
	}
	_ = writeSidechain(h.CWD, h.SessionID, agentID, "run_agent", msgs)
	return loop.LastAssistantDisplay(msgs), nil
}

// RunFork shares the parent system string and a trimmed parent message tail (TS: forkedAgent).
func RunFork(ctx context.Context, h *Host, parent *toolctx.Context, task string, maxParentMessages int) (string, error) {
	if h == nil || h.Client == nil || h.Registry == nil {
		return "", fmt.Errorf("subagent: incomplete host")
	}
	if parent == nil {
		return "", fmt.Errorf("subagent: nil parent tool context")
	}
	if parent.MaxSubagentDepth <= 0 {
		parent.MaxSubagentDepth = 3
	}
	if parent.SubagentDepth >= parent.MaxSubagentDepth {
		return "subagent nesting depth limit reached", nil
	}
	if strings.TrimSpace(h.ParentSystem) == "" {
		return "", fmt.Errorf("subagent: fork requires parent system prompt")
	}
	if maxParentMessages <= 0 {
		maxParentMessages = h.maxInherited()
	}

	reg := h.Registry
	var err error
	if parent.SubagentDepth >= 1 {
		reg, err = WithoutMetaTools(reg, "run_agent", "fork_context")
		if err != nil {
			return "", err
		}
	}

	child := parent.ChildContext()
	child.ImportReadCacheFrom(parent)
	agentID := newSubAgentID()
	slog.Info("subagent.fork", "agent_id", agentID, "depth", child.SubagentDepth, "parent_tail", maxParentMessages)

	msgs := make([]openai.ChatCompletionMessageParamUnion, 0)
	if h.ParentMessages != nil {
		msgs = append(msgs, trimMessages(*h.ParentMessages, maxParentMessages)...)
	}

	cfg := loop.Config{
		Client:      h.Client,
		Model:       h.Model,
		System:      h.ParentSystem,
		MaxTokens:   h.MaxTokens,
		MaxSteps:    h.MaxSteps,
		Messages:    &msgs,
		Registry:    reg,
		ToolContext: child,
		CanUseTool:  wrapConservative(h.CanUseTool),
		Outbound:    nil,
		Budget:      h.HistoryBudget,
	}
	if err := loop.RunTurn(ctx, cfg, routing.Inbound{Text: task}); err != nil {
		return "", err
	}
	_ = writeSidechain(h.CWD, h.SessionID, agentID, "fork_context", msgs)
	return loop.LastAssistantDisplay(msgs), nil
}

func buildSubagentSystem(cwd, role string) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(role))
	b.WriteString("\n\n# Environment\nWorking directory: ")
	b.WriteString(cwd)
	b.WriteString("\nYou are running as a nested sub-agent; only your final assistant text is returned upstream.")
	return b.String()
}

func trimMessages(src []openai.ChatCompletionMessageParamUnion, max int) []openai.ChatCompletionMessageParamUnion {
	if max <= 0 || len(src) <= max {
		out := make([]openai.ChatCompletionMessageParamUnion, len(src))
		copy(out, src)
		return out
	}
	out := make([]openai.ChatCompletionMessageParamUnion, max)
	copy(out, src[len(src)-max:])
	return out
}

func wrapConservative(parent tools.CanUseTool) tools.CanUseTool {
	return func(ctx context.Context, name string, input json.RawMessage, tctx *toolctx.Context) (bool, string) {
		if name == "bash" {
			return false, "bash disabled in fork_context (non-interactive sub-context)"
		}
		if parent != nil {
			return parent(ctx, name, input, tctx)
		}
		return true, ""
	}
}

func newSubAgentID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return "sub_" + hex.EncodeToString(b[:])
}

func writeSidechain(cwd, sessionID, agentID, kind string, msgs []openai.ChatCompletionMessageParamUnion) error {
	if cwd == "" {
		return nil
	}
	dir := filepath.Join(cwd, ".oneclaw", "sidechain")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	name := fmt.Sprintf("%s_%s.jsonl", sanitizeID(sessionID), agentID)
	path := filepath.Join(dir, name)
	raw, err := loop.MarshalMessages(msgs)
	if err != nil {
		return err
	}
	rec := struct {
		SessionID  string          `json:"session_id"`
		AgentID    string          `json:"agent_id"`
		Kind       string          `json:"kind"`
		Transcript json.RawMessage `json:"transcript"`
	}{
		SessionID:  sessionID,
		AgentID:    agentID,
		Kind:       kind,
		Transcript: raw,
	}
	line, err := json.Marshal(&rec)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(line, '\n'))
	return err
}

func sanitizeID(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "nosess"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}
