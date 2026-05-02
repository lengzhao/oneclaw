# 入站与路由（实现）

与 [outbound-events-design.md](outbound-events-design.md) 配套。本文描述 **当前 oneclaw 与 clawbridge** 的入站形状、ToolContext 合并规则，以及会话编排入口。

---

## 1. 目标与主路径

- **入站**：各渠道（clawbridge driver、`statichttp` 等）将用户侧输入统一为 **`github.com/lengzhao/clawbridge/bus.InboundMessage`**，经 **`cmd/oneclaw`** 进入 **`session.WorkerPool.SubmitUser` → `session.Engine.SubmitUser` → `loop.RunTurn`**。
- **出站**：助手可见文本等由 **`session.Engine`** 经 **`Engine.publishOutbound`** / **`Engine.updateInboundStatus`**（**`Engine.Bridge`** 的 **`Bus().PublishOutbound`** 与 **`Bridge.UpdateStatus`**；**`Bridge` 为 nil 时** `ErrNotInitialized` 在合适路径上静默或向上返回）；与 **`tools.Registry`** 正交。
- **ToolContext**：每轮在 `loop.RunTurn` 开头合并入站元数据到 **`toolctx.SessionHost.TurnInbound`**，供工具与策略使用（§2.1）。

---

## 2. 入站字段（概念表）

实现真源为 **`bus.InboundMessage`**。下表为阅读文档时的**概念对照**（与 §2.1 合并语义一致）。

| 概念 | 说明 |
|------|------|
| 渠道 / 实例 | `ClientID` 等，区分配置与连接 |
| 正文 | `Content`，由会话编排进入模型 |
| 用户 / 会话 | `Sender`、`Peer`、`SessionID` 等 |
| 会话键 | 由 `session.InboundSessionKey` 等从消息派生，用于 `SessionHandle` |
| 附件 / 媒体 | `MediaPaths` 等；引擎可格式化为独立 user 消息或 `read_file` 路径提示 |
| 元数据 | `Metadata`；可选写入 `<inbound-context>`（**不含**不进模型的敏感 id 时按实现裁剪） |

---

### 2.1 ToolContext 上的入站元数据合并

- **`loop.RunTurn`** 对 `cfg.ToolContext` 调用 **`toolctx.Context.ApplyTurnInboundToToolContext(in)`**，内部以 **`mergeTurnInbound(&TurnInbound, in)`**（`toolctx/context.go`）写入 **`toolctx.SessionHost.TurnInbound`**（类型 **`bus.InboundMessage`**）。
- **非空字段覆盖**：`ClientID`、`SessionID`、`MessageID`、`Sender`、`Peer`、`ReceivedAt`、`Metadata` 等按实现逐字段合并；**正文 `Content` 不参与合并**。
- **附件路径**：若本轮 `in.MediaPaths` 为空，则将 `TurnInbound.MediaPaths` **置空**，避免嵌套 `RunTurn` 继承父轮附件路径。

---

## 3. 未实现项（设计草案）

以下能力**尚未在代码中作为主路径实现**（可选演进）：`context` 透传入站元数据；`SinkRegistry` / `SinkFactory` 按渠道解析出站；`OutboundSender` 与 `Engine.SendMessage` 收窄等。取舍见 [`architecture-modularity-simplification.md`](architecture-modularity-simplification.md)。

---

## 4. 出站（当前实现）

