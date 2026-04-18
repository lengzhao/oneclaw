# 代码简化与边界（oneclaw）

对架构的只读审视：**已落实项**仅作摘要，避免与 [`todo.md`](todo.md) backlog **#19–#22** 重复维护。**仍待文档化或可选演进**见 §2–§4；实施前结合 [inbound-routing-design.md](inbound-routing-design.md)、[outbound-events-design.md](outbound-events-design.md) 与代码再定优先级。

---

## 1. 已落实（摘要）

| 主题 | 现状 |
|------|------|
| `SubmitUser` / `submitLocalSlashTurn` 共享准备 | `session/turn_prepare.go`：`prepareSharedTurn` |
| 本地 slash 与维护 | 旁路，不跑 `PostTurn` / 维护（[memory-maintain-dual-entry-design.md](memory-maintain-dual-entry-design.md) §2.4） |
| `subagent` 前置去重 | `validateNestedHost` / `validateNestedParent` / `stripMetaForNested`（`subagent/run.go`） |
| `toolctx` | `SessionHost` 嵌入 + `ApplyTurnInboundToToolContext`（内联 `MergeNonEmptyRouting`） |
| `channel.DrainTextReply` | `statichttp` 等出站聚合 |
| `PostTurnInput` | `SubmitUser` 尾部单次构建，供 `PostTurn` 与 `MaybePostTurnMaintain` 共用 |
| Emitter `context` | `Text` 用回合 ctx；`Done` 用 `context.Background()`（见 `engine.go` / `loop/runner.go` 注释） |
| `NestedMemoryPaths` | 已从 `Context` 移除 |

---

## 2. 仍建议补充的文档（对应 todo #23–#26）

- **`routing.DefaultRegistry`**：进程级单例；测试与多实例时注意隐式全局（[inbound-routing-design.md](inbound-routing-design.md)）。
- **`SinkRegistry` vs `SinkFactory`**：默认主路径 vs 高级 per-turn 绑定（`config.md` 或 inbound 文交叉一句）。
- **单 `Engine` vs `SessionResolver` vs `WorkerPool`**：`cmd/oneclaw` 用 WorkerPool 分片、每任务新建 `Engine`；`SessionResolver` 用于测试等懒复用场景（[config.md](config.md)「会话与多通道」、下文 §3）。
- **子 agent 工具表**：**最终工具集 = catalog ∩ 过滤 − meta**（[claude-code-subagent-system.md](claude-code-subagent-system.md) 或 `subagent` 包注释）。

---

## 3. `SessionResolver` 与出站（§5.2 保留）

`session/resolver.go` 按 `SessionHandle` 懒创建并**复用** `Engine`（测试等）。**`cmd/oneclaw/main.go`** 使用 **`session.WorkerPool`**：固定 worker 数、按 session 哈希分片、每任务 **新建 `Engine` 后丢弃**，持久化依赖落盘。详见 [config.md](config.md)。

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
