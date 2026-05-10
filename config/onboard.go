package config

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	cbconfig "github.com/lengzhao/clawbridge/config"
	"github.com/lengzhao/clawbridge/client"
	wx "github.com/lengzhao/clawbridge/drivers/weixin"
	"github.com/lengzhao/oneclaw/memory"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

// MergeClawbridgeResultIntoRoot upserts res.Config.clients into root["clawbridge"].clients by client id
// and sets clawbridge.media.root when res provides a non-empty root.
func MergeClawbridgeResultIntoRoot(root map[string]any, res cbconfig.Config) (changed bool, err error) {
	if len(res.Clients) == 0 {
		return false, nil
	}
	cbSec := clawbridgeSection(root)

	raw, err := yaml.Marshal(&res)
	if err != nil {
		return false, fmt.Errorf("config.onboard: marshal clawbridge config: %w", err)
	}
	var cm map[string]any
	if err := yaml.Unmarshal(raw, &cm); err != nil {
		return false, fmt.Errorf("config.onboard: parse clawbridge map: %w", err)
	}

	incoming, _ := cm["clients"].([]any)
	if len(incoming) == 0 {
		return false, nil
	}
	existing, _ := cbSec["clients"].([]any)
	newClients := upsertClawbridgeClients(existing, incoming)
	if !clawbridgeClientsEqual(existing, newClients) {
		cbSec["clients"] = newClients
		changed = true
	}

	if mm, ok := cm["media"].(map[string]any); ok {
		if r, ok := scalarString(mm["root"]); ok && strings.TrimSpace(r) != "" {
			mediaMap, _ := ensureClawbridgeMediaMap(cbSec)
			if cur, _ := scalarString(mediaMap["root"]); cur != r {
				mediaMap["root"] = r
				changed = true
			}
		}
	}

	root["clawbridge"] = cbSec
	return changed, nil
}

func ensureClawbridgeMediaMap(cbSec map[string]any) (map[string]any, bool) {
	raw := cbSec["media"]
	if sm, ok := raw.(map[string]any); ok {
		return sm, false
	}
	if am, ok := raw.(map[any]any); ok {
		sm := make(map[string]any, len(am))
		for k, v := range am {
			if ks, ok := k.(string); ok {
				sm[ks] = v
			}
		}
		cbSec["media"] = sm
		return sm, true
	}
	sm := map[string]any{}
	cbSec["media"] = sm
	return sm, true
}

func clientYAMLID(el any) string {
	m, ok := el.(map[string]any)
	if !ok {
		am, ok := el.(map[any]any)
		if !ok {
			return ""
		}
		m = make(map[string]any, len(am))
		for k, v := range am {
			if ks, ok := k.(string); ok {
				m[ks] = v
			}
		}
	}
	id, _ := scalarString(m["id"])
	return strings.TrimSpace(id)
}

func upsertClawbridgeClients(existing []any, incoming []any) []any {
	out := append([]any(nil), existing...)
	byID := make(map[string]int)
	for i, el := range out {
		id := clientYAMLID(el)
		if id != "" {
			byID[id] = i
		}
	}
	for _, el := range incoming {
		id := clientYAMLID(el)
		if id == "" {
			continue
		}
		copyEl := deepCopyYAMLValue(el)
		if idx, ok := byID[id]; ok {
			out[idx] = copyEl
		} else {
			byID[id] = len(out)
			out = append(out, copyEl)
		}
	}
	return out
}

func clawbridgeClientsEqual(a, b []any) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		ya, err := yaml.Marshal(a[i])
		if err != nil {
			return false
		}
		yb, err := yaml.Marshal(b[i])
		if err != nil {
			return false
		}
		if string(ya) != string(yb) {
			return false
		}
	}
	return true
}

func splitAllowFromCSV(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{"*"}
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{"*"}
	}
	return out
}

