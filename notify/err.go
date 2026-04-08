package notify

import (
	"context"
	"errors"
	"strings"
)

// TurnErrorCode maps common errors to a stable code for integrations.
func TurnErrorCode(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) {
		return "context_canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "context_deadline"
	}
	msg := err.Error()
	if strings.Contains(msg, "max model steps") {
		return "max_steps"
	}
	return "error"
}
