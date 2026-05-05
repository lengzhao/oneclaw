package config

import "testing"

func TestUpsertModelProfile_replaceByID(t *testing.T) {
	f := &File{
		Models: []ModelProfile{
			{ID: "default", Provider: "openai_compatible"},
			{ID: "backup", Provider: "mock"},
		},
	}
	UpsertModelProfile(f, ModelProfile{ID: "default", Provider: "mock"})
	if f.Models[0].Provider != "mock" {
		t.Fatalf("got %q", f.Models[0].Provider)
	}
	if len(f.Models) != 2 {
		t.Fatalf("len %d", len(f.Models))
	}
}

func TestUpsertModelProfile_append(t *testing.T) {
	f := &File{Models: []ModelProfile{{ID: "a"}}}
	UpsertModelProfile(f, ModelProfile{ID: "b", Provider: "mock"})
	if len(f.Models) != 2 || f.Models[1].ID != "b" {
		t.Fatalf("%+v", f.Models)
	}
}
