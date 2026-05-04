package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lengzhao/oneclaw/catalog"
	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/preturn"
	"github.com/lengzhao/oneclaw/tools/builtin"
)

// 记忆「召回」路径：workflow load_memory_snapshot → MemoryRecallSection（树状摘要），正文读取走 read_memory_month（见 tools_contract 回合测试）。

func TestE2E_memory_recallSection_listsWrittenMonthFile(t *testing.T) {
	tmp := t.TempDir()
	mm := memory.MonthUTC(time.Now().UTC())
	p := filepath.Join(tmp, "memory", mm, "extract-note.md")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("# facts\nTOKEN_MEM_DIGEST_q8w\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	block := preturn.MemoryRecallSection(tmp, preturn.DefaultBudget())
	if !strings.Contains(block, "## Memory recall") {
		t.Fatalf("missing header:\n%s", block)
	}
	// 树摘要只列路径与体积，不包含正文（正文靠 read_memory_month）。
	if !strings.Contains(block, "extract-note.md") {
		t.Fatalf("expected path in tree digest:\n%s", block)
	}
	if !strings.Contains(block, "read_memory_month") {
		t.Fatal("expected guidance to use read_memory_month for month files")
	}
}

func TestE2E_memory_extractor_builtin_hasMonthTools(t *testing.T) {
	cat, err := catalog.Load("")
	if err != nil {
		t.Fatal(err)
	}
	ag := cat.Get("memory_extractor")
	if ag == nil {
		t.Fatal("missing builtin memory_extractor")
	}
	want := map[string]bool{
		builtin.NameReadRunJournal:    false,
		builtin.NameWriteMemoryMonth:  false,
		builtin.NameAppendMemoryMonth: false,
		builtin.NameReadMemoryMonth:   false,
	}
	for _, n := range ag.Tools {
		if _, ok := want[n]; ok {
			want[n] = true
		}
	}
	for k, v := range want {
		if !v {
			t.Fatalf("memory_extractor agent missing tool %q (have %v)", k, ag.Tools)
		}
	}
}

func TestE2E_skill_generator_builtin_hasRunJournalTool(t *testing.T) {
	cat, err := catalog.Load("")
	if err != nil {
		t.Fatal(err)
	}
	ag := cat.Get("skill_generator")
	if ag == nil {
		t.Fatal("missing builtin skill_generator")
	}
	found := false
	for _, n := range ag.Tools {
		if n == builtin.NameReadRunJournal {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("skill_generator agent missing tool %q (have %v)", builtin.NameReadRunJournal, ag.Tools)
	}
}
