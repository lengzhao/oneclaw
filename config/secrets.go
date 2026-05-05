package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/lengzhao/oneclaw/keyfiles"
)

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

// ApplyAuthTokenFiles fills APIKey from auth.token_file (UserDataRoot-relative) when APIKey is still empty.
func ApplyAuthTokenFiles(userDataRoot string, f *File) {
	if f == nil || strings.TrimSpace(userDataRoot) == "" {
		return
	}
	root := filepath.Clean(strings.TrimSpace(userDataRoot))
	for i := range f.Models {
		if f.Models[i].APIKey != "" {
			continue
		}
		tf := strings.TrimSpace(f.Models[i].Auth.TokenFile)
		if tf == "" || strings.Contains(tf, "..") {
			continue
		}
		abs := filepath.Join(root, filepath.FromSlash(tf))
		sec, err := keyfiles.ReadBearerSecret(abs)
		if err != nil || strings.TrimSpace(sec) == "" {
			continue
		}
		f.Models[i].APIKey = sec
	}
}

// ApplyUserDataSecrets applies env-backed keys then key_files token files (call after resolving UserDataRoot).
func ApplyUserDataSecrets(userDataRoot string, f *File) {
	ApplyEnvSecrets(f)
	ApplyAuthTokenFiles(userDataRoot, f)
}
