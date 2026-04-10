# 统一配置（config 包）

开发与生产共用同一套加载规则：`github.com/lengzhao/oneclaw/config`。

## 配置文件路径与合并顺序

从低到高优先级（后者覆盖前者）：

1. **用户级**：`~/.oneclaw/config.yaml`
2. **项目级**：`<cwd>/.oneclaw/config.yaml`
3. **显式文件**：`oneclaw -config /path/to.yaml` 或 `maintain -config /path/to.yaml`（相对路径相对于 `-cwd` / 当前进程的 cwd）

缺失的文件会被忽略；若 `-config` 指向的路径不存在，启动报错。

**初始化项目**：`oneclaw -init`（可选 `-cwd <dir>`）会把 `config/init_template/` **整棵复制**到 `<cwd>/.oneclaw/`（目标路径已存在则**不覆盖**），并创建记忆目录。其中 `config.yaml`：若 init 前**已存在**，则在**保留已有键值**的前提下，把嵌入模板里**缺失**的键补进该文件（嵌套 mapping 递归合并；已存在的列表、标量不覆盖）。仅当确有新增键时才会重写 `config.yaml`（重写后 YAML 注释可能丢失）。模板目录还包含默认 `AGENT.md`、`memory/MEMORY.md` 等，见 [`config/init_template/`](../config/init_template/)。

## 敏感项（API Key）

在合并后的 YAML 中设置 `openai.api_key`，由 `openai.NewClient(config.OpenAIOptions()...)` 注入客户端。**oneclaw 不再从进程环境读取 `OPENAI_API_KEY` / `ONCLAW_*` 等做运行时配置**；未配置 key 时进程会报错并提示在 YAML 中填写。

## 运行时扁平化（`PushRuntime` / `rtopts`）

`config.Load` 合并 YAML 后，入口应调用 `(*Resolved).PushRuntime()`：将预算、路径、`features.disable_*`、`maintain.*`、`chat.transport` 等写入包 `rtopts` 的进程内快照。`memory`、`budget`、`loop` 等包通过 `rtopts.Current()` 读取，避免与 `config` 循环依赖。

单测可 `rtopts.Set(nil)` 或 `rtopts.Set(&customSnapshot)` 覆盖；合并逻辑仍以 YAML 为准。

## CLI 与日志

| 标志 | 说明 |
|------|------|
| `-cwd` | 项目根目录（默认当前目录） |
| `-config` | 额外 YAML 层（见上合并顺序） |
| `-log-level` | `debug` / `info` / `warn` / `error`，非空时覆盖配置里的 `log.level` |
| `-log-format` | `text` / `json`，非空时覆盖 `log.format` |
| `-init` | 初始化 `.oneclaw`；无 `config.yaml` 则写入模板，已有则合并补全缺失键（不覆盖），仅用上述日志标志 |
| `-maintain-once` | 单次远场维护后退出（需 YAML 中的 API key 等） |

## YAML 字段速查（无环境变量覆盖）

