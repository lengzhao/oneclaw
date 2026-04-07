package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/test/openaistub"
	"gopkg.in/yaml.v3"
)

// e2eBuildHome is HOME at package init (before any t.Setenv); go build must not use test HOME=t.TempDir().
var e2eBuildHome = os.Getenv("HOME")

// mergeEnv starts from os.Environ, then overrides keys (pairs: k,v,k,v...).
func mergeEnv(pairs ...string) []string {
	if len(pairs)%2 != 0 {
		panic("mergeEnv: odd pair count")
	}
	m := make(map[string]string)
	for _, e := range os.Environ() {
		k, v, ok := strings.Cut(e, "=")
		if ok {
			m[k] = v
		}
	}
	for i := 0; i < len(pairs); i += 2 {
		m[pairs[i]] = pairs[i+1]
	}
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	return out
}

func buildOneclawBinary(t *testing.T, repoRoot string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "oneclaw_e2e")
	build := exec.Command("go", "build", "-o", bin, "./cmd/oneclaw")
	build.Dir = repoRoot
	buildHome := e2eBuildHome
	if buildHome == "" {
		var err error
		buildHome, err = os.UserHomeDir()
		if err != nil {
			t.Fatalf("home for go build: %v", err)
		}
	}
	build.Env = mergeEnv("HOME", buildHome)
	out, err := build.CombinedOutput()
	if err != nil {
		t.Fatalf("go build ./cmd/oneclaw: %v\n%s", err, out)
	}
	return bin
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		nd := filepath.Dir(dir)
		if nd == dir {
			t.Fatal("go.mod not found from cwd")
		}
		dir = nd
	}
}

// E2E-96 oneclaw -maintain-once：子进程 + stub，一轮蒸馏写入 project `.oneclaw/memory/YYYY-MM-DD.md`
func TestE2E_96_MaintainCLIOnce(t *testing.T) {
	stub := openaistub.New(t)
	date := time.Now().Format("2006-01-02")
	section := "## Auto-maintained (" + date + ")\n- E2E96_CLI_MAINTAIN_MARKER\n"
	stub.Enqueue(openaistub.CompletionStop("", section))

	home := t.TempDir()
	cwd := t.TempDir()
	t.Setenv("HOME", home)
	memBase := filepath.Join(home, memory.DotDir)
	if err := os.MkdirAll(filepath.Join(cwd, memory.DotDir), 0o755); err != nil {
		t.Fatal(err)
	}
	var projCfg config.File
	projCfg.OpenAI.APIKey = "sk-test-stub"
	projCfg.OpenAI.BaseURL = stub.BaseURL()
	projCfg.Chat.Transport = "non_stream" // stub: avoid stream+non_stream double dequeue
	projCfg.Paths.MemoryBase = memBase
	projCfg.Maintain.Interval = "1h"
	projCfg.Maintain.Model = "gpt-4o"
	projCfg.Maintain.MinLogBytes = 50
	cfgBody, err := yaml.Marshal(&projCfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, memory.DotDir, "config.yaml"), cfgBody, 0o644); err != nil {
		t.Fatal(err)
	}

	lay := memory.DefaultLayout(cwd, home)
	logPath := memory.DailyLogPath(lay.Auto, date)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		t.Fatal(err)
	}
	logBody := strings.Repeat("e2e-96 daily log padding for min bytes. ", 20)
	if err := os.WriteFile(logPath, []byte(logBody), 0o644); err != nil {
		t.Fatal(err)
	}

	bin := buildOneclawBinary(t, repoRoot(t))
	cmd := exec.Command(bin, "-cwd", cwd, "-maintain-once")
	cmd.Env = mergeEnv("HOME", home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("oneclaw -maintain-once: %v\n%s", err, out)
	}

	epPath := memory.ProjectEpisodeDailyPath(cwd, date)
	raw, err := os.ReadFile(epPath)
	if err != nil {
		t.Fatalf("episodic digest: %v", err)
	}
	if !strings.Contains(string(raw), "E2E96_CLI_MAINTAIN_MARKER") {
		t.Fatalf("expected marker in:\n%s", string(raw))
	}
}

// E2E-97 oneclaw -init：子进程写入项目 .oneclaw/config.yaml（无需 API）
func TestE2E_97_OneclawInitWritesProjectConfig(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	t.Setenv("HOME", home)

	bin := buildOneclawBinary(t, repoRoot(t))
	cmd := exec.Command(bin, "-cwd", cwd, "-init", "-log-level", "error")
	cmd.Env = mergeEnv("HOME", home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("oneclaw -init: %v\n%s", err, out)
	}

	cfgPath := filepath.Join(cwd, memory.DotDir, "config.yaml")
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("config.yaml: %v", err)
	}
	if !strings.Contains(string(raw), "openai:") {
		t.Fatalf("expected openai section in:\n%s", string(raw))
	}
}
