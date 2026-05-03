# User agents

Place `*.md` agent definitions here (YAML frontmatter + body). Catalog **id** is the filename stem (`worker.md` → `worker`). This `README.md`, `*.readme.md`, and `*.tmpl` are ignored by the loader.

On first `oneclaw init`, **`default.md`** is written from the embedded **`templates/agents/default.md`** (still parsed as Go `text/template` once at bootstrap; exposes `{{.UserDataRoot}}`). Edit **`default.md`** after bootstrap as needed.

The **system prompt layout** for the main turn is **built into the binary** by default. Advanced users may override it by adding **`agents/<agent_type>.prompt.tmpl`** under the catalog root (same layout as the built-in default).

See built-in catalog defaults and `docs/eino-md-chain-architecture.md`.
