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

	"github.com/cloudwego/eino/schema"
	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/budget"
	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/notify"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/lengzhao/oneclaw/tools"
	"github.com/lengzhao/oneclaw/workspace"
)

// maxSidechainTranscriptMessages limits messages persisted per sidechain JSONL record (oldest dropped first).
const maxSidechainTranscriptMessages = 128

// Host carries parent-session knobs for nested loops (TS: cache-safe / tool-use shell).
type Host struct {
	Model          string
	MaxTokens      int64
	MaxSteps       int
	Registry       *tools.Registry
	CanUseTool     tools.CanUseTool
	CWD            string
	SessionID      string
	Catalog        *Catalog
	ParentSystem   string
	ParentMessages *[]*schema.Message
	// MaxInheritedMessages caps fork / inheritContext parent tail (0 → default 32).
	MaxInheritedMessages int
	// HistoryBudget trims nested transcript each step (usually same as main session).
	HistoryBudget budget.Global
	// EinoOpenAIAPIKey / EinoOpenAIBaseURL mirror main Engine for nested turns (same OpenAI-compatible backend).
	EinoOpenAIAPIKey  string
	EinoOpenAIBaseURL string
	// Notify and parent correlation for nested loop lifecycle (optional).
	Notify notify.Sink
	// AppendExec writes structured execution rows to the parent turn's execution shard (optional).
	AppendExec          func(context.Context, map[string]any)
	ParentAgentID       string
	ParentTurnID        string
	ParentCorrelationID string
	// RunTurn executes one nested user turn (required). The session package sets this from the parent Engine's TurnRunner so nested turns use the same backend as the main thread.
	RunTurn func(ctx context.Context, cfg loop.Config, in bus.InboundMessage) error
}

func (h *Host) maxInherited() int {
	if h == nil || h.MaxInheritedMessages <= 0 {
		return 32
	}
	return h.MaxInheritedMessages
}