// RunOnboardInteractive runs terminal prompts for LLM settings and one clawbridge driver onboarding,
// then merges the result into ~/.oneclaw/config.yaml (with plaintext credentials where applicable).
func RunOnboardInteractive(home string, stdin *os.File, stdout, stderr io.Writer) error {
	if stdin == nil || stdout == nil {
		return fmt.Errorf("config.onboard: stdin and stdout are required")
	}
	if stderr == nil {
		stderr = io.Discard
	}
	if !term.IsTerminal(int(stdin.Fd())) {
		return fmt.Errorf("config.onboard: requires an interactive terminal (TTY)")
	}

	if err := InitWorkspace(home, home); err != nil {
		return fmt.Errorf("config.onboard: init workspace: %w", err)
	}

	userRoot := filepath.Join(home, memory.DotDir)
	cfgPath := filepath.Join(userRoot, "config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return fmt.Errorf("config.onboard: read %s: %w", cfgPath, err)
	}
	root, err := parseYAMLRootMap(data)
	if err != nil {
		return fmt.Errorf("config.onboard: parse config: %w", err)
	}

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "── oneclaw onboard：OpenAI + clawbridge 驱动，写入 ~/.oneclaw/config.yaml ──")

	llmChanged, br, err := promptLLMAndMaintainSessions(stdin, stdout, root)
	if err != nil {
		return err
	}

	drivers := client.ListOnboardingDrivers()
	sort.Strings(drivers)
	if len(drivers) == 0 {
		return fmt.Errorf("config.onboard: no onboarding drivers registered (import github.com/lengzhao/clawbridge/drivers)")
	}

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "── clawbridge 驱动 onboarding ──")
	for i, d := range drivers {
		fmt.Fprintf(stdout, "  %d) %s\n", i+1, d)
	}
	fmt.Fprint(stdout, "选择序号或驱动名 [weixin]: ")
	line, err := readLine(br)
	if err != nil {
		return fmt.Errorf("config.onboard: read driver: %w", err)
	}
	driverPick := strings.TrimSpace(strings.ToLower(line))
	driverName := driverPick
	if driverPick == "" {
		driverName = "weixin"
	} else if n, err := strconv.Atoi(driverPick); err == nil && n >= 1 && n <= len(drivers) {
		driverName = drivers[n-1]
	}
	found := false
	for _, d := range drivers {
		if d == driverName {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("config.onboard: unknown driver %q", driverName)
	}

	defID := driverName + "-1"
	fmt.Fprintf(stdout, "clawbridge clients[].id [%s]: ", defID)
	idLine, err := readLine(br)
	if err != nil {
		return fmt.Errorf("config.onboard: read client id: %w", err)
	}
	clientID := strings.TrimSpace(idLine)
	if clientID == "" {
		clientID = defID
	}

	params := map[string]any{}
	switch driverName {
	case "weixin":
		defListen := "127.0.0.1:8769"
		fmt.Fprintf(stdout, "微信 HTTP 二维码页监听地址 [%s]: ", defListen)
		listenLine, err := readLine(br)
		if err != nil {
			return fmt.Errorf("config.onboard: read http_listen: %w", err)
		}
		listen := strings.TrimSpace(listenLine)
		if listen == "" {
			listen = defListen
		}
		fmt.Fprint(stdout, "HTTP 代理 proxy（可选，回车跳过）: ")
		proxyLine, err := readLine(br)
		if err != nil {
			return fmt.Errorf("config.onboard: read proxy: %w", err)
		}
		opts := wx.OnboardingOpts{
			HTTPListen: listen,
			Proxy:      strings.TrimSpace(proxyLine),
		}
		params = opts.DriverMap()
	}

	fmt.Fprint(stdout, "allow-from（逗号分隔，* 不限制）[*]: ")
	afLine, err := readLine(br)
	if err != nil {
		return fmt.Errorf("config.onboard: read allow-from: %w", err)
	}
	allowFrom := splitAllowFromCSV(afLine)

	stateDir := filepath.Join(userRoot, "clawbridge-state", clientID)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return fmt.Errorf("config.onboard: mkdir %s: %w", stateDir, err)
	}

	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	spec := client.NewOnboarding(driverName, clientID).
		WithParams(params).
		WithStateDir(stateDir).
		WithAllowFrom(allowFrom...)

	res, err := client.RunOnboarding(rootCtx, spec)
	if err != nil {
		return fmt.Errorf("config.onboard: %w", err)
	}

	pm, err := client.ParseOnboardingPrintMode("human")
	if err != nil {
		return err
	}
	client.ReportOnboarding(stdout, pm, res, client.ReportOptions{
		MaskSecrets: false,
		ErrWriter:   stderr,
	})

	cbChanged, err := MergeClawbridgeResultIntoRoot(root, res.Config)
	if err != nil {
		return err
	}

	changed := llmChanged || cbChanged
	if !changed {
		fmt.Fprintln(stdout, "未修改任何项，跳过写入 config.yaml。")
		return nil
	}

	out, err := yaml.Marshal(root)
	if err != nil {
		return fmt.Errorf("config.onboard: marshal config: %w", err)
	}
	if err := os.WriteFile(cfgPath, out, 0o644); err != nil {
		return fmt.Errorf("config.onboard: write %s: %w", cfgPath, err)
	}
	slog.Info("config.onboard.done", "path", cfgPath)
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "已写入:", cfgPath)
	if res.Phase == client.OnboardingPhaseManual {
		fmt.Fprintln(stdout, "当前驱动为说明型 onboarding：已在 config 中写入占位 client（enabled: false），请按上文步骤补全 options 后改为 enabled: true。")
	}
	return nil
}
