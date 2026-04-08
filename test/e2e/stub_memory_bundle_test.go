//go:build e2e

package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/rtopts"
)

// E2E-20 project MEMORY.md（规则）出现在 BuildTurn 的 AgentMdBlock 中
func TestE2E_20_MemoryMDInAgentMdBlock(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	e2eIsolateUserMemory(t, home)
	lay := memory.DefaultLayout(cwd, home)
	if err := os.MkdirAll(lay.Project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lay.Project, "MEMORY.md"), []byte("E2E20_IDX_MARKER\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b := memory.BuildTurn(lay, home, "hi", nil, 0)
	if !strings.Contains(b.AgentMdBlock, "E2E20_IDX_MARKER") {
		t.Fatalf("agent md block missing marker:\n%s", b.AgentMdBlock)
	}
}

// E2E-21 MEMORY.md 行数截断含 WARNING（注入 AgentMdBlock）
func TestE2E_21_MemoryMDLineTruncationWarning(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	e2eIsolateUserMemory(t, home)
	lay := memory.DefaultLayout(cwd, home)
	if err := os.MkdirAll(lay.Project, 0o755); err != nil {
		t.Fatal(err)
	}
	var lines []string
	for i := 0; i < memory.MaxEntrypointLines+5; i++ {
		lines = append(lines, "line")
	}
	body := strings.Join(lines, "\n")
	if err := os.WriteFile(filepath.Join(lay.Project, "MEMORY.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	b := memory.BuildTurn(lay, home, "hi", nil, 0)
	if !strings.Contains(b.AgentMdBlock, "WARNING") || !strings.Contains(b.AgentMdBlock, "lines") {
		t.Fatalf("expected line-cap warning in:\n%s", b.AgentMdBlock)
	}
}

// E2E-22 MEMORY.md 字节截断含 WARNING（注入 AgentMdBlock）
func TestE2E_22_MemoryMDByteTruncationWarning(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	e2eIsolateUserMemory(t, home)
	lay := memory.DefaultLayout(cwd, home)
	if err := os.MkdirAll(lay.Project, 0o755); err != nil {
		t.Fatal(err)
	}
	// Few lines, total bytes > MaxEntrypointBytes → byte cap triggers
	line := strings.Repeat("x", 6000)
	body := strings.Join([]string{line, line, line, line, line}, "\n") // 5*6000 + 4 > 25_000
	if err := os.WriteFile(filepath.Join(lay.Project, "MEMORY.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	b := memory.BuildTurn(lay, home, "hi", nil, 0)
	if !strings.Contains(b.AgentMdBlock, "WARNING") {
		t.Fatalf("expected byte-cap warning in:\n%s", b.AgentMdBlock)
	}
}

// E2E-52 features.disable_auto_memory 关闭 auto 在 system 文案中的展示
func TestE2E_52_AutoMemoryDisabledOmitsAutoBullet(t *testing.T) {
	t.Cleanup(func() { rtopts.Set(nil) })
	s := rtopts.DefaultSnapshot()
	s.DisableAutoMemory = true
	rtopts.Set(&s)
	home := t.TempDir()
	cwd := t.TempDir()
	e2eIsolateUserMemory(t, home)
	lay := memory.DefaultLayout(cwd, home)
	b := memory.BuildTurn(lay, home, "hi", nil, 0)
	if strings.Contains(b.SystemSuffix, "**auto**") {
		t.Fatalf("auto bullet should be omitted: %s", b.SystemSuffix)
	}
}
