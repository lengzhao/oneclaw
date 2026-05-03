package adkhost

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/model/openai"

	"github.com/lengzhao/oneclaw/config"
)

// NewOpenAIChatModel builds an OpenAI-compatible ChatModel from a config profile (FR-EINO-01).
func NewOpenAIChatModel(ctx context.Context, m *config.ModelProfile) (*openai.ChatModel, error) {
	if m == nil {
		return nil, fmt.Errorf("adkhost: nil model profile")
	}
	if m.APIKey == "" {
		env := m.APIKeyEnv
		if env == "" {
			env = "OPENAI_API_KEY"
		}
		return nil, fmt.Errorf("adkhost: missing API key for profile %q (set api_key or env %s)", m.ID, env)
	}
	return openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  m.APIKey,
		BaseURL: m.BaseURL,
		Model:   m.DefaultModel,
	})
}
