# oneclaw 需求说明（参考实现基线）

本文档从 oneclaw **`README.md`、`docs/*.md`、`cmd/oneclaw`、`config/`、主要业务包** 归纳 **已实现或承诺的行为**，用 **FR-xx / NFR-xx** 编号便于验收与差异跟踪。

- **与套件内其他文档**：抽象原则见 [reference-from-oneclaw.md](reference-from-oneclaw.md) **§3**（PRD 条目）；本文 **§3** 为其细化展开。术语见 [glossary.md](glossary.md)；路径不变量摘要见 [appendix-data-layout.md](appendix-data-layout.md)。
- **复制到新项目**：若不做 oneclaw 兼容实现，可**删除全文**，仅保留 `reference-from-oneclaw.md` + `eino-md-chain-architecture.md`。若 fork oneclaw，请随代码变更更新本节，并以仓库内 `docs/config.md`、`docs/runtime-flow.md` 为真源。

---

## 1. 文档属性

| 项 | 说明 |
|----|------|
| 产品定位 | **Claw** 生态下的 **Go Agent 运行时**：连接 OpenAI 兼容模型、内置/MCP 工具、clawbridge 等渠道，自动化推进用户任务并落盘可追溯状态 |
| 追溯范围 | oneclaw 仓库根 `README.md`、`docs/agent-runtime-golang-plan.md`、`docs/config.md`、`docs/runtime-flow.md`、`docs/inbound-routing-design.md` 及对应 Go 包 |
| 排除 | 未在 `config.File` / 主路径实现的 YAML 键（如文档所述已移除的 maintain 流水线等）；标注为设计草案未落地的能力 |

---

## 2. 产品概述

### 2.1 价值主张

- 提供 **可长期运行** 的会话编排：模型调用与工具循环由 **Eino ADK** 执行（须配置 `openai.api_key`）。
- **文件化记忆与规则**（`AGENT.md`、`MEMORY.md`、`rules/`、`agents/*.md` 等）按轮注入，配合 **字节预算** 控制上下文，避免无界塞满历史。
- 支持 **多会话、多通道**（clawbridge）、**定时合成入站**、**子 Agent**、**主动出站**（`send_message`），便于自动化完成用户需求而非单次问答。

### 2.2 明确不做（范围外）

- 不训练或微调模型权重。
- 不把全量历史无差别注入上下文；不以向量库替代文件真源（向量检索为可选后续）。
- **无**进程内 LLM 自动维护 memory 的常驻流水线；演进依赖工具与用户/模型 **显式写文件**。

---

## 3. 功能性需求

### 3.1 配置与初始化

| ID | 需求描述 | 备注 / 追溯 |
|----|-----------|-------------|
| FR-CFG-01 | 运行时配置以 **合并后的 YAML** 为唯一真源；**不**用环境变量覆盖 OpenAI 密钥等运行时项 | `docs/config.md` |
| FR-CFG-02 | 配置合并顺序：`~/.oneclaw/config.yaml` 底层，`oneclaw -config <path>` 高层；`-config` 相对路径相对于 `~/.oneclaw/` | `config.Load` |
| FR-CFG-03 | 合并后调用 `PushRuntime()`，向 `rtopts` 注入预算、开关、路径等快照供低耦合包读取 | `docs/config.md` |
| FR-CFG-04 | `-init`：在用户数据根写入模板（含 `config.yaml`、AGENT.md、MEMORY 等）；已有 `config.yaml` 时 **仅补缺失键、不覆盖** | `cmd/oneclaw`、`config.InitWorkspace` |
| FR-CFG-05 | TTY 下 `-init` 可对密钥、模型、`sessions.isolate_workspace`、clawbridge 预设等交互提示 | `cmd/oneclaw/main.go` |
| FR-CFG-06 | `-export-session`：将 `UserDataRoot` 快照复制到指定目录，**无需 API Key** | `workspace.ExportSessionSnapshot` |
| FR-CFG-07 | CLI 覆盖：`-log-level`、`-log-format`、`-log-file`（路径规则见 config 文档） | `docs/config.md` |

### 3.2 模型与执行内核

| ID | 需求描述 | 备注 / 追溯 |
|----|-----------|-------------|
| FR-MOD-01 | 使用 **OpenAI 兼容 HTTP API**；`openai.base_url` 等由 YAML 配置 | `docs/config.md` |
| FR-MOD-02 | 主会话模型回合 **固定走 Eino ADK**（`einoTurnRunner`）；**必须**配置 `openai.api_key`，否则模型回合失败 | `docs/runtime-flow.md` §3.1 |
| FR-MOD-03 | `agent.max_steps`、`agent.max_tokens` 控制每用户回合内迭代与补全上限 | `docs/config.md` |
| FR-MOD-04 | `loop` 包保留 Chat Completions 循环实现，供测试或非 ADK 路径；**主进程会话不以之为默认内核** | `docs/agent-runtime-golang-plan.md` |

### 3.3 会话编排与并发

