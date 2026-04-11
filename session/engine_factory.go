package session

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/lengzhao/clawbridge"
	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/tools"
	"github.com/openai/openai-go"
)

// MainEngineFactoryDeps wires a cmd/oneclaw-style Engine for each worker-pool job.
type MainEngineFactoryDeps struct {
	Resolved *config.Resolved
	Registry *tools.Registry
	Client   openai.Client
	Model    string
	// MCPSystemNote is optional MCP section for the main-thread system prompt.
	MCPSystemNote string
	LLMAudit      bool
	OrchAudit     bool
	VisAudit      bool
	// NewRecallPersister, if non-nil, provides recall persistence for the given handle (e.g. sessdb bridge).
	NewRecallPersister func(SessionHandle) RecallPersister
}

// MainEngineFactory returns a factory suitable for NewWorkerPool.
func MainEngineFactory(deps MainEngineFactoryDeps) func(SessionHandle) (*Engine, error) {
	return func(h SessionHandle) (*Engine, error) {
		if deps.Resolved == nil {
			return nil, fmt.Errorf("session: MainEngineFactory: nil Resolved")
		}
		sid := StableSessionID(h)
		userRoot := deps.Resolved.UserDataRoot()
		if userRoot == "" {
			return nil, fmt.Errorf("session: empty UserDataRoot (config not loaded with Home?)")
		}
		sessionHome := filepath.Join(userRoot, "sessions", sid)
		dot := filepath.Join(sessionHome, memory.DotDir)
		if err := os.MkdirAll(dot, 0o755); err != nil {
			return nil, fmt.Errorf("session: mkdir session workspace: %w", err)
		}

		eng := NewEngine(sessionHome, deps.Registry)
		eng.SessionID = sid
		eng.UserDataRoot = userRoot
		eng.Client = deps.Client
		eng.Model = deps.Model
		eng.MaxSteps = deps.Resolved.MainAgentMaxSteps()
		eng.MaxTokens = deps.Resolved.MainAgentMaxCompletionTokens()
		eng.ChatTransport = deps.Resolved.ChatTransport()
		tp, wp := deps.Resolved.SessionTranscriptPaths(sid)
		eng.TranscriptPath = tp
		eng.WorkingTranscriptPath = wp
		eng.WorkingTranscriptMaxMessages = deps.Resolved.WorkingTranscriptMaxMessages()
		eng.DisableMultimodalImage = deps.Resolved.MultimodalImageDisabled()
		eng.DisableMultimodalAudio = deps.Resolved.MultimodalAudioDisabled()
		if deps.MCPSystemNote != "" {
			eng.MCPSystemNote = deps.MCPSystemNote
		}
		// Requires cmd/oneclaw to call clawbridge.SetDefault after New; see clawbridge.PublishOutbound / UpdateStatus.
		eng.PublishOutbound = clawbridge.PublishOutbound
		eng.UpdateInboundStatus = clawbridge.UpdateStatus
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
