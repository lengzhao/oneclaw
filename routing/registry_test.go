package routing

import "testing"

func TestMapRegistry(t *testing.T) {
	r := NewMapRegistry()
	if _, err := r.SinkFor("cli"); err == nil {
		t.Fatal("expected error")
	}
	var noop NoopSink
	r.Register("cli", noop)
	s, err := r.SinkFor("cli")
	if err != nil || s == nil {
		t.Fatalf("SinkFor: %v %v", s, err)
	}
}
