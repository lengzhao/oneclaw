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
		DefaultModel: "p/api-model",
		Models:       []ModelProfile{{ID: "p"}},
	}
	ApplyDefaults(f)
	PushRuntime(f)
	v := Runtime()
	if v == nil || len(v.Config.Models) != 1 || v.Config.DefaultModel != "p/api-model" {
		t.Fatalf("snapshot %+v", v)
	}
	f.DefaultModel = "mutate-after-push"
	if Runtime().Config.DefaultModel != "p/api-model" {
		t.Fatal("snapshot should be cloned")
	}
}
