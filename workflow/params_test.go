package workflow

import "testing"

func TestParseAgentContextAttachments_explicitContext(t *testing.T) {
	p := map[string]any{
		"context": []any{
			map[string]any{"ref": "run_journal", "scope": "full"},
		},
	}
	atts := ParseAgentContextAttachments(p)
	if len(atts) != 1 || atts[0].Ref != AgentContextRefRunJournal || atts[0].Scope != AgentContextScopeFull {
		t.Fatalf("got %+v", atts)
	}
}

func TestParseAgentContextAttachments_userSourceAloneIgnored(t *testing.T) {
	atts := ParseAgentContextAttachments(map[string]any{"user_source": "run_journal"})
	if len(atts) != 0 {
		t.Fatalf("legacy user_source must not synthesize context; got %+v", atts)
	}
}
