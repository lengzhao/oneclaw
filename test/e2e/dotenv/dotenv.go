// Package dotenv loads KEY=VALUE lines for e2e local credentials (minimal parser, no external deps).
package dotenv

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

const (
	EnvBaseURL      = "ONECLAW_E2E_BASE_URL"
	EnvAPIKey       = "ONECLAW_E2E_API_KEY"
	EnvDefaultModel = "ONECLAW_E2E_DEFAULT_MODEL"
)

// canonicalEnv maps alternate keys from .env files (case-insensitive) to ONECLAW_E2E_* names.
func canonicalEnv(raw string) (string, bool) {
	k := strings.TrimSpace(strings.ToUpper(strings.ReplaceAll(raw, "-", "_")))
	switch k {
	case "ONECLAW_E2E_BASE_URL", "BASE_URL":
		return EnvBaseURL, true
	case "ONECLAW_E2E_API_KEY", "API_KEY":
		return EnvAPIKey, true
	case "ONECLAW_E2E_DEFAULT_MODEL", "DEFAULT_MODEL":
		return EnvDefaultModel, true
	default:
		return "", false
	}
}

// unquote trims optional surrounding " or ' on a value.
func unquote(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 {
		if v[0] == '"' && v[len(v)-1] == '"' {
			return strings.TrimSpace(v[1 : len(v)-1])
		}
		if v[0] == '\'' && v[len(v)-1] == '\'' {
			return strings.TrimSpace(v[1 : len(v)-1])
		}
	}
	return v
}

// Load reads path and calls os.Setenv for recognized keys.
// Does not override variables already present in the process environment.
func Load(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(strings.ToLower(line), "export ") {
			line = strings.TrimSpace(line[7:])
		}
		idx := strings.IndexByte(line, '=')
		if idx <= 0 {
			return fmt.Errorf("dotenv %s:%d: missing '='", path, lineNo)
		}
		rawKey := strings.TrimSpace(line[:idx])
		val := unquote(line[idx+1:])
		canonical, ok := canonicalEnv(rawKey)
		if !ok {
			continue
		}
		if _, exists := os.LookupEnv(canonical); exists && strings.TrimSpace(os.Getenv(canonical)) != "" {
			continue
		}
		if err := os.Setenv(canonical, val); err != nil {
			return fmt.Errorf("dotenv %s:%d: %w", path, lineNo, err)
		}
	}
	return sc.Err()
}
