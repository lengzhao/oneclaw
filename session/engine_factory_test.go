package session

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/tools"
	"github.com/openai/openai-go"
)

func TestMainEngineFactory_WiresEinoRuntimeAndCredentials(t *testing.T) {
	home := t.TempDir()
	dot := filepath.Join(home, ".oneclaw")
	if err := os.MkdirAll(dot, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dot, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(`
openai:
  api_key: "k-test"
  base_url: "https://example.com/v1"
model: gpt-4o-mini
`), 0o644); err != nil {
		t.Fatal(err)
	}
	resolved, err := config.Load(config.LoadOptions{Home: home})
	if err != nil {
		t.Fatal(err)
	}
	f := MainEngineFactory(MainEngineFactoryDeps{
		Resolved: resolved,
		Registry: tools.NewRegistry(),
		Client:   openai.NewClient(),
		Model:    resolved.ChatModel(),
	})
	eng, err := f(SessionHandle{Source: "test", SessionKey: "s1"})
	if err != nil {
		t.Fatal(err)
	}
	if eng.TurnRunner == nil || eng.TurnRunner.Name() != "eino" {
		t.Fatalf("runner = %#v", eng.TurnRunner)
	}
	if eng.EinoOpenAIAPIKey != "k-test" {
		t.Fatalf("api key = %q", eng.EinoOpenAIAPIKey)
	}
	if eng.EinoOpenAIBaseURL != "https://example.com/v1" {
		t.Fatalf("base url = %q", eng.EinoOpenAIBaseURL)
	}
}

