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
	"time"
	"unicode/utf8"

	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/routing"
	"github.com/lengzhao/oneclaw/tools"
	"github.com/openai/openai-go"
)

// Engine holds conversation state and configuration for one chat session.
type Engine struct {
	Client openai.Client
	Model  string
	// System is optional text appended at the end of the main-thread system prompt (see prompts/templates/main_thread_system.tmpl).
	// Identity and section order live in that template; leave empty if you do not need extra constraints.
	System    string
	MaxTokens int64
	MaxSteps  int
	Messages  []openai.ChatCompletionMessageParamUnion
	// Transcript is persisted user + assistant pairs: user is appended before the model loop (and
	// saved when TranscriptPath is set); assistant is appended after a successful RunTurn.
	Transcript []openai.ChatCompletionMessageParamUnion
	Registry   *tools.Registry
	CWD        string
	CanUseTool tools.CanUseTool
	SessionID  string
	// SinkRegistry resolves sinks by routing.Inbound.Source. If nil, no outbound emission.
	SinkRegistry routing.SinkRegistry
	// SinkFactory builds a per-turn Sink (e.g. bind IM thread). If nil, only SinkRegistry is used.
	// When non-nil, NewSink runs first; ErrUseRegistrySink falls back to SinkRegistry.
	SinkFactory routing.SinkFactory
	// TranscriptPath is where SaveTranscript writes after each successful loop.RunTurn (before PostTurn / MaybePostTurnMaintain). Empty disables auto-save.
	TranscriptPath string
	// WorkingTranscriptPath persists Messages (tool rounds + semantic compact); empty when transcript disabled.
	WorkingTranscriptPath string
	// RecallState tracks memory recall surfacing across turns (phase B).
	RecallState memory.RecallState
	// ChatTransport overrides ONCLAW_CHAT_TRANSPORT when non-empty (from unified config).
	ChatTransport string
}

// NewEngine builds an engine with sensible defaults.
func NewEngine(cwd string, reg *tools.Registry) *Engine {
	e := &Engine{
		Client:    openai.NewClient(),
		Model:     string(openai.ChatModelGPT4o),
		MaxTokens: 8192,
		MaxSteps:  32,
		System:    "",
		Registry:  reg,
		CWD:       cwd,
		SessionID: newSessionID(),
	}
	if m := os.Getenv("ONCLAW_MODEL"); m != "" {
		e.Model = m
	}
	return e
}

