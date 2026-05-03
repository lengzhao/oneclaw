package config

import "testing"

func TestOrderedModelProfiles_priority(t *testing.T) {
	f := &File{
		Models: []ModelProfile{
			{ID: "slow", Priority: 100},
			{ID: "fast", Priority: 0},
			{ID: "mid", Priority: 50},
		},
	}
	ApplyDefaults(f)
	ord := OrderedModelProfiles(f)
	if len(ord) != 3 {
		t.Fatal(len(ord))
	}
	if ord[0].ID != "fast" || ord[1].ID != "mid" || ord[2].ID != "slow" {
		t.Fatalf("got %v %v %v", ord[0].ID, ord[1].ID, ord[2].ID)
	}
}
