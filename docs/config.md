# 统一配置（config 包）

开发与生产共用同一套加载规则：`github.com/lengzhao/oneclaw/config`。

## 配置文件路径与合并顺序

从低到高优先级（后者覆盖前者）：

1. **用户级**：`~/.oneclaw/config.yaml`（`config.Load` 的 `Home` 为 `os.UserHomeDir()`）
2. **显式文件**：`oneclaw -config <path>`；**相对路径**相对于 `<Home>/.oneclaw/`（例如 `extra.yaml` 即 `~/.oneclaw/extra.yaml`）

**不再**读取「当前进程工作目录」或「项目 `<cwd>/.oneclaw/config.yaml`」；运行时数据根见 `Resolved.UserDataRoot()` 与 [session-home-isolation-design.md](session-home-isolation-design.md)。

缺失的用户配置文件会被忽略；若 `-config` 指向的路径不存在，启动报错。

**初始化**：`oneclaw -init` 在 **`~/.oneclaw/`**（`InitWorkspace(home, home)`）下写入模板，行为与此前「在指定目录下创建 `.oneclaw`」相同，只是目录固定为用户主目录下的 `.oneclaw`。

## 敏感项（API Key）

在合并后的 YAML 中设置 `openai.api_key`，由 `openai.NewClient(config.OpenAIOptions()...)` 注入客户端。**oneclaw 不再从进程环境读取 `OPENAI_API_KEY` / `ONCLAW_*` 等做运行时配置**；未配置 key 时进程会报错并提示在 YAML 中填写。

## 运行时扁平化（`PushRuntime` / `rtopts`）

`config.Load` 合并 YAML 后，入口应调用 `(*Resolved).PushRuntime()`：将预算、`paths`、`features.disable_*`、`chat.transport`、`schedule`、`usage`、`sidechain_merge`、`semantic_compact`、`skills`、`agent.completion_extra` 等写入包 `rtopts` 的进程内快照。`budget`、`loop` 等与 `config` 低耦合的包通过 `rtopts.Current()` 读取，避免循环依赖。

单测可 `rtopts.Set(nil)` 或 `rtopts.Set(&customSnapshot)` 覆盖；合并逻辑仍以 YAML 为准。

## CLI 与日志

| 标志 | 说明 |
|------|------|
| `-config` | 额外 YAML 层（见上合并顺序；相对路径相对 `~/.oneclaw/`） |
| `-log-level` | `debug` / `info` / `warn` / `error`，非空时覆盖配置里的 `log.level` |
| `-log-format` | `text` / `json`，非空时覆盖 `log.format` |
| `-log-file` | 追加日志到该文件（UTF-8），**同时仍输出 stderr**；非空时覆盖配置里的 `log.file`；相对路径在加载配置后相对 `UserDataRoot()` |
| `-init` | 初始化 `~/.oneclaw`；无 `config.yaml` 则写入模板，已有则合并补全缺失键（不覆盖） |
| `-export-session` | 从用户数据根（`~/.oneclaw`）导出快照到指定目录 |

## YAML 字段速查（无环境变量覆盖）

