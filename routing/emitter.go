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

// TextWithAttachments emits assistant-visible text plus optional media (paths or inline text),
// for proactive outbound sends (see session.Engine.SendMessage). JSON consumers read data.attachments as an array of objects.
func (e *Emitter) TextWithAttachments(ctx context.Context, content string, attachments []Attachment) error {
	if content == "" && len(attachments) == 0 {
		return nil
	}
	data := map[string]any{}
	if content != "" {
		data["content"] = content
	}
	if len(attachments) > 0 {
		data["attachments"] = attachmentsToWireMaps(attachments)
	}
	return e.emit(ctx, KindText, data)
}

func attachmentsToWireMaps(atts []Attachment) []map[string]any {
	out := make([]map[string]any, 0, len(atts))
	for _, a := range atts {
		m := map[string]any{
			"name": a.Name,
			"mime": a.MIME,
		}
		if a.Path != "" {
			m["path"] = a.Path
		}
		if a.Text != "" {
			m["text"] = a.Text
		}
		out = append(out, m)
	}
	return out
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
