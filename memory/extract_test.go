package memory

import (
	"strings"
	"testing"

	lzmodel "github.com/lengzhao/memory/model"
	"github.com/lengzhao/oneclaw/loop"
)

func TestBuildDailyLogLineToolSummary(t *testing.T) {
	line := buildDailyLogLine("hello", "world", []loop.ToolTraceEntry{
		{Name: "read_file", OK: true},
		{Name: "exec", OK: false, Err: "exit 1"},
	})
	if !strings.Contains(line, "| tools: ") {
		t.Fatalf("missing tools: %q", line)
	}
	if !strings.Contains(line, "read_file:ok") || !strings.Contains(line, "exec:err") {
		t.Fatalf("unexpected content: %q", line)
	}
}

func TestBuildDailyLogLineNoTools(t *testing.T) {
	line := buildDailyLogLine("a", "b", nil)
	if strings.Contains(line, "| tools:") {
		t.Fatalf("should not add tools section: %q", line)
	}
}

func TestResolveExtractLLM_FixedTemperatureModels(t *testing.T) {
	for _, model := range []string{"gpt-5", "openai/gpt-5-chat-latest", "o3-mini", "o1-preview", "kimi-k2.5", "moonshot/kimi-k2-250411"} {
		t.Run(model, func(t *testing.T) {
			out := resolveExtractLLM(&lzmodel.LLMConfig{
				APIKey:      "k",
				Model:       model,
				MaxTokens:   1024,
				Temperature: 0.2,
			}, 4096, true)
			if out == nil {
				t.Fatal("nil config")
			}
			if out.Temperature != 1 {
				t.Fatalf("temperature %v, want 1", out.Temperature)
			}
		})
	}
}

func TestResolveExtractLLM_DefaultTemperature(t *testing.T) {
	out := resolveExtractLLM(&lzmodel.LLMConfig{
		APIKey:    "k",
		Model:     "gpt-4o",
		MaxTokens: 1024,
	}, 4096, true)
	if out == nil {
		t.Fatal("nil config")
	}
	if out.Temperature != 0.2 {
		t.Fatalf("temperature %v, want 0.2", out.Temperature)
	}
}
