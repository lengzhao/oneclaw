package session

import (
	"crypto/sha256"
	"encoding/hex"
)

// StableSessionID returns a deterministic, filesystem-safe id for a logical chat handle.
// It is stable across process restarts for the same Source + SessionKey pair.
func StableSessionID(h SessionHandle) string {
	sum := sha256.Sum256([]byte(h.key()))
	return hex.EncodeToString(sum[:12])
}
