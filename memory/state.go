package memory

// RecallState tracks surfaced recall attachments across turns (path dedupe + byte budget).
type RecallState struct {
	SurfacedPaths map[string]struct{}
	SurfacedBytes int
}

// MaxSurfacedRecallBytes is the default cap for total recall content per session.
const MaxSurfacedRecallBytes = 12_000

func (s *RecallState) cloneMaps() *RecallState {
	if s == nil {
		return &RecallState{SurfacedPaths: make(map[string]struct{})}
	}
	n := &RecallState{
		SurfacedPaths: make(map[string]struct{}),
		SurfacedBytes: s.SurfacedBytes,
	}
	if s.SurfacedPaths != nil {
		for k := range s.SurfacedPaths {
			n.SurfacedPaths[k] = struct{}{}
		}
	}
	return n
}
