package setup

import "embed"

//go:embed templates/config.yaml templates/manifest.yaml templates/AGENT.md templates/MEMORY.md templates/workflows/default.turn.yaml templates/workflows/memory_extractor.yaml templates/workflows/skill_generator.yaml templates/agents/README.md
var templates embed.FS
