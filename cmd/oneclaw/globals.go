package main

import (
	"fmt"
	"strings"
)

// globalOpts are flags that may appear before the subcommand (FR-CFG-04).
type globalOpts struct {
	ConfigPath string
	LogLevel   string
	LogFormat  string
}

// parseLeadingFlags consumes leading -/-- arguments until the first non-flag token.
func parseLeadingFlags(argv []string) (globalOpts, []string, error) {
	var g globalOpts
	rest := argv
	for len(rest) > 0 {
		a := rest[0]
		if a == "--" {
			return g, rest[1:], nil
		}
		if !strings.HasPrefix(a, "-") {
			break
		}

		switch {
		case a == "-config" || a == "--config":
			if len(rest) < 2 {
				return g, nil, fmt.Errorf("flag %s requires a value", a)
			}
			g.ConfigPath = rest[1]
			rest = rest[2:]

		case strings.HasPrefix(a, "--config="):
			g.ConfigPath = strings.TrimPrefix(a, "--config=")
			rest = rest[1:]

		case a == "-log-level" || a == "--log-level":
			if len(rest) < 2 {
				return g, nil, fmt.Errorf("flag %s requires a value", a)
			}
			g.LogLevel = rest[1]
			rest = rest[2:]

		case strings.HasPrefix(a, "--log-level="):
			g.LogLevel = strings.TrimPrefix(a, "--log-level=")
			rest = rest[1:]

		case a == "-log-format" || a == "--log-format":
			if len(rest) < 2 {
				return g, nil, fmt.Errorf("flag %s requires a value", a)
			}
			g.LogFormat = rest[1]
			rest = rest[2:]

		case strings.HasPrefix(a, "--log-format="):
			g.LogFormat = strings.TrimPrefix(a, "--log-format=")
			rest = rest[1:]

		default:
			return g, nil, fmt.Errorf("unknown global flag: %s", a)
		}
	}
	return g, rest, nil
}
