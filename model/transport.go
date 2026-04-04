package model

import (
	"os"
	"strings"
)

// Chat transport (OpenAI-compatible). Env: ONCLAW_CHAT_TRANSPORT
//
//   - auto (default): try streaming, then non-streaming on failure (some gateways
//     e.g. Kimi/Moonshot return broken SSE with empty JSON chunks).
//   - stream: streaming only (no fallback).
//   - non_stream: single JSON request only (fastest when your base URL does not
//     support OpenAI-style SSE reliably).
type chatTransport int

const (
	transportAuto chatTransport = iota
	transportStream
	transportNonStream
)

func chatTransportFromEnv() chatTransport {
	s := strings.ToLower(strings.TrimSpace(os.Getenv("ONCLAW_CHAT_TRANSPORT")))
	switch s {
	case "stream":
		return transportStream
	case "non_stream", "nonstream", "json", "rest":
		return transportNonStream
	default:
		return transportAuto
	}
}
