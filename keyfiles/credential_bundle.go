package keyfiles

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CredentialBundle is the unified key_files JSON layout (docs/plans/unified-model-auth.md).
// Bearer resolution: api_key > access_token (legacy files may still carry unused token fields).
type CredentialBundle struct {
	SchemaVersion int             `json:"schema_version,omitempty"`
	Provider      string          `json:"provider,omitempty"`
	APIKey        string          `json:"api_key,omitempty"`
	AccessToken   string          `json:"access_token,omitempty"`
	RefreshToken  string          `json:"refresh_token,omitempty"`
	ExpiresAt     string          `json:"expires_at,omitempty"`
	TokenType     string          `json:"token_type,omitempty"`
	Extra         json.RawMessage `json:"extra,omitempty"`
}

// AlibabaCredential is an alias for persisted DashScope / Alibaba-shaped JSON.
type AlibabaCredential = CredentialBundle

// Bearer returns api_key if set, else access_token.
func (c *CredentialBundle) Bearer() string {
	if c == nil {
		return ""
	}
	if s := strings.TrimSpace(c.APIKey); s != "" {
		return s
	}
	return strings.TrimSpace(c.AccessToken)
}

// ReadCredentialBundle reads JSON credential file (missing file → error).
func ReadCredentialBundle(absPath string) (*CredentialBundle, error) {
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	trim := strings.TrimSpace(string(raw))
	var v CredentialBundle
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	if strings.HasPrefix(trim, "{") && v.APIKey == "" && v.AccessToken == "" {
		return nil, fmt.Errorf("keyfiles: JSON in %s has no api_key/access_token", absPath)
	}
	return &v, nil
}

// ReadAlibabaCredential reads JSON (alias for ReadCredentialBundle).
func ReadAlibabaCredential(absPath string) (*AlibabaCredential, error) {
	return ReadCredentialBundle(absPath)
}

// WriteCredentialBundle writes cred atomically with mode 0600.
func WriteCredentialBundle(absPath string, cred *CredentialBundle) error {
	if cred == nil {
		return fmt.Errorf("keyfiles: nil credential")
	}
	raw, err := json.MarshalIndent(cred, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o700); err != nil {
		return err
	}
	return os.WriteFile(absPath, raw, 0o600)
}

// WriteAlibabaCredential marshals cred and writes atomically with mode 0600.
func WriteAlibabaCredential(absPath string, cred *AlibabaCredential) error {
	return WriteCredentialBundle(absPath, cred)
}
