# oneclaw 运行时流程（整理版）

本文描述 **当前实现** 中从进程启动到单轮对话、转写与 dialog 落盘、定时任务与出站的**主路径**，便于对照代码阅读。细节配置见 [`config.md`](config.md)；入站/出站抽象见 [`inbound-routing-design.md`](inbound-routing-design.md)、[`outbound-events-design.md`](outbound-events-design.md)。

---

## 1. 进程入口：`cmd/oneclaw`

解析 **`$HOME`**、**`-config`**（可选）后，按标志分为多条互斥路径（**`-init` / `-export-session` / 常驻**）：

| 路径 | 条件 | 行为摘要 |
|------|------|----------|
| **初始化** | `-init` | `config.InitWorkspace`：补全 `.oneclaw` 与 `config.yaml`，退出 |
| **导出快照** | `-export-session <dir>` | `workspace.ExportSessionSnapshot`：复制用户数据根到指定目录，退出（无需 API key） |
| **常驻服务** | 默认 | 加载配置 → MCP（可选）→ **TurnHub** → **clawbridge** → 每客户端可选 **schedule poller** → 消费入站 |

常驻模式要求配置中至少有一个启用的 `clawbridge.clients`，否则进程报错退出。

```mermaid
flowchart TB
  start([main 启动]) --> home[home + UserDataRoot]
  home --> init{-init?}
  init -->|是| ws[InitWorkspace] --> exit1([退出])
  init -->|否| exp{-export-session?}
  exp -->|是| snap[ExportSessionSnapshot] --> exit05([退出])
  exp -->|否| load[Load 合并 YAML + PushRuntime]
  load --> api{有 API key?}
  api -->|否| err([退出: 缺密钥])
  api -->|是| hub[NewTurnHub + MainEngineFactory]
  hub --> cb[clawbridge.New / Start]
  cb --> poll[每客户端 schedule poller 可选]
  poll --> loop[goroutine: ConsumeInbound → SubmitUser]
  loop --> sig[等待 SIGINT/SIGTERM]
  sig --> shutdown[bridge.Stop / drain]
```

---

## 2. 常驻模式：组件关系

```mermaid
flowchart TB
  subgraph im [IM / 渠道]
    drivers[clawbridge drivers]
  end
  bus[bus: Inbound / Outbound]
  th[session.TurnHub]
  fac[MainEngineFactory]
  eng[Engine per job]
  tr[einoTurnRunner]
  adk[Eino ADK]
  disk[transcript + dialog_history 落盘]
  drivers <--> bus
  bus -->|ConsumeInbound| th
  th --> fac
  fac --> eng
  eng --> tr
  tr --> adk
  adk --> eng
  eng --> disk
  eng -->|publishOutbound| bus
```

- **TurnHub**：按 **`SessionHandle`** 维护 **每会话一个 coordinator**（mailbox），**同一会话内入站串行**（或由 **`sessions.turn_policy`** 决定 insert/preempt）；每轮任务 **`factory(handle)` 新建 Engine**，执行完 `SubmitUser` 后丢弃，避免无界 Engine 映射。
- **TurnRunner**：固定为 **`einoTurnRunner`**（`MainEngineFactory` / `NewEngine`）；须配置 **`openai.api_key`**（映射到 **`Engine.EinoOpenAIAPIKey`**），否则 **`SubmitUser` 报错**。见 **§3.1**。
- **SessionHandle**：由入站的 `ClientID`（clawbridge client id）+ 会话键（`InboundSessionKey`：优先 `SessionID`，否则 `Peer.ID`）派生；**StableSessionID**（SHA256 截断）用于目录名、转写路径等。**Engine.CWD** 为 **`<UserDataRoot>/workspace`**（默认）或 **`<UserDataRoot>/sessions/<StableSessionID>/workspace`**（`sessions.isolate_workspace: true`），见 `session.MainEngineFactory` 与 [session-home-isolation-design.md](session-home-isolation-design.md)。

---

## 3. 单轮用户回合：`SubmitUser`（概要）

以下为主路径（非本地 slash）；实现见 `session/engine.go`。

```mermaid
flowchart TB
  WP[TurnHub: factory → SubmitUser] --> N1[prepareInbound / notify inbound]
  N1 --> N2[prepareSharedTurn：布局、bundle、system、budget、tctx]
  N2 --> N3[写 user 行到 transcript]
  N3 --> RT[TurnRunner.RunTurn]
  RT --> OK{成功?}
  OK -->|否| ERR[返回 err]
  OK -->|是| N4[ToUserVisible + SaveTranscript / WorkingTranscript]
  N4 --> DH[appendDialogHistoryIfComplete]
```

**要点**：