| ID | 需求描述 | 备注 / 追溯 |
|----|-----------|-------------|
| FR-SES-01 | 常驻模式：`TurnHub` + `MainEngineFactory`；按 `SessionHandle`（client + session key）区分会话 | `docs/runtime-flow.md`、`docs/inbound-routing-design.md` §8 |
| FR-SES-02 | 同会话入站按 `sessions.turn_policy`（serial / insert / preempt）编排 | `docs/config.md` |
| FR-SES-03 | **每条入站任务新建 `Engine`，`SubmitUser` 结束后丢弃**；持久状态依赖转写等落盘 | `docs/inbound-routing-design.md` §8 |
| FR-SES-04 | `sessions.isolate_workspace`：**false** 时共享 `<UserDataRoot>/workspace`；**true** 时每会话 `<UserDataRoot>/sessions/<id>/workspace` | `docs/config.md` |
| FR-SES-05 | 稳定会话目录名：`StableSessionID` 由 client + session key 派生 | `session` |
| FR-SES-06 | 常驻模式需至少一个启用的 `clawbridge.clients`，否则退出 | `docs/runtime-flow.md` |

### 3.4 单轮用户回合（SubmitUser）

| ID | 需求描述 | 备注 / 追溯 |
|----|-----------|-------------|
| FR-TURN-01 | 装配：`instructions` / memory bundle、预算、`ToolContext`、入站元数据；写用户行到 transcript（成功保存策略见落盘） | `session/engine.go`、`docs/runtime-flow.md` |
| FR-TURN-02 | 成功后：可见消息折叠、保存 transcript / working transcript、`dialog_history` 追加 | `README.md`、`workspace/dialog_history.go` |
| FR-TURN-03 | 内置 **本地斜杠**：`/help`、`/model`、`/session`、`/status`、`/paths`、`/reset`、`/stop` 等 **不调用模型** | `session/slash_local.go` |
| FR-TURN-04 | `/stop` 与 `TurnHub.CancelInflightTurn` 配合取消当前执行中的回合 | `docs/runtime-flow.md` |
| FR-TURN-05 | CLI REPL：`/exit` 退出（终端侧） | `README.md` |

### 3.5 内置工具

| ID | 需求描述 | 备注 / 追溯 |
|----|-----------|-------------|
| FR-TOOL-01 | 默认注册：`read_file`、`write_file`、`grep`、`glob`、`list_dir`、`exec`、`run_agent`、`fork_context`、`invoke_skill`、`task_create`、`task_update`、`cron`、`send_message` | `tools/builtin/default.go` |
| FR-TOOL-02 | 针对 AGENT.md / rules / skills / MEMORY 等的写入走 **策略约束**（`write_behavior_policy`） | `tools/builtin/write_behavior_policy.go` |
| FR-TOOL-03 | `exec`：`sh -c`、默认前台超时（README 称约 30s）、可选 background | `README.md` |
| FR-TOOL-04 | `cron`：持久化 `scheduled_jobs.json`（默认 `UserDataRoot`），到期经 poller **合成入站** 走完整回合 | `docs/runtime-flow.md` §6 |
| FR-TOOL-05 | `send_message`：主动推送到当前或指定 channel，**不经模型再生成一轮** | `README.md` |
| FR-TOOL-06 | `features.disable_scheduled_tasks` 关闭定时总开关 | `docs/runtime-flow.md` |

### 3.6 记忆与指令注入

| ID | 需求描述 | 备注 / 追溯 |
|----|-----------|-------------|
| FR-MEM-01 | 每轮注入 `MEMORY.md`、`AGENT.md`、规则等与布局相关的说明块；受 `budget.*` 与各段 `*_max_bytes` 约束 | `README.md`、`docs/config.md` |
| FR-MEM-02 | `features.disable_transcript` 等开关关闭对应落盘能力 | `README.md` |

### 3.7 Skills

| ID | 需求描述 | 备注 / 追溯 |
|----|-----------|-------------|
| FR-SKL-01 | Skills 从用户 catalog + 会话/项目 catalog 加载；主线程 prompt 中带索引列表，通过 **`invoke_skill`** 拉取 `SKILL.md` 全文 | `skills/`、`session/system.go` |
| FR-SKL-02 | `skills.recent_path` 可覆盖最近使用记录路径 | `docs/config.md` |

### 3.8 子 Agent / 多 Agent

| ID | 需求描述 | 备注 / 追溯 |
|----|-----------|-------------|
| FR-AGT-01 | `agents/*.md`：YAML frontmatter + 正文为 system；字段含 `agent_type`/`name`、`description`、`tools`、`max_turns`、`model`、`omit_memory_injection` 等 | `subagent/definition.go`、`subagent/catalog.go` |
| FR-AGT-02 | `run_agent`：按定义过滤父 Registry，去掉元工具；`fork_context` 另一套收缩规则 | `docs/inbound-routing-design.md` §9 |
| FR-AGT-03 | 内置 definition 与用户文件同名时 **用户覆盖** | `subagent/catalog.go` |

### 3.9 入站 / 出站与路由

