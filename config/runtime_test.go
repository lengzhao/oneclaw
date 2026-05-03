package config

import (
	"testing"
)

func TestPushRuntime_roundTrip(t *testing.T) {
	defer PushRuntime(nil)
	PushRuntime(nil)
	if Runtime() != nil {
		t.Fatal("expected nil")
	}
	f := &File{
		Models: []ModelProfile{{ID: "p", DefaultModel: "x"}},
	}
	ApplyDefaults(f)
	PushRuntime(f)
	v := Runtime()
	if v == nil || len(v.Config.Models) != 1 || v.Config.Models[0].DefaultModel != "x" {
		t.Fatalf("snapshot %+v", v)
	}
	f.Models[0].DefaultModel = "mutate-after-push"
	if Runtime().Config.Models[0].DefaultModel != "x" {
		t.Fatal("snapshot should be cloned")
	}
}
