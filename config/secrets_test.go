package config

import (
	"testing"
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
