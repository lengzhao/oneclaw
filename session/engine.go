package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/lengzhao/clawbridge"
	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/clawbridge/client"
	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/notify"
	"github.com/lengzhao/oneclaw/rtopts"
	"github.com/lengzhao/oneclaw/tools"
	"github.com/lengzhao/oneclaw/workspace"
	"github.com/openai/openai-go"
)

// DefaultRootAgentID is the default notify segment id for the root Engine created by NewEngine.
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
	// Messages is the live API history. After each successful SubmitUser turn it is collapsed to
	// user-visible rows only (loop.ToUserVisibleMessages): injections and tool rounds are dropped
	// to save tokens; facts can be re-fetched with tools or instruction files.
	Messages []openai.ChatCompletionMessageParamUnion
	// Transcript is the slim persisted log (real user lines + final assistant text per turn), same
	// visibility rules as Messages; agentMd / inbound / tool rows are not persisted here.
	Transcript []openai.ChatCompletionMessageParamUnion
	Registry   *tools.Registry
	CWD        string
	// WorkspaceFlat: when true, session runtime files (tasks.json, memory/, agents/, exec_log/, …) live directly under
	// <InstructionRoot>/ ; CWD is <InstructionRoot>/workspace. When false, legacy layout uses CWD/.
	WorkspaceFlat bool
	// UserDataRoot is the IM host directory (~/.oneclaw): shared config parent; cron/schedule jobs file; empty in tests or non-IM engines.
	UserDataRoot string
	// MediaRoot is the clawbridge local media store root (default <UserDataRoot>/media). Inbound locators from drivers are ingested into workspace/media/inbound when they sit outside Engine.CWD.
	MediaRoot string
	// InstructionRoot is the IM directory containing AGENT.md and MEMORY.md (same dir as CWD/workspace parent); empty for non-IM engines.
	InstructionRoot string
	CanUseTool      tools.CanUseTool
	SessionID       string
	// RootAgentID is the stable id of this Engine (main thread only). It is copied into toolctx.Context.AgentID each turn; subagents use their own ctx.AgentID. Empty means DefaultRootAgentID in EffectiveRootAgentID.
	RootAgentID string
	// Notify is a fan-out list (notify.Multi) of lifecycle sinks; default empty no-op.
	// Register handlers with RegisterNotify(sink) or assign notify.Multi{…} directly.
	// Only user_input, turn_start, and turn_end are emitted; finer-grained steps go to sharded execution logs (see executionLogRel).
	Notify notify.Multi
	execMu sync.Mutex // execution log append
	// executionLogTurnID / executionLogDay / executionLogRel are set per user turn for execution/<agent>/<day>/<turn>.jsonl.
	executionLogTurnID string
	executionLogDay    string
	executionLogRel    string // path under UserDataRoot when known (for hooks); else logical suffix
	// TranscriptPath is where SaveTranscript writes after each successful SubmitUser turn. Empty disables auto-save.
	TranscriptPath string
	// WorkingTranscriptPath persists Messages (already user-visible in memory after each turn).
	WorkingTranscriptPath string
	// WorkingTranscriptMaxMessages is the max tail messages to persist; 0 means default 30; negative means unlimited.
	WorkingTranscriptMaxMessages int
	// ChatTransport overrides default transport when non-empty (from unified config).
	ChatTransport string
	// EinoOpenAI* are used by the Eino executor path.
	EinoOpenAIAPIKey  string
	EinoOpenAIBaseURL string
	// MCPSystemNote is optional; non-empty injects the MCP section in the main-thread system prompt.
	MCPSystemNote string
	// DisableMultimodalImage when true, image attachments use read_file hints only (no image_url API parts).
	DisableMultimodalImage bool
	// DisableMultimodalAudio when true, wav/mp3 attachments use read_file hints only (no input_audio parts).
	DisableMultimodalAudio bool
	// Bridge is the IM bus for outbound publish and inbound message status. When nil, outbound/status calls return [clawbridge.ErrNotInitialized]. [cmd/oneclaw] / [MainEngineFactoryDeps] set this from [clawbridge.New]; tests set it from a noop bridge.
	Bridge *clawbridge.Bridge
	// BeforeModelStep is optional; when set, passed to the runtime turn runner
	// to append mid-turn user lines (e.g. [TurnHub] insert policy).
	BeforeModelStep func(ctx context.Context, step int, msgs *[]openai.ChatCompletionMessageParamUnion) error
	// TurnRunner is the runtime execution backend for one user turn.
	// NewEngine defaults to eino TurnRunner; pass a non-nil Registry.
	TurnRunner TurnRunner
}

