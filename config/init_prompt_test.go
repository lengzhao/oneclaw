package config

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestPromptInitIfTerminalSkipsNonTTY(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	before := "openai:\n  api_key: \"\"\n  base_url: \"\"\nmodel: gpt-4o\n"
	if err := os.WriteFile(cfgPath, []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}

	devNull, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0)
	if err != nil {
		t.Skip("no /dev/null:", err)
	}
	defer devNull.Close()

	if err := PromptInitIfTerminal(cfgPath, devNull, io.Discard); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != before {
		t.Fatalf("non-tty run should not rewrite config; got:\n%s", string(after))
	}
}
