package workflow

import "testing"

func TestPhase3Uses_initializedFromBuiltinUses(t *testing.T) {
	if len(Phase3Uses) < len(Phase3BuiltinUses) {
		t.Fatalf("Phase3Uses smaller than slice: %d vs %d", len(Phase3Uses), len(Phase3BuiltinUses))
	}
	for _, u := range Phase3BuiltinUses {
		if _, ok := Phase3Uses[u]; !ok {
			t.Fatalf("Phase3Uses missing %q", u)
		}
	}
}
