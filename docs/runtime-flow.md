# oneclaw 运行时流程（整理版）

本文描述 **当前实现** 中从进程启动到单轮对话、记忆维护、定时任务与出站的**主路径**，便于对照代码阅读。细节配置见 [`config.md`](config.md)；维护双入口见 [`memory-maintain-dual-entry-design.md`](memory-maintain-dual-entry-design.md)；入站/出站抽象见 [`inbound-routing-design.md`](inbound-routing-design.md)、[`outbound-events-design.md`](outbound-events-design.md)。

---

## 1. 进程入口：`cmd/oneclaw`

解析 **`$HOME`**、**`-config`**（可选）后，按标志分为多条互斥路径（**`-init` / `-export-session` / `-maintain-once` / 常驻**）：

| 路径 | 条件 | 行为摘要 |
|------|------|----------|
| **初始化** | `-init` | `config.InitWorkspace`：补全 `.oneclaw` 与 `config.yaml`，退出 |
| **单次远场维护** | `-maintain-once` | 加载配置 → `memory.RunScheduledMaintain`（只读工具 registry），退出；不启动 IM |
| **常驻服务** | 默认 | 加载配置 → MCP（可选）→ **WorkerPool** → **maintainloop**（可选）→ **clawbridge** → 消费入站 |

常驻模式要求配置中至少有一个启用的 `clawbridge.clients`，否则进程报错退出。

```mermaid
flowchart TB
  start([main 启动]) --> home[home + UserDataRoot]
  home --> init{-init?}
  init -->|是| ws[InitWorkspace] --> exit1([退出])
  init -->|否| load[Load 合并 YAML + PushRuntime]
  load --> mo{-maintain-once?}
  mo -->|是| sm[RunScheduledMaintain] --> exit2([退出])
  mo -->|否| api{有 API key?}
  api -->|否| err([退出: 缺密钥])
  api -->|是| pool[NewWorkerPool + MainEngineFactory]
  pool --> mloop[maintainloop.Start 可选]
  mloop --> cb[clawbridge.New / Start]
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
  wp[session.WorkerPool]
  fac[MainEngineFactory]
  eng[Engine per job]
  loop[loop.RunTurn]
  mem[memory: PostTurn / Maintain]
  drivers <--> bus
  bus -->|ConsumeInbound| wp
  wp --> fac
  fac --> eng
  eng --> loop
  loop --> eng
  eng --> mem
  eng -->|PublishOutbound| bus
```

- **WorkerPool**：按 `hash(session_key) % N` 分片，**同一会话固定落在同一 worker**，每轮任务 **新建 Engine**（`factory`），执行完 `SubmitUser` 后丢弃，避免无界 Engine 映射。
- **SessionHandle**：由入站的 `ClientID`（clawbridge client id）+ 会话键（`InboundSessionKey`：优先 `SessionID`，否则 `Peer.ID`）派生；**StableSessionID**（SHA256 截断）用于 sqlite、目录名等。**Engine.CWD** 为 `<UserDataRoot>/sessions/<StableSessionID>/`（见 `config.UserDataRoot()` 与 [session-home-isolation-design.md](session-home-isolation-design.md)）。

---

## 3. 单轮用户回合：`SubmitUser`（概要）

以下为主路径（非本地 slash）；实现见 `session/engine.go`。

```mermaid
flowchart TB
  WP[WorkerPool: factory → SubmitUser] --> N1[prepareInbound / notify inbound]
  N1 --> N2[prepareSharedTurn：布局、bundle、system、budget、tctx]
  N2 --> N3[写 user 行到 transcript]
  N3 --> RT[loop.RunTurn]
  RT --> OK{成功?}
  OK -->|否| ERR[返回 err]
  OK -->|是| N4[ToUserVisible + SaveTranscript / WorkingTranscript]
  N4 --> PT[memory.PostTurn]
  N4 --> MT[go MaybePostTurnMaintain]
  N4 --> PR[persistRecall 可选 sessdb]
```

**要点**：

- **prepareSharedTurn**：注入 `MEMORY.md` / recall、预算、`ToolContext` 与入站路由字段合并（见入站设计文档 §2.1）。
- **loop.RunTurn**：模型 ↔ 工具循环；工具轨迹可在 `ToolTraceSink` 中收集，供 `PostTurnInput` 与 notify 使用。
- **PostTurn**：同步写回合相关记忆管线（如 daily log）；**MaybePostTurnMaintain** 在独立 goroutine 中执行，**不阻塞**渠道返回。
- **本地 slash**（如 `/help`、`/status`、`/paths`、`/recall reset`、`/reset`、`/stop`；CLI 的 `/exit` 由终端处理）：走 `submitLocalSlashTurn`，**不**调用 `loop.RunTurn`，**不**走 `PostTurn` / `MaybePostTurnMaintain`（刻意设计）。`/stop` 在入站 goroutine 内还会先调用 `WorkerPool.CancelInflightTurn` 取消**当前已在执行**的该会话轮次（`context.WithCancel(root)`）。内置列表见 `session/slash_local.go` 与 `/help`。