| 区域 | 主要 YAML 路径 | 说明 |
|------|----------------|------|
| 模型 | `model` | 默认聊天模型；空则代码内默认 |
| 主会话循环 | `agent.max_steps`、`agent.max_tokens` | `max_steps`：每用户回合内模型调用步数（默认 **100**，范围 1–256）。`max_tokens`：每步 **`max_completion_tokens`**（默认 **32768**，范围 1024–131072；YAML 写 0 或不写则用默认）。`cmd/oneclaw` 经 `MainEngineFactory` 写入 `Engine.MaxTokens`。 |
| 传输 | `chat.transport` | `auto`（先流式、失败再非流式）、`non_stream`、`stream`；兼容网关仅支持非流式时建议 `non_stream` |
| OpenAI 兼容 | `openai.api_key`、`openai.base_url`、`openai.org_id`、`openai.project_id` | `base_url` 需含 `/v1/` 后缀（若网关要求） |
| 路径 | `paths.memory_base`、`paths.transcript`、`paths.working_transcript`、`paths.working_transcript_max_messages` | 相对路径相对 `-cwd`；**`cmd/oneclaw` 多会话模式**下，每逻辑会话的转写落盘见下节「会话」，**不再**使用此处全局 `transcript` / `working_transcript` 路径；`working_transcript_max_messages` 仍适用。其他入口若仍用单 `Engine`，行为见各命令文档。单 `Engine` 时：主线程在每轮成功 `RunTurn` 后把 **内存 `Messages`** 折叠为**用户可见**（去掉 agentMd / 路由 / recall / compact 注入与 tool 轮次等）；`working_transcript` 与内存同形；`working_transcript_max_messages` 截尾部可见条数，`0` 默认 **30**，负数不限制 |
| 会话 | `sessions.disable_sqlite`、`sessions.sqlite_path`、`sessions.worker_count` | 见下 **「会话与多通道（`cmd/oneclaw`）」** |
| 预算 | `budget.*` | 见下表 |
| 开关 | `features.disable_*` | `true` 为关闭；省略或 `false` 为开启 |
| 通知审计 | `features.disable_audit_sinks`、`disable_audit_llm`、`disable_audit_orchestration`、`disable_audit_visible` | 默认三路全开；`disable_audit_sinks` 关闭全部；其余按路径关闭。`cmd/oneclaw` 有 `SessionID` 时 JSONL 在 `.oneclaw/sessions/<id>/audit/...`（见 [notify-sinks-audit-design.md](notify-sinks-audit-design.md)） |
| 入站多模态 | `features.disable_multimodal_image`、`features.disable_multimodal_audio` | 默认 **不** 禁用：图片注入 Chat Completions `image_url`（data URL），wav/mp3 注入 `input_audio`；任一为 `true` 时对应类型仅保留 read_file 路径提示，不送多模态载荷 |
| 维护 | `maintain.*` | 定时/远场/回合后参数；`maintain.interval` 非空时主进程内 `maintainloop` 周期唤醒 |
| 日志 | `log.level`、`log.format` | 可被 CLI 覆盖 |
| 侧链 | `sidechain_merge` | 留空关闭；`1` / `true` / `tool` / `append` / `user` 等见历史设计文档 |
| 用量 | `usage.*` | 见下节 |
| 调度睡眠 | `schedule.min_sleep`、`schedule.idle_sleep` | Agent 定时任务调度用 Go duration 字符串 |
| 语义 compact | `semantic_compact.summary_max_bytes` | |
| Skills | `skills.recent_path` | 可选覆盖 skills 最近列表路径 |
| MCP | `mcp.enabled`、`mcp.servers.<name>.*` | 显式 `mcp.enabled: true` 后连接外部 MCP；`servers` 下每项 `enabled`、`command`+`args`（stdio）或 `url`+`type`（`sse`/`http`），可选 `env`、`env_file`、`headers`；工具以 `mcp_*` 前缀注册 |

**`disable_scheduled_maintenance`**：关闭进程内 `maintainloop` 与 `cmd/maintain` 的 interval 循环；**不**影响 `oneclaw -maintain-once` / `maintain -once`。

### 上下文预算（`budget.*`，UTF-8 字节）

| YAML | 说明 |
|------|------|
| `max_prompt_bytes` | 总上下文字节上限（与 `max_context_tokens` 二选一优先） |
| `max_context_tokens` | 未设 `max_prompt_bytes` 时：字节上限 ≈ **token×2**（默认 token **110000**） |
| `history_max_bytes` | 历史消息文本上限；**0** 表示按比例自动 |
| `system_extra_max_bytes` | 系统里「文件记忆」后缀 |
| `agent_md_max_bytes` | agent 注入段 |
| `skill_index_max_bytes` | Skills 索引列表 |
| `inherited_messages` | 子 agent 继承父消息条数 |
| `recall_max_bytes` | recall 注入与总上限比例取小 |
| `min_transcript_messages` | 裁剪历史时至少保留条数，默认 **6** |

`features.disable_context_budget`：关闭预算收紧。

### 会话与多通道（`cmd/oneclaw`）

主进程 **`oneclaw`（非 `-init` / `-maintain-once`）** 使用 **`session.SessionResolver`**：按 **入站 `Channel` + `Peer.ID`（逻辑 session_key）** 懒创建 **`session.Engine`**，**同一 handle 内串行**处理回合，避免多线程共用一个 `Engine` 的 data race。

