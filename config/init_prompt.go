package config

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

// PromptInitIfTerminal interactively fills openai（api_key/base_url）、model、maintain（model/scheduled_model）、
// sessions.isolate_workspace、clawbridge.clients 预设（noop / webchat 组合及监听地址）when stdin is a TTY.
// 非终端（CI、管道、子进程）下立即返回且不读 stdin，不改变文件。
func PromptInitIfTerminal(cfgPath string, stdin *os.File, stdout io.Writer) error {
	if stdin == nil || stdout == nil {
		return nil
	}
	if !term.IsTerminal(int(stdin.Fd())) {
		return nil
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return fmt.Errorf("config.init.prompt: read %s: %w", cfgPath, err)
	}
	root, err := parseYAMLRootMap(data)
	if err != nil {
		return fmt.Errorf("config.init.prompt: parse %s: %w", cfgPath, err)
	}

	openaiMap := openaiSection(root)
	curKey, _ := scalarString(openaiMap["api_key"])
	curBase, _ := scalarString(openaiMap["base_url"])
	curModel, _ := scalarString(root["model"])

	flushWriter(stdout)

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "── oneclaw 配置引导（OpenAI 兼容 API，直接回车保留当前值）──")

	fmt.Fprint(stdout, "API Key")
	if curKey != "" {
		fmt.Fprint(stdout, " [已配置，回车保留]")
	}
	fmt.Fprint(stdout, ": ")
	flushWriter(stdout)
	keyBytes, err := term.ReadPassword(int(stdin.Fd()))
	if err != nil {
		return fmt.Errorf("config.init.prompt: read api_key: %w", err)
	}
	fmt.Fprintln(stdout)
	keyInput := strings.TrimSpace(string(keyBytes))

	br := bufio.NewReader(stdin)
	fmt.Fprintf(stdout, "Base URL [%s]: ", promptDefault(curBase))
	baseLine, err := readLine(br)
	if err != nil {
		return fmt.Errorf("config.init.prompt: read base_url: %w", err)
	}
	baseInput := strings.TrimSpace(baseLine)

	fmt.Fprintf(stdout, "主模型 model [%s]: ", promptDefault(curModel))
	modelLine, err := readLine(br)
	if err != nil {
		return fmt.Errorf("config.init.prompt: read model: %w", err)
	}
	modelInput := strings.TrimSpace(modelLine)

	maintainMap := maintainSection(root)
	curMaintModel, _ := scalarString(maintainMap["model"])
	curSchedModel, _ := scalarString(maintainMap["scheduled_model"])

	fmt.Fprintf(stdout, "维护模型 maintain.model [%s]: ", promptDefault(curMaintModel))
	maintModelLine, err := readLine(br)
	if err != nil {
		return fmt.Errorf("config.init.prompt: read maintain.model: %w", err)
	}
	maintModelInput := strings.TrimSpace(maintModelLine)

	fmt.Fprintf(stdout, "定时维护模型 maintain.scheduled_model [%s]: ", promptDefault(curSchedModel))
	schedModelLine, err := readLine(br)
	if err != nil {
		return fmt.Errorf("config.init.prompt: read maintain.scheduled_model: %w", err)
	}
	schedModelInput := strings.TrimSpace(schedModelLine)

	sessionsMap := sessionsSection(root)
	curIsolate := false
	if v, ok := sessionsMap["isolate_workspace"]; ok {
		if b, ok := yamlBool(v); ok {
			curIsolate = b
		}
	}
	curIsolateLabel := "关闭（多会话共享同一工作区）"
	if curIsolate {
		curIsolateLabel = "开启（每会话独立 sessions/<id>/）"
	}
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "会话工作区隔离 sessions.isolate_workspace？当前: %s\n", curIsolateLabel)
	fmt.Fprint(stdout, "  输入 y/yes 开启，n/no 关闭，回车保留: ")
	isolateLine, err := readLine(br)
	if err != nil {
		return fmt.Errorf("config.init.prompt: read isolate_workspace: %w", err)
	}
	sessionsChanged := false
	if newVal, explicit, valid := parseYesNoInput(isolateLine); !valid && strings.TrimSpace(isolateLine) != "" {
		fmt.Fprintln(stdout, "无效输入，已保留当前设置。")
	} else if explicit {
		if newVal != curIsolate {
			sessionsMap["isolate_workspace"] = newVal
			sessionsChanged = true
		}
	}
	root["sessions"] = sessionsMap

	clientsChanged, err := promptClawbridgeClients(br, stdout, root)
	if err != nil {
		return err
	}

	changed := false
	if keyInput != "" {
		openaiMap["api_key"] = keyInput
		changed = true
	}
	if baseInput != "" {
		openaiMap["base_url"] = baseInput
		changed = true
	}
	if modelInput != "" {
		root["model"] = modelInput
		changed = true
	}
	root["openai"] = openaiMap

	if maintModelInput != "" {
		maintainMap["model"] = maintModelInput
		changed = true
	}
	if schedModelInput != "" {
		maintainMap["scheduled_model"] = schedModelInput
		changed = true
	}
	root["maintain"] = maintainMap

	if clientsChanged {
		changed = true
	}
	if sessionsChanged {
		changed = true
	}

	if !changed {
		fmt.Fprintln(stdout, "未修改任何项，保留现有 config.yaml。")
		return nil
	}

	out, err := yaml.Marshal(root)
	if err != nil {
		return fmt.Errorf("config.init.prompt: marshal: %w", err)
	}
	if err := os.WriteFile(cfgPath, out, 0o644); err != nil {
		return fmt.Errorf("config.init.prompt: write %s: %w", cfgPath, err)
	}
	slog.Info("config.init.prompt.done", "path", cfgPath)
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "已写入。更多项请编辑:", cfgPath)
	return nil
}