// SubmitUser runs one user turn (may involve multiple internal model calls).
// in.Text is the primary user line; Attachments add leading user messages. Routing fields feed Sink 选择与可选 <inbound-context>。
// Cancel ctx to abort in-flight model and tool calls.
func (e *Engine) SubmitUser(ctx context.Context, in routing.Inbound) error {
	if err := e.prepareInboundAttachments(&in); err != nil {
		return err
	}
	if strings.TrimSpace(in.Text) == "" && len(in.Attachments) == 0 {
		return fmt.Errorf("session: empty inbound text")
	}

	if reply, ok := e.trySlashLocalTurn(in); ok {
		return e.submitLocalSlashTurn(ctx, in, reply)
	}

	preview := combinedInboundPreview(in)
	slog.Debug("session.submit", "cwd", e.CWD, "model", e.Model, "preview_chars", utf8.RuneCountInString(preview))

	prep, err := e.prepareSharedTurn(ctx, in, preview, true)
	if err != nil {
		return err
	}
	bg := prep.bg
	memOK := prep.memOK
	layout := prep.layout
	bundle := prep.bundle
	em := prep.em
	system := prep.system

	var traceSink *loop.ToolTraceSink
	uv := memory.UserTurnPreview(preview)
	var onToolLogged func(loop.ToolTraceEntry)
	if memOK && memory.MemoryExtractEnabled() {
		traceSink = &loop.ToolTraceSink{}
		sid, cid := e.SessionID, in.CorrelationID
		lay := layout
		onToolLogged = func(ent loop.ToolTraceEntry) {
			memory.AppendTurnToolLogJSONL(lay, sid, cid, uv, ent)
		}
	}

	userLine := ModelUserLine(in.Text, len(in.Attachments) > 0)
	cfg := loop.Config{
		Client:                &e.Client,
		Model:                 e.Model,
		System:                system,
		MaxTokens:             e.MaxTokens,
		MaxSteps:              e.MaxSteps,
		Messages:              &e.Messages,
		Registry:              e.Registry,
		ToolContext:           prep.tctx,
		CanUseTool:            e.CanUseTool,
		Outbound:              em,
		MemoryAgentMd:         bundle.AgentMdBlock,
		MemoryRecall:          bundle.RecallBlock,
		InboundMeta:           InboundMetaForModel(in),
		InboundAttachmentMsgs: FormatInboundAttachmentMessages(in.Attachments),
		UserLine:              userLine,
		Budget:                bg,
		ChatTransport:         e.ChatTransport,
		ToolTrace:             traceSink,
		OnToolLogged:          onToolLogged,
		SlimTranscript: func(assistantText string) {
			e.Transcript = append(e.Transcript, openai.AssistantMessage(assistantText))
		},
	}

	// Claude Code–style: record user turn before query loop so crash/interrupt still leaves the request on disk.
	e.Transcript = append(e.Transcript, openai.UserMessage(SlimTranscriptUserLine(in)))
	if err := e.SaveTranscript(); err != nil {
		slog.Error("session.transcript_save_user", "err", err)
	}
	if err := e.SaveWorkingTranscript(); err != nil {
		slog.Error("session.working_transcript_save_user", "err", err)
	}

	err = loop.RunTurn(ctx, cfg, in)
	if err != nil {
		return err
	}
	if err := e.SaveTranscript(); err != nil {
		slog.Error("session.transcript_save", "err", err)
	}
	if err := e.SaveWorkingTranscript(); err != nil {
		slog.Error("session.working_transcript_save", "err", err)
	}
	e.appendDialogHistoryIfComplete()
	if memOK && memory.MemoryExtractEnabled() {
		memory.AppendTurnAssistantFinalJSONL(layout, e.SessionID, in.CorrelationID, uv, loop.LastAssistantDisplay(e.Messages))
	}
	if memOK {
		var tools []loop.ToolTraceEntry
		if traceSink != nil {
			tools = traceSink.Snapshot()
		}
		memory.PostTurn(layout, memory.PostTurnInput{
			SessionID:        e.SessionID,
			CorrelationID:    in.CorrelationID,
			UserText:         preview,
			AssistantVisible: loop.LastAssistantDisplay(e.Messages),
			Tools:            tools,
		})
		memory.MaybePostTurnMaintain(ctx, layout, &e.Client, e.Model, e.MaxTokens, &memory.PostTurnInput{
			SessionID:        e.SessionID,
			CorrelationID:    in.CorrelationID,
			UserText:         preview,
			AssistantVisible: loop.LastAssistantDisplay(e.Messages),
			Tools:            tools,
		})
	}
	return nil
}

// SendMessage delivers text and/or attachments to the Sink for in.Source without running the model
// (proactive outbound, similar in spirit to picoclaw Channel.Send + optional media).
// in.Source must match a registered channel instance id; routing fields (SessionKey, RawRef, …) are passed to SinkFactory like SubmitUser.
func (e *Engine) SendMessage(ctx context.Context, in routing.Inbound) error {
	if strings.TrimSpace(in.Source) == "" {
		return fmt.Errorf("session: SendMessage requires Inbound.Source")
	}
	inCopy := in
	if len(in.Attachments) > 0 {
		inCopy.Attachments = append([]routing.Attachment(nil), in.Attachments...)
	}
	if err := e.prepareInboundAttachments(&inCopy); err != nil {
		return err
	}
	if strings.TrimSpace(inCopy.Text) == "" && len(inCopy.Attachments) == 0 {
		return fmt.Errorf("session: SendMessage: empty text and attachments")
	}
	sink, err := routing.ResolveTurnSink(ctx, e.SinkRegistry, e.SinkFactory, inCopy)
	if err != nil {
		return err
	}
	if sink == nil {
		return fmt.Errorf("session: no sink for source %q", inCopy.Source)
	}
	jobID := strings.TrimSpace(inCopy.CorrelationID)
	em := routing.NewEmitter(sink, e.SessionID, jobID)
	return em.TextWithAttachments(ctx, strings.TrimSpace(inCopy.Text), inCopy.Attachments)
}

