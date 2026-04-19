package usageledger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/rtopts"
)

func TestUserInteractionKey(t *testing.T) {
	k := UserInteractionKey(bus.InboundMessage{
		ClientID: "slack",
		Sender:  bus.SenderInfo{CanonicalID: "U1", Platform: "T"},
	})
	if k != "T/U1@slack" {
		t.Fatalf("got %q", k)
	}
	k2 := UserInteractionKey(bus.InboundMessage{
		ClientID: "cli",
		Peer:    bus.Peer{ID: "th"},
	})
	if k2 != "session:th@cli" {
		t.Fatalf("got %q", k2)
	}
}

func TestMaybeRecord_writesFiles(t *testing.T) {
	t.Cleanup(func() { rtopts.Set(nil) })
	rtopts.Set(&rtopts.Snapshot{})
	cwd := t.TempDir()
	MaybeRecord(RecordParams{
		CWD:              cwd,
		SessionID:        "s1",
		Model:            "gpt-4o",
		Step:             0,
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		UsageJSON:        `{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150}`,
		Inbound: bus.InboundMessage{
			ClientID: "cli",
			Sender:  bus.SenderInfo{CanonicalID: "alice"},
		},
	})
	root := filepath.Join(cwd, "usage")
	raw, err := os.ReadFile(filepath.Join(root, "interactions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, `"prompt_tokens":100`) || !strings.Contains(s, `"usage":`) {
		t.Fatalf("interactions line: %s", raw)
	}
	if strings.Contains(s, `"cost_usd"`) {
		t.Fatalf("expected no cost_usd without API cost or usage_estimate_cost: %s", raw)
	}
	entries, _ := filepath.Glob(filepath.Join(root, "daily", "*.json"))
	if len(entries) != 1 {
		t.Fatalf("daily files = %d", len(entries))
	}
	var dayRoll Rollup
	if b, err := os.ReadFile(entries[0]); err != nil {
		t.Fatal(err)
	} else if err := json.Unmarshal(b, &dayRoll); err != nil {
		t.Fatal(err)
	} else if dayRoll.PromptTokens != 100 || dayRoll.CompletionTokens != 50 {
		t.Fatalf("daily rollup %+v", dayRoll)
	}
	userFiles, _ := filepath.Glob(filepath.Join(root, "users", "*.json"))
	if len(userFiles) != 1 {
		t.Fatalf("user files = %d", len(userFiles))
	}
}

func TestMaybeRecord_costFromUsageJSON(t *testing.T) {
	t.Cleanup(func() { rtopts.Set(nil) })
	rtopts.Set(&rtopts.Snapshot{})
	cwd := t.TempDir()
	MaybeRecord(RecordParams{
		CWD:              cwd,
		Model:            "gpt-4o",
		PromptTokens:     10,
		CompletionTokens: 5,
		TotalTokens:      15,
		UsageJSON:        `{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"cost_usd":0.001}`,
		Inbound:          bus.InboundMessage{ClientID: "cli"},
	})
	raw, err := os.ReadFile(filepath.Join(cwd, "usage", "interactions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"cost_source":"usage"`) || !strings.Contains(string(raw), `"cost_usd":0.001`) {
		t.Fatalf("want usage-sourced cost: %s", raw)
	}
}

func TestMaybeRecord_estimateCostEnv(t *testing.T) {
	t.Cleanup(func() { rtopts.Set(nil) })
	rtopts.Set(&rtopts.Snapshot{UsageEstimateCost: true})
	cwd := t.TempDir()
	MaybeRecord(RecordParams{
		CWD:              cwd,
		Model:            "gpt-4o",
		PromptTokens:     1_000_000,
		CompletionTokens: 0,
		TotalTokens:      1_000_000,
		UsageJSON:        `{"prompt_tokens":1000000,"completion_tokens":0,"total_tokens":1000000}`,
		Inbound:          bus.InboundMessage{ClientID: "cli"},
	})
	raw, err := os.ReadFile(filepath.Join(cwd, "usage", "interactions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"cost_source":"estimated"`) {
		t.Fatalf("want estimated cost: %s", raw)
	}
}

func TestMaybeRecord_skipsWhenDisabled(t *testing.T) {
	t.Cleanup(func() { rtopts.Set(nil) })
	rtopts.Set(&rtopts.Snapshot{DisableUsageLedger: true})
	cwd := t.TempDir()
	MaybeRecord(RecordParams{CWD: cwd, Model: "gpt-4o", PromptTokens: 1, CompletionTokens: 1, Inbound: bus.InboundMessage{ClientID: "x"}})
	if _, err := os.Stat(filepath.Join(cwd, "usage")); err == nil {
		t.Fatal("expected no usage dir when disabled")
	}
}