func sessionsSection(root map[string]any) map[string]any {
	raw, ok := root["sessions"]
	if !ok || raw == nil {
		m := map[string]any{}
		root["sessions"] = m
		return m
	}
	if sm, ok := raw.(map[string]any); ok {
		return sm
	}
	if am, ok := raw.(map[any]any); ok {
		sm := make(map[string]any, len(am))
		for k, v := range am {
			ks, ok := k.(string)
			if !ok {
				continue
			}
			sm[ks] = v
		}
		root["sessions"] = sm
		return sm
	}
	m := map[string]any{}
	root["sessions"] = m
	return m
}

func yamlBool(v any) (bool, bool) {
	switch x := v.(type) {
	case bool:
		return x, true
	case int:
		return x != 0, true
	case int64:
		return x != 0, true
	default:
		return false, false
	}
}

// parseYesNoInput: empty line => explicit=false（保留）; y/n => explicit=true。
func parseYesNoInput(s string) (newVal bool, explicit bool, valid bool) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return false, false, true
	}
	if s == "y" || s == "yes" || s == "1" || s == "true" {
		return true, true, true
	}
	if s == "n" || s == "no" || s == "0" || s == "false" {
		return false, true, true
	}
	return false, false, false
}

func maintainSection(root map[string]any) map[string]any {
	raw, ok := root["maintain"]
	if !ok || raw == nil {
		m := map[string]any{}
		root["maintain"] = m
		return m
	}
	if sm, ok := raw.(map[string]any); ok {
		return sm
	}
	if am, ok := raw.(map[any]any); ok {
		sm := make(map[string]any, len(am))
		for k, v := range am {
			ks, ok := k.(string)
			if !ok {
				continue
			}
			sm[ks] = v
		}
		root["maintain"] = sm
		return sm
	}
	m := map[string]any{}
	root["maintain"] = m
	return m
}

func clawbridgeSection(root map[string]any) map[string]any {
	raw, ok := root["clawbridge"]
	if !ok || raw == nil {
		cb := map[string]any{
			"media":   map[string]any{"root": ""},
			"clients": []any{},
		}
		root["clawbridge"] = cb
		return cb
	}
	var cb map[string]any
	switch x := raw.(type) {
	case map[string]any:
		cb = x
	case map[any]any:
		cb = make(map[string]any, len(x))
		for k, v := range x {
			if ks, ok := k.(string); ok {
				cb[ks] = v
			}
		}
		root["clawbridge"] = cb
	default:
		cb = map[string]any{
			"media":   map[string]any{"root": ""},
			"clients": []any{},
		}
		root["clawbridge"] = cb
		return cb
	}
	if _, ok := cb["media"]; !ok {
		cb["media"] = map[string]any{"root": ""}
	} else if am, ok := cb["media"].(map[any]any); ok {
		sm := make(map[string]any, len(am))
		for k, v := range am {
			if ks, ok := k.(string); ok {
				sm[ks] = v
			}
		}
		cb["media"] = sm
	}
	if _, ok := cb["clients"]; !ok {
		cb["clients"] = []any{}
	}
	return cb
}

func findWebchatOptions(cb map[string]any) map[string]any {
	raw, ok := cb["clients"]
	if !ok {
		return nil
	}
	slice, ok := raw.([]any)
	if !ok {
		return nil
	}
	for _, el := range slice {
		m, ok := el.(map[string]any)
		if !ok {
			continue
		}
		d, _ := scalarString(m["driver"])
		if d != "webchat" {
			continue
		}
		optRaw, ok := m["options"]
		if !ok {
			return map[string]any{}
		}
		if om, ok := optRaw.(map[string]any); ok {
			return om
		}
		if oa, ok := optRaw.(map[any]any); ok {
			out := make(map[string]any)
			for k, v := range oa {
				if ks, ok := k.(string); ok {
					out[ks] = v
				}
			}
			return out
		}
	}
	return nil
}

