package catalog

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestManifest_ResolvedDefaultTurn_nestedAndLegacy(t *testing.T) {
	var m Manifest
	if err := yaml.Unmarshal([]byte(`workflows:
  default_turn: wf.nested
`), &m); err != nil {
		t.Fatal(err)
	}
	if m.ResolvedDefaultTurn() != "wf.nested" {
		t.Fatal(m.ResolvedDefaultTurn())
	}
	var m2 Manifest
	if err := yaml.Unmarshal([]byte(`default_turn: wf.legacy
`), &m2); err != nil {
		t.Fatal(err)
	}
	if m2.ResolvedDefaultTurn() != "wf.legacy" {
		t.Fatal(m2.ResolvedDefaultTurn())
	}
	if (*Manifest)(nil).ResolvedDefaultTurn() != "default.turn" {
		t.Fatal()
	}
}