- **prepareSharedTurn**：注入 `MEMORY.md`、预算、`ToolContext` 与入站路由字段合并（见入站设计文档 §2.1）。
- **TurnRunner.RunTurn**（`session.TurnRunner`）：**Eino ADK** + `tools.Registry` 的 Eino 绑定；**无 API key 则失败**（不再调用 **`loop.RunTurn`**）。细粒度模型步 / 工具事件见会话 **`execution/*.jsonl`**（`session/exec_journal.go`），与精简版 `notify` 分工。
- **成功后**：折叠可见消息、**`SaveTranscript` / `SaveWorkingTranscript`**，并 **`appendDialogHistoryIfComplete`**（`workspace.AppendDialogHistoryPair`）。
- **本地 slash**（如 `/help`、`/status`、`/paths`、`/reset`、`/stop`；CLI 的 `/exit` 由终端处理）：走 `submitLocalSlashTurn`，**不**调用模型回合（刻意设计）。`/stop` 在入站路径上还会先调用 **`TurnHub.CancelInflightTurn`** 取消**当前已在执行**的该会话轮次（`context.WithCancel(root)`）。内置列表见 `session/slash_local.go` 与 `/help`。

### 3.1 不再分支「双运行时」

- **内核**：始终 **`einoTurnRunner`**（`agent.runtime` 等历史 YAML 键不再解析进 `config.File`，写入 YAML 时会被忽略）。
- **密钥**：必须配置 **`openai.api_key`**（以及按需 **`base_url`**），经 **`MainEngineFactory`** / **`Engine`** 写入 **`EinoOpenAIAPIKey` / `EinoOpenAIBaseURL`**；**缺密钥则 `SubmitUser` 失败**。单测须设置与 **`openai.Client`** 一致的 stub 密钥与 URL。
- **嵌套子代理**（`run_agent` / `fork_context`）：**`subagent.Host.RunTurn`** 与主线程一致；嵌套 **`loop.Config`** 必须带 **`EinoOpenAI*`**，否则与父轮次同样报错。
- **`session.NewEngine`**：默认 **`einoTurnRunner`**，须传入**非 nil** **`tools.Registry`**。

---

## 4. `loop.RunTurn` 内部（概念）：独立测试与循环语义

```mermaid
flowchart TB
  A[合并 system / memory / user / inbound 元数据] --> B[Chat Completions 请求]
  B --> C{停止原因}
  C -->|tool_calls| D[执行 Registry 工具]
  D --> B
  C -->|stop / length| E[最终 assistant 文本]
  E --> F[OutboundText / SlimTranscript / Lifecycle notify]
```

主会话 **`SubmitUser`** 走 **Eino ADK**，不沿用本节逐步展开图；本节目的是说明 **`loop` 包**内 Chat Completions ↔ 工具循环（仍可由 **`loop.RunTurn`** 直接调用，部分 E2E 使用）。

具体步数上限、流式传输、Abort 等由 `loop.Config` 与 `Engine` 字段决定；ADK 侧由 `MaxIterations` / `TurnMaxSteps` 等映射。

---

## 5. 转写与 dialog 落盘

- 回合成功后写 **转写**（`sessions/<id>/transcript.json` 等）与 **`workspace` 包下的 `dialog_history.json`**（见 `workspace/dialog_history.go`）。

---

## 6. Agent 定时任务（`cron` 工具 / `scheduled_jobs.json`）

- 任务持久化在 **`UserDataRoot` 下的 `scheduled_jobs.json`**（默认 **`~/.oneclaw/scheduled_jobs.json`**；见 `schedule.JobsFilePath`）。
- 每个启用的 clawbridge **client** 可启动 **`schedule.StartHostPollerIfEnabled`**：轮询到期任务，构造 **合成入站** `bus.InboundMessage`，调用与人工消息相同的 **`TurnHub.Submit`**（经 `submitInbound`），从而走完整模型回合。

```mermaid
flowchart TB
  JSON[scheduled_jobs.json] --> poller[host poller per client]
  poller --> syn[合成 InboundMessage]
  syn --> WP[TurnHub.Submit]
```

`features.disable_scheduled_tasks` 为关总开关。

---

## 7. 出站消息

- **模型回合内**的可见回复通过 `loop` 内配置的 **`OutboundText`**（`Engine.publishOutbound` → **`Bridge.Bus().PublishOutbound`**）写入 bus。
- **不经模型**的主动推送由工具 **`send_message`** 或 **`Engine.SendMessage`** 经 **`Engine.publishOutbound`**，最终 **`bridge.Bus().PublishOutbound`** 分发到对应渠道。

`cmd/oneclaw` 在构造 **`MainEngineFactoryDeps`** 时注入 **`clawbridge.New`** 的 **`Bridge`** 指针（见 `session/bridge.go`）。

---

## 8. 可选横切能力

| 能力 | 作用 |
|------|------|
| **MCP** | `mcpclient.RegisterIfEnabled` 向共享 Registry 注册工具，系统提示可选 `MCPSystemNote` |
| **Notify 生命周期** | `notify` 事件：入站、回合起止、工具结束等（见 [`notification-hooks-design.md`](notification-hooks-design.md)） |

---

## 9. 与仓库其它文档的关系

- **配置合并与运行时推送**：[`config.md`](config.md)  
- **范式与包职责摘要**：[`agent-runtime-golang-plan.md`](agent-runtime-golang-plan.md)  
- **Prompt 拼装**：[`prompts/README.md`](prompts/README.md)  

README 中的简化图仍可作一页纸总览；**以本文 + 上述设计文档为准**做实现级对照时更贴近当前代码路径。
