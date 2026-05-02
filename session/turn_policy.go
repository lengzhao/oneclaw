package session

import (
	"strings"

	"github.com/lengzhao/clawbridge/bus"
)

// TurnPolicy selects how concurrent user messages for the same session interact
// with an in-flight model turn (see TurnHub).
type TurnPolicy int

const (
	// TurnPolicySerial: queue messages; one SubmitUser at a time (default).
	TurnPolicySerial TurnPolicy = iota
	// TurnPolicyInsert: plain-text follow-ups while a turn runs are appended as
	// extra user lines before each ChatModel call ([Engine.BeforeChatModel], ADK
	// BeforeModelRewriteState).
	TurnPolicyInsert
	// TurnPolicyPreempt: new inbounds are still ordered through the session mailbox (like serial) so they
	// run after the current turn; use [TurnHub.CancelInflightTurn] (e.g. /stop) to hard-abort the active turn.
	TurnPolicyPreempt
)

// ParseTurnPolicy maps YAML/config strings to TurnPolicy. Unknown values default to serial.
func ParseTurnPolicy(s string) TurnPolicy {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "insert":
		return TurnPolicyInsert
	case "preempt":
		return TurnPolicyPreempt
	case "", "serial":
		return TurnPolicySerial
	default:
		return TurnPolicySerial
	}
}

func injectableInboundText(in bus.InboundMessage) bool {
	if len(in.MediaPaths) > 0 {
		return false
	}
	t := strings.TrimSpace(in.Content)
	if t == "" {
		return false
	}
	if strings.HasPrefix(t, "/") {
		return false
	}
	return true
}
