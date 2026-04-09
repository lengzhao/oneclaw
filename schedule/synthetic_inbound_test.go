package schedule

import "testing"

func TestSyntheticInboundFromDelivery_setsChatIDAndPeerKind(t *testing.T) {
	d := TurnDelivery{
		Text:          "x",
		CorrelationID: "c1",
		SessionKey:    "browser",
		TargetChatID:  "wc-tab-abc",
		PeerKind:      "direct",
		UserID:        "u1",
	}
	got := syntheticInboundFromDelivery("webchat-1", d)
	if got.Channel != "webchat-1" {
		t.Fatalf("Channel: %q", got.Channel)
	}
	if got.ChatID != "wc-tab-abc" {
		t.Fatalf("ChatID: %q", got.ChatID)
	}
	if got.Peer.Kind != "direct" || got.Peer.ID != "browser" {
		t.Fatalf("Peer: %+v", got.Peer)
	}
	if got.Content != "x" || got.MessageID != "c1" {
		t.Fatalf("content/id: %+v", got)
	}
}

func TestSyntheticInboundFromDelivery_defaultPeerKind(t *testing.T) {
	d := TurnDelivery{Text: "t", CorrelationID: "c", TargetChatID: "room"}
	got := syntheticInboundFromDelivery("cli", d)
	if got.Peer.Kind != "schedule" {
		t.Fatalf("Peer.Kind: %q", got.Peer.Kind)
	}
	if got.ChatID != "room" {
		t.Fatalf("ChatID: %q", got.ChatID)
	}
}
