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

// 记忆「召回」路径：wfexec MemoryRecall（SQLite 为主 + 可选路径摘要）；正文读取走 read_file(memory/…)（见 tools_contract）。

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
	// 树摘要只列路径与体积，不包含正文。
	if !strings.Contains(block, "extract-note.md") {
		t.Fatalf("expected path in tree digest:\n%s", block)
	}
	if !strings.Contains(block, "read_file") || !strings.Contains(block, "memory/") {
		t.Fatal("expected guidance to use read_file under memory/")
	}
}

func TestE2E_memory_extractor_builtin_usesUnifiedFileTools(t *testing.T) {
	cat, err := catalog.Load("")
	if err != nil {
		t.Fatal(err)
	}
	ag := cat.Get("memory_extractor")
	if ag == nil {
		t.Fatal("missing builtin memory_extractor")
	}
	want := map[string]bool{
		builtin.NameReadRunJournal: false,
		builtin.NameReadFile:       false,
		builtin.NameWriteFile:      false,
	}
	for _, n := range ag.Tools {
		if _, ok := want[n]; ok {
			want[n] = true
		}
		if strings.Contains(n, "memory_month") {
			t.Fatalf("memory_extractor should use unified file tools, got %q in %v", n, ag.Tools)
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
	foundJournal := false
	foundWrite := false
	for _, n := range ag.Tools {
		if n == builtin.NameReadRunJournal {
			foundJournal = true
		}
		if n == builtin.NameWriteFile {
			foundWrite = true
		}
		if strings.Contains(n, "skill_file") {
			t.Fatalf("skill_generator should use unified write_file, got %q in %v", n, ag.Tools)
		}
	}
	if !foundJournal {
		t.Fatalf("skill_generator agent missing tool %q (have %v)", builtin.NameReadRunJournal, ag.Tools)
	}
	if !foundWrite {
		t.Fatalf("skill_generator agent missing tool %q (have %v)", builtin.NameWriteFile, ag.Tools)
	}
}