| 概念 | 说明 |
|------|------|
| **session_key** | 来自 clawbridge `bus.InboundMessage` 的线程/话题键（`session.InboundSessionKey`）；决定「哪一条会话」 |
| **Engine.SessionID** | 由 `Source` + `session_key` **稳定派生**的十六进制 id（`session.StableSessionID`），用于审计 jsonl、`dialog_history` 分文件等 |
| **转写文件** | `<cwd>/.oneclaw/sessions/<SessionID>/transcript.json` 与 `working_transcript.json`（与 YAML `paths.transcript` 无关，避免多线程混写同一文件） |
| **SQLite** | 默认 `<cwd>/.oneclaw/sessions.sqlite`（可用 `sessions.sqlite_path` 覆盖）：会话行 + **memory recall 去重状态**（`RecallState`）；**规则 / episodic** 等仍以**文件**为主；**notify 三路审计**与 transcript 同会话时在 `sessions/<id>/audit/` |
| **dialog_history** | 按日落盘到 `<cwd>/.oneclaw/memory/YYYY-MM-DD/<SessionID>/dialog_history.json`，不同会话互不追加到同一文件 |

**`sessions.disable_sqlite: true`**：不打开数据库，仅依赖上述文件布局（适合测试或禁止落库的环境）。

**`sessions.worker_count`**：主进程用于处理入站回合的 **固定 worker 数**（默认 **8**，配置为 **0** 或未写时与 `<1` 同样走默认）。每个 session 按稳定哈希落到其中一个 worker，**同一 session 内消息在该 worker 上串行**；每条消息 **临时 `NewEngine`、落盘后丢弃**，避免无限增长的内存 map。worker 数不随会话数量增加。

### LLM 用量（`<cwd>/.oneclaw/usage/`）

每次成功的 chat completion（含工具多步）在 `ToolContext.CWD` 非空且返回非零 token 时落盘。

| YAML（`usage.*`） | 说明 |
|-------------------|------|
| （无） | 写入由 `features.disable_usage_ledger` 控制 |
| `default_input_per_mtok` / `default_output_per_mtok` | 与费用**估算**联用时的默认美元/百万 token |
| `features.disable_usage_ledger` | 关闭写入 |
| `features.usage_estimate_cost` | 响应无费用字段时用内置价目表估算 `cost_usd` |

**路径**：`interactions.jsonl`、`daily/YYYY-MM-DD.json`、`users/<16-hex>.json`（详见实现与历史说明）。

### 维护（`maintain.*`）

- **回合后**（`MaybePostTurnMaintain`）：`maintain.post_turn.*`（如 `min_log_bytes`、`memory_preview_bytes`、`timeout_seconds`、`max_tokens` 等）。
- **定时 / 远场**（`RunScheduledMaintain`、`oneclaw -maintain-once`）：`maintain.model` / `maintain.scheduled_model`、`maintain.max_tokens`、`maintain.log_days`、`maintain.min_log_bytes`、`maintain.max_log_bytes`、`maintain.scheduled_timeout_seconds`、`maintain.scheduled_max_steps`、`maintain.incremental_overlap`、`maintain.incremental_max_span` 等。
- **`opts.Interval > 0`**（`maintainloop`、`cmd/maintain -interval`）：daily log **增量**模式（行内时间戳 + `<cwd>/.oneclaw/scheduled_maintain_state.json`）。
- **`Interval == 0` 或 `-once`**：按日历天 `log_days` 窗口做体量探测；远场为多步、只读工具，需 `opts.ToolRegistry`（如 `builtin.ScheduledMaintainReadRegistry()`）。

`features.disable_auto_maintenance`：关闭回合后维护。`features.disable_scheduled_maintenance`：关闭后台定时循环（见上）。

详见 [memory-maintain-dual-entry-design.md](memory-maintain-dual-entry-design.md)、[embedded-maintain-scheduler-design.md](embedded-maintain-scheduler-design.md)。

## 示例

新项目默认文件树：[`config/init_template/`](../config/init_template/)（**`oneclaw -init`** 嵌入并复制到 `<项目>/.oneclaw/`；亦可手动将其中 `config.yaml` 复制为 `~/.oneclaw/config.yaml` 或 `<项目>/.oneclaw/config.yaml`）。

## 与第三方 autoload 的关系

若项目仍 `import _ "github.com/lengzhao/conf/autoload"` 加载 `.env`，**不会**被 oneclaw 当作配置源；有效配置仍以合并后的 YAML 与 `PushRuntime` 结果为准。
