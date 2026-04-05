package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/lengzhao/oneclaw/budget"
	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/routing"
	"github.com/lengzhao/oneclaw/subagent"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/lengzhao/oneclaw/tools"
	"github.com/openai/openai-go"
)

// Engine holds conversation state and configuration for one chat session.
type Engine struct {
	Client     openai.Client
	Model      string
	System     string
	MaxTokens  int64
	MaxSteps   int
	Messages   []openai.ChatCompletionMessageParamUnion
	Registry   *tools.Registry
	CWD        string
	CanUseTool tools.CanUseTool
	SessionID  string
	// SinkRegistry resolves sinks by routing.Inbound.Source. If nil, no outbound emission.
	SinkRegistry routing.SinkRegistry
	// TranscriptPath is where SaveTranscript writes after each successful loop.RunTurn (before PostTurn / MaybeMaintain). Empty disables auto-save.
	TranscriptPath string
	// RecallState tracks memory recall surfacing across turns (phase B).
	RecallState memory.RecallState
}

// NewEngine builds an engine with sensible defaults.
func NewEngine(cwd string, reg *tools.Registry) *Engine {
	e := &Engine{
		Client:    openai.NewClient(),
		Model:     string(openai.ChatModelGPT4o),
		MaxTokens: 8192,
		MaxSteps:  32,
		System:    defaultSystem,
		Registry:  reg,
		CWD:       cwd,
		SessionID: newSessionID(),
	}
	if m := os.Getenv("ONCLAW_MODEL"); m != "" {
		e.Model = m
	}
	return e
}

const defaultSystem = `You are Oneclaw, a coding agent. Be concise. Use tools when they help answer accurately.
Working directory for file tools is the session cwd. Prefer read_file before editing.`

// SubmitUser runs one user turn (may involve multiple internal model calls).
// in.Text is the user message appended to the conversation; other fields are for routing/registry (and future use).
// Cancel ctx to abort in-flight model and tool calls.
func (e *Engine) SubmitUser(ctx context.Context, in routing.Inbound) error {
	if strings.TrimSpace(in.Text) == "" {
		return fmt.Errorf("session: empty inbound text")
	}
	if in.Source == "" {
		in.Source = routing.SourceCLI
	}

	slog.Debug("session.submit", "cwd", e.CWD, "model", e.Model, "user_chars", utf8.RuneCountInString(in.Text))
	bg := budget.FromEnv()
	tctx := toolctx.New(e.CWD, ctx)
	home, herr := os.UserHomeDir()
	memOK := herr == nil && os.Getenv("ONCLAW_DISABLE_MEMORY") != "1"
	var traceSink *loop.ToolTraceSink
	var layout memory.Layout
	var bundle memory.TurnBundle
	if memOK {
		layout = memory.DefaultLayout(e.CWD, home)
		bundle = memory.BuildTurn(layout, home, in.Text, &e.RecallState, bg.RecallBytes())
		memory.ApplyTurnBudget(&bundle, bg)
		if bundle.UpdatedRecall != nil {
			e.RecallState = *bundle.UpdatedRecall
		}
		tctx.MemoryWriteRoots = layout.WriteRoots()
	} else if herr != nil {
		slog.Warn("session.user_home", "err", herr)
	}
	var em *routing.Emitter
	if e.SinkRegistry != nil {
		sink, err := e.SinkRegistry.SinkFor(in.Source)
		if err != nil {
			return fmt.Errorf("routing: %w", err)
		}
		em = routing.NewEmitter(sink, e.SessionID, "")
	}
	system := e.System
	if memOK {
		system += bundle.SystemSuffix
	}
	cat := subagent.LoadCatalog(e.CWD)
	tctx.Subagent = &subRunner{eng: e, turnSystem: system, catalog: cat, bg: bg}

	var onToolLogged func(loop.ToolTraceEntry)
	if memOK && memory.MemoryExtractEnabled() {
		traceSink = &loop.ToolTraceSink{}
		sid, cid := e.SessionID, in.CorrelationID
		uv := memory.UserTurnPreview(in.Text)
		lay := layout
		onToolLogged = func(ent loop.ToolTraceEntry) {
			memory.AppendTurnToolLogJSONL(lay, sid, cid, uv, ent)
		}
	}

	cfg := loop.Config{
		Client:        &e.Client,
		Model:         e.Model,
		System:        system,
		MaxTokens:     e.MaxTokens,
		MaxSteps:      e.MaxSteps,
		Messages:      &e.Messages,
		Registry:      e.Registry,
		ToolContext:   tctx,
		CanUseTool:    e.CanUseTool,
		Outbound:      em,
		MemoryAgentMd: bundle.AgentMdBlock,
		MemoryRecall:  bundle.RecallBlock,
		Budget:        bg,
		ToolTrace:     traceSink,
		OnToolLogged:  onToolLogged,
	}
	err := loop.RunTurn(ctx, cfg, in)
	if err != nil {
		return err
	}
	if err := e.SaveTranscript(); err != nil {
		slog.Error("session.transcript_save", "err", err)
	}
	if memOK && memory.MemoryExtractEnabled() {
		memory.AppendTurnAssistantFinalJSONL(layout, e.SessionID, in.CorrelationID, memory.UserTurnPreview(in.Text), loop.LastAssistantDisplay(e.Messages))
	}
	if memOK {
		var tools []loop.ToolTraceEntry
		if traceSink != nil {
			tools = traceSink.Snapshot()
		}
		memory.PostTurn(layout, memory.PostTurnInput{
			SessionID:        e.SessionID,
			CorrelationID:    in.CorrelationID,
			UserText:         in.Text,
			AssistantVisible: loop.LastAssistantDisplay(e.Messages),
			Tools:            tools,
		})
		memory.MaybeMaintain(ctx, layout, &e.Client, e.Model, e.MaxTokens)
	}
	return nil
}

func newSessionID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "sess_" + hex.EncodeToString(b[:])
}

// SaveTranscript writes MarshalTranscript to TranscriptPath. No-op if TranscriptPath is empty.
func (e *Engine) SaveTranscript() error {
	return e.SaveTranscriptTo(e.TranscriptPath)
}

// SaveTranscriptTo writes the current conversation to path. No-op if path is empty.
func (e *Engine) SaveTranscriptTo(path string) error {
	if path == "" {
		return nil
	}
	b, err := e.MarshalTranscript()
	if err != nil {
		return fmt.Errorf("marshal transcript: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir transcript dir: %w", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("write transcript: %w", err)
	}
	slog.Debug("transcript saved", "path", path)
	return nil
}

// MarshalTranscript returns JSON suitable for disk persistence.
func (e *Engine) MarshalTranscript() ([]byte, error) {
	return loop.MarshalMessages(e.Messages)
}

// LoadTranscript replaces in-memory messages.
func (e *Engine) LoadTranscript(data []byte) error {
	msgs, err := loop.UnmarshalMessages(data)
	if err != nil {
		return err
	}
	e.Messages = msgs
	slog.Info("transcript loaded", "messages", len(msgs))
	return nil
}
