package config

import "testing"

func TestValidate_apiKeyEnvMustBeEnvName(t *testing.T) {
	f := &File{
		Models: []ModelProfile{{
			ID:          "default",
			APIKeyEnv:   "sk-fake-secret-not-env-name",
			BaseURL:     "https://api.example.com/v1",
			DefaultModel: "x",
		}},
	}
	ApplyDefaults(f)
	if err := Validate(f); err == nil {
		t.Fatal("expected error")
	}
}
