package engine

import "context"

// ReadSnapshot is a point-in-time copy of fields that the compose driver may
// mutate between scheduling an async node and that node's handler acquiring ExecMu.
// It does not include Assistant or Bundle — those are updated by other handlers and
// must be read live from RuntimeContext.
type ReadSnapshot struct {
	UserPrompt      string
	SessionRoot     string
	SessionSegment  string
	ProfileID       string
	ModelName       string
	UseMock         bool
	UserDataRoot    string
	InstructionRoot string
	WorkspacePath   string
}

type readSnapshotCtxKey struct{}

// WithReadSnapshot attaches a ReadSnapshot for async workflow handlers.
func WithReadSnapshot(ctx context.Context, s ReadSnapshot) context.Context {
	return context.WithValue(ctx, readSnapshotCtxKey{}, s)
}

// ReadSnapshotFromContext returns the snapshot attached by WithReadSnapshot.
func ReadSnapshotFromContext(ctx context.Context) (ReadSnapshot, bool) {
	s, ok := ctx.Value(readSnapshotCtxKey{}).(ReadSnapshot)
	return s, ok
}

// CaptureReadSnapshot copies read-mostly rtx fields; caller should hold rtx.ExecMu if rtx may be mutated concurrently.
func CaptureReadSnapshot(rtx *RuntimeContext) ReadSnapshot {
	if rtx == nil {
		return ReadSnapshot{}
	}
	return ReadSnapshot{
		UserPrompt:      rtx.UserPrompt,
		SessionRoot:     rtx.SessionRoot,
		SessionSegment:  rtx.SessionSegment,
		ProfileID:       rtx.ProfileID,
		ModelName:       rtx.ModelName,
		UseMock:         rtx.UseMock,
		UserDataRoot:    rtx.UserDataRoot,
		InstructionRoot: rtx.InstructionRoot,
		WorkspacePath:   rtx.WorkspacePath,
	}
}

func (rtx *RuntimeContext) readOverlay() (ReadSnapshot, bool) {
	if rtx == nil || rtx.GoCtx == nil {
		return ReadSnapshot{}, false
	}
	return ReadSnapshotFromContext(rtx.GoCtx)
}

func (rtx *RuntimeContext) EffectiveUserPrompt() string {
	if s, ok := rtx.readOverlay(); ok {
		return s.UserPrompt
	}
	if rtx == nil {
		return ""
	}
	return rtx.UserPrompt
}

func (rtx *RuntimeContext) EffectiveSessionRoot() string {
	if s, ok := rtx.readOverlay(); ok {
		return s.SessionRoot
	}
	if rtx == nil {
		return ""
	}
	return rtx.SessionRoot
}

func (rtx *RuntimeContext) EffectiveSessionSegment() string {
	if s, ok := rtx.readOverlay(); ok {
		return s.SessionSegment
	}
	if rtx == nil {
		return ""
	}
	return rtx.SessionSegment
}

func (rtx *RuntimeContext) EffectiveProfileID() string {
	if s, ok := rtx.readOverlay(); ok {
		return s.ProfileID
	}
	if rtx == nil {
		return ""
	}
	return rtx.ProfileID
}

func (rtx *RuntimeContext) EffectiveModelName() string {
	if s, ok := rtx.readOverlay(); ok {
		return s.ModelName
	}
	if rtx == nil {
		return ""
	}
	return rtx.ModelName
}

func (rtx *RuntimeContext) EffectiveUseMock() bool {
	if s, ok := rtx.readOverlay(); ok {
		return s.UseMock
	}
	if rtx == nil {
		return false
	}
	return rtx.UseMock
}

func (rtx *RuntimeContext) EffectiveUserDataRoot() string {
	if s, ok := rtx.readOverlay(); ok {
		return s.UserDataRoot
	}
	if rtx == nil {
		return ""
	}
	return rtx.UserDataRoot
}

func (rtx *RuntimeContext) EffectiveInstructionRoot() string {
	if s, ok := rtx.readOverlay(); ok {
		return s.InstructionRoot
	}
	if rtx == nil {
		return ""
	}
	return rtx.InstructionRoot
}

func (rtx *RuntimeContext) EffectiveWorkspacePath() string {
	if s, ok := rtx.readOverlay(); ok {
		return s.WorkspacePath
	}
	if rtx == nil {
		return ""
	}
	return rtx.WorkspacePath
}
