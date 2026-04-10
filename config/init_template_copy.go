package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const initTemplateRoot = "init_template"

// initTemplateConfigYAML returns the embedded default config.yaml bytes (for merge when user's config already exists).
func initTemplateConfigYAML() ([]byte, error) {
	return initTemplateFS.ReadFile(initTemplateRoot + "/config.yaml")
}

// copyInitTemplate copies embedded init_template files into dotDir; skips any destination path that already exists.
func copyInitTemplate(dotDir string) error {
	return fs.WalkDir(initTemplateFS, initTemplateRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, ok := strings.CutPrefix(path, initTemplateRoot+"/")
		if !ok {
			return fmt.Errorf("config.init: unexpected embed path %q", path)
		}
		dest := filepath.Join(dotDir, filepath.FromSlash(rel))
		if _, err := os.Stat(dest); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(dest), err)
		}
		b, err := initTemplateFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embed %s: %w", path, err)
		}
		if err := os.WriteFile(dest, b, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", dest, err)
		}
		return nil
	})
}
