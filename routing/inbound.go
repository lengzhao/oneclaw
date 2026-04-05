package routing

// Inbound is wire-neutral metadata for one user turn (see docs/inbound-routing-design.md).
// Text is the user-visible message for this turn (fed to the model by session.Engine.SubmitUser).
// Source is the channel instance id: it must match the key used when registering the Sink for that
// instance (often from config `channels[].id`, e.g. slack1 / slack2).
type Inbound struct {
	Source        string
	Text          string
	UserID        string
	TenantID      string
	SessionKey    string
	CorrelationID string
	RawRef        any
}
