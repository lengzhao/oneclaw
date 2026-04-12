package toolctx

import (
	"context"
	"testing"

	"github.com/lengzhao/clawbridge/bus"
)

func TestApplyTurnInboundToToolContext_mergesMetadata(t *testing.T) {
	c := New("", context.Background())
	in := bus.InboundMessage{
		ClientID: "cli",
		Metadata: map[string]string{"oneclaw_synthetic_schedule": "1", "k": "v"},
	}
	c.ApplyTurnInboundToToolContext(in)
	if c.TurnInbound.Metadata["oneclaw_synthetic_schedule"] != "1" {
		t.Fatalf("metadata: %#v", c.TurnInbound.Metadata)
	}
}
