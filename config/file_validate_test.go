package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestUnmarshal_ModelAuth_rejectsLegacyProviderGrantKind(t *testing.T) {
	for _, raw := range []string{
		`models:
  - id: default
    provider: openai_compatible
    auth:
      provider: alibaba
      token_file: key_files/x.json
`,
		`models:
  - id: default
    provider: openai_compatible
    auth:
      grant: oauth_loopback
`,
		`models:
  - id: default
    provider: openai_compatible
    auth:
      kind: alibaba_oauth
`,
	} {
		var f File
		err := yaml.Unmarshal([]byte(raw), &f)
		if err == nil {
			t.Fatalf("expected error for:\n%s", raw)
		}
		if !strings.Contains(err.Error(), "auth.") {
			t.Fatalf("unexpected error %v for:\n%s", err, raw)
		}
	}
}

func TestUnmarshal_ModelAuth_rejectsUnknownField(t *testing.T) {
	raw := `models:
  - id: default
    provider: openai_compatible
    auth:
      token_file: key_files/x.json
      extra_field: "x"
`
	var f File
	err := yaml.Unmarshal([]byte(raw), &f)
	if err == nil || !strings.Contains(err.Error(), "unknown auth field") {
		t.Fatalf("got err=%v", err)
	}
}

func TestUnmarshal_ModelAuth_tokenFileOnlyOK(t *testing.T) {
	raw := `models:
  - id: default
    provider: openai_compatible
    auth:
      token_file: key_files/x.json
`
	var f File
	if err := yaml.Unmarshal([]byte(raw), &f); err != nil {
		t.Fatal(err)
	}
	if f.Models[0].Auth.TokenFile != "key_files/x.json" {
		t.Fatalf("got %+v", f.Models[0].Auth)
	}
}

func TestValidate_apiKeyEnvMustBeEnvName(t *testing.T) {
	f := &File{
		DefaultModel: "default/x",
		Models: []ModelProfile{{
			ID:        "default",
			APIKeyEnv: "sk-fake-secret-not-env-name",
			BaseURL:   "https://api.example.com/v1",
		}},
	}
	ApplyDefaults(f)
	if err := Validate(f); err == nil {
		t.Fatal("expected error")
	}
}