func (h *Host) runTurn(ctx context.Context, cfg loop.Config, in bus.InboundMessage) error {
	if h == nil || h.RunTurn == nil {
		return fmt.Errorf("subagent: Host.RunTurn is required")
	}
	return h.RunTurn(ctx, cfg, in)
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

// effectiveModel returns def.Model when set, otherwise the host default.
func effectiveModel(h *Host, def Definition) string {
	if m := strings.TrimSpace(def.Model); m != "" {
		return m
	}
	if h == nil {
		return ""
	}
	return h.Model
}

func validateNestedHost(h *Host) error {
	if h == nil || h.Registry == nil || strings.TrimSpace(h.EinoOpenAIAPIKey) == "" {
		return fmt.Errorf("subagent: incomplete host")
	}
	if h.RunTurn == nil {
		return fmt.Errorf("subagent: Host.RunTurn is required")
	}
	return nil
}

func validateNestedParent(parent *toolctx.Context) error {
	if parent == nil {
		return fmt.Errorf("subagent: nil parent tool context")
	}
	if parent.MaxSubagentDepth <= 0 {
		parent.MaxSubagentDepth = 3
	}
	return nil
}

func nestingDepthLimited(parent *toolctx.Context) bool {
	return parent.SubagentDepth >= parent.MaxSubagentDepth
}

// stripMetaForNested removes run_agent / fork_context when always is true (run_agent path), or when
// parent is already nested (fork_context path). Matches prior RunAgent vs RunFork behavior.
func stripMetaForNested(parent *toolctx.Context, reg *tools.Registry, always bool) (*tools.Registry, error) {
	if !always && parent.SubagentDepth < 1 {
		return reg, nil
	}
	return WithoutMetaTools(reg, "run_agent", "fork_context")
}

// RunAgent starts an isolated sub-agent with its own transcript slice (sidechain).
func RunAgent(ctx context.Context, h *Host, parent *toolctx.Context, agentType, task string, inheritContext bool) (reply string, err error) {
	if err := validateNestedHost(h); err != nil {
		return "", err
	}
	if err := validateNestedParent(parent); err != nil {
		return "", err
	}
	if nestingDepthLimited(parent) {
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
	reg, err = stripMetaForNested(parent, reg, true)
	if err != nil {
		return "", err
	}

	child := parent.ChildContext()
	child.AgentID = strings.TrimSpace(def.AgentType)
	childRunID := newSubAgentID()
	nestedTurnID := newNestedTurnID()
	slog.Info("subagent.run_agent", "agent_id", childRunID, "agent_type", def.AgentType, "depth", child.SubagentDepth, "inherit", inheritContext)

	traceSubagent := h.Notify != nil || h.AppendExec != nil
	if traceSubagent {
		emitSubagentStart(h, ctx, "run_agent", def.AgentType, task, inheritContext, child.SubagentDepth, childRunID, nestedTurnID)
	}
	defer func() {
		if !traceSubagent {
			return
		}
		emitSubagentEnd(h, ctx, "run_agent", def.AgentType, child.SubagentDepth, childRunID, nestedTurnID, reply, err)
	}()

	msgs := make([]*schema.Message, 0)
	if inheritContext && h.ParentMessages != nil {
		msgs = append(msgs, trimInheritedParentMessages(*h.ParentMessages, h.maxInherited())...)
	}

	sys := buildSubagentSystem(h.CWD, def.SystemPrompt)
	maxSteps := h.maxStepsForDef(def)
	if maxSteps < 1 {
		maxSteps = 1
	}
	cfg := loop.Config{
		Model:             effectiveModel(h, def),
		System:            sys,
		MaxTokens:         h.MaxTokens,
		MaxSteps:          maxSteps,
		Messages:          &msgs,
		Registry:          reg,
		ToolContext:       child,
		SessionID:         h.SessionID,
		CanUseTool:        h.CanUseTool,
		OutboundText:      nil,
		MemoryAgentMd:     "",
		Budget:            h.HistoryBudget,
		EinoOpenAIAPIKey:  h.EinoOpenAIAPIKey,
		EinoOpenAIBaseURL: h.EinoOpenAIBaseURL,
		TurnMaxSteps:      maxSteps,
	}
	if err = h.runTurn(ctx, cfg, bus.InboundMessage{Content: task}); err != nil {
		return "", err
	}
	msgs = loop.ToUserVisibleMessages(msgs)
	scPath, _ := writeSidechain(child, childRunID, "run_agent", msgs)
	reply = loop.LastAssistantDisplay(msgs)
	applySidechainMerge(parent, "run_agent", childRunID, scPath, &reply)
	return reply, nil
}

// RunFork shares the parent system string and a trimmed parent message tail (TS: forkedAgent).
func RunFork(ctx context.Context, h *Host, parent *toolctx.Context, task string, maxParentMessages int) (reply string, err error) {
	if err := validateNestedHost(h); err != nil {
		return "", err
	}
	if err := validateNestedParent(parent); err != nil {
		return "", err
	}
	if nestingDepthLimited(parent) {
		return "subagent nesting depth limit reached", nil
	}
	if strings.TrimSpace(h.ParentSystem) == "" {
		return "", fmt.Errorf("subagent: fork requires parent system prompt")
	}
	if maxParentMessages <= 0 {
		maxParentMessages = h.maxInherited()
	}

	reg, err := stripMetaForNested(parent, h.Registry, false)
	if err != nil {
		return "", err
	}

	child := parent.ChildContext()
	child.ImportReadCacheFrom(parent)
	const forkAgentID = "fork_context"
	child.AgentID = forkAgentID
	childRunID := newSubAgentID()
	nestedTurnID := newNestedTurnID()
	slog.Info("subagent.fork", "agent_id", childRunID, "depth", child.SubagentDepth, "parent_tail", maxParentMessages)

	traceSubagent := h.Notify != nil || h.AppendExec != nil
	if traceSubagent {
		emitSubagentStart(h, ctx, "fork_context", forkAgentID, task, false, child.SubagentDepth, childRunID, nestedTurnID)
	}
	defer func() {
		if !traceSubagent {
			return
		}
		emitSubagentEnd(h, ctx, "fork_context", forkAgentID, child.SubagentDepth, childRunID, nestedTurnID, reply, err)
	}()

	msgs := make([]*schema.Message, 0)
	if h.ParentMessages != nil {
		msgs = append(msgs, trimInheritedParentMessages(*h.ParentMessages, maxParentMessages)...)
	}

	maxSteps := h.MaxSteps
	if maxSteps < 1 {
		maxSteps = 1
	}
	cfg := loop.Config{
		Model:             h.Model,
		System:            h.ParentSystem,
		MaxTokens:         h.MaxTokens,
		MaxSteps:          maxSteps,
		Messages:          &msgs,
		Registry:          reg,
		ToolContext:       child,
		SessionID:         h.SessionID,
		CanUseTool:        wrapConservative(h.CanUseTool),
		OutboundText:      nil,
		Budget:            h.HistoryBudget,
		EinoOpenAIAPIKey:  h.EinoOpenAIAPIKey,
		EinoOpenAIBaseURL: h.EinoOpenAIBaseURL,
		TurnMaxSteps:      maxSteps,
	}
	if err = h.runTurn(ctx, cfg, bus.InboundMessage{Content: task}); err != nil {
		return "", err
	}
	msgs = loop.ToUserVisibleMessages(msgs)
	scPath, _ := writeSidechain(child, childRunID, "fork_context", msgs)
	reply = loop.LastAssistantDisplay(msgs)
	applySidechainMerge(parent, "fork_context", childRunID, scPath, &reply)
	return reply, nil
}

func applySidechainMerge(parent *toolctx.Context, kind, agentID, scPath string, reply *string) {
	if parent == nil || reply == nil {
		return
	}
	if SidechainMergeUserAfter() {
		note := fmt.Sprintf(
			"[Sidechain merge] kind=%s agent_id=%s\nTranscript file: %s\nThe preceding tool result carries the sub-agent's final reply.",
			kind, agentID, scPath)
		parent.DeferUserMessageAfterToolBatch(schema.UserMessage(note))
		return
	}
	if SidechainMergeToolSuffix() && scPath != "" {
		*reply = *reply + "\n\n---\n[sidechain] transcript file: " + scPath
	}
}

func buildSubagentSystem(cwd, role string) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(role))
	b.WriteString("\n\n# Environment\nWorking directory: ")
	b.WriteString(cwd)
	b.WriteString("\nYou are running as a nested sub-agent; only your final assistant text is returned upstream.")
	return b.String()
}