func noopClient() map[string]any {
	return map[string]any{
		"id":      "noop-1",
		"driver":  "noop",
		"enabled": true,
		"options": map[string]any{},
	}
}

func webchatClient(listen, path, displayName string) map[string]any {
	return map[string]any{
		"id":      "webchat-1",
		"driver":  "webchat",
		"enabled": true,
		"options": map[string]any{
			"display_name": displayName,
			"listen":       listen,
			"path":         path,
		},
	}
}

func promptWebchatOpts(br *bufio.Reader, stdout io.Writer, defListen, defPath, defDisplay string) (listen, path, display string, err error) {
	fmt.Fprintf(stdout, "Webchat 监听地址 [%s]: ", defListen)
	line, err := readLine(br)
	if err != nil {
		return "", "", "", err
	}
	listen = strings.TrimSpace(line)
	if listen == "" {
		listen = defListen
	}
	fmt.Fprintf(stdout, "Webchat 路径 [%s]: ", defPath)
	line2, err := readLine(br)
	if err != nil {
		return "", "", "", err
	}
	path = strings.TrimSpace(line2)
	if path == "" {
		path = defPath
	}
	fmt.Fprintf(stdout, "Webchat 显示名 [%s]: ", defDisplay)
	line3, err := readLine(br)
	if err != nil {
		return "", "", "", err
	}
	display = strings.TrimSpace(line3)
	if display == "" {
		display = defDisplay
	}
	return listen, path, display, nil
}

// promptClawbridgeClients returns whether clawbridge.clients was replaced.
func promptClawbridgeClients(br *bufio.Reader, stdout io.Writer, root map[string]any) (bool, error) {
	cb := clawbridgeSection(root)
	defListen, defPath := "127.0.0.1:8765", "/"
	defDisplay := "You"
	if cur := findWebchatOptions(cb); cur != nil {
		if s, _ := scalarString(cur["listen"]); s != "" {
			defListen = s
		}
		if s, _ := scalarString(cur["path"]); s != "" {
			defPath = s
		}
		if s, _ := scalarString(cur["display_name"]); s != "" {
			defDisplay = s
		}
	}

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "── clawbridge.clients 预设 [回车=1] ──")
	fmt.Fprintln(stdout, "  1) noop + webchat（本地浏览器调试）")
	fmt.Fprintln(stdout, "  2) 仅 noop（无 IM 入站）")
	fmt.Fprintln(stdout, "  3) 仅 webchat")
	fmt.Fprintln(stdout, "  4) 不修改当前 clients")
	fmt.Fprint(stdout, "请选择 [1]: ")
	line, err := readLine(br)
	if err != nil {
		return false, fmt.Errorf("config.init.prompt: read clients preset: %w", err)
	}
	ch := strings.TrimSpace(line)
	if ch == "" {
		ch = "1"
	}
	switch ch {
	case "4":
		return false, nil
	case "2":
		cb["clients"] = []any{noopClient()}
		return true, nil
	case "3":
		listen, path, display, err := promptWebchatOpts(br, stdout, defListen, defPath, defDisplay)
		if err != nil {
			return false, err
		}
		cb["clients"] = []any{webchatClient(listen, path, display)}
		return true, nil
	case "1":
		listen, path, display, err := promptWebchatOpts(br, stdout, defListen, defPath, defDisplay)
		if err != nil {
			return false, err
		}
		cb["clients"] = []any{noopClient(), webchatClient(listen, path, display)}
		return true, nil
	default:
		fmt.Fprintln(stdout, "无效选项，跳过修改 clients。")
		return false, nil
	}
}

func openaiSection(root map[string]any) map[string]any {
	raw, ok := root["openai"]
	if !ok || raw == nil {
		m := map[string]any{}
		root["openai"] = m
		return m
	}
	if sm, ok := raw.(map[string]any); ok {
		return sm
	}
	if am, ok := raw.(map[any]any); ok {
		sm := make(map[string]any, len(am))
		for k, v := range am {
			ks, ok := k.(string)
			if !ok {
				continue
			}
			sm[ks] = v
		}
		root["openai"] = sm
		return sm
	}
	m := map[string]any{}
	root["openai"] = m
	return m
}

func scalarString(v any) (string, bool) {
	if v == nil {
		return "", false
	}
	switch x := v.(type) {
	case string:
		return x, true
	default:
		return fmt.Sprint(x), true
	}
}

func promptDefault(s string) string {
	if s == "" {
		return "留空"
	}
	return s
}

func readLine(br *bufio.Reader) (string, error) {
	line, err := br.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSuffix(line, "\n"), nil
}

func flushWriter(w io.Writer) {
	f, ok := w.(*os.File)
	if !ok {
		return
	}
	_ = f.Sync()
}
