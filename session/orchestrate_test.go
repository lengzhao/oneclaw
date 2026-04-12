package session

import (
	"strings"
	"testing"

	"github.com/lengzhao/clawbridge/bus"
)

func TestInboundMetaForModel_prefersSessionIDOverPeerID(t *testing.T) {
	in := bus.InboundMessage{
		ClientID:  "webchat-1",
		SessionID: "wc-tab-9",
		Peer:      bus.Peer{ID: "browser"},
	}
	s := InboundMetaForModel(in)
	if !strings.Contains(s, "session_key: wc-tab-9") {
		t.Fatalf("want wc-tab-9 routing key, got:\n%s", s)
	}
	if strings.Contains(s, "session_key: browser") {
		t.Fatalf("should not use Peer.ID when SessionID set:\n%s", s)
	}
	if !strings.Contains(s, "workspace_session_id:") {
		t.Fatalf("missing workspace line:\n%s", s)
	}
}
