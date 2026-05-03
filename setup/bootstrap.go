package setup

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"text/template"

	"github.com/lengzhao/oneclaw/config"
)

// AgentBootstrapVars is passed when rendering the embedded templates/agents/default.md during Bootstrap (Go text/template).
type AgentBootstrapVars struct {
	UserDataRoot string
}

// Bootstrap creates UserDataRoot layout and writes templates only when missing (FR-CFG-02).
func Bootstrap(userDataRoot string) error {
	if err := os.MkdirAll(userDataRoot, 0o755); err != nil {
		return err
	}
	dirs := []string{
		filepath.Join(userDataRoot, "agents"),
		filepath.Join(userDataRoot, "skills"),
		filepath.Join(userDataRoot, "workflows"),
		filepath.Join(userDataRoot, "prompts"),
		filepath.Join(userDataRoot, "sessions"),
		filepath.Join(userDataRoot, "knowledge", "sources"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}

	cfgPath := filepath.Join(userDataRoot, "config.yaml")
	defaults, err := templates.ReadFile("templates/config.yaml")
	if err != nil {
		return err
	}
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := os.WriteFile(cfgPath, defaults, 0o644); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		if err := config.MergeYAMLMissingFile(cfgPath, defaults); err != nil {
			return err
		}
	}

	type fileJob struct {
		tmpl string
		dst  string
	}
	jobs := []fileJob{
		{"templates/manifest.yaml", filepath.Join(userDataRoot, "manifest.yaml")},
		{"templates/AGENT.md", filepath.Join(userDataRoot, "AGENT.md")},
		{"templates/MEMORY.md", filepath.Join(userDataRoot, "MEMORY.md")},
		{"templates/workflows/default.turn.yaml", filepath.Join(userDataRoot, "workflows", "default.turn.yaml")},
		{"templates/workflows/memory_extractor.yaml", filepath.Join(userDataRoot, "workflows", "memory_extractor.yaml")},
		{"templates/workflows/skill_generator.yaml", filepath.Join(userDataRoot, "workflows", "skill_generator.yaml")},
		{"templates/agents/README.md", filepath.Join(userDataRoot, "agents", "README.md")},
	}
	for _, j := range jobs {
		if err := copyTemplateIfMissing(j.tmpl, j.dst); err != nil {
			return err
		}
	}
	if err := renderAgentMarkdownTemplateIfMissing(
		"templates/agents/default.md",
		filepath.Join(userDataRoot, "agents", "default.md"),
		AgentBootstrapVars{UserDataRoot: userDataRoot},
	); err != nil {
		return err
	}
	return bootstrapSkillsFromTemplates(userDataRoot)
}

// bootstrapSkillsFromTemplates copies embedded templates/skills/** into UserDataRoot/skills/ when missing (never overwrites).
func bootstrapSkillsFromTemplates(userDataRoot string) error {
	const prefix = "templates/skills"
	return fs.WalkDir(templates, prefix, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(prefix, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(userDataRoot, "skills", filepath.FromSlash(rel))
		return copyTemplateIfMissing(path, dst)
	})
}

func renderAgentMarkdownTemplateIfMissing(tmplPath, dst string, data AgentBootstrapVars) error {
	if _, err := os.Stat(dst); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	raw, err := templates.ReadFile(tmplPath)
	if err != nil {
		return fmt.Errorf("setup: read %s: %w", tmplPath, err)
	}
	t, err := template.New(filepath.Base(tmplPath)).Option("missingkey=error").Parse(string(raw))
	if err != nil {
		return fmt.Errorf("setup: parse %s: %w", tmplPath, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Errorf("setup: execute %s: %w", tmplPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, buf.Bytes(), 0o644)
}

func copyTemplateIfMissing(tmplPath, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	b, err := templates.ReadFile(tmplPath)
	if err != nil {
		return fmt.Errorf("setup: read %s: %w", tmplPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o644)
}

// TemplateFS exposes embedded templates for tests.
func TemplateFS() fs.FS {
	return templates
}
