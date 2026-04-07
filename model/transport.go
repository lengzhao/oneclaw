package model

import (
	"strings"

	"github.com/lengzhao/oneclaw/rtopts"
)

// Chat transport (OpenAI-compatible). Config: chat.transport in oneclaw YAML (via rtopts).
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

func parseChatTransportString(s string) chatTransport {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "stream":
		return transportStream
	case "non_stream", "nonstream", "json", "rest":
		return transportNonStream
	case "auto", "":
		return transportAuto
	default:
		return transportAuto
	}
}

// resolveTransport uses hint when non-empty, otherwise chat.transport from config.
func resolveTransport(hint string) chatTransport {
	if strings.TrimSpace(hint) != "" {
		return parseChatTransportString(hint)
	}
	return parseChatTransportString(rtopts.Current().ChatTransport)
}
