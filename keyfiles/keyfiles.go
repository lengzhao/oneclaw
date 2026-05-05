// Package keyfiles holds UserDataRoot/key_files conventions (docs/plans/unified-model-auth.md).
package keyfiles

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Dir returns <UserDataRoot>/key_files.
func Dir(userDataRoot string) string {
	return filepath.Join(userDataRoot, "key_files")
}

// Ensure creates key_files with restrictive permissions.
func Ensure(userDataRoot string) error {
	return os.MkdirAll(Dir(userDataRoot), 0o700)
}

// MergeWriteAPIKey sets api_key on disk, merging into existing JSON when present.
func MergeWriteAPIKey(absPath, apiKey string) error {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return fmt.Errorf("keyfiles: empty api key")
	}
	var cred CredentialBundle
	if raw, err := os.ReadFile(absPath); err == nil {
		_ = json.Unmarshal(raw, &cred)
	}
	cred.APIKey = apiKey
	return WriteCredentialBundle(absPath, &cred)
}

// ReadBearerSecret reads Bearer material from JSON (api_key 优先于 access_token),
// or treats non‑JSON file body as a single secret line.
func ReadBearerSecret(absPath string) (string, error) {
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}
	trim := strings.TrimSpace(string(raw))
	var v CredentialBundle
	if err := json.Unmarshal(raw, &v); err == nil {
		if s := v.Bearer(); s != "" {
			return s, nil
		}
		if strings.HasPrefix(trim, "{") {
			return "", fmt.Errorf("keyfiles: JSON in %s has no api_key/access_token", absPath)
		}
	}
	if trim == "" {
		return "", fmt.Errorf("keyfiles: empty secret in %s", absPath)
	}
	return trim, nil
}
