package session

import (
	"context"
	"log/slog"
	"os"

	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/tools"
	"github.com/openai/openai-go"
)

// MainEngineFactoryDeps wires a cmd/oneclaw-style Engine for each worker-pool job.
type MainEngineFactoryDeps struct {
	CWD           string
	Resolved      *config.Resolved
	Registry      *tools.Registry
	Client        openai.Client
	Model         string
	MCPSystemNote string
	LLMAudit      bool
	OrchAudit     bool
	VisAudit      bool
	// OutboundPublisher returns the current PublishOutbound hook; nil return skips outbound.
	OutboundPublisher func() func(context.Context, *bus.OutboundMessage) error
	// NewRecallPersister, if non-nil, provides recall persistence for the given handle (e.g. sessdb bridge).
	NewRecallPersister func(SessionHandle) RecallPersister
}

// MainEngineFactory returns a factory suitable for NewWorkerPool.
func MainEngineFactory(deps MainEngineFactoryDeps) func(SessionHandle) (*Engine, error) {
	return func(h SessionHandle) (*Engine, error) {
		eng := NewEngine(deps.CWD, deps.Registry)
		sid := StableSessionID(h)
		eng.SessionID = sid
		eng.Client = deps.Client
		eng.Model = deps.Model
		if deps.Resolved != nil {
			eng.MaxSteps = deps.Resolved.MainAgentMaxSteps()
			eng.ChatTransport = deps.Resolved.ChatTransport()
			tp, wp := deps.Resolved.SessionTranscriptPaths(sid)
			eng.TranscriptPath = tp
			eng.WorkingTranscriptPath = wp
			eng.WorkingTranscriptMaxMessages = deps.Resolved.WorkingTranscriptMaxMessages()
			eng.DisableMultimodalImage = deps.Resolved.MultimodalImageDisabled()
			eng.DisableMultimodalAudio = deps.Resolved.MultimodalAudioDisabled()
		}
		if deps.MCPSystemNote != "" {
			eng.MCPSystemNote = deps.MCPSystemNote
		}
		eng.PublishOutbound = func(ctx context.Context, msg *bus.OutboundMessage) error {
			pub := deps.OutboundPublisher
			if pub == nil {
				return nil
			}
			if fn := pub(); fn != nil {
				return fn(ctx, msg)
			}
			return nil
		}
		eng.RegisterAuditSinks(deps.LLMAudit, deps.OrchAudit, deps.VisAudit)
		if np := deps.NewRecallPersister; np != nil {
			eng.RecallPersister = np(h)
			if eng.RecallPersister != nil {
				if st, err := eng.RecallPersister.LoadRecall(sid); err != nil {
					slog.Warn("session.recall_load", "session_id", sid, "err", err)
				} else {
					eng.RecallState = st
				}
			}
		}
		if tp := eng.TranscriptPath; tp != "" {
			if b, err := os.ReadFile(tp); err == nil {
				if err := eng.LoadTranscript(b); err != nil {
					return nil, err
				}
			} else if !os.IsNotExist(err) {
				return nil, err
			}
		}
		if wp := eng.WorkingTranscriptPath; wp != "" {
			if b, err := os.ReadFile(wp); err == nil {
				if err := eng.LoadWorkingTranscript(b); err != nil {
					return nil, err
				}
			} else if !os.IsNotExist(err) {
				return nil, err
			}
		}
		return eng, nil
	}
}
