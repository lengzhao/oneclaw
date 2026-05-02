// Package toolctx carries per-session state for tool execution (ToolUseContext).
package toolctx

import (
	"context"
	"maps"
	"strings"
	"sync"

	"github.com/cloudwego/eino/schema"
	"github.com/lengzhao/clawbridge/bus"
)

// SessionHost groups turn-level capabilities wired by session.Engine: inbound defaults for tools,
// proactive outbound, and nested agent runner. Embedded on Context so tools still use tctx.TurnInbound, etc.
type SessionHost struct {
	// TurnInbound is the bus metadata for the current SubmitUser turn (e.g. cron add defaults).
	TurnInbound bus.InboundMessage

	// SendMessage, when set by the host, delivers proactive outbound notifications (session.Engine.SendMessage).
	SendMessage func(ctx context.Context, in bus.InboundMessage) error

	// Subagent runs nested loops when non-nil (run_agent / fork_context).
	Subagent SubagentRunner
}

// Context is the Go analogue of ToolUseContext: tools share cwd, abort signal,
// optional read cache, and an embedded SessionHost for routing / outbound / subagents.
type Context struct {
	SessionHost

	CWD string

	// HostDataRoot is the IM config/data root (~/.oneclaw) for host-wide files (e.g. scheduled_jobs.json). Empty uses project-style paths under CWD only.
	HostDataRoot string

	// WorkspaceFlat: when true, tasks.json / agents / exec_log / skills-recent / usage live under <InstructionRoot>/ when set (see workspace.JoinSessionWorkspaceWithInstruction).
	WorkspaceFlat bool
	// InstructionRoot is the IM directory containing AGENT.md/MEMORY.md; empty in repo-style or legacy contexts.
	InstructionRoot string

	// SessionID is the stable logical session (e.g. Engine.SessionID). Empty in tests or non-session hosts.
	SessionID string

	// AgentID is the logical agent for this tool-execution frame: root Engine id on the main thread,
	// or the subagent type (e.g. catalog AgentType / fork_context) inside nested run_agent / fork_context.
	// Session seeds it from Engine.RootAgentID; subagent overwrites it on child contexts.
	AgentID string

	// HomeDir is the session user's home directory (for policy paths). Empty if unknown.
	HomeDir string

	// Abort is cancelled when the user or host aborts the turn.
	Abort context.Context

	ReadFileCache   map[string]string
	readFileCacheMu sync.RWMutex

	// MemoryWriteRoots are extra absolute directories where read/write_file may access (memory scopes).
	MemoryWriteRoots []string

	// DeferredUserAfterToolBatch are user messages appended to the transcript immediately after
	// the current step's tool result messages (e.g. sidechain merge in user mode).
	DeferredUserAfterToolBatch []*schema.Message
	deferMu                    sync.Mutex

	// SubagentDepth is 0 on the main thread; incremented for each nested run_agent/fork_context.
	SubagentDepth int
	// MaxSubagentDepth limits nested agent runs (inclusive of child depth).
	MaxSubagentDepth int
}

// New builds a tool context. If abort is nil, context.Background() is used.
func New(cwd string, abort context.Context) *Context {
	if abort == nil {
		abort = context.Background()
	}
	return &Context{
		CWD:              cwd,
		Abort:            abort,
		ReadFileCache:    make(map[string]string),
		MaxSubagentDepth: 3,
	}
}

// ApplyTurnInboundToToolContext merges envelope routing into TurnInbound.
// Engine.prepareSharedTurn calls this when wiring tool context so tools see ClientID/SessionID/Peer/…
// and nested Content-only turns do not keep parent MediaPaths.
func (c *Context) ApplyTurnInboundToToolContext(in bus.InboundMessage) {
	if c == nil {
		return
	}
	mergeTurnInbound(&c.TurnInbound, in)
}

// mergeTurnInbound copies non-empty routing fields from src into dst. Content is not merged.
// If src has no MediaPaths, dst.MediaPaths is set to nil so nested turns do not inherit parent attachments.
func mergeTurnInbound(dst *bus.InboundMessage, src bus.InboundMessage) {
	if dst == nil {
		return
	}
	if s := strings.TrimSpace(src.ClientID); s != "" {
		dst.ClientID = s
	}
	if s := strings.TrimSpace(src.SessionID); s != "" {
		dst.SessionID = s
	}
	if s := strings.TrimSpace(src.MessageID); s != "" {
		dst.MessageID = s
	}
	if senderNonEmpty(src.Sender) {
		dst.Sender = mergeSender(dst.Sender, src.Sender)
	}
	if peerNonEmpty(src.Peer) {
		dst.Peer = mergePeer(dst.Peer, src.Peer)
	}
	if len(src.MediaPaths) > 0 {
		dst.MediaPaths = append([]string(nil), src.MediaPaths...)
	} else {
		dst.MediaPaths = nil
	}
	if src.ReceivedAt != 0 {
		dst.ReceivedAt = src.ReceivedAt
	}
	if len(src.Metadata) > 0 {
		if dst.Metadata == nil {
			dst.Metadata = maps.Clone(src.Metadata)
		} else {
			for k, v := range src.Metadata {
				if strings.TrimSpace(v) != "" {
					dst.Metadata[k] = v
				}
			}
		}
	}
}

