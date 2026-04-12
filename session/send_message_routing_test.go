package session

import (
	"encoding/json"
	"testing"

	"github.com/lengzhao/clawbridge/bus"
)

func TestSendMessageTargetOverridesTurn(t *testing.T) {
	tin := bus.InboundMessage{
		ClientID:  "webchat-1",
		SessionID: "wc-1",
		Peer:      bus.Peer{ID: "browser", Kind: "direct"},
		Sender:    bus.SenderInfo{PlatformID: "u1", CanonicalID: "u1", Platform: "ten"},
	}
	empty := SendMessageRoutingArgs{}
	if SendMessageTargetOverridesTurn(tin, empty) {
		t.Fatal("empty overrides")
	}
	if !SendMessageTargetOverridesTurn(tin, SendMessageRoutingArgs{ClientID: "slack-1"}) {
		t.Fatal("client_id override")
	}
	if !SendMessageTargetOverridesTurn(tin, SendMessageRoutingArgs{SessionID: "wc-2"}) {
		t.Fatal("session_id override")
	}
	if !SendMessageTargetOverridesTurn(tin, SendMessageRoutingArgs{PeerID: "other"}) {
		t.Fatal("peer_id override")
	}
	if !SendMessageTargetOverridesTurn(tin, SendMessageRoutingArgs{PeerKind: "channel"}) {
		t.Fatal("peer_kind override")
	}
	if !SendMessageTargetOverridesTurn(tin, SendMessageRoutingArgs{ToUserID: "other"}) {
		t.Fatal("to_user_id override")
	}
	if !SendMessageTargetOverridesTurn(tin, SendMessageRoutingArgs{TenantID: "otherten"}) {
		t.Fatal("tenant override")
	}
}

func TestSendMessageToolRoutingFromJSON_legacy(t *testing.T) {
	raw := json.RawMessage(`{"source":"x","session_key":"y","user_id":"z"}`)
	a := SendMessageToolRoutingFromJSON(raw)
	if a.ClientID != "x" || a.SessionID != "y" || a.ToUserID != "z" {
		t.Fatalf("%+v", a)
	}
}