| 区域 | 主要 YAML 路径 | 说明 |
|------|----------------|------|
| 模型 | `model` | 默认聊天模型；空则代码内默认 |
| 主会话循环 | `agent.max_steps`、`agent.max_tokens` | `max_steps`：每用户回合内模型调用步数（默认 **100**，范围 1–256）。`max_tokens`：每步 **`max_completion_tokens`**（默认 **32768**，范围 1024–131072；YAML 写 0 或不写则用默认）。`cmd/oneclaw` 经 `MainEngineFactory` 写入 `Engine.MaxTokens`。单次 chat completion 的 context 超时由 `model` 包默认 **2 分钟**（`model.Complete` / `CompleteWithTransport`），非 YAML 配置项。对 **context 超时**、**HTTP 5xx / 429 / 408**、**网络读超时** 等瞬时失败，同一 completion 会自动重试最多 **2 次**（共 **3** 次尝试），间隔约 **400ms / 800ms**（上限 2s），仍非 YAML 配置项。 |
| Chat Completions 额外参数 | `agent.completion_extra` | 任意与 **OpenAI Chat Completions** 请求体 JSON 对齐的嵌套键（对应 `openai.ChatCompletionNewParams`）。在 `PushRuntime` 时序列化为 JSON 注入 `rtopts`，每步模型调用先 **`json.Unmarshal` 到参数结构体**，再由运行时**强制覆盖** `model`、`messages`、`max_completion_tokens`、`stream_options`，以及有工具时的 `tools` / `parallel_tool_calls`。用于 `temperature`、`reasoning_effort`、`web_search_options`、服务商扩展字段等；**网关/模型不支持的键会导致 API 报错**。多层 YAML 合并时对嵌套 map **递归合并**。 |
| 传输 | `chat.transport` | `auto`（先流式、失败再非流式）、`non_stream`、`stream`；兼容网关仅支持非流式时建议 `non_stream` |
| OpenAI 兼容 | `openai.api_key`、`openai.base_url`、`openai.org_id`、`openai.project_id` | `base_url` 需含 `/v1/` 后缀（若网关要求） |
| 路径 | `paths.memory_base`、`paths.transcript`、`paths.working_transcript`、`paths.working_transcript_max_messages` | 相对路径相对 **`UserDataRoot()`**（默认 `~/.oneclaw`）；IM 下每会话转写见下节「会话」。`working_transcript_max_messages` 仍适用。单 `Engine` 时：每轮成功 `RunTurn` 后折叠 **内存 `Messages`** 为**用户可见**；`working_transcript` 与内存同形；`working_transcript_max_messages` 截尾部可见条数，`0` 默认 **30**，负数不限制 |
| 会话 | `sessions.worker_count`、`sessions.isolate_workspace` | 见下 **「会话与多通道（`cmd/oneclaw`）」** |
| 预算 | `budget.*` | 见下表 |
| 开关 | `features.disable_*` | `true` 为关闭；省略或 `false` 为开启 |
| 入站多模态 | `features.disable_multimodal_image`、`features.disable_multimodal_audio` | 默认 **不** 禁用：图片注入 Chat Completions `image_url`（data URL），wav/mp3 注入 `input_audio`；任一为 `true` 时对应类型仅保留 read_file 路径提示，不送多模态载荷 |
| 日志 | `log.level`、`log.format`、`log.file` | `log.file`：可选，追加落盘（与 stderr 双写）；相对路径相对 `UserDataRoot()`；可被 `-log-file` 覆盖 |
| 侧链 | `sidechain_merge` | 留空关闭；`1` / `true` / `tool` / `append` / `user` 等见历史设计文档 |
| 用量 | `usage.*` | 见下节 |
| 调度睡眠 | `schedule.min_sleep`、`schedule.idle_sleep` | Agent 定时任务调度用 Go duration 字符串 |
| 语义 compact | `semantic_compact.summary_max_bytes` | |
| Skills | `skills.recent_path` | 可选覆盖 skills 最近列表路径 |
| MCP | `mcp.enabled`、`mcp.servers.<name>.*` | 显式 `mcp.enabled: true` 后连接外部 MCP；`servers` 下每项 `enabled`、`command`+`args`（stdio）或 `url`+`type`（`sse`/`http`），可选 `env`、`env_file`、`headers`；工具以 `mcp_*` 前缀注册 |

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
| `min_transcript_messages` | 裁剪历史时至少保留条数，默认 **6** |

`features.disable_context_budget`：关闭预算收紧。

### 会话与多通道（`cmd/oneclaw`）

主进程 **`oneclaw`（非 `-init` / `-export-session`）** 使用 **`session.WorkerPool`**：入站由 **`session.SessionHandle{Source: ClientID, SessionKey: InboundSessionKey(m)}`** 标识（`SessionKey` 优先 `SessionID`，否则 `Peer.ID` 等，见 `session` 包）。按 **handle 哈希**落到固定 worker，**同一 handle 内串行**处理回合；**每条消息**经 **`MainEngineFactory` 新建 `session.Engine`**，回合结束后丢弃，避免多 goroutine 共用一个 `Engine` 的 data race 与无界内存增长。详见 [`inbound-routing-design.md`](inbound-routing-design.md) §8。

