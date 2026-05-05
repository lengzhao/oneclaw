# Skills under user data

`oneclaw init` walks the embedded **`setup/templates/`** tree and copies each file under **`skills/`** here when missing (**FR-CFG-02**); see **`setup/embed.go`** (`//go:embed templates`).

## **`skill-creator`** (bundled by default)

**`skill-creator`** is the **authoring spec** for reusable skills: layout (`SKILL.md`, optional `scripts/`, `reference/`), quality bar, and how **`write_file`** should write `skills/<skill-id>/...` artifacts.

It is **not** optional fluff: together with the built-in **`skill_generator`** agent and the default workflow’s async **`skill_agent`** step, it forms the **“extract recurring patterns → persist as `skills/<id>/` → inject on later turns”** loop — **固化能力**、减少同一类问题反复教模型。

Edit **`skills/skill-creator/SKILL.md`** after init if your team wants stricter or looser rules; overrides are yours.

## **Repository mirror**

The same **`skill-creator`** tree exists under **`examples/skills/skill-creator/`** in the repo for browsing and docs without running `init`; keep in sync with **`setup/templates/skills/skill-creator/`** when changing the spec.