---

## 4. `loop.RunTurn` 内部（概念）

```mermaid
flowchart TB
  A[合并 system / memory / user / inbound 元数据] --> B[Chat Completions 请求]
  B --> C{停止原因}
  C -->|tool_calls| D[执行 Registry 工具]
  D --> B
  C -->|stop / length| E[最终 assistant 文本]
  E --> F[OutboundText / SlimTranscript / Lifecycle notify]
```

具体步数上限、流式传输、Abort 等由 `loop.Config` 与 `Engine` 字段决定。

---

## 5. Memory 维护：两条 LLM 入口 + 多种触发方式

| 类型 | 函数 | 典型触发 |
|------|------|----------|
| **近场（回合后）** | `MaybePostTurnMaintain` → `RunPostTurnMaintain` | 每轮 `SubmitUser` 成功后异步 goroutine |
| **远场（定时/批量）** | `RunScheduledMaintain` | `maintainloop` 周期、`oneclaw -maintain-once`、`cmd/maintain` |

两者通过 **`maintainPipelineMu` 串行化写盘**，避免与远场维护争抢同一批 memory 文件。开关见 `features.disable_auto_maintenance`、`features.disable_scheduled_maintenance` 及 `maintain.interval` 等（[`config.md`](config.md)）。

```mermaid
flowchart TB
  subgraph post [回合后]
    SU[SubmitUser 成功] --> P[PostTurn 同步]
    P --> M[MaybePostTurnMaintain goroutine]
  end
  subgraph sched [定时远场]
    ML[maintainloop 定时器] --> R[RunScheduledMaintain]
    CLI[oneclaw -maintain-once] --> R
    CM[cmd/maintain] --> R
  end
  M --> MU[maintainPipelineMu]
  R --> MU
  MU --> disk[(.oneclaw/memory 等)]
```

---

## 6. Agent 定时任务（`cron` 工具 / `scheduled_jobs.json`）

- 任务持久化在 **`UserDataRoot` 下的 `scheduled_jobs.json`**（默认 **`~/.oneclaw/scheduled_jobs.json`**；见 `schedule.JobsFilePath`）。
- 每个启用的 clawbridge **client** 可启动 **`schedule.StartHostPollerIfEnabled`**：轮询到期任务，构造 **合成入站** `bus.InboundMessage`，调用与人工消息相同的 **`workerPool.SubmitUser`**，从而走完整模型回合。

```mermaid
flowchart TB
  JSON[scheduled_jobs.json] --> poller[host poller per client]
  poller --> syn[合成 InboundMessage]
  syn --> WP[WorkerPool.SubmitUser]
```

`features.disable_scheduled_tasks` 为关总开关。

---

## 7. 出站消息

- **模型回合内**的可见回复通过 `loop` 内配置的 **`OutboundText`**（及 clawbridge 适配）写入 bus。
- **不经模型**的主动推送由工具 **`send_message`** 或 **`Engine.SendMessage`** 调用 **`PublishOutbound`**，最终 **`bridge.Bus().PublishOutbound`** 分发到对应渠道。

`main` 中在创建 `EngineFactory` 时使用闭包延迟绑定 `publishOutbound`，以便在 `clawbridge.New` 之后挂上真实 `Publish`。

---

## 8. 可选横切能力

| 能力 | 作用 |
|------|------|
| **MCP** | `mcpclient.RegisterIfEnabled` 向共享 Registry 注册工具，系统提示可选 `MCPSystemNote` |
| **sessdb** | 配置 SQLite 路径时，`RecallBridge` 在 factory 中注入，跨重启恢复 `RecallState` |
| **审计类 NotifySink** | `RegisterAuditSinks`：LLM 步 / 编排 / 用户可见等 JSONL（见 [`notify-sinks-audit-design.md`](notify-sinks-audit-design.md)） |
| **Notify 生命周期** | `notify` 事件：入站、回合起止、工具结束等（见 [`notification-hooks-design.md`](notification-hooks-design.md)） |

---

## 9. 与仓库其它文档的关系

- **配置合并与运行时推送**：[`config.md`](config.md)  
- **阶段任务与验收**：[`todo.md`](todo.md)、[`agent-runtime-golang-plan.md`](agent-runtime-golang-plan.md) §9  
- **范式与边界总览**：[`agent-runtime-golang-plan.md`](agent-runtime-golang-plan.md)  
- **Prompt 拼装**：[`prompts/README.md`](prompts/README.md)  

README 中的简化图仍可作一页纸总览；**以本文 + 上述设计文档为准**做实现级对照时更贴近当前代码路径。
