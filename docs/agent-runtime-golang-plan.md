# Golang Agent 运行时（oneclaw）— 立项摘要

本文是 **`docs/`** 与代码对齐的**短真源**：已删除的 `memory` 包、LLM 维护流水线、SQLite recall、JSONL 审计 sink 等**不再**在文中展开；范式对照仍以 **[`third-party/`](third-party/README.md)** 长文为准。

---

## 1. 目标与边界

- **文件与指令为真源**：`AGENT.md`、`MEMORY.md`、规则片段、`tasks.json`、转写与 `dialog_history` 等由 `workspace` / `instructions` / `session` 协同落盘与注入。
- **不训练模型**；不把无界历史塞回上下文；靠预算、`loop` 可见消息折叠与工具读盘补事实。
- **可观测**：`notify` 生命周期事件（薄、可 recover）；**无**内置多路审计 JSONL。

---

## 2. 当前包职责（与实现对齐）

| 区域 | 包 / 入口 |
|------|-----------|
| 进程与配置 | `cmd/oneclaw`、`config`、`rtopts` |
| 会话与编排 | `session`（`Engine`、`TurnRunner`、`turn_prepare`、`TurnHub`、`WorkerPool`） |
| 模型 ↔ 工具循环 | **`session/eino_*`**（Eino ADK；须 OpenAI key）、**`loop`**（Chat Completions 循环实现，可供直接调用）、**`tools`**、**`mcpclient`** |
| 每轮指令装配 | `instructions`（`BuildTurn`） |
| 路径与落盘 | `workspace`（Layout、transcript、`dialog_history`） |
| 定时任务 | `schedule`（`scheduled_jobs.json` + host poller） |
| 子 Agent | `subagent` |
| 提示模板 | `prompts`（如 `main_thread_system.tmpl`） |

**执行内核**：单一 **`einoTurnRunner`**；**`config.File`** 不再包含 **`agent.runtime`**（旧 YAML 中的键可被忽略）。**缺 OpenAI key 则模型回合失败**（不再回退）。嵌套子代理与主线程共用同一 **`TurnRunner`**。**`session.NewEngine`** 调用方需传入非 nil **`Registry`**。细则见 [`runtime-flow.md`](runtime-flow.md) §3.1 与 [`config.md`](config.md)。

---

## 3. 延伸阅读

| 文档 | 说明 |
|------|------|
| [`runtime-flow.md`](runtime-flow.md) | 启动、`SubmitUser`、`einoTurnRunner`（须密钥）、转写、定时、出站 |
| [`config.md`](config.md) | YAML 合并、`PushRuntime`、功能开关 |
| [`third-party/claude-code-vs-oneclaw.md`](third-party/claude-code-vs-oneclaw.md) | 与 Claude Code 产品层面对照 |

主线程 prompt 结构习惯见 [`prompts/10-main-thread.md`](prompts/10-main-thread.md)、[`prompts/00-request-envelope.md`](prompts/00-request-envelope.md)。
