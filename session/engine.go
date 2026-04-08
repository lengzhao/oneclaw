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

	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/notify"
	"github.com/lengzhao/oneclaw/tools"
	"github.com/openai/openai-go"
)

// DefaultRootAgentID is the default notify / audit id for the root Engine created by NewEngine.
const DefaultRootAgentID = "AGENT"

// DefaultMainAgentID is an alias of DefaultRootAgentID for older call sites.
const DefaultMainAgentID = DefaultRootAgentID

// Engine holds conversation state and configuration for one chat session.
type Engine struct {
	Client openai.Client
	Model  string
	// System is optional text appended at the end of the main-thread system prompt (see prompts/templates/main_thread_system.tmpl).
	// Identity and section order live in that template; leave empty if you do not need extra constraints.
	System    string
	MaxTokens int64
	MaxSteps  int
	// Messages is the live API history. After each successful RunTurn it is collapsed to
	// user-visible rows only (loop.ToUserVisibleMessages): injections and tool rounds are dropped
	// to save tokens; facts can be re-fetched with tools or memory recall.
	Messages []openai.ChatCompletionMessageParamUnion
	// Transcript is the slim audit log (real user lines + final assistant text per turn), same
	// visibility rules as Messages; agentMd / inbound / tool rows are not persisted here.
	Transcript []openai.ChatCompletionMessageParamUnion
	Registry   *tools.Registry
	CWD        string
	CanUseTool tools.CanUseTool
	SessionID string
	// RootAgentID is the stable id of this Engine (main thread only). It is copied into toolctx.Context.AgentID each turn; subagents use their own ctx.AgentID. Empty means DefaultRootAgentID in EffectiveRootAgentID.
	RootAgentID string
	// Notify is a fan-out list (notify.Multi) of lifecycle sinks; default empty no-op.
	// Register handlers with RegisterNotify(sink) or assign notify.Multi{…} directly.
	Notify notify.Multi
	// PublishOutbound sends assistant / proactive messages to IM drivers via clawbridge. If nil, outbound is skipped.
	PublishOutbound func(ctx context.Context, msg *bus.OutboundMessage) error
	// TranscriptPath is where SaveTranscript writes after each successful loop.RunTurn (before PostTurn; post-turn maintain runs asynchronously). Empty disables auto-save.
	TranscriptPath string
	// WorkingTranscriptPath persists Messages (already user-visible in memory after each turn).
	WorkingTranscriptPath string
	// WorkingTranscriptMaxMessages is the max tail messages to persist; 0 means default 30; negative means unlimited.
	WorkingTranscriptMaxMessages int
	// RecallState tracks memory recall surfacing across turns (phase B).
	RecallState memory.RecallState
	// ChatTransport overrides default transport when non-empty (from unified config).
	ChatTransport string
	// MCPSystemNote is optional; non-empty injects the MCP section in the main-thread system prompt.
	MCPSystemNote string
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
		RootAgentID: DefaultRootAgentID,
		Notify:    notify.Multi{},
	}
	return e
}

