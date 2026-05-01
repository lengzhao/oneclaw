# oneclaw backlog / 文档与实现对齐

## Path B（已完成方向）

- **`memory/`、`maintainloop/`、`sessdb/`** 已从主路径移除；**`tasks.json`** 为任务真源；指令入口 **`AGENT.md` / `MEMORY.md`**，可选 **`SOUL.md` / `TODO.md`**（与 AGENT 同目录规则见 `workspace` + `instructions`）。
- **审计**：`notify/sinks` 下 JSONL 审计、`AppendMemoryAudit` / `audit/memory-write.jsonl`、以及 YAML **`features.disable_memory_audit` / `disable_audit_*`** 已移除。
- **仓库覆盖层**：若存在 **`<cwd>/.oneclaw/`** 目录，会话运行时（`tasks.json`、`scheduled_jobs.json`、`sidechain/` 等）与 **`DotOrDataRoot`** 类路径锚定在其下；**向上遍历**的 project 指令同时加载每一层目录下的 **`AGENT.md` / `rules/`** 以及 **`/.oneclaw/`** 内同名结构（见 `instructions/discover.go`）。

## 文档待清扫（高优先级）

- **已对齐主路径（节选）**：`docs/runtime-flow.md`、`docs/config.md`、`docs/user-root-workspace-layout.md`、`docs/session-home-isolation-design.md`、根目录 **`README.md`**。
- **仍偏历史规格、需读时自行对照代码**：`docs/embedded-maintain-scheduler-design.md`、`docs/memory-maintain-dual-entry-design.md`、`docs/memory-recall-sqlite-design.md` — 建议文首保留/补充 **「与当前 `cmd/oneclaw` / `config.File` 不一致处以代码为准」**。
- `docs/README.md` — 索引表链到上述文档时标明 **归档 / 历史**（可选）。
- `docs/prompts/README.md` — 若有 `memory` 包旧表述，改为 `instructions` / `workspace`。
- `test/e2e/CASES.md` — 若仍有已删测试的勾选说明，与当前用例对齐。

## 代码 / 测试（可选后续）

- 为 `instructions.DiscoverProjectInstructions` 的「每层 `/.oneclaw`」行为补充**单元测试**（非仅 E2E）。
- `workspace` 包目前无 `go test`；可对 `JoinSessionWorkspaceWithInstruction`、`Layout.DotOrDataRoot` 做小型表驱动测试。

---

*编号 backlog #1–#30 若仍存在于本文件历史版本中，可合并进上表；新工作以上表为准。*
