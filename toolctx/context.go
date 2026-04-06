// Package toolctx carries per-session state for tool execution (ToolUseContext).
package toolctx

import (
	"context"
	"sync"

	"github.com/lengzhao/oneclaw/routing"
	"github.com/openai/openai-go"
)

// Context is the Go analogue of ToolUseContext: tools share cwd, abort signal,
// and optional read cache. NestedMemoryPaths is reserved for phase B (memory).
type Context struct {
	CWD string

	// HomeDir is the session user's home directory (for audit / policy paths). Empty if unknown.
	HomeDir string

	// Abort is cancelled when the user or host aborts the turn.
	Abort context.Context

	ReadFileCache   map[string]string
	readFileCacheMu sync.RWMutex

	// NestedMemoryPaths reserved for future memory path tracking on the tool side.
	NestedMemoryPaths map[string]struct{}

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
	// Subagent runs nested loops when non-nil (phase C).
	Subagent SubagentRunner

	// TurnInbound is the routing metadata for the current SubmitUser turn (e.g. cron add defaults).
	TurnInbound routing.Inbound
}

// New builds a tool context. If abort is nil, context.Background() is used.
func New(cwd string, abort context.Context) *Context {
	if abort == nil {
		abort = context.Background()
	}
	return &Context{
		CWD:               cwd,
		Abort:             abort,
		ReadFileCache:     make(map[string]string),
		NestedMemoryPaths: make(map[string]struct{}),
		MaxSubagentDepth:  3,
	}
}

// ChildContext returns an isolated tool context for a nested agent (fresh read cache, same cwd/abort/memory roots).
func (c *Context) ChildContext() *Context {
	if c == nil {
		return New("", context.Background())
	}
	child := New(c.CWD, c.Abort)
	child.MemoryWriteRoots = append([]string(nil), c.MemoryWriteRoots...)
	child.HomeDir = c.HomeDir
	child.MaxSubagentDepth = c.MaxSubagentDepth
	child.Subagent = c.Subagent
	child.SubagentDepth = c.SubagentDepth + 1
	child.TurnInbound = c.TurnInbound
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