// SubmitUser runs one user turn (may involve multiple internal model calls).
// in.Content is the primary user line; MediaPaths add leading user messages after normalization.
// Cancel ctx to abort in-flight model and tool calls.
func (e *Engine) SubmitUser(ctx context.Context, in bus.InboundMessage) error {
	atts, err := e.prepareInboundFromBus(&in)
	if err != nil {
		return err
	}
	text := strings.TrimSpace(in.Content)
	if text == "" && len(atts) == 0 {
		return fmt.Errorf("session: empty inbound text")
	}

	turnID := newTurnID()
	corrID := strings.TrimSpace(in.MessageID)
	if e.hasNotify() {
		ev := notify.NewEvent(notify.EventInboundReceived, "")
		e.applyNotifyCorrelation(&ev, turnID, corrID)
		ev.Data = inboundNotifyData(in, text, atts)
		notify.EmitSafe(e.Notify, ctx, ev)
	}

	if reply, ok := e.trySlashLocalTurn(in); ok {
		return e.submitLocalSlashTurn(ctx, in, atts, reply, turnID, corrID)
	}

	preview := combinedInboundPreview(text, atts)
	slog.Debug("session.submit", "cwd", e.CWD, "model", e.Model, "preview_chars", utf8.RuneCountInString(preview))

	prep, err := e.prepareSharedTurn(ctx, in, atts, preview, true, turnID, corrID)
	if err != nil {
		if e.hasNotify() {
			ev := notify.NewEvent(notify.EventTurnError, "error")
			e.applyNotifyCorrelation(&ev, turnID, corrID)
			ev.Data = map[string]any{
				"code":    "preparation",
				"message": err.Error(),
			}
			notify.EmitSafe(e.Notify, ctx, ev)
		}
		return err
	}
	bg := prep.bg
	memOK := prep.memOK
	layout := prep.layout
	bundle := prep.bundle
	system := prep.system

	var traceSink *loop.ToolTraceSink
	needToolTrace := (memOK && memory.MemoryExtractEnabled()) || e.hasNotify()
	if needToolTrace {
		traceSink = &loop.ToolTraceSink{}
	}
	uv := memory.UserTurnPreview(preview)
	var memOnTool func(loop.ToolTraceEntry)
	if memOK && memory.MemoryExtractEnabled() {
		sid, cid := e.SessionID, corrID
		lay := layout
		memOnTool = func(ent loop.ToolTraceEntry) {
			memory.AppendTurnToolLogJSONL(lay, sid, cid, uv, ent)
		}
	}
	var onToolLogged func(loop.ToolTraceEntry)
	switch {
	case memOnTool != nil && e.hasNotify():
		onToolLogged = func(ent loop.ToolTraceEntry) {
			memOnTool(ent)
			ev := notify.NewEvent(notify.EventToolCallEnd, "")
			e.applyNotifyCorrelation(&ev, turnID, corrID)
			ev.Data = notify.ToolCallEndData(ent)
			notify.EmitSafe(e.Notify, ctx, ev)
		}
	case memOnTool != nil:
		onToolLogged = memOnTool
	case e.hasNotify():
		onToolLogged = func(ent loop.ToolTraceEntry) {
			ev := notify.NewEvent(notify.EventToolCallEnd, "")
			e.applyNotifyCorrelation(&ev, turnID, corrID)
			ev.Data = notify.ToolCallEndData(ent)
			notify.EmitSafe(e.Notify, ctx, ev)
		}
	}

	userLine := ModelUserLine(text, len(atts) > 0)
	cfg := loop.Config{
		Client:                &e.Client,
		Model:                 e.Model,
		System:                system,
		MaxTokens:             e.MaxTokens,
		MaxSteps:              e.MaxSteps,
		Messages:              &e.Messages,
		Registry:              e.Registry,
		ToolContext:           prep.tctx,
		SessionID:             e.SessionID,
		CanUseTool:            e.CanUseTool,
		OutboundText:          prep.outboundText,
		MemoryAgentMd:         bundle.AgentMdBlock,
		MemoryRecall:          bundle.RecallBlock,
		InboundMeta:           InboundMetaForModel(in),
		InboundAttachmentMsgs: FormatInboundAttachmentMessages(atts),
		UserLine:              userLine,
		Budget:                bg,
		ChatTransport:         e.ChatTransport,
		ToolTrace:             traceSink,
		OnToolLogged:          onToolLogged,
		SlimTranscript: func(assistantText string) {
			e.Transcript = append(e.Transcript, openai.AssistantMessage(assistantText))
		},
		Lifecycle: e.buildLoopLifecycle(turnID, corrID, prep.tctx.AgentID),
	}

	// Claude Code–style: record user turn before query loop so crash/interrupt still leaves the request on disk.
	e.Transcript = append(e.Transcript, openai.UserMessage(SlimTranscriptUserLine(text, atts)))
	if err := e.SaveTranscript(); err != nil {
		slog.Error("session.transcript_save_user", "err", err)
	}
	if err := e.SaveWorkingTranscript(); err != nil {
		slog.Error("session.working_transcript_save_user", "err", err)
	}

	e.emitMemoryTurnContextNotify(ctx, turnID, corrID, memOK, bundle)

	if e.hasNotify() {
		ev := notify.NewEvent(notify.EventAgentTurnStart, "")
		e.applyNotifyCorrelation(&ev, turnID, corrID)
		ev.Data = map[string]any{
			"model":             e.Model,
			"max_steps":         e.MaxSteps,
			"cwd":               e.CWD,
			"user_line_preview": notify.Preview(userLine, notify.DefaultPreviewRunes),
		}
		notify.EmitSafe(e.Notify, ctx, ev)
	}

	err = loop.RunTurn(ctx, cfg, in)
	if err != nil {
		if e.hasNotify() {
			ev := notify.NewEvent(notify.EventTurnError, "error")
			e.applyNotifyCorrelation(&ev, turnID, corrID)
			ev.Data = map[string]any{
				"code":                   notify.TurnErrorCode(err),
				"message":                err.Error(),
				"truncated_by_max_steps": strings.Contains(err.Error(), "max model steps"),
			}
			notify.EmitSafe(e.Notify, ctx, ev)
		}
		return err
	}
	e.Messages = loop.ToUserVisibleMessages(e.Messages)
	if e.hasNotify() {
		toolCount := 0
		if traceSink != nil {
			toolCount = len(traceSink.Snapshot())
		}
		ev := notify.NewEvent(notify.EventTurnComplete, "")
		e.applyNotifyCorrelation(&ev, turnID, corrID)
		ev.Data = map[string]any{
			"tool_count":               toolCount,
			"final_assistant_preview":  notify.Preview(loop.LastAssistantDisplay(e.Messages), notify.DefaultPreviewRunes),
			"truncated_by_max_steps":   false,
		}
		notify.EmitSafe(e.Notify, ctx, ev)
	}
	if err := e.SaveTranscript(); err != nil {
		slog.Error("session.transcript_save", "err", err)
	}
	if err := e.SaveWorkingTranscript(); err != nil {
		slog.Error("session.working_transcript_save", "err", err)
	}
	e.appendDialogHistoryIfComplete()
	if memOK && memory.MemoryExtractEnabled() {
		memory.AppendTurnAssistantFinalJSONL(layout, e.SessionID, strings.TrimSpace(in.MessageID), uv, loop.LastAssistantDisplay(e.Messages))
	}
	if memOK {
		var tools []loop.ToolTraceEntry
		if traceSink != nil {
			tools = traceSink.Snapshot()
		}
		pti := memory.PostTurnInput{
			SessionID:        e.SessionID,
			CorrelationID:    strings.TrimSpace(in.MessageID),
			UserText:         preview,
			AssistantVisible: loop.LastAssistantDisplay(e.Messages),
			Tools:            append([]loop.ToolTraceEntry(nil), tools...),
		}
		memory.PostTurn(layout, pti)
		// Post-turn LLM maintain is best-effort and can be slow; do not block channels waiting on HTTP Done.
		go func(in memory.PostTurnInput) {
			memory.MaybePostTurnMaintain(context.Background(), layout, &e.Client, e.Model, e.MaxTokens, &in)
		}(pti)
	}
	return nil
}

