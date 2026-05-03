package wfexec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/lengzhao/oneclaw/adkhost"
	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/paths"
	"github.com/lengzhao/oneclaw/preturn"
	"github.com/lengzhao/oneclaw/tools"
)

// embeddedAgentPromptTemplate is the default layout when agents/<agent_type>.prompt.tmpl is absent (not shipped in user-data bootstrap).
const embeddedAgentPromptTemplate = `{{.AGENT_MD}}

---

{{.MEMORY_MD}}

{{if .ReferencedSkillsIndex}}
---

{{.ReferencedSkillsIndex}}
{{end}}

---

{{.AgentBody}}

---

{{.SkillsIndexSectionTitle}}

{{.SkillsIndex}}

---

## Tasks (todo.json)

{{.Tasks}}

---

## Current time (UTC)

{{.NowUTC}}
`

func promptTemplatePath(userDataRoot, agentType string) string {
	at := strings.TrimSpace(agentType)
	if at == "" {
		at = "default"
	}
	return filepath.Join(paths.CatalogRoot(strings.TrimSpace(userDataRoot)), "agents", at+".prompt.tmpl")
}

// RenderMainAgentPrompt builds the final system instruction from disk files + workflow template data; optional agents/*.prompt.tmpl overrides the embedded default.
func RenderMainAgentPrompt(rtx *engine.RuntimeContext) (string, error) {
	if rtx == nil || rtx.Agent == nil {
		return "", fmt.Errorf("wfexec: render prompt: nil runtime context or agent")
	}
	ir := strings.TrimSpace(rtx.EffectiveInstructionRoot())
	if ir == "" {
		return "", fmt.Errorf("wfexec: render prompt: empty instruction root")
	}
	ud := strings.TrimSpace(rtx.UserDataRoot)

	agentMd := strings.TrimSpace(readOptionalText(filepath.Join(ir, "AGENT.md")))
	memStr := ""
	if raw, err := os.ReadFile(filepath.Join(ir, "MEMORY.md")); err == nil && len(raw) > 0 {
		raw = memory.TruncateMEMORYMDForInjection(raw)
		memStr = strings.TrimSpace(string(raw))
	}

	data := map[string]any{}
	if rtx.PromptTemplateData != nil {
		for k, v := range rtx.PromptTemplateData {
			data[k] = v
		}
	}
	// Host-controlled fields win over workflow-provided keys.
	data["AGENT_MD"] = agentMd
	data["MEMORY_MD"] = memStr
	data["AgentBody"] = strings.TrimSpace(rtx.Agent.Body)
	refBlock := preturn.ReferencedSkillsIndexMarkdown(ud, rtx.Agent.ReferencedSkillIDs)
	data["ReferencedSkillsIndex"] = refBlock
	if strings.TrimSpace(refBlock) != "" {
		data["SkillsIndexSectionTitle"] = "## Skills index (catalog allowlist)"
	} else {
		data["SkillsIndexSectionTitle"] = "## Skills index (all installed skills)"
	}
	for _, k := range []string{"SkillsIndex", "Tasks"} {
		if _, ok := data[k]; !ok {
			data[k] = ""
		}
		data[k] = stringifyTemplateVal(data[k])
	}
	// MemoryRecall is workflow-filled but sent as a separate user message in adk_main (not merged into system).
	data["MemoryRecall"] = ""
	when := time.Now().UTC()
	if !rtx.RunStartedAt.IsZero() {
		when = rtx.RunStartedAt.UTC()
	}
	data["NowUTC"] = when.Format(time.RFC3339)

	tmplSrc := embeddedAgentPromptTemplate
	p := promptTemplatePath(ud, rtx.Agent.AgentType)
	if b, err := os.ReadFile(p); err == nil && len(strings.TrimSpace(string(b))) > 0 {
		tmplSrc = string(b)
	}

	t, err := template.New(filepath.Base(p)).Option("missingkey=zero").Parse(tmplSrc)
	if err != nil {
		return "", fmt.Errorf("wfexec: parse prompt template: %w", err)
	}
	var sb strings.Builder
	if err := t.Execute(&sb, data); err != nil {
		return "", fmt.Errorf("wfexec: execute prompt template: %w", err)
	}
	out := strings.TrimSpace(sb.String())
	if out == "" {
		out = "You are a helpful assistant."
	}
	return out, nil
}

func stringifyTemplateVal(v any) string {
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	default:
		return strings.TrimSpace(fmt.Sprint(s))
	}
}

func readOptionalText(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func rebuildChatAgentForInstruction(rtx *engine.RuntimeContext, instruction string) error {
	if rtx == nil || rtx.ChatModel == nil || rtx.ToolRegistry == nil {
		return fmt.Errorf("wfexec: rebuild chat agent: missing model or registry")
	}
	reg, ok := rtx.ToolRegistry.(*tools.Registry)
	if !ok || reg == nil {
		return fmt.Errorf("wfexec: rebuild chat agent: registry must be *tools.Registry")
	}
	meta := rtx.AgentShellMeta
	agent, err := adkhost.NewChatModelAgent(rtx.GoCtx, rtx.ChatModel, reg, adkhost.AgentOptions{
		Name:          meta.Name,
		Description:   meta.Description,
		Instruction:   instruction,
		MaxIterations: meta.MaxIterations,
		Handlers:      meta.Handlers,
	})
	if err != nil {
		return err
	}
	rtx.ChatAgent = agent
	return nil
}
