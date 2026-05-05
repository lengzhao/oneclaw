package setup

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"

	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/keyfiles"
)

// LLMProviderPreset is one interactive onboarding choice (eino-ext native or OpenAI-compatible gateway).
type LLMProviderPreset struct {
	MenuKey      string
	Title        string
	ProfileID    string
	YAMLProvider string // models[].provider passed to config / adkhost
	ConsoleURL   string
	BaseURL      string // optional default base URL
	TokenRelPath string // under UserDataRoot, e.g. key_files/openai.json
	ModelDefault string // API model id hint
	Custom       bool   // prompts for id / base / optional console
}

// BuiltinLLMPresets is shown in onboard order (keys 1..n). Moonshot uses OpenAI-compatible HTTP API.
var BuiltinLLMPresets = []LLMProviderPreset{
	{"1", "OpenAI", "openai", "openai", "https://platform.openai.com/api-keys", "https://api.openai.com/v1", "key_files/openai.json", "gpt-4o-mini", false},
	{"2", "Anthropic Claude", "claude", "claude", "https://console.anthropic.com/settings/keys", "", "key_files/claude.json", "claude-sonnet-4-20250514", false},
	{"3", "Google Gemini", "gemini", "gemini", "https://aistudio.google.com/apikey", "", "key_files/gemini.json", "gemini-2.0-flash", false},
	{"4", "火山 Ark（豆包）", "ark", "ark", "https://console.volcengine.com/ark/region:ark+cn-beijing/apiKey?projectName=default", "", "key_files/ark.json", "", false},
	{"5", "Moonshot(kimi)", "moonshot", "moonshot", "https://platform.moonshot.cn/", "https://api.moonshot.cn/v1", "key_files/moonshot.json", "moonshot-v1-8k", false},
	{"6", "阿里云 Qwen（DashScope）", "dashscope", "qwen", "https://dashscope.console.aliyun.com", "https://dashscope.aliyuncs.com/compatible-mode/v1", "key_files/dashscope.json", "qwen-plus", false},
	{"7", "DeepSeek", "deepseek", "deepseek", "https://platform.deepseek.com/api_keys", "", "key_files/deepseek.json", "deepseek-chat", false},
	{"8", "OpenRouter", "openrouter", "openrouter", "https://openrouter.ai/keys", "https://openrouter.ai/api/v1", "key_files/openrouter.json", "openai/gpt-4o-mini", false},
	{"9", "自定义（OpenAI 兼容网关）", "", "openai_compatible", "", "", "", "gpt-4o-mini", true},
}

