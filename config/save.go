package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Save writes f to path as YAML after ApplyDefaults and Validate (0600: may contain clawbridge secrets).
func Save(path string, f *File) error {
	if f == nil {
		return nil
	}
	ApplyDefaults(f)
	if err := Validate(f); err != nil {
		return err
	}
	b, err := yaml.Marshal(f)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}