// SendMessage delivers text and/or attachments without running the model (proactive notify).
// in.Channel must be the clawbridge client id; ChatID / Peer must allow delivery.
func (e *Engine) SendMessage(ctx context.Context, in bus.InboundMessage) error {
	if strings.TrimSpace(in.Channel) == "" {
		return fmt.Errorf("session: SendMessage requires InboundMessage.Channel")
	}
	inCopy := in
	atts, err := e.prepareInboundFromBus(&inCopy)
	if err != nil {
		return err
	}
	body := strings.TrimSpace(inCopy.Content)
	if body == "" && len(atts) == 0 {
		return fmt.Errorf("session: SendMessage: empty text and attachments")
	}
	if e.PublishOutbound == nil {
		return fmt.Errorf("session: PublishOutbound not configured")
	}
	parts := mediaPartsFromAttachments(atts)
	msg := assistantOutboundWithMedia(&inCopy, body, parts)
	if msg == nil {
		return fmt.Errorf("session: SendMessage: missing ChatID (cannot address recipient)")
	}
	return e.PublishOutbound(ctx, msg)
}

func (e *Engine) submitLocalSlashTurn(ctx context.Context, in bus.InboundMessage, atts []Attachment, reply string, turnID, corrID string) error {
	slog.Debug("session.submit_local_slash", "cwd", e.CWD)
	text := strings.TrimSpace(in.Content)
	preview := combinedInboundPreview(text, atts)
	prep, err := e.prepareSharedTurn(ctx, in, atts, preview, false, turnID, corrID)
	if err != nil {
		if e.hasNotify() {
			ev := notify.NewEvent(notify.EventTurnError, "error")
			e.applyNotifyCorrelation(&ev, turnID, corrID)
			ev.Data = map[string]any{
				"code":    "preparation",
				"message": err.Error(),
			}
			notify.EmitSafe(e.Notify, ctx, ev)
		}
		return err
	}
	bundle := prep.bundle
	memOK := prep.memOK

	e.emitMemoryTurnContextNotify(ctx, turnID, corrID, memOK, bundle)

	if e.hasNotify() {
		ev := notify.NewEvent(notify.EventAgentTurnStart, "")
		e.applyNotifyCorrelation(&ev, turnID, corrID)
		ev.Data = map[string]any{
			"local_slash":       true,
			"model":             e.Model,
			"max_steps":         e.MaxSteps,
			"cwd":               e.CWD,
			"user_line_preview": notify.Preview(combinedInboundPreview(text, atts), notify.DefaultPreviewRunes),
		}
		notify.EmitSafe(e.Notify, ctx, ev)
	}

	meta := InboundMetaForModel(in)
	attMsgs := FormatInboundAttachmentMessages(atts)
	userLine := ModelUserLine(text, len(atts) > 0)
	loop.AppendTurnUserMessages(&e.Messages, bundle.AgentMdBlock, bundle.RecallBlock, meta, attMsgs, userLine)
	e.Messages = append(e.Messages, openai.AssistantMessage(reply))
	e.Messages = loop.ToUserVisibleMessages(e.Messages)

	e.Transcript = append(e.Transcript, openai.UserMessage(SlimTranscriptUserLine(text, atts)))
	e.Transcript = append(e.Transcript, openai.AssistantMessage(reply))
	if err := e.SaveTranscript(); err != nil {
		slog.Error("session.transcript_save_local_slash", "err", err)
	}
	if err := e.SaveWorkingTranscript(); err != nil {
		slog.Error("session.working_transcript_save_local_slash", "err", err)
	}
	e.appendDialogHistoryIfComplete()
	if prep.outboundText != nil {
		if err := prep.outboundText(ctx, reply); err != nil {
			slog.Warn("session.local_slash.outbound", "err", err)
		}
	}
	if e.hasNotify() {
		ev := notify.NewEvent(notify.EventTurnComplete, "")
		e.applyNotifyCorrelation(&ev, turnID, corrID)
		ev.Data = map[string]any{
			"tool_count":              0,
			"local_slash":             true,
			"final_assistant_preview": notify.Preview(reply, notify.DefaultPreviewRunes),
			"truncated_by_max_steps":  false,
		}
		notify.EmitSafe(e.Notify, ctx, ev)
	}
	// No memory.PostTurn / MaybePostTurnMaintain: local slash is an intentional bypass (see docs/memory-maintain-dual-entry-design.md §2.4).
	return nil
}

