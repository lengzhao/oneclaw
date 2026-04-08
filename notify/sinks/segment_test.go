package sinks

import "testing"

func TestSanitizeAgentSegment(t *testing.T) {
	if got := SanitizeAgentSegment(""); got != "_default" {
		t.Fatalf("%q", got)
	}
	if got := SanitizeAgentSegment("My Agent!"); got != "My_Agent_" {
		t.Fatalf("%q", got)
	}
	if got := SanitizeAgentSegment("ab"); got != "ab" {
		t.Fatalf("%q", got)
	}
}

func TestOptionsSegment(t *testing.T) {
	o := Options{AgentID: "x/y"}
	if o.Segment() != "x_y" {
		t.Fatalf("%q", o.Segment())
	}
	o2 := Options{AgentID: "a", AgentSegment: "custom.name"}
	if o2.Segment() != "custom.name" {
		t.Fatalf("%q", o2.Segment())
	}
}
