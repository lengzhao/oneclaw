# 文档索引（oneclaw）

实现与扩展 **Go Agent / Memory 运行时** 时，以本目录**根下** Markdown 为设计真源。**[`third-party/`](third-party/README.md)** 存放 **Claude Code 对照长文**、官方 Hooks 调研、OMC 等第三方整理，**非** oneclaw 实现规格。内容已从原 `claude-code-2026-03-31/docs` 归档至此；**删除该子项目不影响本目录**。

---

## 先读这些

| 文档 | 说明 |
|------|------|
| [`agent-runtime-golang-plan.md`](agent-runtime-golang-plan.md) | **立项摘要**：目标与边界、当前包职责、延伸阅读（与实现对齐） |
| [`third-party/claude-code-vs-oneclaw.md`](third-party/claude-code-vs-oneclaw.md) | **与 Claude Code 异同**：对齐点、oneclaw 优化与运维差异、缺失/后置能力一览 |
| [`runtime-flow.md`](runtime-flow.md) | **运行时主路径**：`main`、WorkerPool、`SubmitUser`、`loop.RunTurn`、转写/dialog、定时任务、出站与扩展装配 |
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
| [`inbound-routing-design.md`](inbound-routing-design.md) | 入站字段、ToolContext 合并、`PublishOutbound` / `WorkerPool`（当前实现） |
| [`architecture-modularity-simplification.md`](architecture-modularity-simplification.md) | **模块化路线**：优先抽象/简化、`Engine` 收窄、I/O 与指令文件概念分层；拆仓库后置 |
| [`orchestrator-business-agents.md`](orchestrator-business-agents.md) | 主编排、`.oneclaw/agents`、`run_agent` 约定 |

### 渠道与 I/O

| 文档 | 说明 |
|------|------|
| [`im-channel-technical-design.md`](im-channel-technical-design.md) | 多 IM 接入原则与架构（**clawbridge + `WorkerPool`**） |
| [`picoclaw-channel.md`](picoclaw-channel.md) | 对标 [sipeed/picoclaw](https://github.com/sipeed/picoclaw) 的调研笔记（接口与分层对照） |
| [`clawbridge-migration-design.md`](clawbridge-migration-design.md) | **clawbridge I/O 契约**（字段、`PublishOutbound`、配置）；与 `inbound-routing-design` 配套 |

---

## 实验与可选

| 文档 | 说明 |
|------|------|
| [`self-evolution-plan.md`](self-evolution-plan.md) | 「行为修正」闭环的可复现实验方案（与 §1 自我进化定义互补） |

---

## Claude Code 与第三方对照（归档）

范式与能力对照、**非** oneclaw 实现规格。全文索引与条目说明见 **[`third-party/README.md`](third-party/README.md)**（含 `claude-code-*.md` 主流程/记忆/子 Agent/Skills/工具等，以及 Hooks 官方调研、oh-my-claudecode 等）。

---

## Prompt 结构参考

见 [`prompts/README.md`](prompts/README.md) 及各文件（`00-request-envelope`、`10-main-thread` 等）。

---

*若从仓库移除 `claude-code-2026-03-31`，以本 `docs/` 为唯一设计来源；需要源码对照时请单独克隆参考实现。*