| ID | 需求描述 | 备注 / 追溯 |
|----|-----------|-------------|
| FR-RT-01 | 入站统一为 clawbridge `bus.InboundMessage`；出站经 `Engine.publishOutbound`、`Bridge.Bus().PublishOutbound` | `docs/inbound-routing-design.md` |
| FR-RT-02 | `ToolContext.TurnInbound` 合并元数据；正文不参与合并；附件路径继承规则见实现 | `toolctx/context.go` |
| FR-RT-03 | `statichttp` 等适配：`POST /api/chat` 支持 JSON 与 multipart，附件落盘至约定目录后给模型路径提示 | `docs/inbound-routing-design.md` §7 |

### 3.10 多模态

| ID | 需求描述 | 备注 / 追溯 |
|----|-----------|-------------|
| FR-MM-01 | 默认图片/音频可走 Chat Completions 多模态字段；`features.disable_multimodal_image` / `disable_multimodal_audio` 为 true 时退回路径提示 | `docs/config.md` |

### 3.11 MCP

| ID | 需求描述 | 备注 / 追溯 |
|----|-----------|-------------|
| FR-MCP-01 | `mcp.enabled: true` 后连接配置的 stdio/SSE/HTTP MCP；工具以 `mcp_*` 前缀注册；可选系统提示附注 | `docs/config.md`、`mcpclient/` |

### 3.12 可观测与审计痕迹

| ID | 需求描述 | 备注 / 追溯 |
|----|-----------|-------------|
| FR-OBS-01 | `notify`：`Engine.Notify` 多路 Sink；精简事件含 user_input / turn_start / turn_end 等（详见 `notify`、`session`） | `docs/notification-hooks-design.md` |
| FR-OBS-02 | 按轮执行流水：`execution/<agent_id>/<date>/<turn_id>.jsonl`（`session/exec_journal.go`） | `docs/notification-hooks-design.md` |
| FR-OBS-03 | 结构化日志：`log/slog`，级别与格式由配置与 CLI 覆盖 | `README.md` |

### 3.13 上下文预算

| ID | 需求描述 | 备注 / 追溯 |
|----|-----------|-------------|
| FR-BUD-01 | `budget.max_prompt_bytes` 或 `max_context_tokens`、`history_max_bytes`、`min_transcript_messages` 等控制注入与历史裁剪 | `docs/config.md` |
| FR-BUD-02 | `features.disable_context_budget` 关闭预算收紧 | `docs/config.md` |

---

## 4. 非功能性需求

| ID | 类别 | 描述 |
|----|------|------|
| NFR-01 | 运行时 | Go 版本见根目录 `go.mod`（README 写明最低版本） |
| NFR-02 | 安全 | 工具执行与写文件需路径与策略校验；exec 超时与后台策略见工具实现 |
| NFR-03 | 可靠性 | 模型/工具失败按会话返回错误；Hook 内 panic recover，不翻转回合语义（见 notify 设计） |
| NFR-04 | 可维护性 | 包职责分割：`session`、`loop`、`workspace`、`tools`、`subagent`、`routing`、`schedule`、`config`（见 README 仓库布局） |

---

## 5. 数据与目录（摘要）

| 概念 | 默认 / 说明 |
|------|-------------|
| `UserDataRoot` | `~/.oneclaw` |
| 转写（IM） | `<UserDataRoot>/sessions/<SessionID>/transcript.json`、`working_transcript.json` |
| 定时任务 | `<UserDataRoot>/scheduled_jobs.json` |
| Agent 定义 | `workspace` 解析下的 `agents/*.md` |
| 初始化模板源 | `config/init_template/`（`-init` 嵌入复制） |

路径不变量（InstructionRoot / 会话隔离）见套件内 [appendix-data-layout.md](appendix-data-layout.md)；字段级说明仍以 oneclaw `docs/config.md` 与 `workspace/` 为准。

---

## 6. 对外依赖（摘要）

- **clawbridge**：常驻模式入站/出站与 drivers。
- **Eino / eino-ext**：ADK 与 OpenAI ChatModel 组件。
- **OpenAI 官方 Go SDK**：部分类型与默认模型常量（如 `cmd/oneclaw/main.go`）。

---

## 7. 需求与 oneclaw 源码映射（可选对照）

**以下路径相对于 oneclaw 仓库根目录**。仅在与 oneclaw 对齐或二次开发时使用；复制本套件到其他项目且无 oneclaw 树时，**可整节删除**。

| 主题 | oneclaw 文档 | 主要 Go 包 |
|------|----------------|------------|
| 启动与主路径 | `docs/runtime-flow.md` | `cmd/oneclaw`、`session` |
| 配置项全集 | `docs/config.md` | `config`、`rtopts` |
| 入站出站 | `docs/inbound-routing-design.md` | `session`、`routing` |
| 包职责 | `docs/agent-runtime-golang-plan.md` | 见该文内表 |
| 生命周期 Hook | `docs/notification-hooks-design.md` | `notify`、`session` |

---

## 8. 修订记录

| 日期 | 说明 |
|------|------|
| （文档创建） | 根据当前仓库 README + docs + 主路径代码梳理首版 |
