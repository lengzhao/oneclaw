package session

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/schedule"
	"github.com/lengzhao/oneclaw/toolctx"
)

func TestDenySendMessageOnSyntheticScheduleTurn(t *testing.T) {
	tc := toolctx.New("", context.Background())
	deny, _ := denySendMessageOnSyntheticScheduleTurn(tc, "send_message", nil)
	if deny {
		t.Fatal("expected allow without schedule metadata")
	}
	tc.TurnInbound = bus.InboundMessage{
		Metadata: map[string]string{schedule.MetadataKeySyntheticScheduleFire: "1"},
		ClientID: "webchat-1",
		Peer:     bus.Peer{ID: "browser"},
	}
	payload, _ := json.Marshal(map[string]string{"text": "hi"})
	deny, reason := denySendMessageOnSyntheticScheduleTurn(tc, "send_message", payload)
	if !deny || reason == "" {
		t.Fatalf("expected deny same-target send_message, deny=%v reason=%q", deny, reason)
	}
	cross, _ := json.Marshal(map[string]string{"text": "hi", "client_id": "slack-1"})
	if deny2, _ := denySendMessageOnSyntheticScheduleTurn(tc, "send_message", cross); deny2 {
		t.Fatal("cross-client send_message should be allowed on schedule turns")
	}
	if deny3, _ := denySendMessageOnSyntheticScheduleTurn(tc, "exec", nil); deny3 {
		t.Fatal("exec should not be denied")
	}
}
