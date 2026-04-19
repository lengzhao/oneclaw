# Claude Code 与 oneclaw：异同说明

本文从**产品设计**与**运行时能力**两方面对照 **Claude Code**（Anthropic 的 IDE 内 Agent 产品及其可参考的 TS 实现形态）与 **oneclaw**（本仓库的 Go Agent / Memory 运行时）。细节实现以代码与 [`runtime-flow.md`](../runtime-flow.md) 为准；Claude Code 行为以 [`claude-code-main-flow-analysis.md`](claude-code-main-flow-analysis.md) 等对照文为纲。

**结论先说**：oneclaw **复用同一类分层与闭环语义**（编排 → query 循环 → 工具 → 记忆写回 → 子 Agent），但目标是 **可部署的后端运行时**（多通道、统一配置、可审计），而非 IDE 插件；因此会**刻意砍掉**部分产品化能力，并在**配置、路径、中文 recall、运维面**做加强。

---

## 1. 刻意对齐的部分（「同一范式」）

| 维度 | Claude Code（概念） | oneclaw |
|------|---------------------|---------|
| 整体形态 | 有状态的 Agent 运行时：多轮工具直到收敛 | 同左：`session` + `loop` 包 |
| 入口编排 | 用户输入 → slash / 附件 / 是否进模型 | `Inbound`、`SubmitUser`、本地 slash 旁路（见 [`memory-maintain-dual-entry-design.md`](../memory-maintain-dual-entry-design.md)） |
| 主循环 | 模型 → `tool_use` → 执行 → 回灌 | `loop.RunTurn` |
| 工具 | 权限、结果大小、失败与中止 | `tools` + `CanUseTool`；只读并行、写串行 |
| 上下文控制 | compact、budget、记忆预取 | 语义 compact（最小可用）+ `budget` + `ApplyTurnBudget`（UTF-8 字节与各段上限，见 [`config.md`](../config.md)） |
| Memory | scope、`MEMORY.md`、topic、daily log、维护整理 | `memory` 包全链路；回合后 + 定时双入口维护 |
| 子 Agent | 全量子 Agent + fork、侧链 transcript | `subagent` 包 + 侧链可选合回（`sidechain_merge`） |
| Skills | SKILL.md 发现与渐进注入 | `skills` 包 + `invoke_skill`（续作见 [`todo.md`](../todo.md)） |
| MCP | 连接外部工具 | `mcpclient` + YAML `mcp.servers`（续作：discovery、UI 级权限等） |

---

## 2. oneclaw 的差异化与优化（相对常见 TS/IDE 形态）

### 2.1 部署与配置

- **单一 YAML 真源**：合并顺序固定，敏感项以配置文件为主，**不依赖**向子进程泄漏式的环境变量约定（见 [`config.md`](../config.md)）。适合长期驻留进程、IM 渠道与 cron。
- **日志**：统一 **`log/slog`**，可文本/JSON、落盘与 stderr 双写。
- **Go 单二进制**：无 Node 运行时依赖，便于与 systemd / 容器 / 边车同部署。

### 2.2 数据布局与多会话

- **用户数据根**：默认 `~/.oneclaw`，与「项目 cwd 即宇宙中心」脱钩；配置、全局 `AGENT.md` / rules 在用户根聚合。
- **SessionHome 隔离（可选）**：IM 主路径上可将每会话 `cwd` 收到 `~/.oneclaw/sessions/<id>/`，减少多会话任务文件与相对路径串线（见 [`session-home-isolation-design.md`](../session-home-isolation-design.md)、`sessions.isolate_workspace`）。
- **WorkerPool**：按会话哈希分片串行，每任务新建 `Engine`、状态靠落盘恢复，控制常驻内存（见 [`config.md`](../config.md)「会话与多通道」）。

### 2.3 记忆与 recall

- **不实现 `@include`**：记忆与规则以**磁盘正文**为准，降低解析与可审计复杂度（见 [`agent-runtime-golang-plan.md`](../agent-runtime-golang-plan.md) §3）。
- **SelectRecall 中文友好**：CJK 使用重叠 bigram 等规则，避免整句被粘成单 token 导致召回失效（见 [`agent-runtime-golang-plan.md`](../agent-runtime-golang-plan.md) §3.1）。

### 2.4 工程与编排上的收敛