// NewEngine builds an engine with sensible defaults.
func NewEngine(cwd string, reg *tools.Registry) *Engine {
	e := &Engine{
		Client:      openai.NewClient(),
		Model:       string(openai.ChatModelGPT4o),
		MaxTokens:   32768,
		MaxSteps:    32,
		System:      "",
		Registry:    reg,
		CWD:         cwd,
		SessionID:   newSessionID(),
		RootAgentID: DefaultRootAgentID,
		Notify:      notify.Multi{},
	}
	e.TurnRunner = defaultTurnRunner()
	return e
}

// SubmitUser runs one user turn (may involve multiple internal model calls).
// in.Content is the primary user line; MediaPaths add leading user messages after normalization.
// Cancel ctx to abort in-flight model and tool calls.
func (e *Engine) SubmitUser(ctx context.Context, in bus.InboundMessage) (err error) {
	defer func() {
		if err != nil && e != nil {
			e.publishOutboundSubmitUserError(context.WithoutCancel(ctx), &in, err)
		}
	}()
	atts, err := e.prepareInboundFromBus(&in)
	if err != nil {
		return err
	}
	text := strings.TrimSpace(in.Content)
	if text == "" && len(atts) == 0 {
		return fmt.Errorf("session: empty inbound text")
	}

	e.applyInboundMessageStatus(ctx, in, bus.StatusProcessing)
	defer func() {
		st := bus.StatusCompleted
		if err != nil {
			st = bus.StatusFailed
		}
		e.applyInboundMessageStatus(context.WithoutCancel(ctx), in, st)
	}()

	turnID := newTurnID()
	e.bindExecutionLogShard(turnID)
	corrID := strings.TrimSpace(in.MessageID)
	turnT0 := time.Now()
	e.emitUserInputHook(ctx, turnID, corrID, inboundNotifyData(in, text, atts))

	if reply, ok := e.trySlashLocalTurn(in); ok {
		return e.submitLocalSlashTurn(ctx, in, atts, reply, turnID, corrID)
	}

	preview := combinedInboundPreview(text, atts)
	slog.Debug("session.submit", "cwd", e.CWD, "model", e.Model, "preview_chars", utf8.RuneCountInString(preview))

	prep, err := e.prepareSharedTurn(ctx, in, true, turnID, corrID)
	if err != nil {
		e.emitTurnEndHook(ctx, turnID, corrID, false, map[string]any{
			"code":    "preparation",
			"message": err.Error(),
		})
		return err
	}
	slog.Info("session.turn_prepared",
		"prepare_ms", time.Since(turnT0).Milliseconds(),
		"cwd", e.CWD,
		"model", e.Model,
		"client_id", strings.TrimSpace(in.ClientID),
	)
	bg := prep.bg
	memOK := prep.memOK
	bundle := prep.bundle
	system := prep.system

	var traceSink *loop.ToolTraceSink
	needToolTrace := e.wantsLifecycle()
	if needToolTrace {
		traceSink = &loop.ToolTraceSink{}
	}
	var onToolLogged func(loop.ToolTraceEntry)
	if needToolTrace {
		onToolLogged = func(ent loop.ToolTraceEntry) {
			rec := toolCallEndRecord(ent)
			rec["turn_id"] = turnID
			rec["correlation_id"] = corrID
			e.appendExecutionRecord(ctx, rec)
		}
	}

	userLine := ModelUserLine(text, len(atts) > 0)
	attChunks := InboundUserChunksForAttachments(e.CWD, atts, !e.DisableMultimodalImage, !e.DisableMultimodalAudio)
	extraJSON := rtopts.Current().ChatCompletionExtraJSON
	cfg := loop.Config{
		Client:                  &e.Client,
		Model:                   e.Model,
		System:                  system,
		MaxTokens:               e.MaxTokens,
		MaxSteps:                e.MaxSteps,
		Messages:                &e.Messages,
		Registry:                e.Registry,
		ToolContext:             prep.tctx,
		SessionID:               e.SessionID,
		CanUseTool:              e.CanUseTool,
		OutboundText:            prep.outboundText,
		MemoryAgentMd:           bundle.AgentMdBlock,
		InboundMeta:             InboundMetaForModel(in),
		InboundAttachmentChunks: attChunks,
		UserLine:                userLine,
		Budget:                  bg,
		ChatTransport:           e.ChatTransport,
		ChatCompletionExtraJSON: extraJSON,
		EinoOpenAIAPIKey:        e.EinoOpenAIAPIKey,
		EinoOpenAIBaseURL:       e.EinoOpenAIBaseURL,
		ToolTrace:               traceSink,
		OnToolLogged:            onToolLogged,
		SlimTranscript: func(assistantText string) {
			e.Transcript = append(e.Transcript, openai.AssistantMessage(assistantText))
		},
		Lifecycle:       e.buildLoopLifecycle(turnID, corrID, prep.tctx.AgentID),
		BeforeModelStep: e.BeforeModelStep,
		TurnMaxSteps:    e.MaxSteps,
	}
	if cfg.TurnMaxSteps < 1 {
		cfg.TurnMaxSteps = 1
	}

	visibleCountBefore := len(loop.ToUserVisibleMessages(e.Transcript))
	// Claude Code–style: record user turn before query loop so crash/interrupt still leaves the request on disk.
	e.Transcript = append(e.Transcript, openai.UserMessage(SlimTranscriptUserLine(text, atts)))
	if err := e.SaveTranscript(); err != nil {
		slog.Error("session.transcript_save_user", "err", err)
	}
	if err := e.SaveWorkingTranscript(); err != nil {
		slog.Error("session.working_transcript_save_user", "err", err)
	}

	e.emitInstructionContextJournal(ctx, turnID, corrID, memOK, bundle)

	runner := e.TurnRunner
	if runner == nil {
		runner = defaultTurnRunner()
	}
	runnerName := strings.TrimSpace(runner.Name())
	if runnerName == "" {
		runnerName = "unknown"
	}

	e.emitTurnStartHook(ctx, turnID, corrID, map[string]any{
		"model":             e.Model,
		"max_steps":         e.MaxSteps,
		"runtime_runner":    runnerName,
		"cwd":               e.CWD,
		"user_line_preview": notify.Preview(userLine, notify.DefaultPreviewRunes),
	})

	slog.Info("session.model_loop_start",
		"since_inbound_ms", time.Since(turnT0).Milliseconds(),
		"live_messages", len(e.Messages),
		"turn_id", turnID,
		"runtime_runner", runnerName,
	)
	err = runner.RunTurn(ctx, cfg, in)
	if err != nil {
		e.emitTurnEndHook(ctx, turnID, corrID, false, map[string]any{
			"code":                   notify.TurnErrorCode(err),
			"message":                err.Error(),
			"runtime_runner":         runnerName,
			"truncated_by_max_steps": notify.TurnErrorCode(err) == "max_steps",
		})
		return err
	}
	e.Messages = loop.ToUserVisibleMessages(e.Messages)
	toolCount := 0
	if traceSink != nil {
		toolCount = len(traceSink.Snapshot())
	}
	e.emitTurnEndHook(ctx, turnID, corrID, true, map[string]any{
		"runtime_runner":          runnerName,
		"tool_count":              toolCount,
		"final_assistant_preview": notify.Preview(loop.LastAssistantDisplay(e.Messages), notify.DefaultPreviewRunes),
		"truncated_by_max_steps":  false,
		"messages":                loop.VisibleTranscriptAppendSince(e.Transcript, visibleCountBefore),
	})
	if err := e.SaveTranscript(); err != nil {
		slog.Error("session.transcript_save", "err", err)
	}
	if err := e.SaveWorkingTranscript(); err != nil {
		slog.Error("session.working_transcript_save", "err", err)
	}
	e.appendDialogHistoryIfComplete()
	return nil
}

