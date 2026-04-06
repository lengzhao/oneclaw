package routing

import "strings"

// ParseLeadingSlash treats a user line as a slash command when it starts with "/" (after TrimSpace).
// cmd is the first path segment lowercased; args is the remainder of the line after that segment (may be empty).
// "/" alone returns ok=false (not a command).
func ParseLeadingSlash(text string) (cmd, args string, ok bool) {
	t := strings.TrimSpace(text)
	if !strings.HasPrefix(t, "/") {
		return "", "", false
	}
	rest := strings.TrimSpace(t[1:])
	if rest == "" {
		return "", "", false
	}
	i := 0
	for i < len(rest) && rest[i] == '/' {
		i++
	}
	rest = rest[i:]
	if rest == "" {
		return "", "", false
	}
	// First segment: run until space or end
	end := strings.IndexByte(rest, ' ')
	if end < 0 {
		return strings.ToLower(rest), "", true
	}
	cmd = strings.ToLower(strings.TrimSpace(rest[:end]))
	args = strings.TrimSpace(rest[end:])
	if cmd == "" {
		return "", "", false
	}
	return cmd, args, true
}
