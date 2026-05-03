package config

import "os"

// ApplyEnvSecrets fills APIKey from APIKeyEnv per profile when APIKey is empty (FR-CFG-01).
func ApplyEnvSecrets(f *File) {
	if f == nil {
		return
	}
	for i := range f.Models {
		if f.Models[i].APIKey != "" {
			continue
		}
		if f.Models[i].APIKeyEnv == "" {
			continue
		}
		if v := os.Getenv(f.Models[i].APIKeyEnv); v != "" {
			f.Models[i].APIKey = v
		}
	}
}
