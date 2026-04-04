package routing

// Source identifies the channel; must match keys registered on SinkRegistry.
const (
	SourceCLI = "cli"
)

// Inbound is wire-neutral metadata for one user turn (see docs/inbound-routing-design.md).
// Text is the user-visible message for this turn (fed to the model by session.Engine.SubmitUser).
type Inbound struct {
	Source        string
	Text          string
	UserID        string
	TenantID      string
	SessionKey    string
	CorrelationID string
	RawRef        any
}