func (e *Engine) prepareInboundFromBus(in *bus.InboundMessage) ([]Attachment, error) {
	atts := AttachmentsFromMediaPaths(in.MediaPaths)
	atts = NormalizeAttachments(atts)
	if err := ValidateInboundMediaPaths(e.CWD, atts); err != nil {
		return nil, err
	}
	if err := PersistInlineAttachmentFiles(e.CWD, &atts); err != nil {
		return nil, err
	}
	return atts, nil
}

func mediaPartsFromAttachments(atts []Attachment) []bus.MediaPart {
	var out []bus.MediaPart
	for _, a := range atts {
		if p := strings.TrimSpace(a.Path); p != "" {
			out = append(out, bus.MediaPart{Path: p, Filename: a.Name, ContentType: a.MIME})
		}
	}
	return out
}

func newSessionID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "sess_" + hex.EncodeToString(b[:])
}

func newTurnID() string {
	var b [10]byte
	_, _ = rand.Read(b[:])
	return "turn_" + hex.EncodeToString(b[:])
}

func (e *Engine) applyNotifyCorrelation(ev *notify.Event, turnID, corrID string) {
	e.stampNotify(ev, turnID, turnID, corrID, e.EffectiveRootAgentID(), "", "")
}

// EffectiveRootAgentID returns RootAgentID trimmed, or DefaultRootAgentID if unset.
func (e *Engine) EffectiveRootAgentID() string {
	if e == nil {
		return DefaultRootAgentID
	}
	if s := strings.TrimSpace(e.RootAgentID); s != "" {
		return s
	}
	return DefaultRootAgentID
}

