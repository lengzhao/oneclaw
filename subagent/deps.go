package subagent

import (
	"io"

	"github.com/lengzhao/oneclaw/catalog"
	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/toolhost"
)

// SubAgentChunkFunc streams assistant chunks from nested agents (optional; complements Stdout).
type SubAgentChunkFunc func(correlationID, subRunID, agentType, chunk string)

// RunAgentDeps carries parent runtime state for run_agent and nested sub-agents.
// Values are snapshotted when RegisterRunAgent binds a tool; pointer fields are shared read-only.
type RunAgentDeps struct {
	Catalog         *catalog.Catalog
	Cfg             *config.File
	UserDataRoot    string
	InstructionRoot string
	SessionRoot     string
	SessionSegment  string
	ParentWorkspace string
	ProfileID       string
	UseMock         bool
	Stdout          io.Writer
	OnSubAgentChunk SubAgentChunkFunc

	// CorrelationID identifies one top-level turn / invocation (propagates to subs).
	CorrelationID string
	// DelegationDepth is the current nested run_agent stack level (root tool binding starts at 0).
	DelegationDepth int

	// ParentRegistry is the registry that hosts run_agent; RegisterRunAgent sets this to that registry.
	ParentRegistry toolhost.Registry
}
