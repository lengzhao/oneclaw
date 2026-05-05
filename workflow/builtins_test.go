package workflow

import "testing"

func TestAllowedUses_initializedFromBuiltinUses(t *testing.T) {
	if len(AllowedUses) < len(BuiltinUses) {
		t.Fatalf("AllowedUses smaller than slice: %d vs %d", len(AllowedUses), len(BuiltinUses))
	}
	for _, u := range BuiltinUses {
		if _, ok := AllowedUses[u]; !ok {
			t.Fatalf("AllowedUses missing %q", u)
		}
	}
}