func (e *Engine) submitLocalSlashTurn(ctx context.Context, in routing.Inbound, reply string) error {
	slog.Debug("session.submit_local_slash", "cwd", e.CWD)
	preview := combinedInboundPreview(in)
	prep, err := e.prepareSharedTurn(ctx, in, preview, false)
	if err != nil {
		return err
	}
	bundle := prep.bundle

	meta := InboundMetaForModel(in)
	attMsgs := FormatInboundAttachmentMessages(in.Attachments)
	userLine := ModelUserLine(in.Text, len(in.Attachments) > 0)
	loop.AppendTurnUserMessages(&e.Messages, bundle.AgentMdBlock, bundle.RecallBlock, meta, attMsgs, userLine)
	e.Messages = append(e.Messages, openai.AssistantMessage(reply))

	e.Transcript = append(e.Transcript, openai.UserMessage(SlimTranscriptUserLine(in)))
	e.Transcript = append(e.Transcript, openai.AssistantMessage(reply))
	if err := e.SaveTranscript(); err != nil {
		slog.Error("session.transcript_save_local_slash", "err", err)
	}
	if err := e.SaveWorkingTranscript(); err != nil {
		slog.Error("session.working_transcript_save_local_slash", "err", err)
	}
	e.appendDialogHistoryIfComplete()
	if prep.em != nil {
		emitCtx := context.Background()
		if err := prep.em.Text(ctx, reply); err != nil {
			slog.Warn("session.local_slash.emit_text", "err", err)
		}
		if err := prep.em.Done(emitCtx, true, ""); err != nil {
			slog.Warn("session.local_slash.emit_done", "err", err)
		}
	}
	// No memory.PostTurn / MaybePostTurnMaintain: local slash is an intentional bypass (see docs/memory-maintain-dual-entry-design.md §2.4).
	return nil
}

func (e *Engine) prepareInboundAttachments(in *routing.Inbound) error {
	if err := ValidateInboundMediaPaths(e.CWD, in.Attachments); err != nil {
		return err
	}
	in.Attachments = routing.NormalizeAttachments(in.Attachments)
	return PersistInlineAttachmentFiles(e.CWD, &in.Attachments)
}

func newSessionID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "sess_" + hex.EncodeToString(b[:])
}

func (e *Engine) appendDialogHistoryIfComplete() {
	if e.TranscriptPath == "" {
		return
	}
	n := len(e.Transcript)
	if n < 2 {
		return
	}
	u := e.Transcript[n-2]
	a := e.Transcript[n-1]
	if u.OfUser == nil || a.OfAssistant == nil {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		slog.Warn("session.dialog_history.home", "err", err)
		return
	}
	layout := memory.DefaultLayout(e.CWD, home)
	date := time.Now().Format("2006-01-02")
	if err := memory.AppendDialogHistoryPair(layout, date, u, a); err != nil {
		slog.Warn("session.dialog_history.append", "err", err)
	}
}

// SaveTranscript writes MarshalTranscript to TranscriptPath. No-op if TranscriptPath is empty.
func (e *Engine) SaveTranscript() error {
	return e.SaveTranscriptTo(e.TranscriptPath)
}

// SaveWorkingTranscript writes Messages to WorkingTranscriptPath. No-op if path is empty.
func (e *Engine) SaveWorkingTranscript() error {
	return e.SaveWorkingTranscriptTo(e.WorkingTranscriptPath)
}

// SaveWorkingTranscriptTo writes the current model message list to path. No-op if path is empty.
func (e *Engine) SaveWorkingTranscriptTo(path string) error {
	if path == "" {
		return nil
	}
	b, err := loop.MarshalMessages(e.Messages)
	if err != nil {
		return fmt.Errorf("marshal working transcript: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir working transcript dir: %w", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("write working transcript: %w", err)
	}
	slog.Debug("working transcript saved", "path", path, "messages", len(e.Messages))
	return nil
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
	return loop.MarshalMessages(e.Transcript)
}

// LoadTranscript restores Transcript and seeds Messages from it (same as a fresh session after restart).
func (e *Engine) LoadTranscript(data []byte) error {
	msgs, err := loop.UnmarshalMessages(data)
	if err != nil {
		return err
	}
	e.Transcript = msgs
	e.Messages = append([]openai.ChatCompletionMessageParamUnion(nil), msgs...)
	slog.Info("transcript loaded", "messages", len(msgs))
	return nil
}

// LoadWorkingTranscript replaces Messages only (Transcript unchanged). Used after LoadTranscript to restore compacted context.
func (e *Engine) LoadWorkingTranscript(data []byte) error {
	msgs, err := loop.UnmarshalMessages(data)
	if err != nil {
		return err
	}
	e.Messages = msgs
	slog.Info("working transcript loaded", "messages", len(msgs))
	return nil
}
