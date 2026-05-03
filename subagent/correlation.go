package subagent

import (
	"crypto/rand"
	"encoding/hex"
)

// NewCorrelationID returns a stable-enough hex id for one top-level turn (logging / audit correlation).
func NewCorrelationID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