- **共享回合准备**：`prepareSharedTurn` 统一 `SubmitUser` 与本地 slash 的前半段，减少双路径漂移（见 [`code-simplification-opportunities.md`](../code-simplification-opportunities.md)）。
- **本地 slash 旁路**：无模型回合**不**跑 `PostTurn` / 回合后维护，避免噪声维护（与 Claude Code「有对话增量」前提对齐方式不同，属**显式产品选择**）。
- **出站聚合**：`channel.DrainTextReply` 等助手统一「多段文本 → 单条回复 + Done」语义，便于 HTTP/IM 接入。
- **可观测**：Notify Hook + 多路审计 JSONL（LLM 步 / 编排 / 用户可见），可按 feature 关闭（见 [`notification-hooks-design.md`](../notification-hooks-design.md)、[`notify-sinks-audit-design.md`](../notify-sinks-audit-design.md)）。

### 2.5 维护与运维

- **维护双入口**：回合后近场 + `maintain.interval` 进程内定时 + `oneclaw -maintain-once` / `maintain -once`，与独立维护进程分工清晰（见 [`memory-maintain-dual-entry-design.md`](../memory-maintain-dual-entry-design.md)、[`embedded-maintain-scheduler-design.md`](../embedded-maintain-scheduler-design.md)）。

---

## 3. 缺少或刻意后置的能力（相对 Claude Code 产品全集）

下列项**不等于**「永远不做」，多数在 [`todo.md`](../todo.md) 中有 backlog 或阶段位；此处强调与 Claude Code **完整产品**或对照文全量能力的差距。

| 类别 | 说明 |
|------|------|
| **IDE 内体验** | 无 VS Code/Cursor 深度集成（diff 内联、一键应用编辑等）；oneclaw 面向 CLI / HTTP / IM connector。 |
| **多实体协作** | teammate / swarm、mailbox、长期成员等 **未** 按产品级落地（[`todo.md`](../todo.md) #16、[`prompts/40-teammate.md`](../prompts/40-teammate.md) 仍为结构参考）。 |
| **向量 recall** | 文件型 recall 已有；**D3 向量插件**未接，文件仍为真源（#14）。 |
| **多 LLM / 多协议** | 当前主路径为 OpenAI 兼容栈；**`llm.Provider` 分阶段扩展**见 [`multi-llm-provider-design.md`](../multi-llm-provider-design.md)（#28）。 |
| **compact 高阶** | 最小语义 compact 已接；多段摘要、与模型协同的 collapse 等 **后置**（#29）。 |
| **全量遥测** | 有审计 JSONL 与日志；**产品级全链路遥测**未作为目标（#29）。 |
| **MCP 其余面** | 客户端主干已接；**tool discovery、UI 级权限流、进程内暴露 MCP Server** 等为续作（#30 说明）。 |
| **Skills 深度** | 主干已有；**审计、条件 paths、动态子目录**等续作（[`claude-code-skills-mechanism.md`](claude-code-skills-mechanism.md)）。 |
| **预算精细化** | UTF-8 + YAML 段上限 + usage 落盘已接；**按模型 tokenizer 的精确计量**为可选加强（[`todo.md`](../todo.md)「未完成任务一览」）。 |

---

## 4. 小结表

| | Claude Code（参考范式） | oneclaw |
|--|-------------------------|---------|
| **定位** | IDE 内深度集成、产品化交互 | 可部署运行时、多通道与运维友好 |
| **配置** | 随产品演变 | 统一 YAML + `PushRuntime` / `rtopts` |
| **记忆** | 含 `@include` 等丰富能力（对照文） | 磁盘正文优先；中文 recall 规则优化 |
| **协作** | teammate / swarm 等 | 主线程 + 子 Agent / fork；swarm 未齐 |
| **模型** | 随官方产品绑定 | 当前 OpenAI 兼容为主；多 provider 设计在后置 |
| **观测** | 产品内调试体验 | Hook + JSONL 审计 + `slog` |

---

## 5. 延伸阅读

- 范式总览与阶段验收：[`agent-runtime-golang-plan.md`](../agent-runtime-golang-plan.md)  
- 任务与缺口清单：[`todo.md`](../todo.md)  
- 主流程与代码路径：[`runtime-flow.md`](../runtime-flow.md)  
- Claude Code 分层说明：[`claude-code-main-flow-analysis.md`](claude-code-main-flow-analysis.md)

---

*对照文描述的是 Claude Code 类系统的**设计语义**；Anthropic 商业产品行为以官方为准。oneclaw 不声称兼容任何专有协议或 UI。*
