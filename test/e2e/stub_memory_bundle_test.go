package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/memory"
)

// E2E-20 project MEMORY.md 出现在 BuildTurn 的 system 后缀中（公开 API 子切面）
func TestE2E_20_MemoryMDInSystemSuffix(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	lay := memory.DefaultLayout(cwd, home)
	if err := os.MkdirAll(lay.Project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lay.Project, "MEMORY.md"), []byte("E2E20_IDX_MARKER\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b := memory.BuildTurn(lay, home, "hi", nil)
	if !strings.Contains(b.SystemSuffix, "E2E20_IDX_MARKER") {
		t.Fatalf("system suffix missing marker:\n%s", b.SystemSuffix)
	}
}

// E2E-21 MEMORY.md 行数截断含 WARNING
func TestE2E_21_MemoryMDLineTruncationWarning(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
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
	b := memory.BuildTurn(lay, home, "hi", nil)
	if !strings.Contains(b.SystemSuffix, "WARNING") || !strings.Contains(b.SystemSuffix, "lines") {
		t.Fatalf("expected line-cap warning in:\n%s", b.SystemSuffix)
	}
}

// E2E-22 MEMORY.md 字节截断含 WARNING
func TestE2E_22_MemoryMDByteTruncationWarning(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
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
	b := memory.BuildTurn(lay, home, "hi", nil)
	if !strings.Contains(b.SystemSuffix, "WARNING") {
		t.Fatalf("expected byte-cap warning in:\n%s", b.SystemSuffix)
	}
}

// E2E-52 ONCLAW_DISABLE_AUTO_MEMORY 关闭 auto 在 system 文案中的展示
func TestE2E_52_AutoMemoryDisabledOmitsAutoBullet(t *testing.T) {
	t.Setenv("ONCLAW_DISABLE_AUTO_MEMORY", "1")
	home := t.TempDir()
	cwd := t.TempDir()
	lay := memory.DefaultLayout(cwd, home)
	b := memory.BuildTurn(lay, home, "hi", nil)
	if strings.Contains(b.SystemSuffix, "**auto**") {
		t.Fatalf("auto bullet should be omitted: %s", b.SystemSuffix)
	}
}
