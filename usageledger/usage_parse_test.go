package usageledger

import "testing"

func TestParseCostUSDFromUsageJSON(t *testing.T) {
	got, ok := ParseCostUSDFromUsageJSON(`{"prompt_tokens":1,"cost_usd":0.0042}`)
	if !ok || got != 0.0042 {
		t.Fatalf("cost_usd: got %v ok=%v", got, ok)
	}
	got2, ok2 := ParseCostUSDFromUsageJSON(`{"prompt_tokens":1,"cost":0.01}`)
	if !ok2 || got2 != 0.01 {
		t.Fatalf("cost: got %v ok=%v", got2, ok2)
	}
	_, ok3 := ParseCostUSDFromUsageJSON(`{"prompt_tokens":10,"completion_tokens":2}`)
	if ok3 {
		t.Fatal("expected no cost")
	}
}
