package config

import "testing"

func TestFile_ResolvedDefaultTurn(t *testing.T) {
	var f File
	f.Catalog.Workflows.DefaultTurn = "custom.turn"
	if f.ResolvedDefaultTurn() != "custom.turn" {
		t.Fatalf("got %q", f.ResolvedDefaultTurn())
	}
	if (*File)(nil).ResolvedDefaultTurn() != "default.turn" {
		t.Fatalf("nil File default turn")
	}
}

func TestFile_ResolvedDefaultAgent(t *testing.T) {
	var f File
	f.Catalog.DefaultAgent = "worker"
	if f.ResolvedDefaultAgent() != "worker" {
		t.Fatalf("got %q", f.ResolvedDefaultAgent())
	}
	if (*File)(nil).ResolvedDefaultAgent() != "default" {
		t.Fatalf("nil File default agent")
	}
	f2 := File{}
	if f2.ResolvedDefaultAgent() != "default" {
		t.Fatalf("empty catalog default agent")
	}
}
