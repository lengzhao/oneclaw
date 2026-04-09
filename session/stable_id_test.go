package session

import "testing"

func TestStableSessionID_stable(t *testing.T) {
	h := SessionHandle{Source: "slack", SessionKey: "thread-1"}
	a := StableSessionID(h)
	b := StableSessionID(h)
	if a != b || len(a) != 24 {
		t.Fatalf("got %q len=%d", a, len(a))
	}
}

func TestStableSessionID_distinctKeys(t *testing.T) {
	a := StableSessionID(SessionHandle{Source: "slack", SessionKey: "a"})
	b := StableSessionID(SessionHandle{Source: "slack", SessionKey: "b"})
	if a == b {
		t.Fatal("expected different ids")
	}
}

func TestStableSessionID_emptySessionKeyUsesDefaultSlot(t *testing.T) {
	a := StableSessionID(SessionHandle{Source: "http", SessionKey: ""})
	b := StableSessionID(SessionHandle{Source: "http", SessionKey: ""})
	if a != b {
		t.Fatal("expected same default slot")
	}
}
