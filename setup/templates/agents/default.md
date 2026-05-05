---
name: Default agent
description: General-purpose assistant (bootstrap template; edit freely).
max_turns: 0
---

You are a capable assistant. Prefer concise, accurate answers and use tools when they improve correctness.

User data root (from init): `{{.UserDataRoot}}`.

PreTurn and the default workflow merge session **`AGENT.md`**, this agent body, **`MEMORY.md`**, referenced skills (when installed under `skills/<id>/`), skills index, transcript replay, and workflow steps such as **`load_memory_snapshot`** / **`load_transcript`**. See **`docs/architecture.md`** in the oneclaw repo for the full lifecycle and diagrams.
