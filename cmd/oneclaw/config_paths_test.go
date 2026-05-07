package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMergedConfigPaths_explicitConfig(t *testing.T) {
	g := globalOpts{ConfigPath: "/tmp/x.yaml"}
	got, err := mergedConfigPaths(g)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "/tmp/x.yaml" {
		t.Fatalf("%v", got)
	}
}

func TestMergedConfigPaths_defaultWhenFileExists(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ONECLAW_USER_DATA_ROOT", root)
	cfg := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(cfg, []byte("catalog:\n  default_agent: default\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := mergedConfigPaths(globalOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != cfg {
		t.Fatalf("%v", got)
	}
}
