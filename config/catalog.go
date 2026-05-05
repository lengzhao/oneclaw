package config

import "strings"

// CatalogConfig is the config.yaml `catalog` block: default agent id and workflow YAML fallback stem.
type CatalogConfig struct {
	DefaultAgent string `yaml:"default_agent,omitempty"`
	Workflows    struct {
		DefaultTurn string `yaml:"default_turn,omitempty"`
	} `yaml:"workflows,omitempty"`
}

// ResolvedDefaultAgent returns catalog.default_agent or "default".
func (f *File) ResolvedDefaultAgent() string {
	if f == nil {
		return "default"
	}
	s := strings.TrimSpace(f.Catalog.DefaultAgent)
	if s == "" {
		return "default"
	}
	return s
}

// ResolvedDefaultTurn returns catalog.workflows.default_turn or "default.turn".
func (f *File) ResolvedDefaultTurn() string {
	if f == nil {
		return "default.turn"
	}
	s := strings.TrimSpace(f.Catalog.Workflows.DefaultTurn)
	if s == "" {
		return "default.turn"
	}
	return s
}