// RunLLMProviderOnboard interactively picks a provider, opens the console (unless noBrowser),
// writes API key to key_files, and returns the model profile + root default_model (profile_id/api_model).
// When user chooses "0) 跳过", skipped is true and profile/defaultModel are zero values.
func RunLLMProviderOnboard(out io.Writer, in io.Reader, userDataRoot string, noBrowser bool) (config.ModelProfile, string, bool, error) {
	if err := keyfiles.Ensure(userDataRoot); err != nil {
		return config.ModelProfile{}, "", false, err
	}
	if in == nil {
		in = strings.NewReader("")
	}

	fmt.Fprintln(out, "选择模型提供商（eino-ext 原生适配；Moonshot 走 OpenAI 兼容 HTTP）：")
	fmt.Fprintln(out, "  0) 跳过（保留默认或已有模型配置）")
	for _, p := range BuiltinLLMPresets {
		fmt.Fprintf(out, "  %s) %s\n", p.MenuKey, p.Title)
	}
	fmt.Fprint(out, "请输入序号并回车 [0]: ")
	line, err := readLine(in)
	if err != nil {
		return config.ModelProfile{}, "", false, err
	}
	choice := strings.TrimSpace(line)
	if choice == "" {
		choice = "0"
	}
	if choice == "0" {
		fmt.Fprintln(out, "已跳过 LLM 配置；沿用默认或已有模型配置。")
		return config.ModelProfile{}, "", true, nil
	}

	var preset LLMProviderPreset
	found := false
	for _, p := range BuiltinLLMPresets {
		if p.MenuKey == choice {
			preset = p
			found = true
			break
		}
	}
	if !found {
		return config.ModelProfile{}, "", false, fmt.Errorf("onboard: invalid choice %q", choice)
	}

	if preset.Custom {
		fmt.Fprint(out, "凭证 profile id（仅字母数字与 -/_ ，将作为 default_model 左侧）例如 mygpt [custom]: ")
		idLine, err := readLine(in)
		if err != nil {
			return config.ModelProfile{}, "", false, err
		}
		id := sanitizeProfileID(strings.TrimSpace(idLine))
		if id == "" {
			id = "custom"
		}
		fmt.Fprint(out, "API Base URL（须兼容 OpenAI，例如 https://api.example.com/v1）: ")
		baseLine, err := readLine(in)
		if err != nil {
			return config.ModelProfile{}, "", false, err
		}
		base := strings.TrimSpace(baseLine)
		if base == "" {
			return config.ModelProfile{}, "", false, fmt.Errorf("onboard: base_url required")
		}
		fmt.Fprint(out, "控制台链接（可选，用于打开浏览器；可直接回车跳过）: ")
		consoleLine, err := readLine(in)
		if err != nil {
			return config.ModelProfile{}, "", false, err
		}
		preset.ProfileID = id
		preset.BaseURL = base
		preset.ConsoleURL = strings.TrimSpace(consoleLine)
		preset.TokenRelPath = filepath.ToSlash(filepath.Join("key_files", id+".json"))
	}

	if preset.ProfileID == "ark" && strings.TrimSpace(preset.ModelDefault) == "" {
		fmt.Fprint(out, "Ark 推理接入点 ID（控制台复制的 Endpoint ID）[必填]: ")
		epLine, err := readLine(in)
		if err != nil {
			return config.ModelProfile{}, "", false, err
		}
		ep := strings.TrimSpace(epLine)
		if ep == "" {
			return config.ModelProfile{}, "", false, fmt.Errorf("onboard: ark model (endpoint id) required")
		}
		preset.ModelDefault = ep
	}

	if strings.TrimSpace(preset.ConsoleURL) != "" && !noBrowser {
		_ = openBrowser(preset.ConsoleURL)
	}

	fmt.Fprintf(out, "\n在控制台创建 API Key 后粘贴到下方并回车：\n")
	if strings.TrimSpace(preset.ConsoleURL) != "" {
		fmt.Fprintf(out, "（控制台：%s）\n", preset.ConsoleURL)
	}
	keyLine, err := readLine(in)
	if err != nil {
		return config.ModelProfile{}, "", false, err
	}
	apiKey := strings.TrimSpace(keyLine)
	if apiKey == "" {
		return config.ModelProfile{}, "", false, fmt.Errorf("onboard: empty API key")
	}

	abs := filepath.Join(userDataRoot, filepath.FromSlash(preset.TokenRelPath))
	if err := keyfiles.MergeWriteAPIKey(abs, apiKey); err != nil {
		return config.ModelProfile{}, "", false, err
	}
	fmt.Fprintf(out, "已写入 %s\n", preset.TokenRelPath)

	modelHint := preset.ModelDefault
	fmt.Fprintf(out, "API 模型名 [%s]: ", modelHint)
	modelLine, err := readLine(in)
	if err != nil {
		return config.ModelProfile{}, "", false, err
	}
	modelID := strings.TrimSpace(modelLine)
	if modelID == "" {
		modelID = modelHint
	}
	if modelID == "" {
		return config.ModelProfile{}, "", false, fmt.Errorf("onboard: empty model id")
	}

	prof := config.ModelProfile{
		ID:       preset.ProfileID,
		Priority: 0,
		Provider: preset.YAMLProvider,
		BaseURL:  preset.BaseURL,
		Auth: config.ModelAuth{
			TokenFile: preset.TokenRelPath,
		},
	}
	defaultModel := preset.ProfileID + "/" + modelID
	return prof, defaultModel, false, nil
}

func sanitizeProfileID(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	out = strings.ReplaceAll(out, "--", "-")
	return out
}

func readLine(in io.Reader) (string, error) {
	br := bufio.NewReader(in)
	s, err := br.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSuffix(s, "\n"), nil
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
