// Package toolctx carries per-session state for tool execution (ToolUseContext).
package toolctx

import (
	"context"
	"sync"
)

// Context is the Go analogue of ToolUseContext: tools share cwd, abort signal,
// and optional read cache. NestedMemoryPaths is reserved for phase B (memory).
type Context struct {
	CWD string

	// Abort is cancelled when the user or host aborts the turn.
	Abort context.Context

	ReadFileCache   map[string]string
	readFileCacheMu sync.RWMutex

	// NestedMemoryPaths tracks loaded memory paths for nested includes (phase B).
	NestedMemoryPaths map[string]struct{}
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
