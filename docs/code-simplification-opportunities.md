# 代码简化与边界（oneclaw）

对架构的只读审视：**已落实项**仅作摘要，避免与 [`todo.md`](todo.md) backlog **#19–#22** 重复维护。**仍待文档化或可选演进**见 §2–§4；实施前结合 [inbound-routing-design.md](inbound-routing-design.md)、[outbound-events-design.md](outbound-events-design.md) 与代码再定优先级。

---

## 1. 已落实（摘要）

| 主题 | 现状 |
|------|------|
| `SubmitUser` / `submitLocalSlashTurn` 共享准备 | `session/turn_prepare.go`：`prepareSharedTurn` |
| 本地 slash 与维护 | 旁路，不跑 `PostTurn` / 维护（[memory-maintain-dual-entry-design.md](memory-maintain-dual-entry-design.md) §2.4） |
| `subagent` 前置去重 | `validateNestedHost` / `validateNestedParent` / `stripMetaForNested`（`subagent/run.go`） |
| `toolctx` | `SessionHost` 嵌入 + `ApplyTurnInboundToToolContext`（`mergeTurnInbound`，见 `toolctx/context.go`） |
| `channel.DrainTextReply` | `statichttp` 等出站聚合 |
| `PostTurnInput` | `SubmitUser` 尾部单次构建，供 `PostTurn` 与 `MaybePostTurnMaintain` 共用 |
| Emitter `context` | `Text` 用回合 ctx；`Done` 用 `context.Background()`（见 `engine.go` / `loop/runner.go` 注释） |
| `NestedMemoryPaths` | 已从 `Context` 移除 |

---

## 2. 仍建议补充的文档（对应 todo #23–#26）

- **工具 `DefaultRegistry` vs 出站**：见 [inbound-routing-design.md](inbound-routing-design.md) §5（工具侧 **`tools/builtin.DefaultRegistry()`**，与 **`PublishOutbound`** 正交）。
- **`SinkRegistry` vs `SinkFactory`**：见 [inbound-routing-design.md](inbound-routing-design.md) §4（可选演进）。
- **`WorkerPool` vs 直接 `NewEngine`**：`cmd/oneclaw` 用 WorkerPool 分片、**每任务新建 `Engine`**；单测 / e2e 常直接 **`session.NewEngine`**。见 [config.md](config.md)「会话与多通道」、[inbound-routing-design.md](inbound-routing-design.md) §8。
- **子 agent 工具表**：**`run_agent`** = `FilterRegistry` ∩ 父表 + 去 meta；**`fork_context`** = 父表去 meta（见 [inbound-routing-design.md](inbound-routing-design.md) §9、`subagent/registry.go`）。

---

## 3. `WorkerPool` 与出站（§5.2 保留）

**`cmd/oneclaw/main.go`** 使用 **`session.WorkerPool`**：固定 worker 数、按 `SessionHandle` 哈希分片、每任务 **`MainEngineFactory` 新建 `Engine` 后丢弃**，持久化依赖落盘。出站由 **`clawbridge.SetDefault`** 后的包级 **`PublishOutbound` / `UpdateStatus`** 承担（`Engine` 不再持有出站回调字段）。详见 [config.md](config.md)、[inbound-routing-design.md](inbound-routing-design.md) §4、§8。

---

## 4. 可选：`OutboundSender` 与全局 `Engine`（todo #27）

- `Engine.SendMessage` 依赖 CWD、`SinkRegistry`/`SinkFactory`、`SessionID`，不宜做成无状态包级接口。
- 较稳妥：**收窄接口 + 已完成的 `toolctx` 分组**，或 **`context` 挂载窄 `OutboundSender`**，与 `toolctx.SessionHost.SendMessage` 二选一演进，避免与「全局 `Engine` 单例」并行两套。

详见 [inbound-routing-design.md](inbound-routing-design.md) §3。

---

## 5. 包边界（保持现状）

- `memory` 不 import `tools/builtin`；由 `builtin.ScheduledMaintainReadRegistry()` 等做依赖倒置，避免随意打通造成循环。
- `tools/registry.go` 与 `loop` 批处理策略已集中，改动优先级低。

---

*剩余勾选以 [`todo.md`](todo.md) **#23–#27** 为准。*
