package paths

import "strings"

const defaultSessionSegment = "cli-default"

// SanitizeSessionPathSegment maps unsafe characters in a session id to '_' so the result is safe
// as a single path segment under sessions/<id>/ (no separators or "..").
func SanitizeSessionPathSegment(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return defaultSessionSegment
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := strings.Trim(b.String(), "_.")
	if out == "" {
		return defaultSessionSegment
	}
	return out
}
