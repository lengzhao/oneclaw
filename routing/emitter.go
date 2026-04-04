package routing

import (
	"context"
	"sync"
)

// Emitter assigns monotonic seq and optional session/job ids per logical turn.
type Emitter struct {
	sink      Sink
	sessionID string
	jobID     string
	mu        sync.Mutex
	seq       int64
}

// NewEmitter builds an emitter. sink must be non-nil; use NoopSink{} if needed.
func NewEmitter(sink Sink, sessionID, jobID string) *Emitter {
	if sink == nil {
		sink = NoopSink{}
	}
	return &Emitter{sink: sink, sessionID: sessionID, jobID: jobID}
}

func (e *Emitter) emit(ctx context.Context, kind Kind, data map[string]any) error {
	e.mu.Lock()
	e.seq++
	n := e.seq
	e.mu.Unlock()
	return e.sink.Emit(ctx, Record{
		Seq:       n,
		Kind:      kind,
		Data:      data,
		SessionID: e.sessionID,
		JobID:     e.jobID,
	})
}

// Text emits visible assistant text (one or more per turn).
func (e *Emitter) Text(ctx context.Context, content string) error {
	if content == "" {
		return nil
	}
	return e.emit(ctx, KindText, map[string]any{"content": content})
}

// ToolStart marks a tool invocation.
func (e *Emitter) ToolStart(ctx context.Context, name string) error {
	return e.emit(ctx, KindTool, map[string]any{"name": name, "phase": "start"})
}

// ToolEnd marks a tool completion.
func (e *Emitter) ToolEnd(ctx context.Context, name string, ok bool) error {
	return e.emit(ctx, KindTool, map[string]any{"name": name, "phase": "end", "ok": ok})
}

// Done marks the end of the user turn.
func (e *Emitter) Done(ctx context.Context, ok bool, errMsg string) error {
	data := map[string]any{"ok": ok}
	if errMsg != "" {
		data["error"] = errMsg
	}
	return e.emit(ctx, KindDone, data)
}
