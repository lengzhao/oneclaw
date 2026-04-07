// Delegated agents：主线程 system 含目录；run_agent 工具为静态 description（E2E-116）。
package e2e_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/routing"
	"github.com/lengzhao/oneclaw/test/openaistub"
)

func writeBusinessAgent(t *testing.T, cwd, agentType, description, body string) {
	t.Helper()
	dir := filepath.Join(cwd, ".oneclaw", "agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nagent_type: " + agentType + "\ndescription: " + description + "\ntools:\n  - read_file\n---\n\n" + body + "\n"
	path := filepath.Join(dir, agentType+".md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runAgentToolDescriptionFromChatBody(body []byte) (string, bool) {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return "", false
	}
	raw, _ := m["tools"].([]any)
	for _, ti := range raw {
		tmap, ok := ti.(map[string]any)
		if !ok {
			continue
		}
		fn, ok := tmap["function"].(map[string]any)
		if !ok {
			continue
		}
		if fn["name"] != "run_agent" {
			continue
		}
		desc, _ := fn["description"].(string)
		return desc, true
	}
	return "", false
}

// E2E-116 存在 .oneclaw/agents 业务定义时：system 含 Delegated agents 段；run_agent 在请求中出现且 description 不含动态目录附录。
func TestE2E_116_AgentCatalogInSystemAndRunAgentTool(t *testing.T) {
	cwd := t.TempDir()
	writeBusinessAgent(t, cwd, "e2e-biz", "E2E116_BIZ_DESC", "You are the e2e business agent.")

	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "ok"))
	e2eEnvMinimal(t, stub)

	e := newStubEngine(t, cwd)
	if err := e.SubmitUser(context.Background(), routing.Inbound{Text: "ping"}); err != nil {
		t.Fatal(err)
	}
	bodies := stub.ChatRequestBodies()
	if len(bodies) < 1 {
		t.Fatal("no chat request captured")
	}
	sys, err := openaistub.ChatRequestSystemTextConcat(bodies[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sys, "Delegated agents") {
		t.Fatalf("system missing Delegated agents:\n%s", sys)
	}
	if !strings.Contains(sys, "e2e-biz") || !strings.Contains(sys, "E2E116_BIZ_DESC") {
		t.Fatalf("system missing custom agent:\n%s", sys)
	}
	desc, ok := runAgentToolDescriptionFromChatBody(bodies[0])
	if !ok {
		t.Fatal("run_agent tool missing from request")
	}
	if strings.Contains(desc, "Available agent_type") || strings.Contains(desc, "e2e-biz") {
		t.Fatalf("run_agent description should be static (no catalog appendix):\n%s", desc)
	}
	if !strings.Contains(desc, "Run a named sub-agent") {
		t.Fatalf("run_agent description unexpected:\n%s", desc)
	}
}
