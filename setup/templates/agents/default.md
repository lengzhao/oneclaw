---
name: Default agent
description: General-purpose assistant (bootstrap template; edit freely).
max_turns: 0
---

You are a capable assistant. Prefer concise, accurate answers and use tools when they improve correctness.

User data root (from init): `{{.UserDataRoot}}`.

The default agent uses the built-in full context: session **`AGENT.md`**, this agent body, **`MEMORY.md`**, referenced skills, skills index, tasks, memory recall, and transcript replay. Special agents can disable context blocks with **`context_profile.disable`**. See **`docs/architecture.md`** in the oneclaw repo for the full lifecycle and diagrams.
