package notify

import (
	"context"
	"fmt"
	"testing"
)

func TestTurnErrorCode(t *testing.T) {
	if TurnErrorCode(context.Canceled) != "context_canceled" {
		t.Fatal()
	}
	if TurnErrorCode(context.DeadlineExceeded) != "context_deadline" {
		t.Fatal()
	}
	if TurnErrorCode(fmt.Errorf("max model steps (99) exceeded")) != "max_steps" {
		t.Fatal()
	}
	if TurnErrorCode(fmt.Errorf("other")) != "error" {
		t.Fatal()
	}
}
