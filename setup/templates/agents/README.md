# User agents

Place `*.md` agent definitions here (YAML frontmatter + body). Catalog **id** is the filename stem (`worker.md` → `worker`). This `README.md`, `*.readme.md`, and `*.tmpl` are ignored by the loader.

On first `oneclaw init`, embedded agent templates are copied when missing: **`default.md`**, **`memory_extractor.md`**, and **`skill_generator.md`**. Only **`default.md`** is parsed as Go `text/template` at bootstrap (exposes `{{.UserDataRoot}}`).

The **system prompt layout** for the main turn is **built into the binary** by default. Advanced users may override it by adding **`agents/<agent_type>.prompt.tmpl`** under the catalog root (same layout as the built-in default).

`memory_extractor.md` and `skill_generator.md` are intentionally slim by default via `context_profile.disable` (no `MEMORY` recall/transcript blocks unless you re-enable them).

If `default.md`, `memory_extractor.md`, or `skill_generator.md` is deleted, runtime still falls back to embedded built-in Catalog agents of the same ids, so turns and post-turn evolution keep working without requiring those files on disk.

See built-in catalog defaults and `docs/eino-md-chain-architecture.md`.
