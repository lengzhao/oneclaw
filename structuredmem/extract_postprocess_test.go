package structuredmem

import (
	"os"
	"strings"
	"testing"
	"time"

	lzmem "github.com/lengzhao/memory"

	memcore "github.com/lengzhao/oneclaw/memory"
)

func TestApplyExtractPostprocess_dropsTransientClockFacts(t *testing.T) {
	res := &lzmem.ExtractResult{
		Status:       "completed",
		ExtractionID: "ext-1",
		Memories: []lzmem.ExtractedMemory{
			{
				Namespace:  lzmem.NamespaceTransient,
				Title:      "当前时间",
				Summary:    "现在是 2026-05-05（UTC）",
				Confidence: 0.95,
			},
			{
				Namespace:  lzmem.NamespaceKnowledge,
				Title:      "会议主题",
				Summary:    "明天 PMO + AI Infra 对齐",
				Confidence: 0.9,
			},
		},
	}
	ApplyExtractPostprocess(t.TempDir(), time.Now().UTC(), res)
	if len(res.Memories) != 1 {
		t.Fatalf("expected transient dropped, got %+v", res.Memories)
	}
	if !strings.EqualFold(string(res.Memories[0].Namespace), string(lzmem.NamespaceKnowledge)) {
		t.Fatalf("unexpected memory kept: %+v", res.Memories[0])
	}
}

func TestApplyExtractPostprocess_resolvesPMOAIInfraMeetingCountConflict(t *testing.T) {
	dir := t.TempDir()
	res := &lzmem.ExtractResult{
		Status:       "completed",
		ExtractionID: "ext-2",
		Memories: []lzmem.ExtractedMemory{
			{
				Namespace:  lzmem.NamespaceAction,
				Title:      "两场会议待办",
				Summary:    "把明天的两场会议拆成两条子任务（PMO / AI Infra）",
				Confidence: 0.9,
			},
			{
				Namespace:  lzmem.NamespaceKnowledge,
				Title:      "单场会议",
				Summary:    "明天 PMO + AI Infra 是同一场会议（不是两场分开的）",
				Confidence: 0.85,
			},
		},
	}
	at := time.Now().UTC()
	ApplyExtractPostprocess(dir, at, res)

	for _, m := range res.Memories {
		if strings.TrimSpace(string(m.Namespace)) == string(lzmem.NamespaceAction) && strings.Contains(strings.TrimSpace(m.Title), "两场会议待办") {
			t.Fatalf("expected conflicting action memory removed after single-meeting clarification, got %+v", res.Memories)
		}
	}

	rel, err := memcore.NormalizeMemoryMonthRel("")
	if err != nil {
		t.Fatal(err)
	}
	abs, err := memcore.ResolveMemoryMonthMarkdown(dir, rel)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("expected conflict markdown appended: %v", err)
	}
	body := string(raw)
	if !strings.Contains(body, "structuredmem conflict resolution") {
		t.Fatalf("expected conflict audit fragment in journal, got:\n%s", body)
	}
}