// SendMessage delivers text and/or attachments without running the model (proactive notify).
// in.ClientID must be the clawbridge client id; SessionID / Peer must allow delivery.
func (e *Engine) SendMessage(ctx context.Context, in bus.InboundMessage) error {
	if strings.TrimSpace(in.ClientID) == "" {
		return fmt.Errorf("session: SendMessage requires InboundMessage.ClientID")
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
	parts := mediaPartsFromAttachments(atts)
	msg := assistantOutboundWithMedia(&inCopy, body, parts)
	if msg == nil {
		return fmt.Errorf("session: SendMessage: missing SessionID (cannot address recipient)")
	}
	if inCopy.Metadata != nil {
		if u := strings.TrimSpace(inCopy.Metadata[MetadataKeyOutboundRecipientUserID]); u != "" {
			msg.To.UserID = u
			delete(inCopy.Metadata, MetadataKeyOutboundRecipientUserID)
		}
	}
	slog.Debug("session.send_message.publish",
		"client_id", msg.ClientID,
		"to_session_id", msg.To.SessionID,
		"to_kind", msg.To.Kind,
	)
	return e.publishOutbound(ctx, msg)
}

func (e *Engine) submitLocalSlashTurn(ctx context.Context, in bus.InboundMessage, atts []Attachment, reply string, turnID, corrID string) error {
	slog.Debug("session.submit_local_slash", "cwd", e.CWD)
	e.bindExecutionLogShard(turnID)
	text := strings.TrimSpace(in.Content)
	prep, err := e.prepareSharedTurn(ctx, in, false, turnID, corrID)
	if err != nil {
		e.emitTurnEndHook(ctx, turnID, corrID, false, map[string]any{
			"code":    "preparation",
			"message": err.Error(),
		})
		return err
	}
	bundle := prep.bundle
	memOK := prep.memOK

	e.emitInstructionContextJournal(ctx, turnID, corrID, memOK, bundle)

	e.emitTurnStartHook(ctx, turnID, corrID, map[string]any{
		"local_slash":       true,
		"model":             e.Model,
		"max_steps":         e.MaxSteps,
		"cwd":               e.CWD,
		"user_line_preview": notify.Preview(combinedInboundPreview(text, atts), notify.DefaultPreviewRunes),
	})

	visibleCountBefore := len(loop.ToUserVisibleMessages(e.Transcript))
	// Local slash replies are user-visible only via PublishOutbound (when configured with ClientID+SessionID).
	// Do not append to Messages / Transcript / dialog history so they never appear in later turns or on-disk transcripts.
	if err := prep.outboundText(ctx, reply); err != nil {
		slog.Warn("session.local_slash.outbound", "err", err)
	}
	e.emitTurnEndHook(ctx, turnID, corrID, true, map[string]any{
		"tool_count":              0,
		"local_slash":             true,
		"final_assistant_preview": notify.Preview(reply, notify.DefaultPreviewRunes),
		"truncated_by_max_steps":  false,
		"messages":                loop.VisibleTranscriptAppendSince(e.Transcript, visibleCountBefore),
	})
	return nil
}

// publishOutboundSubmitUserError sends a user-visible assistant line with the failure reason when
// the inbound message has channel addressing (same rules as normal replies). Best-effort: no bridge
// or PublishOutbound failure is ignored except for logging unexpected errors.
func (e *Engine) publishOutboundSubmitUserError(ctx context.Context, in *bus.InboundMessage, err error) {
	if e == nil || err == nil || in == nil {
		return
	}
	text := fmt.Sprintf("处理失败：%v", err)
	msg := assistantTextOutbound(in, text)
	if msg == nil {
		slog.Debug("session.submit_user_error_outbound_skip", "reason", "no_client_or_session")
		return
	}
	if pubErr := e.publishOutbound(ctx, msg); pubErr != nil && !errors.Is(pubErr, clawbridge.ErrNotInitialized) {
		slog.Warn("session.submit_user_error_outbound", "err", pubErr)
	}
}

// ApplyInboundMessageStatus updates driver-visible inbound status when MessageID, ClientID, and SessionID are set.
func (e *Engine) ApplyInboundMessageStatus(ctx context.Context, in bus.InboundMessage, state string) {
	e.applyInboundMessageStatus(ctx, in, state)
}

func (e *Engine) applyInboundMessageStatus(ctx context.Context, in bus.InboundMessage, state string) {
	state = strings.TrimSpace(state)
	if state == "" {
		return
	}
	if strings.TrimSpace(in.MessageID) == "" || strings.TrimSpace(in.ClientID) == "" || strings.TrimSpace(in.SessionID) == "" {
		return
	}
	if err := e.updateInboundStatus(ctx, &in, clawbridge.UpdateStatusState(state), nil); err != nil {
		if errors.Is(err, client.ErrCapabilityUnsupported) || errors.Is(err, clawbridge.ErrNotInitialized) {
			return
		}
		slog.Warn("session.inbound_status", "state", state, "err", err)
	}
}

func (e *Engine) prepareInboundFromBus(in *bus.InboundMessage) ([]Attachment, error) {
	atts := AttachmentsFromMediaPaths(in.MediaPaths)
	atts = NormalizeAttachments(atts)
	if err := e.rehomeInboundAttachments(&atts); err != nil {
		return nil, err
	}
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
		"client_id":        strings.TrimSpace(in.ClientID),
		"session_id":       strings.TrimSpace(in.SessionID),
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
	layout := e.MemoryLayout(home)
	date := time.Now().Format("2006-01-02")
	if err := workspace.AppendDialogHistoryPair(layout, date, e.SessionID, u, a); err != nil {
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
