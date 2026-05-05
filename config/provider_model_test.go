package config

import (
	"strings"
	"testing"
)

func TestSplitProviderModel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in       string
		wantCred string
		wantMod  string
		ok       bool
	}{
		{"alibaba/qwen-plus", "alibaba", "qwen-plus", true},
		{" openrouter / org/model ", "openrouter", "org/model", true},
		{"nogslash", "", "", false},
		{"/onlymodel", "", "", false},
		{"only/", "", "", false},
	}
	for _, tt := range tests {
		c, m, ok := SplitProviderModel(tt.in)
		if ok != tt.ok || c != tt.wantCred || m != tt.wantMod {
			t.Fatalf("SplitProviderModel(%q) = (%q,%q,%v) want (%q,%q,%v)", tt.in, c, m, ok, tt.wantCred, tt.wantMod, tt.ok)
		}
	}
}

func TestEffectiveModelSelector(t *testing.T) {
	t.Parallel()
	if s := EffectiveModelSelector("", "m"); s != "m" {
		t.Fatalf("got %q", s)
	}
	if s := EffectiveModelSelector("p", "m"); s != "p" {
		t.Fatalf("got %q", s)
	}
	if s := EffectiveModelSelector("  x ", ""); s != "x" {
		t.Fatalf("got %q", s)
	}
}

func TestResolveModelForTurn_providerModel(t *testing.T) {
	t.Parallel()
	f := &File{
		DefaultModel: "b/fallback",
		Models: []ModelProfile{
			{ID: "a", Priority: 0, Provider: "openai_compatible"},
			{ID: "b", Priority: 10, Provider: "mock"},
		},
	}
	ApplyDefaults(f)
	p, err := ResolveModelForTurn(f, "a/custom-model")
	if err != nil || p.ID != "a" || p.DefaultModel != "custom-model" {
		t.Fatalf("got %+v err=%v", p, err)
	}
}

func TestResolveModelForTurn_emptySelectorUsesRoot(t *testing.T) {
	t.Parallel()
	f := &File{
		DefaultModel: "a/from-root",
		Models: []ModelProfile{
			{ID: "a", Priority: 0},
			{ID: "b", Priority: 10},
		},
	}
	ApplyDefaults(f)
	p, err := ResolveModelForTurn(f, "")
	if err != nil || p.ID != "a" || p.DefaultModel != "from-root" {
		t.Fatalf("got %+v err=%v", p, err)
	}
}

func TestResolveModelForTurn_rejectsBareSelector(t *testing.T) {
	t.Parallel()
	f := &File{
		DefaultModel: "default/x",
		Models:       []ModelProfile{{ID: "default"}},
	}
	ApplyDefaults(f)
	if _, err := ResolveModelForTurn(f, "default"); err == nil || !strings.Contains(err.Error(), "must be profile_id_or_provider/api_model_id") {
		t.Fatalf("err=%v", err)
	}
}

func TestResolveProfilesForCredentialKey_idOverridesProviderGroup(t *testing.T) {
	t.Parallel()
	f := &File{
		Models: []ModelProfile{
			{ID: "openai_compatible", Priority: 0, Provider: "mock"},
			{ID: "z", Priority: 10, Provider: "openai_compatible"},
		},
	}
	ApplyDefaults(f)
	got, err := ResolveProfilesForCredentialKey(f, "openai_compatible")
	if err != nil || len(got) != 1 || got[0].ID != "openai_compatible" {
		t.Fatalf("want single id match, got %+v err=%v", got, err)
	}
}

func TestResolveProfilesForCredentialKey_providerFailoverOrder(t *testing.T) {
	t.Parallel()
	f := &File{
		Models: []ModelProfile{
			{ID: "backup-openai", Priority: 10, Provider: "openai_compatible"},
			{ID: "primary-openai", Priority: 0, Provider: "openai_compatible"},
			{ID: "other", Priority: 0, Provider: "mock"},
		},
	}
	ApplyDefaults(f)
	got, err := ResolveProfilesForCredentialKey(f, "openai_compatible")
	if err != nil || len(got) != 2 {
		t.Fatalf("got %+v err=%v", got, err)
	}
	if got[0].ID != "primary-openai" || got[1].ID != "backup-openai" {
		t.Fatalf("failover order wrong: %#v", got)
	}
}

func TestResolveModelForTurn_rejectsBareRootDefault(t *testing.T) {
	t.Parallel()
	f := &File{
		DefaultModel: "gpt-only",
		Models:       []ModelProfile{{ID: "default"}},
	}
	ApplyDefaults(f)
	if _, err := ResolveModelForTurn(f, ""); err == nil {
		t.Fatal("want error")
	}
}

func TestValidate_profileIDSlash(t *testing.T) {
	f := &File{
		DefaultModel: "ok/model",
		Models: []ModelProfile{
			{ID: "ok"},
			{ID: "bad/id"},
		},
	}
	ApplyDefaults(f)
	if err := Validate(f); err == nil {
		t.Fatal("want error")
	}
}
