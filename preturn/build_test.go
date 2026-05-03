package preturn

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/catalog"
	"github.com/lengzhao/oneclaw/memory"
)

func TestBuild_omitMemory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENT.md"), []byte("agent"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "MEMORY.md"), []byte("SECRET_MEMORY"), 0o644); err != nil {
		t.Fatal(err)
	}
	ag := &catalog.Agent{AgentType: "x"}
	bundle, err := Build(dir, dir, ag, DefaultBudget(), &BuildOpts{OmitMemory: true})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(bundle.Instruction, "SECRET_MEMORY") {
		t.Fatalf("memory should be omitted: %q", bundle.Instruction)
	}
	bundle2, err := Build(dir, dir, ag, DefaultBudget(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(bundle2.Instruction, "SECRET_MEMORY") {
		t.Fatalf("expected MEMORY in instruction: %q", bundle2.Instruction)
	}
}

func TestBuild_memoryMDByteCap(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENT.md"), []byte("agent"), 0o644); err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("m", memory.MEMORYMDMaxBytes+400)
	if err := os.WriteFile(filepath.Join(dir, "MEMORY.md"), []byte(long), 0o644); err != nil {
		t.Fatal(err)
	}
	bundle, err := Build(dir, dir, &catalog.Agent{AgentType: "x"}, Budget{MemoryMaxRunes: 1 << 20}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(bundle.Instruction, "MEMORY snapshot") {
		t.Fatalf("missing snapshot header: %q", bundle.Instruction)
	}
	if !strings.Contains(bundle.Instruction, "truncated") {
		t.Fatal("expected MEMORY truncation marker for oversized MEMORY.md")
	}
	blockLen := strings.Index(bundle.Instruction, "## MEMORY snapshot")
	if blockLen < 0 {
		t.Fatal("snapshot header missing")
	}
	snapshot := bundle.Instruction[blockLen:]
	if len([]byte(snapshot)) > memory.MEMORYMDMaxBytes+200 {
		t.Fatalf("snapshot block unexpectedly large: %d bytes", len([]byte(snapshot)))
	}
}

func TestMemoryRecallSection(t *testing.T) {
	dir := t.TempDir()
	if s := MemoryRecallSection(dir, DefaultBudget()); s != "" {
		t.Fatalf("expected empty, got %q", s)
	}
	memDir := filepath.Join(dir, "memory", "2026-05")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(memDir, "x.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := MemoryRecallSection(dir, DefaultBudget())
	if !strings.Contains(s, "memory/2026-05/x.md") {
		t.Fatalf("got %q", s)
	}
}

func TestBuild_memoryExtractorMemoryTree(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory", "2026-05")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(memDir, "note.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ag := &catalog.Agent{AgentType: "memory_extractor"}
	bundle, err := Build(dir, dir, ag, DefaultBudget(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(bundle.Instruction, "## Memory folder") {
		t.Fatalf("expected memory folder section: %q", bundle.Instruction)
	}
	if !strings.Contains(bundle.Instruction, "memory/2026-05/note.md") {
		t.Fatalf("expected listed md path: %q", bundle.Instruction)
	}
}
