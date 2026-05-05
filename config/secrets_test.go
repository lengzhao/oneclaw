package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lengzhao/oneclaw/keyfiles"
)

func TestApplyEnvSecrets(t *testing.T) {
	t.Setenv("ONECLAW_TEST_KEY", "secret")
	f := &File{
		Models: []ModelProfile{
			{ID: "t", APIKeyEnv: "ONECLAW_TEST_KEY"},
		},
	}
	ApplyDefaults(f)
	ApplyEnvSecrets(f)
	if f.Models[0].APIKey != "secret" {
		t.Fatalf("api key %q", f.Models[0].APIKey)
	}
}

func TestApplyAuthTokenFiles(t *testing.T) {
	root := t.TempDir()
	kf := filepath.Join(root, "key_files")
	if err := os.MkdirAll(kf, 0o700); err != nil {
		t.Fatal(err)
	}
	rel := "key_files/dashscope.json"
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := keyfiles.MergeWriteAPIKey(p, "dashscope-key"); err != nil {
		t.Fatal(err)
	}
	f := &File{
		Models: []ModelProfile{{
			ID:   "default",
			Auth: ModelAuth{TokenFile: rel},
		}},
	}
	ApplyDefaults(f)
	ApplyAuthTokenFiles(root, f)
	if f.Models[0].APIKey != "dashscope-key" {
		t.Fatalf("got %q", f.Models[0].APIKey)
	}
}
