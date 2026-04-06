// Package toolctx carries per-session state for tool execution (ToolUseContext).
package toolctx

import (
	"context"
	"sync"

	"github.com/lengzhao/oneclaw/routing"
	"github.com/openai/openai-go"
)

// SessionHost groups turn-level capabilities wired by session.Engine: routing defaults for tools,
// proactive outbound, and nested agent runner. Embedded on Context so tools still use tctx.TurnInbound, etc.
type SessionHost struct {
	// TurnInbound is the routing metadata for the current SubmitUser turn (e.g. cron add defaults).
	TurnInbound routing.Inbound

	// SendMessage, when set by the host, delivers proactive outbound notifications (session.Engine.SendMessage).
	SendMessage func(ctx context.Context, in routing.Inbound) error

	// Subagent runs nested loops when non-nil (run_agent / fork_context).
	Subagent SubagentRunner
}

// Context is the Go analogue of ToolUseContext: tools share cwd, abort signal,
// optional read cache, and an embedded SessionHost for routing / outbound / subagents.
type Context struct {
	SessionHost

	CWD string

	// HomeDir is the session user's home directory (for audit / policy paths). Empty if unknown.
	HomeDir string

	// Abort is cancelled when the user or host aborts the turn.
	Abort context.Context

	ReadFileCache   map[string]string
	readFileCacheMu sync.RWMutex

	// MemoryWriteRoots are extra absolute directories where read/write_file may access (memory scopes).
	MemoryWriteRoots []string

	// DeferredUserAfterToolBatch are user messages appended to the transcript immediately after
	// the current step's tool result messages (e.g. sidechain merge in user mode).
	DeferredUserAfterToolBatch []openai.ChatCompletionMessageParamUnion
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

// ApplyTurnInboundToToolContext merges envelope routing into TurnInbound (see routing.MergeNonEmptyRouting).
// RunTurn calls this at the start of each turn so tools see Source/SessionKey/… and nested Text-only turns do not keep parent attachments.
func (c *Context) ApplyTurnInboundToToolContext(in routing.Inbound) {
	if c == nil {
		return
	}
	routing.MergeNonEmptyRouting(&c.TurnInbound, in)
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
func (c *Context) DeferUserMessageAfterToolBatch(m openai.ChatCompletionMessageParamUnion) {
	if c == nil {
		return
	}
	c.deferMu.Lock()
	defer c.deferMu.Unlock()
	c.DeferredUserAfterToolBatch = append(c.DeferredUserAfterToolBatch, m)
}

// TakeDeferredUserMessagesAfterToolBatch returns and clears queued user messages.
func (c *Context) TakeDeferredUserMessagesAfterToolBatch() []openai.ChatCompletionMessageParamUnion {
	if c == nil {
		return nil
	}
	c.deferMu.Lock()
	defer c.deferMu.Unlock()
	out := c.DeferredUserAfterToolBatch
	c.DeferredUserAfterToolBatch = nil
	return out
}
