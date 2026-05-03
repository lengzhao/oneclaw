package setup

import "embed"

//go:embed templates/config.yaml templates/manifest.yaml templates/AGENT.md templates/MEMORY.md templates/workflows/default.turn.yaml templates/agents/README.md
var templates embed.FS