| 概念 | 说明 |
|------|------|
| **session_key** | 来自 clawbridge `bus.InboundMessage` 的线程/话题键（`session.InboundSessionKey`）；决定「哪一条会话」 |
| **Engine.SessionID** | 由 `ClientID`（`SessionHandle.Source`）+ `session_key` **稳定派生**的十六进制 id（`session.StableSessionID`），用于转写路径分文件等 |
| **Engine.CWD（IM）** | 由 **`sessions.isolate_workspace`** 控制（默认 **false**）：**false** 时 CWD = `<UserDataRoot>/workspace`（多会话共享同一 `workspace/`）；**true** 时 CWD = `<UserDataRoot>/sessions/<SessionID>/workspace`（每会话独立 `workspace/`）。用户数据根目录树内**不再**出现嵌套的 `.oneclaw` 子目录名（见 [`user-root-workspace-layout.md`](user-root-workspace-layout.md)） |
| **转写文件（IM）** | 始终 `<UserDataRoot>/sessions/<SessionID>/transcript.json` 与 `working_transcript.json`（与 YAML `paths.transcript` 无关） |
| **定时任务文件（IM）** | `<UserDataRoot>/scheduled_jobs.json`（与项目树下 `.oneclaw/scheduled_jobs.json` 二选一：工具通过 `HostDataRoot` 写用户根） |
| **dialog_history** | 成功回合后由 `workspace.AppendDialogHistoryPair` 写入；路径由 `workspace.Layout`（`workspace.LayoutForIMWorkspace` / `IMHostMaintainLayout` 等）解析，典型为 `<InstructionRoot>/memory/<日期>/<session_id>/dialog_history.json`（见 `workspace/dialog_history.go`） |

**`sessions.worker_count`**：主进程用于处理入站回合的 **固定 worker 数**（默认 **8**，配置为 **0** 或未写时与 `<1` 同样走默认）。每个 session 按稳定哈希落到其中一个 worker，**同一 session 内消息在该 worker 上串行**；每条消息 **临时 `NewEngine`、落盘后丢弃**，避免无限增长的内存 map。worker 数不随会话数量增加。

### LLM 用量（`usage/` 目录）

每次成功的 chat completion（含工具多步）在 `ToolContext.CWD` 非空且返回非零 token 时落盘。路径为 **`<InstructionRoot>/usage/`**（IM 主进程在存在 `InstructionRoot` 时写入该目录；与 `Engine.CWD` 的 `workspace/` 分离）。

| YAML（`usage.*`） | 说明 |
|-------------------|------|
| （无） | 写入由 `features.disable_usage_ledger` 控制 |
| `default_input_per_mtok` / `default_output_per_mtok` | 与费用**估算**联用时的默认美元/百万 token |
| `features.disable_usage_ledger` | 关闭写入 |
| `features.usage_estimate_cost` | 响应无费用字段时用内置价目表估算 `cost_usd` |

**路径**：`interactions.jsonl`、`daily/YYYY-MM-DD.json`、`users/<16-hex>.json`（详见实现与历史说明）。

> **说明**：`maintain.*`、`memory.recall.*`、进程内 **`maintainloop`**、**`-maintain-once`** 等在 **`config.File` / `cmd/oneclaw` 当前实现中不存在**；历史设计仍保留于 [memory-maintain-dual-entry-design.md](memory-maintain-dual-entry-design.md)、[embedded-maintain-scheduler-design.md](embedded-maintain-scheduler-design.md)、[memory-recall-sqlite-design.md](memory-recall-sqlite-design.md) 等文档，**勿与现网 YAML 混用为真值**。

## 示例

初始化模板文件树：[`config/init_template/`](../config/init_template/)（**`oneclaw -init`** 嵌入并复制到 `~/.oneclaw/`；亦可手动将其中 `config.yaml` 复制为 `~/.oneclaw/config.yaml`）。

## 与第三方 autoload 的关系

若项目仍 `import _ "github.com/lengzhao/conf/autoload"` 加载 `.env`，**不会**被 oneclaw 当作配置源；有效配置仍以合并后的 YAML 与 `PushRuntime` 结果为准。
