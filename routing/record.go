package routing

// Kind is the simplified outbound event type (see docs/outbound-events-design.md).
type Kind string

const (
	KindText Kind = "text"
	KindTool Kind = "tool"
	KindDone Kind = "done"
)

// Record is one outbound event (JSON-friendly).
type Record struct {
	Seq       int64          `json:"seq"`
	Kind      Kind           `json:"kind"`
	Data      map[string]any `json:"data"`
	SessionID string         `json:"session_id,omitempty"`
	JobID     string         `json:"job_id,omitempty"`
}
