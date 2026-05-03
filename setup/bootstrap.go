package setup

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/lengzhao/oneclaw/config"
)

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
		{"templates/agents/README.md", filepath.Join(userDataRoot, "agents", "README.md")},
	}
	for _, j := range jobs {
		if err := copyTemplateIfMissing(j.tmpl, j.dst); err != nil {
			return err
		}
	}
	return nil
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
