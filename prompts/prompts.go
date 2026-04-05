// Package prompts offers TemplateFiles (embedded templates/*.tmpl) and Render(name, data).
package prompts

import (
	"bytes"
	"embed"
	"errors"
	"strings"
	"sync"
	"text/template"
)

// Template names for [Render] (basename of files under templates/, as registered by ParseFS).
const (
	NameMainThreadSystem  = "main_thread_system"
	NameMaintenanceSystem = "maintenance_system"
	NameCompactEnvelope   = "compact_envelope"
)

// TemplateFiles holds all embedded .tmpl assets under templates/ (single place to edit copy).
//
//go:embed templates/*.tmpl
var TemplateFiles embed.FS

var (
	parseOnce sync.Once
	root      *template.Template
	parseErr  error
)

func loadTemplates() (*template.Template, error) {
	parseOnce.Do(func() {
		root, parseErr = template.ParseFS(TemplateFiles, "templates/*.tmpl")
	})
	return root, parseErr
}

// Render parses embedded templates on first use, then runs ExecuteTemplate with name (see [NameMainThreadSystem] and siblings).
// If data is nil, an empty map[string]any is used (map-based templates only); struct-based callers pass a value explicitly.
func Render(name string, data any) (string, error) {
	if name == "" {
		return "", errors.New("prompts: empty template name")
	}
	t, err := loadTemplates()
	if err != nil {
		return "", err
	}
	if data == nil {
		data = map[string]any{}
	}
	if !strings.HasSuffix(name, ".tmpl") {
		name = name + ".tmpl"
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, name, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
