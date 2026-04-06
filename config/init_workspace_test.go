package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/logx"
	"github.com/lengzhao/oneclaw/memory"
)

func TestInitWorkspaceWritesConfigAndDirs(t *testing.T) {
	logx.Init("error", "text")
	home := t.TempDir()
	cwd := t.TempDir()
	if err := config.InitWorkspace(cwd, home); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(cwd, memory.DotDir, "config.yaml")
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("config: %v", err)
	}
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 100 || raw[0] != '#' {
		t.Fatalf("unexpected config content")
	}
	// Second run: must not error and must not truncate config
	if err := config.InitWorkspace(cwd, home); err != nil {
		t.Fatal(err)
	}
	raw2, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != string(raw2) {
		t.Fatal("config should be unchanged on second init")
	}
}
