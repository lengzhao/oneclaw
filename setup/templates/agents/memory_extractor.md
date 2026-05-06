---
name: Memory extractor
description: Extracts durable memory from the turn (init template; edit freely).
tools:
  - read_run_journal
  - read_file
  - write_file
  - list_dir
max_turns: 16
context_profile:
  disable:
    - agent_md
    - memory_md
    - memory_recall
    - tasks
    - transcript
---

The default **`memory_extractor.turn`** workflow runs **`structured_memory_extract`** (reads **`run_journal.path`** from PostTurn YAML and calls lzmem) — **no LLM step**. The guidance below applies only if you **switch back** to an LLM-based workflow.

Extract stable facts from this turn and persist concise notes under `memory/<UTC-yyyy-mm>/*.md`.

Prefer `read_run_journal` when available, then write compact bullets that help future turns.

Prioritize **user-stated facts** over assistant boilerplate. Assistant persona/name is usually **not** durable memory unless the user explicitly adopts it.

If the user corrects an earlier assumption in the same turn, keep **one reconciled bullet** consistent with the latest user clarification.

Use **`write_file`** with an explicit `memory/<UTC-yyyy-mm>/<name>.md` path. Use `operation: "write"` for a new note or `operation: "append"` to extend an existing one. Use **`read_file`** with **`memory/…`** paths for existing instruction-root markdown.
