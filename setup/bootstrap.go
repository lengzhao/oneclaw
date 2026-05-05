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

const embeddedTemplatesRoot = "templates"

// AgentBootstrapVars is passed when rendering templates/agents/default.md during Bootstrap (Go text/template).
type AgentBootstrapVars struct {
	UserDataRoot string
}

// Bootstrap creates UserDataRoot layout and writes embedded templates/** only when missing (FR-CFG-02).
// The full tree under setup/templates is embedded (see embed.go); add or edit files there to change defaults.
func Bootstrap(userDataRoot string) error {
	if err := os.MkdirAll(userDataRoot, 0o755); err != nil {
		return err
	}
	// Keep runtime-only directories that do not need placeholder files under templates/.
	for _, d := range []string{
		filepath.Join(userDataRoot, "sessions"),
		filepath.Join(userDataRoot, "prompts"),
		filepath.Join(userDataRoot, "knowledge", "sources"),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}

	cfgPath := filepath.Join(userDataRoot, "config.yaml")
	defaults, err := templates.ReadFile(embeddedTemplatesRoot + "/config.yaml")
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

	return fs.WalkDir(templates, embeddedTemplatesRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(embeddedTemplatesRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "config.yaml" {
			return nil
		}
		dst := filepath.Join(userDataRoot, filepath.FromSlash(rel))
		// embed.FS paths must use "/" (see go:embed docs), even on Windows.
		tmplKey := embeddedTemplatesRoot + "/" + rel
		if rel == "agents/default.md" {
			return renderAgentMarkdownTemplateIfMissing(
				tmplKey,
				dst,
				AgentBootstrapVars{UserDataRoot: userDataRoot},
			)
		}
		return copyTemplateIfMissing(tmplKey, dst)
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
