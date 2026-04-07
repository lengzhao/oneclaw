# 文档索引（oneclaw）

实现 **Go Agent / Memory 运行时** 时，请先阅读此处文档。内容已从原 `claude-code-2026-03-31/docs` 归档到本目录，**删除该子项目不影响本目录**。

## 立项与排期

| 文档 | 说明 |
|------|------|
| [todo.md](todo.md) | 可勾选任务清单（与阶段 A–D 一一对应） |
| [agent-runtime-golang-plan.md](agent-runtime-golang-plan.md) | Golang 实现的目标、边界、与 Claude Code 范式对应关系、分期里程碑 |
| [go-runtime-development-plan.md](go-runtime-development-plan.md) | 分阶段任务拆解、验收标准、包布局建议 |

## oneclaw 运行时设计

| 文档 | 说明 |
|------|------|
| [config.md](config.md) | 统一 YAML 配置：合并顺序、`--config`、API key、`PushRuntime` / `rtopts` |
| [multi-llm-provider-design.md](multi-llm-provider-design.md) | 多 LLM / 多协议支持：与 picoclaw 对齐的配置形态、`llm.Provider` 抽象与分阶段改造 |
| [outbound-events-design.md](outbound-events-design.md) | 出站事件 envelope、`Record`/`Sink`、CLI/HTTP 行为 |
| [inbound-routing-design.md](inbound-routing-design.md) | 入站 `Inbound` 字段表、可选 `ctx` 透传、`SinkRegistry`（`routing` 核心 + `channel/*` 子包按渠道注册终端 `Sink` 等） |
| [embedded-maintain-scheduler-design.md](embedded-maintain-scheduler-design.md) | 主进程 **`maintainloop`**：合并 YAML **`maintain.interval` 非空** 时启动；先跑 1 次再周期；**`RunScheduledMaintain`**；**`disable_scheduled_maintenance`**；与 **`oneclaw -maintain-once`** / `cmd/maintain` 互斥写盘 |
| [memory-maintain-dual-entry-design.md](memory-maintain-dual-entry-design.md) | 回合后 vs 定时维护：**两个代码入口**（`RunPostTurnMaintain` / `RunScheduledMaintain`）、开关独立、共享边界与迁移步骤 |
| [code-simplification-opportunities.md](code-simplification-opportunities.md) | **工程梳理**：可简化代码的路径（session 双路径、`toolctx`、routing、channel、subagent、memory）、优先级与实施顺序（不含具体补丁） |
| [orchestrator-business-agents.md](orchestrator-business-agents.md) | **主编排 / 业务 Agent**：主线程注入 `.oneclaw/agents` 目录、`run_agent` 动态描述、子 Agent 禁止再委派；异步并行预留 |

## 设计参考（原 Claude Code 分析文）

| 文档 | 说明 |
|------|------|
| [claude-code-main-flow-analysis.md](claude-code-main-flow-analysis.md) | 主流程与分层 |
| [claude-code-memory-system.md](claude-code-memory-system.md) | 记忆系统 |
| [claude-code-subagent-system.md](claude-code-subagent-system.md) | 子 Agent |
| [claude-code-callstack-and-parameter-flow.md](claude-code-callstack-and-parameter-flow.md) | 调用栈与参数流 |
| [claude-code-core-tools.md](claude-code-core-tools.md) | 核心工具 |
| [claude-code-agenttool-deep-dive.md](claude-code-agenttool-deep-dive.md) | Agent 工具深入 |

## Prompt 结构参考

见 [prompts/README.md](prompts/README.md) 及各文件（`00-request-envelope`、`10-main-thread`、`50-memory` 等）。

---

*更新说明：若从仓库中移除 `claude-code-2026-03-31`，请以本 `docs/` 为唯一设计来源；需要源码对照时请单独克隆参考实现。*
