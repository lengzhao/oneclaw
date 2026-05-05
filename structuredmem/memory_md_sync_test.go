package structuredmem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	lzmem "github.com/lengzhao/memory"

	memcore "github.com/lengzhao/oneclaw/memory"
)

func TestFormatExtractResultMarkdown_emptyMemories(t *testing.T) {
	got := FormatExtractResultMarkdown(time.Now().UTC(), &lzmem.ExtractResult{
		Status:       "completed",
		ExtractionID: "ext-empty",
		Memories:     nil,
	})
	if strings.TrimSpace(got) != "" {
		t.Fatalf("expected empty markdown, got %q", got)
	}
}

func TestSyncMemoryMDFromExtract_promotesAssistantName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "MEMORY.md")
	initial := "# Memory\n\n## Notes\n\n- baseline\n"
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}
	res := &lzmem.ExtractResult{
		Memories: []lzmem.ExtractedMemory{
			{
				Namespace:  lzmem.NamespaceProfile,
				Summary:    "用户希望我称呼他为“庆哥”。",
				Content:    "用户明确说：请叫我“庆哥”。",
				Confidence: 0.9,
				Tags:       []string{"user", "preferred_name"},
			},
		},
	}
	if err := SyncMemoryMDFromExtract(dir, res); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if !strings.Contains(body, "庆哥") {
		t.Fatalf("expected user-facing naming preference promoted into MEMORY.md, got:\n%s", body)
	}
	if !strings.Contains(body, autoStartMarker) || !strings.Contains(body, autoEndMarker) {
		t.Fatalf("expected auto section markers, got:\n%s", body)
	}
}

func TestSyncMemoryMDFromExtract_respectsMemoryMDBudget(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "MEMORY.md")
	almostFull := strings.Repeat("x", memcore.MEMORYMDMaxBytes-64)
	if err := os.WriteFile(path, []byte(almostFull), 0o644); err != nil {
		t.Fatal(err)
	}
	res := &lzmem.ExtractResult{
		Memories: []lzmem.ExtractedMemory{
			{
				Namespace:  lzmem.NamespaceProfile,
				Summary:    "请叫我小张（preferred name）",
				Confidence: 0.95,
				Tags:       []string{"user", "preferred_name"},
			},
		},
	}
	if err := SyncMemoryMDFromExtract(dir, res); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > memcore.MEMORYMDMaxBytes {
		t.Fatalf("MEMORY.md exceeds max bytes: got=%d max=%d", len(raw), memcore.MEMORYMDMaxBytes)
	}
}
