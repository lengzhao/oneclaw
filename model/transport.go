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
	return parseChatTransportString(s)
}

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

// resolveTransport uses hint when non-empty, otherwise ONCLAW_CHAT_TRANSPORT.
func resolveTransport(hint string) chatTransport {
	if strings.TrimSpace(hint) != "" {
		return parseChatTransportString(hint)
	}
	return chatTransportFromEnv()
}
