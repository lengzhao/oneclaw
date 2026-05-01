package session

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lengzhao/clawbridge"
	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/workspace"
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
	// Bridge is the clawbridge instance for outbound and inbound status; required for IM runs ([cmd/oneclaw] always sets it).
	Bridge *clawbridge.Bridge
}

// MainEngineFactory returns a factory suitable for [NewTurnHub] or [NewWorkerPool].
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
		instructionRoot := userRoot
		if deps.Resolved.SessionIsolateWorkspace() {
			instructionRoot = filepath.Join(userRoot, "sessions", sid)
		}
		workspaceCWD := filepath.Join(instructionRoot, workspace.IMWorkspaceDirName)
		if err := os.MkdirAll(workspaceCWD, 0o755); err != nil {
			return nil, fmt.Errorf("session: mkdir workspace: %w", err)
		}
		eng := NewEngine(workspaceCWD, deps.Registry)
		eng.SessionID = sid
		eng.UserDataRoot = userRoot
		eng.InstructionRoot = instructionRoot
		eng.WorkspaceFlat = true
		eng.Client = deps.Client
		eng.CanUseTool = DefaultCanUseToolWithScheduleGate()
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
		eng.Bridge = deps.Bridge
		// Outbound / inbound status: [Engine.publishOutbound] and [Engine.updateInboundStatus] use [Engine.Bridge] only.
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
		cbCfg, err := deps.Resolved.ClawbridgeConfigForRun()
		if err != nil {
			return nil, fmt.Errorf("session: MainEngineFactory: clawbridge config: %w", err)
		}
		eng.MediaRoot = filepath.Clean(cbCfg.Media.Root)
		return eng, nil
	}
}
