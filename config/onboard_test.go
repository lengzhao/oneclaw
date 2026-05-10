package config

import (
	"testing"

	cbconfig "github.com/lengzhao/clawbridge/config"
)

func TestUpsertClawbridgeClients_replaceByID(t *testing.T) {
	existing := []any{
		map[string]any{"id": "weixin-1", "driver": "weixin", "enabled": false},
		map[string]any{"id": "noop-1", "driver": "noop", "enabled": true},
	}
	incoming := []any{
		map[string]any{"id": "weixin-1", "driver": "weixin", "enabled": true, "options": map[string]any{"token": "x"}},
	}
	got := upsertClawbridgeClients(existing, incoming)
	if len(got) != 2 {
		t.Fatalf("len=%d want 2", len(got))
	}
	m1, _ := got[0].(map[string]any)
	if m1["enabled"] != true {
		t.Fatalf("weixin client not replaced: %#v", got[0])
	}
}

func TestMergeClawbridgeResultIntoRoot_mediaRoot(t *testing.T) {
	root := map[string]any{}
	cb := cbconfig.Config{
		Media: cbconfig.MediaConfig{Root: "/tmp/m"},
		Clients: []cbconfig.ClientConfig{
			{ID: "a", Driver: "noop", Enabled: true},
		},
	}
	changed, err := MergeClawbridgeResultIntoRoot(root, cb)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected changed")
	}
	cbSec := root["clawbridge"].(map[string]any)
	media := cbSec["media"].(map[string]any)
	if media["root"] != "/tmp/m" {
		t.Fatalf("media.root=%v", media["root"])
	}
}