func (e *Engine) stampNotify(ev *notify.Event, turnID, runID, corrID, agentID, parentAgentID, parentRunID string) {
	ev.SessionID = e.SessionID
	ev.CorrelationID = corrID
	ev.TurnID = turnID
	ev.RunID = runID
	ev.AgentID = agentID
	ev.ParentAgentID = parentAgentID
	ev.ParentRunID = parentRunID
}

func inboundNotifyData(in bus.InboundMessage, text string, atts []Attachment) map[string]any {
	full := combinedInboundPreview(text, atts)
	return map[string]any{
		"channel":          strings.TrimSpace(in.Channel),
		"chat_id":          strings.TrimSpace(in.ChatID),
		"message_id":       strings.TrimSpace(in.MessageID),
		"content_preview":  notify.Preview(full, notify.DefaultPreviewRunes),
		"user_content":     full,
		"attachment_count": len(atts),
		"has_media":        len(atts) > 0 || len(in.MediaPaths) > 0,
	}
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
	capN := e.WorkingTranscriptMaxMessages
	if capN == 0 {
		capN = 30
	}
	msgs := loop.ToUserVisibleMessages(e.Messages)
	if capN > 0 && len(msgs) > capN {
		msgs = msgs[len(msgs)-capN:]
	}
	b, err := loop.MarshalMessages(msgs)
	if err != nil {
		return fmt.Errorf("marshal working transcript: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir working transcript dir: %w", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("write working transcript: %w", err)
	}
	slog.Debug("working transcript saved", "path", path, "messages", len(msgs), "total_in_memory", len(e.Messages))
	return nil
}

// SaveTranscriptTo writes the current conversation to path. No-op if path is empty.
func (e *Engine) SaveTranscriptTo(path string) error {
	if path == "" {
		return nil
	}
	e.Transcript = loop.ToUserVisibleMessages(e.Transcript)
	b, err := loop.MarshalMessages(e.Transcript)
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
	return loop.MarshalMessages(loop.ToUserVisibleMessages(e.Transcript))
}

// LoadTranscript restores Transcript and seeds Messages from it (same as a fresh session after restart).
func (e *Engine) LoadTranscript(data []byte) error {
	msgs, err := loop.UnmarshalMessages(data)
	if err != nil {
		return err
	}
	clean := loop.ToUserVisibleMessages(msgs)
	e.Transcript = clean
	e.Messages = append([]openai.ChatCompletionMessageParamUnion(nil), clean...)
	slog.Info("transcript loaded", "messages", len(clean))
	return nil
}

// LoadWorkingTranscript replaces Messages only (Transcript unchanged). Used after LoadTranscript to restore compacted context.
func (e *Engine) LoadWorkingTranscript(data []byte) error {
	msgs, err := loop.UnmarshalMessages(data)
	if err != nil {
		return err
	}
	e.Messages = loop.ToUserVisibleMessages(msgs)
	slog.Info("working transcript loaded", "messages", len(e.Messages))
	return nil
}
