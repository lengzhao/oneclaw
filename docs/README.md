# 文档索引（oneclaw）

实现与扩展 **Go Agent / Memory 运行时** 时，以本目录为设计真源。内容已从原 `claude-code-2026-03-31/docs` 归档至此；**删除该子项目不影响本目录**。

---

## 先读这些

| 文档 | 说明 |
|------|------|
| [`agent-runtime-golang-plan.md`](agent-runtime-golang-plan.md) | **立项总览**：目标与边界、与 Claude Code 语义对应、Memory/Agent 要点、闭环示意、**包布局 + 阶段 A–D 任务与验收**、风险与后置 |
| [`claude-code-vs-oneclaw.md`](claude-code-vs-oneclaw.md) | **与 Claude Code 异同**：对齐点、oneclaw 优化与运维差异、缺失/后置能力一览 |
| [`todo.md`](todo.md) | 可勾选 backlog（P0–后置）与阶段 A–D 历史勾选 |
| [`runtime-flow.md`](runtime-flow.md) | **运行时主路径**：`main`、WorkerPool、`SubmitUser`、`loop.RunTurn`、维护双入口、出站与扩展装配 |
| [`config.md`](config.md) | 统一 YAML：合并顺序、`PushRuntime` / `rtopts`、密钥与功能开关 |

---

## 运行时设计（按主题）

| 文档 | 说明 |
|------|------|
| [`session-home-isolation-design.md`](session-home-isolation-design.md) | 用户根 `~/.oneclaw` 与 SessionHome、会话隔离与落地顺序 |
| [`user-root-workspace-layout.md`](user-root-workspace-layout.md) | **用户数据根 + `workspace/`**：InstructionRoot 与默认 cwd 拆分，`AGENT.md`/`MEMORY.md` 同目录 |
| [`multi-llm-provider-design.md`](multi-llm-provider-design.md) | 多 LLM / 多协议：`llm.Provider` 与分阶段改造 |
| [`outbound-events-design.md`](outbound-events-design.md) | 出站 `Record` / `Sink`、CLI/HTTP 行为 |
| [`notification-hooks-design.md`](notification-hooks-design.md) | 通知 Hook 与 outbound 分工、`NotifySink` |
| [`notify-sinks-audit-design.md`](notify-sinks-audit-design.md) | 审计类 JSONL Sink 与 Transcript 关系 |
| [`inbound-routing-design.md`](inbound-routing-design.md) | `Inbound` 字段、`SinkRegistry`、渠道注册 |
| [`embedded-maintain-scheduler-design.md`](embedded-maintain-scheduler-design.md) | 进程内 `maintainloop` 与 `RunScheduledMaintain` |
| [`memory-maintain-dual-entry-design.md`](memory-maintain-dual-entry-design.md) | 回合后维护 vs 定时维护双入口 |
| [`memory-recall-sqlite-design.md`](memory-recall-sqlite-design.md) | **Memory 片段索引与召回**：本地 SQLite **FTS-only**；语义扩展规划为外部 RAG；与 `SelectRecall` 迁移 |
| [`code-simplification-opportunities.md`](code-simplification-opportunities.md) | 已落实项摘要 + 剩余文档化/可选演进（`DefaultRegistry`、`OutboundSender` 等） |
| [`orchestrator-business-agents.md`](orchestrator-business-agents.md) | 主编排、`.oneclaw/agents`、`run_agent` 约定 |

### 渠道与 I/O

| 文档 | 说明 |
|------|------|
| [`im-channel-technical-design.md`](im-channel-technical-design.md) | 多 IM 接入原则与架构（对应当前 `routing` + `channel`） |
| [`picoclaw-channel.md`](picoclaw-channel.md) | 对标 [sipeed/picoclaw](https://github.com/sipeed/picoclaw) 的调研笔记（接口与分层对照） |
| [`clawbridge-migration-design.md`](clawbridge-migration-design.md) | **提案 / 未实施**：用 clawbridge 统一 I/O；**当前真源仍以** `inbound-routing-design`、**`outbound-events-design`** 与代码为准 |

---

## 实验与可选

| 文档 | 说明 |
|------|------|
| [`self-evolution-plan.md`](self-evolution-plan.md) | 「行为修正」闭环的可复现实验方案（与 §1 自我进化定义互补） |

---

## 设计参考（Claude Code 对照文）

用于理解范式，非本仓库实现规格。可按需阅读：

[`claude-code-main-flow-analysis.md`](claude-code-main-flow-analysis.md)、[`claude-code-memory-system.md`](claude-code-memory-system.md)、[`claude-code-subagent-system.md`](claude-code-subagent-system.md)、[`claude-code-skills-mechanism.md`](claude-code-skills-mechanism.md)、[`claude-code-callstack-and-parameter-flow.md`](claude-code-callstack-and-parameter-flow.md)、[`claude-code-core-tools.md`](claude-code-core-tools.md)、[`claude-code-agenttool-deep-dive.md`](claude-code-agenttool-deep-dive.md)

---

## Prompt 结构参考

见 [`prompts/README.md`](prompts/README.md) 及各文件（`00-request-envelope`、`10-main-thread`、`50-memory` 等）。

---

*若从仓库移除 `claude-code-2026-03-31`，以本 `docs/` 为唯一设计来源；需要源码对照时请单独克隆参考实现。*
