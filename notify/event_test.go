package notify

import (
	"encoding/json"
	"testing"
)

func TestPreview(t *testing.T) {
	if got := Preview("hello", 3); got != "hel…" {
		t.Fatalf("got %q", got)
	}
	if Preview("", 10) != "" {
		t.Fatal("empty")
	}
}

func TestNewEventJSON(t *testing.T) {
	ev := NewEvent(EventUserInput, "")
	ev.SessionID = "s1"
	ev.AgentID = "main"
	ev.Data["x"] = 1
	b, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m["schema_version"].(float64) != 3 || m["event"] != EventUserInput {
		t.Fatalf("%v", m)
	}
	ts, ok := m["ts"].(float64)
	if !ok || ts <= 0 {
		t.Fatalf("ts=%v", m["ts"])
	}
}