- **`Engine.Bridge`**：**`MainEngineFactoryDeps.Bridge`** 注入 **`cmd/oneclaw`** 中 **`clawbridge.New`** 得到的 **`*clawbridge.Bridge`**（唯一出站/状态来源；**不再**依赖进程级 **`SetDefault`**）。出站与入站消息状态经 **`Engine.publishOutbound`** / **`Engine.updateInboundStatus`**，底层为 **`Bridge.Bus().PublishOutbound`** 与 **`Bridge.UpdateStatus`**。
- **`loop.Config.OutboundText`**：在 `prepareSharedTurn` 中绑定为：将助手文本封装为 **`bus.OutboundMessage`** 再 **`Engine.publishOutbound`**；**`ErrNotInitialized`** 在闭包内吞掉，避免无桥时污染模型步日志（见 `session/turn_prepare.go`）。
- **入站状态**：**`Bridge.UpdateStatus(ctx, &in, state, metadata)`**（**`UpdateStatusState`**，见 clawbridge v0.3+），由 **`Engine.updateInboundStatus`** 调用；**`UpdateStatusRequest`** 仍由 **`Bridge.UpdateStatusRequest`** 用于非入站同源路由；能力不支持时由 **`session`** 忽略约定错误。

---

## 5. 注册表与工具（`tools.Registry`）

- **`tools/builtin.DefaultRegistry()`**：主会话 **`Chat` 工具**（`read_file`、`run_agent` 等），由 **`cmd/oneclaw`** 注入 **`session.Engine`** / **`loop.Config`**；进程内通常**共用同一 `*tools.Registry` 实例**。

---

## 6. 与出站文档的关系

- **事件与载荷**：观测与出站草案见 [outbound-events-design.md](outbound-events-design.md)。
- **主路径**：助手回复以 **`bus.OutboundMessage`** 经 **`Outbound.PublishOutbound`** 发出，与 `Record`/`seq` 类 JSON 观测草案可并存（见出站文档）。

---

## 7. 实现细节（入口编排）

- **`session.Engine`** 在记忆块之后注入 **`<inbound-context>`**（**不含** `correlation_id`）；附件为独立 user 消息；**仅附件无正文**时用占位句；内置斜杠 **`/help` / `/model` / `/session` / `/status` / `/paths` / `/reset` / `/stop`** 等由引擎本地应答（**不调用模型**）。
- **`statichttp` POST `/api/chat`**：`application/json` 与 **`multipart/form-data`** 均可。multipart 字段 **`text`**、**`locale`**、**`files`**（或 **`file`**，可重复）；单文件原始上限 **4MB**，整表上限 **32MB**；上传文件写入 **`<cwd>/media/inbound/<UTC-YYYY-MM-DD>/`**；落盘后的相对路径进入 user 消息与 `MediaPaths`；模型侧只给 **`read_file` 路径说明**，不内联文件字节。JSON 内联 `attachments[].text` 由 `session` 在同目录落盘后再走同一套路径提示。
- **clawbridge 入站**：与上述编排共用 **`SubmitUser` → `RunTurn`** 链。

---

## 8. 会话与 TurnHub

- **`cmd/oneclaw`** 使用 **`session.NewTurnHub`** + **`session.MainEngineFactory`**，并按 **`sessions.turn_policy`**（serial / insert / preempt）编排。**每个 `SessionHandle` 一个 coordinator**：同会话入站在该 mailbox 上串行（或通过 policy 插入/抢占）；**每条入站任务** **`factory(handle)` 新建 `*Engine`，`SubmitUser` 结束后丢弃**，状态依赖转写等落盘（实现见 `session/turn_hub.go`）。**`session.WorkerPool`** 仍保留在仓库中供测试或自定义宿主复用，主进程路径不再使用。
- **单测 / e2e**：可直接 **`session.NewEngine(...)`** 调用 **`SubmitUser`**，不经过 **`TurnHub`**。

---

## 9. 子 agent 工具集

实现以 **`subagent/run.go`**、**`subagent/registry.go`** 为准：

- **`run_agent`**：**`FilterRegistry(parent, def.Tools)`**（名单为空则克隆父表全部工具）→ **`WithoutMetaTools`** 去掉 **`run_agent` / `fork_context`**。
- **`fork_context`**：在适用深度上 **`stripMetaForNested`**，**不按 catalog 名单过滤**。

归纳：**`run_agent` 最终工具集 ≈（catalog 允许名单 ∩ 父注册表）− 元工具**；**`fork_context` ≈ 父工具集 − 元工具**（在适用深度上）。详见 **`go doc subagent`**。