func senderNonEmpty(s bus.SenderInfo) bool {
	return strings.TrimSpace(s.Platform) != "" ||
		strings.TrimSpace(s.PlatformID) != "" ||
		strings.TrimSpace(s.CanonicalID) != "" ||
		strings.TrimSpace(s.Username) != "" ||
		strings.TrimSpace(s.DisplayName) != ""
}

func peerNonEmpty(p bus.Peer) bool {
	return strings.TrimSpace(p.Kind) != "" || strings.TrimSpace(p.ID) != ""
}

func mergeSender(dst, src bus.SenderInfo) bus.SenderInfo {
	out := dst
	if strings.TrimSpace(src.Platform) != "" {
		out.Platform = strings.TrimSpace(src.Platform)
	}
	if strings.TrimSpace(src.PlatformID) != "" {
		out.PlatformID = strings.TrimSpace(src.PlatformID)
	}
	if strings.TrimSpace(src.CanonicalID) != "" {
		out.CanonicalID = strings.TrimSpace(src.CanonicalID)
	}
	if strings.TrimSpace(src.Username) != "" {
		out.Username = strings.TrimSpace(src.Username)
	}
	if strings.TrimSpace(src.DisplayName) != "" {
		out.DisplayName = strings.TrimSpace(src.DisplayName)
	}
	return out
}

func mergePeer(dst, src bus.Peer) bus.Peer {
	out := dst
	if strings.TrimSpace(src.Kind) != "" {
		out.Kind = strings.TrimSpace(src.Kind)
	}
	if strings.TrimSpace(src.ID) != "" {
		out.ID = strings.TrimSpace(src.ID)
	}
	return out
}

// ChildContext returns an isolated tool context for a nested agent (fresh read cache, same cwd/abort/memory roots).
func (c *Context) ChildContext() *Context {
	if c == nil {
		return New("", context.Background())
	}
	child := New(c.CWD, c.Abort)
	child.SessionHost = c.SessionHost
	child.MemoryWriteRoots = append([]string(nil), c.MemoryWriteRoots...)
	child.HomeDir = c.HomeDir
	child.HostDataRoot = c.HostDataRoot
	child.WorkspaceFlat = c.WorkspaceFlat
	child.InstructionRoot = c.InstructionRoot
	child.SessionID = c.SessionID
	child.AgentID = c.AgentID
	child.MaxSubagentDepth = c.MaxSubagentDepth
	child.SubagentDepth = c.SubagentDepth + 1
	return child
}

// ImportReadCache copies read-through cache entries from parent (fork-style prefix sharing).
func (c *Context) ImportReadCacheFrom(parent *Context) {
	if c == nil || parent == nil {
		return
	}
	parent.readFileCacheMu.RLock()
	defer parent.readFileCacheMu.RUnlock()
	c.readFileCacheMu.Lock()
	defer c.readFileCacheMu.Unlock()
	for k, v := range parent.ReadFileCache {
		c.ReadFileCache[k] = v
	}
}

// GetCachedRead returns cached file content if present.
func (c *Context) GetCachedRead(absPath string) (string, bool) {
	c.readFileCacheMu.RLock()
	defer c.readFileCacheMu.RUnlock()
	s, ok := c.ReadFileCache[absPath]
	return s, ok
}

// SetCachedRead stores read-through cache entry.
func (c *Context) SetCachedRead(absPath, content string) {
	c.readFileCacheMu.Lock()
	defer c.readFileCacheMu.Unlock()
	c.ReadFileCache[absPath] = content
}

// DeferUserMessageAfterToolBatch queues a user message to append after the current model step's tool outputs.
func (c *Context) DeferUserMessageAfterToolBatch(m *schema.Message) {
	if c == nil {
		return
	}
	c.deferMu.Lock()
	defer c.deferMu.Unlock()
	c.DeferredUserAfterToolBatch = append(c.DeferredUserAfterToolBatch, m)
}

// TakeDeferredUserMessagesAfterToolBatch returns and clears queued user messages.
func (c *Context) TakeDeferredUserMessagesAfterToolBatch() []*schema.Message {
	if c == nil {
		return nil
	}
	c.deferMu.Lock()
	defer c.deferMu.Unlock()
	out := c.DeferredUserAfterToolBatch
	c.DeferredUserAfterToolBatch = nil
	return out
}
