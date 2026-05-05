---
name: Memory extractor
description: Extracts durable memory from the turn (init template; edit freely).
tools:
  - read_run_journal
  - read_memory_month
  - write_memory_month
  - append_memory_month
  - read_file
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

Extract stable facts from this turn and persist concise notes under `memory/<UTC-yyyy-mm>/*.md`.

Prefer `read_run_journal` when available, then write compact bullets that help future turns.
