package catalog

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Manifest is UserDataRoot/manifest.yaml (subset for phase 2 + workflows default_turn).
type Manifest struct {
	DefaultAgent string `yaml:"default_agent,omitempty"`
	Workflows    struct {
		DefaultTurn string `yaml:"default_turn,omitempty"`
	} `yaml:"workflows,omitempty"`
	// DefaultTurn is legacy top-level default_turn (still supported).
	DefaultTurn string `yaml:"default_turn,omitempty"`
}

// ResolvedDefaultTurn returns workflows.default_turn, else top-level default_turn, else default.turn.
func (m *Manifest) ResolvedDefaultTurn() string {
	if m == nil {
		return "default.turn"
	}
	if m.Workflows.DefaultTurn != "" {
		return m.Workflows.DefaultTurn
	}
	if m.DefaultTurn != "" {
		return m.DefaultTurn
	}
	return "default.turn"
}

// LoadManifest reads manifest.yaml under CatalogRoot (pass paths.CatalogRoot(UserDataRoot)).
func LoadManifest(catalogRoot string) (*Manifest, error) {
	path := manifestPath(catalogRoot)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Manifest{DefaultAgent: "default"}, nil
		}
		return nil, err
	}
	var m Manifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if m.DefaultAgent == "" {
		m.DefaultAgent = "default"
	}
	return &m, nil
}

func manifestPath(catalogRoot string) string {
	return filepath.Join(catalogRoot, "manifest.yaml")
}