func trimMessages(src []*schema.Message, max int) []*schema.Message {
	if max <= 0 || len(src) <= max {
		out := make([]*schema.Message, len(src))
		copy(out, src)
		return out
	}
	out := make([]*schema.Message, max)
	copy(out, src[len(src)-max:])
	return out
}

// trimInheritedParentMessages takes the last max parent messages, then removes segments that are
// invalid for chat.completions: orphaned leading tool messages, a trailing assistant with
// unresolved tool_calls (e.g. the in-flight run_agent message), or a partial tool batch after such
// an assistant.
func trimInheritedParentMessages(src []*schema.Message, max int) []*schema.Message {
	out := trimMessages(src, max)
	before := len(out)
	out = dropLeadingOrphanToolMessages(out)
	for {
		n := len(out)
		out = dropTrailingIncompleteAssistantToolBatch(out)
		if len(out) == n {
			break
		}
	}
	if len(out) < before {
		slog.Debug("subagent.inherited_messages_sanitized", "before", before, "after", len(out))
	}
	return out
}

func dropLeadingOrphanToolMessages(msgs []*schema.Message) []*schema.Message {
	i := 0
	for i < len(msgs) && msgs[i] != nil && msgs[i].Role == schema.Tool {
		i++
	}
	return msgs[i:]
}

func dropTrailingIncompleteAssistantToolBatch(msgs []*schema.Message) []*schema.Message {
	if len(msgs) == 0 {
		return msgs
	}
	last := msgs[len(msgs)-1]
	if last != nil && last.Role == schema.Assistant && len(last.ToolCalls) > 0 {
		return msgs[:len(msgs)-1]
	}
	if last == nil || last.Role != schema.Tool {
		return msgs
	}
	toolStart := len(msgs) - 1
	for toolStart >= 0 && msgs[toolStart] != nil && msgs[toolStart].Role == schema.Tool {
		toolStart--
	}
	if toolStart < 0 {
		return nil
	}
	a := msgs[toolStart]
	if a == nil || a.Role != schema.Assistant || len(a.ToolCalls) == 0 {
		return msgs[:toolStart+1]
	}
	want := len(a.ToolCalls)
	got := len(msgs) - 1 - toolStart
	if got != want {
		return msgs[:toolStart]
	}
	needed := make(map[string]struct{}, want)
	for _, tc := range a.ToolCalls {
		if tc.ID != "" {
			needed[tc.ID] = struct{}{}
		}
	}
	for i := toolStart + 1; i < len(msgs); i++ {
		tm := msgs[i]
		if tm == nil || tm.Role != schema.Tool {
			return msgs[:toolStart]
		}
		if _, ok := needed[tm.ToolCallID]; !ok {
			return msgs[:toolStart]
		}
		delete(needed, tm.ToolCallID)
	}
	if len(needed) > 0 {
		return msgs[:toolStart]
	}
	return msgs
}

func wrapConservative(parent tools.CanUseTool) tools.CanUseTool {
	return func(ctx context.Context, name string, input json.RawMessage, tctx *toolctx.Context) (bool, string) {
		if name == "exec" {
			return false, "exec disabled in fork_context (non-interactive sub-context)"
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

func newNestedTurnID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "nturn_" + hex.EncodeToString(b[:])
}

func writeSidechain(tctx *toolctx.Context, agentID, kind string, msgs []*schema.Message) (string, error) {
	if tctx == nil || strings.TrimSpace(tctx.CWD) == "" {
		return "", nil
	}
	dir := workspace.JoinSessionWorkspaceWithInstruction(tctx.CWD, tctx.InstructionRoot, tctx.WorkspaceFlat, "sidechain")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("%s_%s.jsonl", sanitizeID(tctx.SessionID), agentID)
	path := filepath.Join(dir, name)
	beforeN := len(msgs)
	msgs = trimMessages(msgs, maxSidechainTranscriptMessages)
	if len(msgs) < beforeN {
		slog.Debug("subagent.sidechain_transcript_trimmed", "before", beforeN, "after", len(msgs), "cap", maxSidechainTranscriptMessages)
	}
	raw, err := loop.MarshalMessages(msgs)
	if err != nil {
		return "", err
	}
	rec := struct {
		SessionID  string          `json:"session_id"`
		AgentID    string          `json:"agent_id"`
		Kind       string          `json:"kind"`
		Transcript json.RawMessage `json:"transcript"`
	}{
		SessionID:  tctx.SessionID,
		AgentID:    agentID,
		Kind:       kind,
		Transcript: raw,
	}
	line, err := json.Marshal(&rec)
	if err != nil {
		return "", err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err = f.Write(append(line, '\n')); err != nil {
		return "", err
	}
	return path, nil
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
