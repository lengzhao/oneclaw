package adkhost

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino-ext/components/model/gemini"
	openroutercm "github.com/cloudwego/eino-ext/components/model/openrouter"
	"github.com/cloudwego/eino-ext/components/model/qwen"
	"github.com/cloudwego/eino/components/model"
	"google.golang.org/genai"

	"github.com/lengzhao/oneclaw/config"
)

// newChatModelFromProfile builds an eino-ext ToolCallingChatModel from a resolved profile (API key already injected).
func newChatModelFromProfile(ctx context.Context, prof *config.ModelProfile) (model.ToolCallingChatModel, error) {
	if prof == nil {
		return nil, fmt.Errorf("adkhost: nil model profile")
	}
	if err := requireResolvedAPIKey(prof); err != nil {
		return nil, err
	}

	p := strings.ToLower(strings.TrimSpace(prof.Provider))
	switch p {
	case "openai", "openai_compatible":
		return NewOpenAIChatModel(ctx, prof)
	case "moonshot":
		pp := *prof
		if strings.TrimSpace(pp.BaseURL) == "" {
			pp.BaseURL = "https://api.moonshot.cn/v1"
		}
		pp.Provider = "openai_compatible"
		return NewOpenAIChatModel(ctx, &pp)
	case "claude":
		return newClaudeChatModel(ctx, prof)
	case "gemini":
		return newGeminiChatModel(ctx, prof)
	case "ark":
		return newArkChatModel(ctx, prof)
	case "qwen":
		return newQwenChatModel(ctx, prof)
	case "deepseek":
		return newDeepseekChatModel(ctx, prof)
	case "openrouter":
		return newOpenrouterChatModel(ctx, prof)
	default:
		return nil, fmt.Errorf("adkhost: unsupported models.provider %q (use openai, openai_compatible, claude, gemini, ark, moonshot, qwen, deepseek, openrouter)", prof.Provider)
	}
}

func requireResolvedAPIKey(prof *config.ModelProfile) error {
	if strings.TrimSpace(prof.APIKey) != "" {
		return nil
	}
	env := prof.APIKeyEnv
	if env == "" {
		env = "OPENAI_API_KEY"
	}
	return fmt.Errorf("adkhost: missing API key for profile %q (set api_key, export %s, or auth.token_file under UserDataRoot)", prof.ID, env)
}

func newClaudeChatModel(ctx context.Context, prof *config.ModelProfile) (model.ToolCallingChatModel, error) {
	cfg := &claude.Config{
		APIKey:    prof.APIKey,
		Model:     prof.DefaultModel,
		MaxTokens: 8192,
	}
	if u := strings.TrimSpace(prof.BaseURL); u != "" {
		s := u
		cfg.BaseURL = &s
	}
	return claude.NewChatModel(ctx, cfg)
}

func newGeminiChatModel(ctx context.Context, prof *config.ModelProfile) (model.ToolCallingChatModel, error) {
	cc := &genai.ClientConfig{APIKey: prof.APIKey}
	if u := strings.TrimSpace(prof.BaseURL); u != "" {
		cc.HTTPOptions = genai.HTTPOptions{BaseURL: u}
	}
	client, err := genai.NewClient(ctx, cc)
	if err != nil {
		return nil, fmt.Errorf("adkhost gemini client: %w", err)
	}
	return gemini.NewChatModel(ctx, &gemini.Config{
		Client: client,
		Model:  prof.DefaultModel,
	})
}

func newArkChatModel(ctx context.Context, prof *config.ModelProfile) (model.ToolCallingChatModel, error) {
	cfg := &ark.ChatModelConfig{
		APIKey: prof.APIKey,
		Model:  prof.DefaultModel,
	}
	if u := strings.TrimSpace(prof.BaseURL); u != "" {
		cfg.BaseURL = u
	}
	return ark.NewChatModel(ctx, cfg)
}

func newQwenChatModel(ctx context.Context, prof *config.ModelProfile) (model.ToolCallingChatModel, error) {
	base := strings.TrimSpace(prof.BaseURL)
	if base == "" {
		base = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	}
	return qwen.NewChatModel(ctx, &qwen.ChatModelConfig{
		APIKey:  prof.APIKey,
		BaseURL: base,
		Model:   prof.DefaultModel,
	})
}

func newDeepseekChatModel(ctx context.Context, prof *config.ModelProfile) (model.ToolCallingChatModel, error) {
	cfg := &deepseek.ChatModelConfig{
		APIKey: prof.APIKey,
		Model:  prof.DefaultModel,
	}
	if u := strings.TrimSpace(prof.BaseURL); u != "" {
		cfg.BaseURL = u
	}
	return deepseek.NewChatModel(ctx, cfg)
}

func newOpenrouterChatModel(ctx context.Context, prof *config.ModelProfile) (model.ToolCallingChatModel, error) {
	cfg := &openroutercm.Config{
		APIKey: prof.APIKey,
		Model:  prof.DefaultModel,
	}
	if u := strings.TrimSpace(prof.BaseURL); u != "" {
		cfg.BaseURL = u
	}
	return openroutercm.NewChatModel(ctx, cfg)
}
