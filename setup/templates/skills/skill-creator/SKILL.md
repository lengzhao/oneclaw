---
name: skill-creator
description: Layout and quality bar for authoring skills in oneclaw (SKILL.md + optional scripts + reference).
---

# Skill authoring (oneclaw)

When you **create or extend** a skill under `skills/<skill-id>/`, follow this layout so humans and agents can use it reliably.

## Directory layout

| Path | Purpose |
|------|---------|
| `skills/<skill-id>/SKILL.md` | **Required.** Entry point: when to use, steps, constraints, safety. |
| `skills/<skill-id>/scripts/` | Optional. Runnable helpers (shell/python/etc.); keep short and documented. |
| `skills/<skill-id>/reference/` | Optional. Deep docs, API tables, examples (extra `.md` or `.txt`). |

- Use **one skill id per folder** (`skill-id` = stable slug: lowercase, hyphens).
- **SKILL.md** should stand alone: a reader gets 80% of value without opening `reference/`.
- Scripts must state **inputs, outputs, and failure modes** in SKILL.md or a short `scripts/README.md`.

## SKILL.md content

1. **Frontmatter** (optional): `name`, `description`.
2. **When to use** — triggers and non-goals.
3. **Steps** — numbered, testable actions.
4. **Constraints** — paths, secrets, tools, dry-run rules.
5. **Examples** — minimal good/bad snippets.

## Persistence tools

Use `write_skill_file` / `append_skill_file` with paths like:

- `skills/<skill-id>/SKILL.md`
- `skills/<skill-id>/scripts/example.sh`
- `skills/<skill-id>/reference/details.md`

Only use allowed extensions (markdown, text, common config/scripts); no `..` segments.

## Quality bar

- Prefer **short, executable** steps over prose.
- Do not silently overwrite user-owned skills without stating intent in the draft.
- If the pattern is unclear, produce a **minimal** SKILL.md draft and one reference note instead of a large tree.
