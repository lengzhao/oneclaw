package schedule

import "testing"

func TestSyntheticInboundFromDelivery_setsSessionIDAndPeerKind(t *testing.T) {
	d := TurnDelivery{
		Text:            "x",
		CorrelationID:   "c1",
		SessionKey:      "browser",
		TargetSessionID: "wc-tab-abc",
		PeerKind:        "direct",
		UserID:          "u1",
	}
	got := syntheticInboundFromDelivery("webchat-1", d)
	if got.ClientID != "webchat-1" {
		t.Fatalf("ClientID: %q", got.ClientID)
	}
	if got.SessionID != "wc-tab-abc" {
		t.Fatalf("SessionID: %q", got.SessionID)
	}
	if got.Peer.Kind != "direct" || got.Peer.ID != "browser" {
		t.Fatalf("Peer: %+v", got.Peer)
	}
	if got.Content != "x" || got.MessageID != "c1" {
		t.Fatalf("content/id: %+v", got)
	}
	if got.Metadata == nil || got.Metadata[MetadataKeySyntheticScheduleFire] != "1" {
		t.Fatalf("expected synthetic schedule metadata, got %#v", got.Metadata)
	}
	if !IsSyntheticScheduleInbound(&got) {
		t.Fatal("IsSyntheticScheduleInbound should be true")
	}
}

func TestSyntheticInboundFromDelivery_defaultPeerKind(t *testing.T) {
	d := TurnDelivery{Text: "t", CorrelationID: "c", TargetSessionID: "room"}
	got := syntheticInboundFromDelivery("cli", d)
	if got.Peer.Kind != "schedule" {
		t.Fatalf("Peer.Kind: %q", got.Peer.Kind)
	}
	if got.SessionID != "room" {
		t.Fatalf("SessionID: %q", got.SessionID)
	}
}
